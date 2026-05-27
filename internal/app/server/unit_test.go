package server

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"testing"
	"time"

	featsearch "idx/internal/features/search"
	idxipc "idx/internal/shared/ipc"
	sharedjsonrpc "idx/internal/shared/jsonrpc"
)

// writeFailConn wraps a net.Conn so that all Write calls return an error.
type writeFailConn struct{ net.Conn }

func (w *writeFailConn) Write(_ []byte) (int, error) {
	return 0, fmt.Errorf("write deliberately failed")
}

// --- captureWriter ---

func TestCaptureWriterWriteLineAndJoined(t *testing.T) {
	w := &captureWriter{}
	if err := w.WriteLine("line one"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := w.WriteLine("line two"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "line one\nline two"
	if w.joined() != want {
		t.Errorf("joined: want %q, got %q", want, w.joined())
	}
}

func TestCaptureWriterJoinedEmpty(t *testing.T) {
	w := &captureWriter{}
	if w.joined() != "" {
		t.Errorf("expected empty string for empty writer, got %q", w.joined())
	}
}

func TestCaptureWriterFirstLineEmpty(t *testing.T) {
	w := &captureWriter{}
	if w.firstLine() != "" {
		t.Errorf("expected empty firstLine on empty writer, got %q", w.firstLine())
	}
}

func TestCaptureWriterFirstLineReturnFirstOnly(t *testing.T) {
	w := &captureWriter{}
	_ = w.WriteLine("first")
	_ = w.WriteLine("second")
	if w.firstLine() != "first" {
		t.Errorf("expected %q, got %q", "first", w.firstLine())
	}
}

// --- operatorOrDefault ---

func TestOperatorOrDefaultWithEmptyString(t *testing.T) {
	got := operatorOrDefault("")
	if got != featsearch.OperatorAND {
		t.Errorf("expected %q, got %q", featsearch.OperatorAND, got)
	}
}

func TestOperatorOrDefaultWithExplicitOR(t *testing.T) {
	got := operatorOrDefault("OR")
	if got != "OR" {
		t.Errorf("expected %q, got %q", "OR", got)
	}
}

// --- parseSearchJSON ---

func TestParseSearchJSONEmptyLineReturnsEmpty(t *testing.T) {
	resp, err := parseSearchJSON("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Results) != 0 {
		t.Errorf("expected 0 results, got %d", len(resp.Results))
	}
}

func TestParseSearchJSONValidResult(t *testing.T) {
	line := `{"count":1,"results":[{"file":"foo.go","name":"foo.go","path":"/src/foo.go","matches":[{"line":5,"content":"hello","match":true}]}]}`
	resp, err := parseSearchJSON(line)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Count != 1 {
		t.Errorf("expected count 1, got %d", resp.Count)
	}
	if len(resp.Results) != 1 || resp.Results[0].File != "foo.go" {
		t.Errorf("unexpected results: %+v", resp.Results)
	}
	if len(resp.Results[0].Matches) != 1 || !resp.Results[0].Matches[0].Match {
		t.Errorf("unexpected matches: %+v", resp.Results[0].Matches)
	}
}

func TestParseSearchJSONWithScore(t *testing.T) {
	line := `{"count":1,"results":[{"file":"x.go","name":"x.go","path":"/x.go","score":1.5,"matches":[]}]}`
	resp, err := parseSearchJSON(line)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Results[0].Score == nil || *resp.Results[0].Score != 1.5 {
		t.Errorf("expected score 1.5, got %v", resp.Results[0].Score)
	}
}

func TestParseSearchJSONWithStale(t *testing.T) {
	line := `{"count":1,"results":[{"file":"x.go","name":"x.go","path":"/x.go","stale":true,"matches":[]}]}`
	resp, err := parseSearchJSON(line)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.Results[0].Stale {
		t.Errorf("expected stale=true")
	}
}

func TestParseSearchJSONInvalidReturnsError(t *testing.T) {
	_, err := parseSearchJSON("not-valid-json")
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestParseSearchJSONEmptyResults(t *testing.T) {
	line := `{"count":0,"results":[]}`
	resp, err := parseSearchJSON(line)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Count != 0 || len(resp.Results) != 0 {
		t.Errorf("expected empty results, got: %+v", resp)
	}
}

// --- searchOptionsFromRequest ---

func TestSearchOptionsFromRequestMapping(t *testing.T) {
	req := idxipc.SearchRequest{
		Size:             10,
		Operator:         "OR",
		Format:           "json",
		Context:          3,
		FilesOnly:        true,
		AgentCompact:     true,
		Explain:          true,
		From:             5,
		PopularityWeight: 0.5,
		ExtensionQueries: []string{"go"},
		PathQueries:      []string{"internal"},
	}
	opts := searchOptionsFromRequest(req)
	if opts.Size != 10 {
		t.Errorf("Size: want 10, got %d", opts.Size)
	}
	if opts.Operator != "OR" {
		t.Errorf("Operator: want OR, got %q", opts.Operator)
	}
	if opts.Format != "json" {
		t.Errorf("Format: want json, got %q", opts.Format)
	}
	if !opts.FilesOnly || !opts.AgentCompact || !opts.Explain {
		t.Errorf("boolean flags not set: %+v", opts)
	}
	if opts.From != 5 {
		t.Errorf("From: want 5, got %d", opts.From)
	}
}

func TestSearchOptionsFromRequestEmptyOperatorDefaultsToAND(t *testing.T) {
	opts := searchOptionsFromRequest(idxipc.SearchRequest{})
	if opts.Operator != featsearch.OperatorAND {
		t.Errorf("expected AND operator, got %q", opts.Operator)
	}
}

// --- handleConn ---

func testServerWithEcho() *indexServer {
	s := &indexServer{
		deps:       ServerDeps{},
		dispatcher: sharedjsonrpc.NewDispatcher(),
	}
	s.dispatcher.Register("test.echo", func(_ context.Context, params json.RawMessage) (any, error) {
		return string(params), nil
	})
	return s
}

func TestHandleConnDispatchesRequest(t *testing.T) {
	s := testServerWithEcho()
	serverConn, clientConn := net.Pipe()
	defer clientConn.Close()

	id := json.RawMessage(`1`)
	msg := sharedjsonrpc.Message{
		JSONRPC: sharedjsonrpc.Version,
		ID:      &id,
		Method:  "test.echo",
		Params:  json.RawMessage(`"hi"`),
	}

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

	resp, err := sharedjsonrpc.ReadMessage(bufio.NewReader(clientConn))
	clientConn.Close()
	<-done

	if err != nil {
		t.Fatalf("ReadMessage: %v", err)
	}
	if resp.Error != nil {
		t.Errorf("unexpected error: %+v", resp.Error)
	}
}

func TestHandleConnUnknownMethodReturnsError(t *testing.T) {
	s := &indexServer{
		deps:       ServerDeps{},
		dispatcher: sharedjsonrpc.NewDispatcher(),
	}
	serverConn, clientConn := net.Pipe()
	defer clientConn.Close()

	id := json.RawMessage(`1`)
	msg := sharedjsonrpc.Message{
		JSONRPC: sharedjsonrpc.Version,
		ID:      &id,
		Method:  "no.such.method",
	}

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

	resp, err := sharedjsonrpc.ReadMessage(bufio.NewReader(clientConn))
	clientConn.Close()
	<-done

	if err != nil {
		t.Fatalf("ReadMessage: %v", err)
	}
	if resp.Error == nil || resp.Error.Code != sharedjsonrpc.ErrMethodNotFound {
		t.Errorf("expected method-not-found, got: %+v", resp)
	}
}

func TestHandleConnEOFIsIgnored(t *testing.T) {
	s := testServerWithEcho()
	serverConn, clientConn := net.Pipe()
	clientConn.Close() // causes immediate EOF on server side

	done := make(chan struct{})
	go func() {
		defer close(done)
		s.handleConn(context.Background(), serverConn)
	}()
	<-done // must return without panic
}

func TestNewServerRegistersHandlers(t *testing.T) {
	// NewServer must not panic with valid deps structure
	srv := NewServer(ServerDeps{})
	if srv == nil {
		t.Fatal("expected non-nil server")
	}
}

func TestServeListenErrorReturnsError(t *testing.T) {
	srv := NewServer(ServerDeps{SocketPath: "/tmp/nonexistent-dir-abc123xyz/s.sock"})
	err := srv.Serve(context.Background())
	if err == nil {
		t.Error("expected error for socket path in non-existent directory, got nil")
	}
}

func TestHandleConnNonEOFReadErrorIsHandled(t *testing.T) {
	s := testServerWithEcho()
	serverConn, clientConn := net.Pipe()
	defer clientConn.Close()

	done := make(chan struct{})
	go func() {
		defer close(done)
		s.handleConn(context.Background(), serverConn)
	}()

	// Malformed Content-Length triggers a non-EOF parse error inside ReadMessage.
	_, _ = clientConn.Write([]byte("Content-Length: not-a-number\r\n\r\n"))
	clientConn.Close()
	<-done
}

func TestHandleConnNotificationNoIDSkipsWrite(t *testing.T) {
	s := testServerWithEcho()
	serverConn, clientConn := net.Pipe()
	defer clientConn.Close()

	done := make(chan struct{})
	go func() {
		defer close(done)
		s.handleConn(context.Background(), serverConn)
	}()

	// Notification (no ID field) → Dispatcher returns nil → handleConn must not write.
	msg := sharedjsonrpc.Message{
		JSONRPC: sharedjsonrpc.Version,
		Method:  "test.echo",
		Params:  json.RawMessage(`"hello"`),
	}
	var buf bytes.Buffer
	_ = sharedjsonrpc.WriteMessage(&buf, msg)
	_, _ = clientConn.Write(buf.Bytes())
	clientConn.Close()
	<-done
}

func TestHandleConnWriteErrorIsHandledGracefully(t *testing.T) {
	s := testServerWithEcho()
	serverConn, clientConn := net.Pipe()
	defer clientConn.Close()

	// writeFailConn reads from serverConn but makes all Write calls return an error.
	wfc := &writeFailConn{Conn: serverConn}

	done := make(chan struct{})
	go func() {
		defer close(done)
		s.handleConn(context.Background(), wfc)
	}()

	id := json.RawMessage(`1`)
	msg := sharedjsonrpc.Message{
		JSONRPC: sharedjsonrpc.Version,
		ID:      &id,
		Method:  "test.echo",
		Params:  json.RawMessage(`"data"`),
	}
	var buf bytes.Buffer
	_ = sharedjsonrpc.WriteMessage(&buf, msg)
	_, _ = clientConn.Write(buf.Bytes())
	<-done
}

func TestServeAcceptsConnectionAndSpawnsGoroutine(t *testing.T) {
	dir, err := os.MkdirTemp("/tmp", "idx")
	if err != nil {
		t.Fatalf("mkdirtemp: %v", err)
	}
	defer os.RemoveAll(dir)

	sockPath := dir + "/s.sock"
	srv := NewServer(ServerDeps{SocketPath: sockPath})

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- srv.Serve(ctx) }()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if _, statErr := os.Stat(sockPath); statErr == nil {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	// Connect and immediately close — exercises wg.Add + goroutine path in Serve.
	conn, dialErr := net.Dial("unix", sockPath)
	if dialErr == nil {
		conn.Close()
	}
	time.Sleep(20 * time.Millisecond)

	cancel()
	if serveErr := <-done; serveErr != nil {
		t.Errorf("Serve returned unexpected error: %v", serveErr)
	}
}
