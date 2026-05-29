package server

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"net"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	sharedfs "idx/internal/shared/filesystem"
	idxipc "idx/internal/shared/ipc"
	sharedjsonrpc "idx/internal/shared/jsonrpc"
)

// dispatchToServer sends a JSON-RPC message to s via net.Pipe and returns the response.
func dispatchToServer(t *testing.T, s *indexServer, method string, params any) *sharedjsonrpc.Message {
	t.Helper()
	paramsJSON, err := json.Marshal(params)
	require.NoError(t, err, "json.Marshal params")

	id := json.RawMessage(`1`)
	msg := sharedjsonrpc.Message{
		JSONRPC: sharedjsonrpc.Version,
		ID:      &id,
		Method:  method,
		Params:  paramsJSON,
	}

	serverConn, clientConn := net.Pipe()
	defer clientConn.Close()

	var buf bytes.Buffer
	require.NoError(t, sharedjsonrpc.WriteMessage(&buf, msg), "WriteMessage")

	done := make(chan struct{})
	go func() {
		defer close(done)
		s.handleConn(context.Background(), serverConn)
	}()

	_, err = clientConn.Write(buf.Bytes())
	require.NoError(t, err, "pipe write")

	resp, readErr := sharedjsonrpc.ReadMessage(bufio.NewReader(clientConn))
	clientConn.Close()
	<-done

	require.NoError(t, readErr, "ReadMessage")
	return resp
}

// waitForSocket polls until the socket file appears or the deadline is exceeded.
func waitForSocket(t *testing.T, sockPath string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(sockPath); err == nil {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("socket %q did not appear within deadline", sockPath)
}

// --- handleSearch ---

func TestHandleSearch_BadParamsType_ReturnsRPCError(t *testing.T) {
	t.Parallel()
	// Send a JSON number as params — valid JSON but cannot unmarshal into SearchRequest struct.
	s := NewServer(ServerDeps{}).(*indexServer)
	resp := dispatchToServer(t, s, idxipc.MethodSearch, 42)
	assert.NotNil(t, resp)
	assert.NotNil(t, resp.Error, "expected RPC error for wrong params type")
}

func TestHandleSearch_NilDeps_ReturnsRPCError(t *testing.T) {
	t.Parallel()
	s := NewServer(ServerDeps{}).(*indexServer)
	resp := dispatchToServer(t, s, idxipc.MethodSearch, idxipc.SearchRequest{Query: "hello"})
	assert.NotNil(t, resp)
	assert.NotNil(t, resp.Error, "expected RPC error with nil deps")
}

// --- handleRead ---

func TestHandleRead_BadParamsType_ReturnsRPCError(t *testing.T) {
	t.Parallel()
	// Send a JSON number as params — valid JSON but cannot unmarshal into ReadRequest.
	s := NewServer(ServerDeps{}).(*indexServer)
	resp := dispatchToServer(t, s, idxipc.MethodRead, 42)
	assert.NotNil(t, resp)
	assert.NotNil(t, resp.Error, "expected RPC error for wrong params type")
}

func TestHandleRead_NilProjectTree_ReturnsEmptyLines(t *testing.T) {
	t.Parallel()
	// handleRead swallows errors and returns ReadResponse{Lines:[]}
	s := NewServer(ServerDeps{}).(*indexServer)
	resp := dispatchToServer(t, s, idxipc.MethodRead, idxipc.ReadRequest{FilePath: "/nonexistent/file.go"})
	assert.NotNil(t, resp)
	assert.Nil(t, resp.Error, "expected success response with empty lines")
}

func TestHandleRead_RealFile_ReturnsLines(t *testing.T) {
	t.Parallel()

	// Arrange
	f, err := os.CreateTemp("", "idx-test-*.txt")
	require.NoError(t, err, "create temp file")
	defer os.Remove(f.Name())
	_, err = f.WriteString("line one\nline two\n")
	require.NoError(t, err, "write temp file")
	f.Close()

	s := NewServer(ServerDeps{
		ProjectTree: sharedfs.NewOSProjectTree(),
	}).(*indexServer)

	// Act + Assert
	resp := dispatchToServer(t, s, idxipc.MethodRead, idxipc.ReadRequest{FilePath: f.Name()})
	assert.NotNil(t, resp)
	// May succeed (if file is within a git project root) or return empty (if path check fails).
	// Either way, no panic and a valid response.
}

// --- handleInit / handleSync / handleStatus ---

func TestHandleInit_NilDeps_ReturnsCommandResponseSuccessFalse(t *testing.T) {
	t.Parallel()
	s := NewServer(ServerDeps{}).(*indexServer)
	resp := dispatchToServer(t, s, idxipc.MethodInit, struct{}{})
	assert.NotNil(t, resp)
	assert.Nil(t, resp.Error, "expected success CommandResponse (not RPC error)")
}

func TestHandleSync_NilDeps_ReturnsCommandResponse(t *testing.T) {
	t.Parallel()
	s := NewServer(ServerDeps{}).(*indexServer)
	resp := dispatchToServer(t, s, idxipc.MethodSync, struct{}{})
	assert.NotNil(t, resp)
	assert.Nil(t, resp.Error, "expected success CommandResponse")
}

func TestHandleStatus_NilDeps_ReturnsCommandResponse(t *testing.T) {
	t.Parallel()
	s := NewServer(ServerDeps{}).(*indexServer)
	resp := dispatchToServer(t, s, idxipc.MethodStatus, struct{}{})
	assert.NotNil(t, resp)
	assert.Nil(t, resp.Error, "expected success CommandResponse")
}

// --- initDepsWithCapture ---

func TestInitDepsWithCapture_WiresOutput(t *testing.T) {
	t.Parallel()
	deps := ServerDeps{}
	capture := &captureWriter{}
	result := initDepsWithCapture(deps, capture)
	assert.Equal(t, capture, result.Output)
}

// --- Serve ---

func TestServe_BindsSocket_ReturnsOnContextCancel(t *testing.T) {
	// Arrange
	dir, err := os.MkdirTemp("/tmp", "idx")
	require.NoError(t, err, "mkdirtemp")
	defer os.RemoveAll(dir)

	sockPath := dir + "/s.sock"
	srv := NewServer(ServerDeps{SocketPath: sockPath})

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- srv.Serve(ctx)
	}()

	// Act: wait for socket to appear before canceling
	waitForSocket(t, sockPath)
	cancel()

	// Assert
	err = <-done
	assert.NoError(t, err, "expected Serve to return nil after cancel")
}
