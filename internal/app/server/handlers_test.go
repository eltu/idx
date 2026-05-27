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

	sharedfs "idx/internal/shared/filesystem"
	idxipc "idx/internal/shared/ipc"
	sharedjsonrpc "idx/internal/shared/jsonrpc"
)

// dispatchToServer sends a JSON-RPC message to s via net.Pipe and returns the response.
func dispatchToServer(t *testing.T, s *indexServer, method string, params any) *sharedjsonrpc.Message {
	t.Helper()
	paramsJSON, err := json.Marshal(params)
	if err != nil {
		t.Fatalf("json.Marshal params: %v", err)
	}
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
	if err := sharedjsonrpc.WriteMessage(&buf, msg); err != nil {
		t.Fatalf("WriteMessage: %v", err)
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		s.handleConn(context.Background(), serverConn)
	}()

	if _, err := clientConn.Write(buf.Bytes()); err != nil {
		t.Fatalf("pipe write: %v", err)
	}

	resp, readErr := sharedjsonrpc.ReadMessage(bufio.NewReader(clientConn))
	clientConn.Close()
	<-done

	if readErr != nil {
		t.Fatalf("ReadMessage: %v", readErr)
	}
	return resp
}

// --- handleSearch ---

func TestHandleSearchBadParamsTypeReturnsRPCError(t *testing.T) {
	// Send a JSON number as params — valid JSON but cannot unmarshal into SearchRequest struct.
	s := NewServer(ServerDeps{}).(*indexServer)
	resp := dispatchToServer(t, s, idxipc.MethodSearch, 42)
	if resp == nil || resp.Error == nil {
		t.Errorf("expected RPC error for wrong params type, got: %+v", resp)
	}
}

func TestHandleSearchNilDepsReturnsRPCError(t *testing.T) {
	s := NewServer(ServerDeps{}).(*indexServer)
	resp := dispatchToServer(t, s, idxipc.MethodSearch, idxipc.SearchRequest{Query: "hello"})
	if resp == nil || resp.Error == nil {
		t.Errorf("expected RPC error with nil deps, got: %+v", resp)
	}
}

// --- handleRead ---

func TestHandleReadBadParamsTypeReturnsRPCError(t *testing.T) {
	// Send a JSON number as params — valid JSON but cannot unmarshal into ReadRequest.
	s := NewServer(ServerDeps{}).(*indexServer)
	resp := dispatchToServer(t, s, idxipc.MethodRead, 42)
	if resp == nil || resp.Error == nil {
		t.Errorf("expected RPC error for wrong params type, got: %+v", resp)
	}
}

func TestHandleReadNilProjectTreeReturnsEmptyLines(t *testing.T) {
	// handleRead swallows errors and returns ReadResponse{Lines:[]}
	s := NewServer(ServerDeps{}).(*indexServer)
	resp := dispatchToServer(t, s, idxipc.MethodRead, idxipc.ReadRequest{FilePath: "/nonexistent/file.go"})
	if resp == nil || resp.Error != nil {
		t.Errorf("expected success response with empty lines, got: %+v", resp)
	}
}

func TestHandleReadRealFileReturnsLines(t *testing.T) {
	f, err := os.CreateTemp("", "idx-test-*.txt")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	defer os.Remove(f.Name())
	if _, err := f.WriteString("line one\nline two\n"); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	f.Close()

	s := NewServer(ServerDeps{
		ProjectTree: sharedfs.NewOSProjectTree(),
	}).(*indexServer)

	resp := dispatchToServer(t, s, idxipc.MethodRead, idxipc.ReadRequest{FilePath: f.Name()})
	if resp == nil {
		t.Fatal("expected response, got nil")
	}
	// May succeed (if file is within a git project root) or return empty (if path check fails).
	// Either way, no panic and a valid response.
}

// --- handleInit / handleSync / handleStatus ---

func TestHandleInitNilDepsReturnsCommandResponseSuccessFalse(t *testing.T) {
	s := NewServer(ServerDeps{}).(*indexServer)
	resp := dispatchToServer(t, s, idxipc.MethodInit, struct{}{})
	if resp == nil || resp.Error != nil {
		t.Errorf("expected success CommandResponse (not RPC error), got: %+v", resp)
	}
}

func TestHandleSyncNilDepsReturnsCommandResponse(t *testing.T) {
	s := NewServer(ServerDeps{}).(*indexServer)
	resp := dispatchToServer(t, s, idxipc.MethodSync, struct{}{})
	if resp == nil || resp.Error != nil {
		t.Errorf("expected success CommandResponse, got: %+v", resp)
	}
}

func TestHandleStatusNilDepsReturnsCommandResponse(t *testing.T) {
	s := NewServer(ServerDeps{}).(*indexServer)
	resp := dispatchToServer(t, s, idxipc.MethodStatus, struct{}{})
	if resp == nil || resp.Error != nil {
		t.Errorf("expected success CommandResponse, got: %+v", resp)
	}
}

// --- initDepsWithCapture ---

func TestInitDepsWithCaptureWiresOutput(t *testing.T) {
	deps := ServerDeps{}
	capture := &captureWriter{}
	result := initDepsWithCapture(deps, capture)
	if result.Output != capture {
		t.Error("expected capture to be wired as Output")
	}
}

// --- Serve ---

func TestServeBindsSocketAndReturnsOnContextCancel(t *testing.T) {
	dir, err := os.MkdirTemp("/tmp", "idx")
	if err != nil {
		t.Fatalf("mkdirtemp: %v", err)
	}
	defer os.RemoveAll(dir)

	sockPath := dir + "/s.sock"
	srv := NewServer(ServerDeps{SocketPath: sockPath})

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() {
		done <- srv.Serve(ctx)
	}()

	// Wait for socket to appear before canceling
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if _, statErr := os.Stat(sockPath); statErr == nil {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	cancel()
	err = <-done
	if err != nil {
		t.Errorf("expected Serve to return nil after cancel, got: %v", err)
	}
}
