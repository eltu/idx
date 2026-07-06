package server

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"sync"
	"time"

	"go.uber.org/zap"

	idxipc "idx/internal/shared/ipc"
	sharedjsonrpc "idx/internal/shared/jsonrpc"
)

const socketDialTimeout = 300 * time.Millisecond

type indexServer struct {
	deps       ServerDeps
	dispatcher *sharedjsonrpc.Dispatcher
	listener   net.Listener
	stopWatch  func()
}

// SetWatchStopper wires the callback that stops the embedded file-watch loop.
// handleDestroy calls it before removing index files, so the watcher cannot
// react to a .idx deletion by resyncing (and thereby recreating) the very
// directory destroy just removed. The callback is expected to block until the
// watch loop has fully exited.
func (s *indexServer) SetWatchStopper(stop func()) {
	s.stopWatch = stop
}

// NewServer creates a ServerRunner that serves JSON-RPC requests on a Unix socket.
// Example: srv := NewServer(deps); err := srv.Serve(ctx).
func NewServer(deps ServerDeps) ServerRunner {
	s := &indexServer{deps: deps, dispatcher: sharedjsonrpc.NewDispatcher()}
	s.registerHandlers()
	return s
}

func (s *indexServer) registerHandlers() {
	s.dispatcher.Register(idxipc.MethodSearch, s.handleSearch)
	s.dispatcher.Register(idxipc.MethodInit, s.handleInit)
	s.dispatcher.Register(idxipc.MethodSync, s.handleSync)
	s.dispatcher.Register(idxipc.MethodStatus, s.handleStatus)
	s.dispatcher.Register(idxipc.MethodRead, s.handleRead)
	s.dispatcher.Register(idxipc.MethodInspect, s.handleInspect)
	s.dispatcher.Register(idxipc.MethodDestroy, s.handleDestroy)
	s.dispatcher.Register(idxipc.MethodConfig, s.handleConfig)
	s.dispatcher.Register(idxipc.MethodRelated, s.handleRelated)
}

// Serve binds the Unix socket and serves JSON-RPC requests until ctx is canceled.
func (s *indexServer) Serve(ctx context.Context) error {
	if err := s.prepareSocket(); err != nil {
		return err
	}

	lc := net.ListenConfig{}
	listener, err := lc.Listen(ctx, "unix", s.deps.SocketPath)
	if err != nil {
		return err
	}
	s.listener = listener
	defer func() {
		_ = listener.Close()
		_ = os.Remove(s.deps.SocketPath)
		zap.L().Info("idx server stopped", zap.String("socket", s.deps.SocketPath))
	}()

	zap.L().Info("idx server listening", zap.String("socket", s.deps.SocketPath))

	var wg sync.WaitGroup
	go func() {
		<-ctx.Done()
		_ = listener.Close()
	}()

	for {
		conn, err := listener.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) || ctx.Err() != nil {
				break
			}
			zap.L().Warn("accept error", zap.Error(err))
			continue
		}
		wg.Add(1)
		go func(c net.Conn) {
			defer wg.Done()
			s.handleConn(ctx, c)
		}(conn)
	}

	wg.Wait()
	return nil
}

// shutdownAfterDestroy closes the listener so Serve's accept loop exits and the
// daemon process terminates on its own. Destroy deletes the .idx directory that
// holds server.sock and server.state, so an external client can no longer detect
// or signal this process afterward — the server must stop itself instead.
// Closing the listener does not affect the in-flight connection that is still
// writing the destroy response, since that connection was already accepted.
func (s *indexServer) shutdownAfterDestroy() {
	if s.listener != nil {
		_ = s.listener.Close()
	}
}

// prepareSocket removes a stale socket file if present and no process is listening,
// or returns an error if another server is already bound to the path.
func (s *indexServer) prepareSocket() error {
	d := &net.Dialer{Timeout: socketDialTimeout}
	conn, err := d.DialContext(context.Background(), "unix", s.deps.SocketPath)
	if err == nil {
		_ = conn.Close()
		return fmt.Errorf("agent is already running on this project (socket %q is in use)", s.deps.SocketPath)
	}
	// Remove stale socket file so net.Listen can bind cleanly.
	_ = os.Remove(s.deps.SocketPath)
	return nil
}

func (s *indexServer) handleConn(ctx context.Context, conn net.Conn) {
	defer func() { _ = conn.Close() }()

	r := bufio.NewReader(conn)
	msg, err := sharedjsonrpc.ReadMessage(r)
	if err != nil {
		if !errors.Is(err, io.EOF) {
			zap.L().Warn("read error", zap.Error(err))
		}
		return
	}

	resp := s.dispatcher.Dispatch(ctx, msg)
	if resp == nil {
		return
	}

	if err := sharedjsonrpc.WriteMessage(conn, *resp); err != nil {
		zap.L().Warn("write error", zap.Error(err))
	}
}
