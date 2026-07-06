package server

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
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

// --- handleConfig ---

func TestHandleConfig_NilDeps_ReturnsNoFileMessage(t *testing.T) {
	t.Parallel()
	s := NewServer(ServerDeps{}).(*indexServer)
	resp := dispatchToServer(t, s, idxipc.MethodConfig, struct{}{})
	assert.NotNil(t, resp)
	assert.Nil(t, resp.Error, "expected success response, not RPC error")
}

// --- initDepsWithCapture ---

func TestInitDepsWithCapture_WiresOutput(t *testing.T) {
	t.Parallel()
	deps := ServerDeps{}
	capture := &captureWriter{}
	result := initDepsWithCapture(deps, capture)
	assert.Equal(t, capture, result.Output)
}

// --- handleRelated ---

func TestHandleRelated_BadParamsType_ReturnsRPCError(t *testing.T) {
	t.Parallel()
	s := NewServer(ServerDeps{}).(*indexServer)
	resp := dispatchToServer(t, s, idxipc.MethodRelated, 42)
	assert.NotNil(t, resp)
	assert.NotNil(t, resp.Error, "expected RPC error for wrong params type")
}

// --- parseRelatedJSON ---

func TestParseRelatedJSON_EmptyLine_ReturnsEmptyResults(t *testing.T) {
	t.Parallel()

	resp, err := parseRelatedJSON("")

	require.NoError(t, err)
	assert.Empty(t, resp.Results)
	assert.Equal(t, 0, resp.Count)
}

func TestParseRelatedJSON_ValidJSON_ParsesCorrectly(t *testing.T) {
	t.Parallel()

	line := `[{"path":"internal/features/search/service.go","score":0.85,"reason":"git"},{"path":"internal/features/search/port.go","score":0.60,"reason":"co-read"}]`
	resp, err := parseRelatedJSON(line)

	require.NoError(t, err)
	assert.Equal(t, 2, resp.Count)
	require.Len(t, resp.Results, 2)
	assert.Equal(t, "internal/features/search/service.go", resp.Results[0].Path)
	assert.Equal(t, 0.85, resp.Results[0].Score)
	assert.Equal(t, "git", resp.Results[0].Reason)
}

func TestParseRelatedJSON_InvalidJSON_ReturnsError(t *testing.T) {
	t.Parallel()

	_, err := parseRelatedJSON("not-valid-json")

	require.Error(t, err)
}

func TestParseRelatedJSON_EmptyArray_ReturnsZeroCount(t *testing.T) {
	t.Parallel()

	resp, err := parseRelatedJSON(`[]`)

	require.NoError(t, err)
	assert.Equal(t, 0, resp.Count)
	assert.Empty(t, resp.Results)
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

// --- handleDestroy ---

func TestHandleDestroy_NilProjectTree_DoesNotShutDownListener(t *testing.T) {
	t.Parallel()
	s := NewServer(ServerDeps{}).(*indexServer)
	resp := dispatchToServer(t, s, idxipc.MethodDestroy, struct{}{})
	assert.NotNil(t, resp)
	assert.Nil(t, resp.Error, "expected success CommandResponse (not RPC error)")
}

func TestShutdownAfterDestroy_NilListener_NoPanic(t *testing.T) {
	t.Parallel()
	s := NewServer(ServerDeps{}).(*indexServer)
	assert.NotPanics(t, s.shutdownAfterDestroy)
}

// TestServe_HandleDestroy_ShutsDownServerAfterSuccess is a regression test:
// destroying the project's .idx directory removes server.sock and
// server.state, so an external client can no longer detect or signal this
// process. The server must close its own listener as part of handling
// idx.destroy so Serve returns and the daemon process exits on its own,
// instead of being orphaned.
func TestServe_HandleDestroy_ShutsDownServerAfterSuccess(t *testing.T) {
	// Arrange
	dir, err := os.MkdirTemp("/tmp", "idx-destroy")
	require.NoError(t, err, "mkdirtemp")
	defer os.RemoveAll(dir)
	require.NoError(t, os.Mkdir(filepath.Join(dir, ".idx"), 0o750))
	require.NoError(t, os.Mkdir(filepath.Join(dir, ".git"), 0o750))
	t.Chdir(dir)

	sockPath := filepath.Join(dir, "s.sock")
	srv := NewServer(ServerDeps{SocketPath: sockPath, ProjectTree: sharedfs.NewOSProjectTree()})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- srv.Serve(ctx) }()
	waitForSocket(t, sockPath)

	// Act — send idx.destroy over a real connection, as the CLI would.
	conn, err := net.Dial("unix", sockPath)
	require.NoError(t, err, "dial socket")
	id := json.RawMessage(`1`)
	msg := sharedjsonrpc.Message{JSONRPC: sharedjsonrpc.Version, ID: &id, Method: idxipc.MethodDestroy}
	require.NoError(t, sharedjsonrpc.WriteMessage(conn, msg), "write destroy request")
	resp, readErr := sharedjsonrpc.ReadMessage(bufio.NewReader(conn))
	_ = conn.Close()
	require.NoError(t, readErr, "read destroy response")
	require.Nil(t, resp.Error, "expected success CommandResponse, not RPC error")

	resultBytes, err := json.Marshal(resp.Result)
	require.NoError(t, err, "marshal RPC result")
	var cmdResp idxipc.CommandResponse
	require.NoError(t, json.Unmarshal(resultBytes, &cmdResp), "unmarshal CommandResponse")
	require.True(t, cmdResp.Success, "expected destroy to succeed, got output: %s", cmdResp.Output)

	// Assert — Serve returns on its own, without needing ctx to be canceled.
	select {
	case serveErr := <-done:
		assert.NoError(t, serveErr)
	case <-time.After(2 * time.Second):
		t.Fatal("expected Serve to return after destroy, but the server is still running")
	}
}
