package remote

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"strings"
	"testing"

	featsearch "idx/internal/features/search"
	idxipc "idx/internal/shared/ipc"
	sharedjsonrpc "idx/internal/shared/jsonrpc"
)

// fakeOutput collects lines written by the remote adapters.
type fakeOutput struct{ lines []string }

func (w *fakeOutput) WriteLine(text string) error {
	w.lines = append(w.lines, text)
	return nil
}

// fakeJSONRPCServer starts a temporary Unix socket server that responds to one request.
// Uses /tmp to avoid macOS 104-byte Unix socket path limit.
func fakeJSONRPCServer(t *testing.T, respond func(method string, params []byte) any) string {
	t.Helper()
	// Use os.MkdirTemp under /tmp to keep paths short (macOS 104-char AF_UNIX limit).
	dir, err := os.MkdirTemp("/tmp", "idx")
	if err != nil {
		t.Fatalf("mkdirtemp: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	sockPath := fmt.Sprintf("%s/t.sock", dir)

	ln, listenErr := net.Listen("unix", sockPath)
	if listenErr != nil {
		t.Fatalf("listen: %v", listenErr)
	}
	t.Cleanup(func() { ln.Close() })

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go serveOneRequest(conn, respond)
		}
	}()

	return sockPath
}

func serveOneRequest(conn net.Conn, respond func(method string, params []byte) any) {
	defer conn.Close()
	msg, err := sharedjsonrpc.ReadMessage(bufio.NewReader(conn))
	if err != nil {
		return
	}
	result := respond(msg.Method, msg.Params)
	resp := sharedjsonrpc.Message{
		JSONRPC: sharedjsonrpc.Version,
		ID:      msg.ID,
		Result:  result,
	}
	_ = sharedjsonrpc.WriteMessage(conn, resp)
}

// --- SocketClient ---

func TestNewSocketClientStoresPath(t *testing.T) {
	c := NewSocketClient("/tmp/test.sock")
	if c == nil {
		t.Fatal("expected non-nil client")
	}
	if c.socketPath != "/tmp/test.sock" {
		t.Errorf("unexpected socket path: %q", c.socketPath)
	}
}

func TestSocketClientCallServerNotReachableReturnsError(t *testing.T) {
	c := NewSocketClient("/tmp/nonexistent-idx-test.sock")
	var resp any
	err := c.Call("test", struct{}{}, &resp)
	if err == nil {
		t.Fatal("expected error when server not running")
	}
	if !strings.Contains(err.Error(), errNoServerMsg) {
		t.Errorf("expected server-not-running message, got: %q", err.Error())
	}
}

func TestSocketClientCallSuccess(t *testing.T) {
	sock := fakeJSONRPCServer(t, func(_ string, _ []byte) any {
		return idxipc.CommandResponse{Success: true, Output: "done"}
	})

	c := NewSocketClient(sock)
	var resp idxipc.CommandResponse
	if err := c.Call(idxipc.MethodInit, struct{}{}, &resp); err != nil {
		t.Fatalf("Call: %v", err)
	}
	if !resp.Success || resp.Output != "done" {
		t.Errorf("unexpected response: %+v", resp)
	}
}

func TestServerNotRunningErrorContainsHints(t *testing.T) {
	err := serverNotRunningError()
	if err == nil {
		t.Fatal("expected non-nil error")
	}
	msg := err.Error()
	if !strings.Contains(msg, errNoServerStartMsg) {
		t.Errorf("expected start hint in error, got: %q", msg)
	}
}

// --- RemoteIndexCommand ---

func TestRemoteIndexCommandRunSendsInit(t *testing.T) {
	var calledMethod string
	sock := fakeJSONRPCServer(t, func(method string, _ []byte) any {
		calledMethod = method
		return idxipc.CommandResponse{Success: true, Output: "indexed"}
	})

	out := &fakeOutput{}
	cmd := NewRemoteIndexCommand(NewSocketClient(sock), out)
	if err := cmd.Run(); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if calledMethod != idxipc.MethodInit {
		t.Errorf("expected %q, got %q", idxipc.MethodInit, calledMethod)
	}
	if len(out.lines) == 0 || out.lines[0] != "indexed" {
		t.Errorf("expected output %q, got %v", "indexed", out.lines)
	}
}

func TestRemoteIndexCommandSyncSendsSync(t *testing.T) {
	var calledMethod string
	sock := fakeJSONRPCServer(t, func(method string, _ []byte) any {
		calledMethod = method
		return idxipc.CommandResponse{Success: true}
	})

	cmd := NewRemoteIndexCommand(NewSocketClient(sock), &fakeOutput{})
	if err := cmd.Sync(); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if calledMethod != idxipc.MethodSync {
		t.Errorf("expected %q, got %q", idxipc.MethodSync, calledMethod)
	}
}

func TestRemoteIndexCommandStatusSendsStatus(t *testing.T) {
	var calledMethod string
	sock := fakeJSONRPCServer(t, func(method string, _ []byte) any {
		calledMethod = method
		return idxipc.CommandResponse{Success: true}
	})

	cmd := NewRemoteIndexCommand(NewSocketClient(sock), &fakeOutput{})
	if err := cmd.Status(); err != nil {
		t.Fatalf("Status: %v", err)
	}
	if calledMethod != idxipc.MethodStatus {
		t.Errorf("expected %q, got %q", idxipc.MethodStatus, calledMethod)
	}
}

func TestRemoteIndexCommandInspectWritesUnsupportedMessage(t *testing.T) {
	out := &fakeOutput{}
	cmd := NewRemoteIndexCommand(NewSocketClient("/tmp/unused.sock"), out)
	if err := cmd.Inspect(""); err != nil {
		t.Fatalf("Inspect: %v", err)
	}
	if len(out.lines) == 0 {
		t.Error("expected unsupported message, got none")
	}
}

func TestRemoteIndexCommandWatchWritesUnsupportedMessage(t *testing.T) {
	out := &fakeOutput{}
	cmd := NewRemoteIndexCommand(NewSocketClient("/tmp/unused.sock"), out)
	if err := cmd.Watch(false, 0); err != nil {
		t.Fatalf("Watch: %v", err)
	}
	if len(out.lines) == 0 {
		t.Error("expected unsupported message, got none")
	}
}

func TestRemoteIndexCommandSendCommandEmptyOutputIsOK(t *testing.T) {
	sock := fakeJSONRPCServer(t, func(_ string, _ []byte) any {
		return idxipc.CommandResponse{Success: true, Output: ""}
	})

	out := &fakeOutput{}
	cmd := NewRemoteIndexCommand(NewSocketClient(sock), out)
	if err := cmd.Run(); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(out.lines) != 0 {
		t.Errorf("expected no output for empty response, got %v", out.lines)
	}
}

// --- RemoteReader ---

func TestRemoteReaderRunWithOptionsWritesLines(t *testing.T) {
	sock := fakeJSONRPCServer(t, func(_ string, _ []byte) any {
		return idxipc.ReadResponse{Lines: []string{"line1", "line2"}}
	})

	out := &fakeOutput{}
	r := NewRemoteReader(NewSocketClient(sock), out)
	if err := r.RunWithOptions("/some/file.go", 0, 0); err != nil {
		t.Fatalf("RunWithOptions: %v", err)
	}
	if len(out.lines) != 2 || out.lines[0] != "line1" {
		t.Errorf("unexpected output: %v", out.lines)
	}
}

func TestRemoteReaderRunWithOptionsEmptyLinesIsOK(t *testing.T) {
	sock := fakeJSONRPCServer(t, func(_ string, _ []byte) any {
		return idxipc.ReadResponse{Lines: []string{}}
	})

	out := &fakeOutput{}
	r := NewRemoteReader(NewSocketClient(sock), out)
	if err := r.RunWithOptions("/some/file.go", 0, 0); err != nil {
		t.Fatalf("RunWithOptions: %v", err)
	}
	if len(out.lines) != 0 {
		t.Errorf("expected no output for empty lines, got %v", out.lines)
	}
}

// --- searchRequestFromOptions ---

func TestSearchRequestFromOptionsMapsAllFields(t *testing.T) {
	opts := featsearch.Options{
		Size:             5,
		Operator:         "OR",
		Format:           "json",
		Context:          2,
		FilesOnly:        true,
		AgentCompact:     true,
		Explain:          true,
		From:             3,
		PopularityWeight: 0.7,
		ExtensionQueries: []string{"go"},
		PathQueries:      []string{"internal"},
	}
	req := searchRequestFromOptions("hello", opts)
	if req.Query != "hello" {
		t.Errorf("Query: want %q, got %q", "hello", req.Query)
	}
	if req.Size != 5 || req.Operator != "OR" || req.Format != "json" {
		t.Errorf("unexpected req: %+v", req)
	}
	if !req.FilesOnly || !req.AgentCompact || !req.Explain {
		t.Errorf("boolean flags not set: %+v", req)
	}
	if req.From != 3 || req.PopularityWeight != 0.7 {
		t.Errorf("numeric fields off: %+v", req)
	}
}

// --- writeSearchResultsJSON ---

func TestWriteSearchResultsJSONWritesCompact(t *testing.T) {
	resp := idxipc.SearchResponse{Count: 1, Results: []idxipc.SearchResult{{File: "x.go", Name: "x.go", Path: "/x.go"}}}
	out := &fakeOutput{}
	if err := writeSearchResultsJSON(resp, false, out); err != nil {
		t.Fatalf("writeSearchResultsJSON: %v", err)
	}
	if len(out.lines) == 0 {
		t.Fatal("expected output, got none")
	}
	if !strings.Contains(out.lines[0], "x.go") {
		t.Errorf("unexpected output: %q", out.lines[0])
	}
}

func TestWriteSearchResultsJSONWritesPretty(t *testing.T) {
	resp := idxipc.SearchResponse{Count: 0, Results: []idxipc.SearchResult{}}
	out := &fakeOutput{}
	if err := writeSearchResultsJSON(resp, true, out); err != nil {
		t.Fatalf("writeSearchResultsJSON: %v", err)
	}
	if len(out.lines) == 0 {
		t.Fatal("expected output, got none")
	}
}

// --- writeSearchResultsText ---

func TestWriteSearchResultsTextNoResults(t *testing.T) {
	out := &fakeOutput{}
	opts := featsearch.Options{}
	if err := writeSearchResultsText(idxipc.SearchResponse{}, opts, out); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out.lines) == 0 || !strings.Contains(out.lines[0], "No results") {
		t.Errorf("expected no-results message, got %v", out.lines)
	}
}

func TestWriteSearchResultsTextWithResults(t *testing.T) {
	resp := idxipc.SearchResponse{
		Count: 1,
		Results: []idxipc.SearchResult{
			{File: "main.go", Name: "main.go", Path: "/src/main.go", Matches: []idxipc.MatchedLine{
				{Line: 1, Content: "package main", Match: true},
			}},
		},
	}
	out := &fakeOutput{}
	if err := writeSearchResultsText(resp, featsearch.Options{}, out); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out.lines) == 0 {
		t.Fatal("expected output, got none")
	}
}

// --- writeTextResult ---

func TestWriteTextResultFilesOnly(t *testing.T) {
	r := idxipc.SearchResult{Path: "/src/main.go"}
	out := &fakeOutput{}
	opts := featsearch.Options{FilesOnly: true}
	if err := writeTextResult(r, opts, out); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out.lines) != 1 || out.lines[0] != "/src/main.go" {
		t.Errorf("expected path only, got %v", out.lines)
	}
}

func TestWriteTextResultWithMatchesOnly(t *testing.T) {
	r := idxipc.SearchResult{
		Path: "/src/foo.go",
		Matches: []idxipc.MatchedLine{
			{Line: 1, Content: "context line", Match: false},
			{Line: 2, Content: "match line", Match: true},
		},
	}
	out := &fakeOutput{}
	opts := featsearch.Options{MatchesOnly: true}
	if err := writeTextResult(r, opts, out); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// path line + only matched lines
	if len(out.lines) != 2 {
		t.Errorf("expected 2 lines (path + match), got %d: %v", len(out.lines), out.lines)
	}
}

func TestWriteTextResultAllMatches(t *testing.T) {
	r := idxipc.SearchResult{
		Path: "/src/bar.go",
		Matches: []idxipc.MatchedLine{
			{Line: 1, Content: "line one\r\n", Match: true},
			{Line: 2, Content: "line two", Match: false},
		},
	}
	out := &fakeOutput{}
	if err := writeTextResult(r, featsearch.Options{}, out); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// path + 2 match lines
	if len(out.lines) != 3 {
		t.Errorf("expected 3 lines, got %d: %v", len(out.lines), out.lines)
	}
}

// --- RemoteSearcher ---

func TestRemoteSearcherRunWithOptionsJSON(t *testing.T) {
	sock := fakeJSONRPCServer(t, func(_ string, _ []byte) any {
		return idxipc.SearchResponse{Count: 1, Results: []idxipc.SearchResult{{File: "a.go", Name: "a.go", Path: "/a.go", Matches: []idxipc.MatchedLine{}}}}
	})

	out := &fakeOutput{}
	s := NewRemoteSearcher(NewSocketClient(sock), out)
	opts := featsearch.Options{Format: featsearch.OutputJSON}
	if err := s.RunWithOptions("query", opts); err != nil {
		t.Fatalf("RunWithOptions: %v", err)
	}
	if len(out.lines) == 0 {
		t.Error("expected JSON output, got none")
	}
}

func TestRemoteSearcherRunWithOptionsText(t *testing.T) {
	sock := fakeJSONRPCServer(t, func(_ string, _ []byte) any {
		return idxipc.SearchResponse{Count: 0, Results: []idxipc.SearchResult{}}
	})

	out := &fakeOutput{}
	s := NewRemoteSearcher(NewSocketClient(sock), out)
	opts := featsearch.Options{Format: featsearch.OutputText}
	if err := s.RunWithOptions("query", opts); err != nil {
		t.Fatalf("RunWithOptions: %v", err)
	}
	if len(out.lines) == 0 {
		t.Error("expected text output, got none")
	}
}

// --- Call with RPC error ---

// serveOneRPCError responds with a JSON-RPC error object instead of a result.
func serveOneRPCError(conn net.Conn, code int, message string) {
	defer conn.Close()
	msg, err := sharedjsonrpc.ReadMessage(bufio.NewReader(conn))
	if err != nil {
		return
	}
	rpcErr := &sharedjsonrpc.RPCError{Code: code, Message: message}
	resp := sharedjsonrpc.Message{
		JSONRPC: sharedjsonrpc.Version,
		ID:      msg.ID,
		Error:   rpcErr,
	}
	_ = sharedjsonrpc.WriteMessage(conn, resp)
}

// fakeJSONRPCServerWithRPCError starts a server that always returns the given RPC error.
func fakeJSONRPCServerWithRPCError(t *testing.T, code int, message string) string {
	t.Helper()
	dir, err := os.MkdirTemp("/tmp", "idx")
	if err != nil {
		t.Fatalf("mkdirtemp: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	sockPath := fmt.Sprintf("%s/t.sock", dir)

	ln, listenErr := net.Listen("unix", sockPath)
	if listenErr != nil {
		t.Fatalf("listen: %v", listenErr)
	}
	t.Cleanup(func() { ln.Close() })

	go func() {
		for {
			conn, connErr := ln.Accept()
			if connErr != nil {
				return
			}
			go serveOneRPCError(conn, code, message)
		}
	}()

	return sockPath
}

func TestSocketClientCallRPCErrorReturnsError(t *testing.T) {
	sock := fakeJSONRPCServerWithRPCError(t, sharedjsonrpc.ErrInternalError, "internal error")
	c := NewSocketClient(sock)
	var resp idxipc.CommandResponse
	err := c.Call(idxipc.MethodInit, struct{}{}, &resp)
	if err == nil {
		t.Fatal("expected error for RPC error response")
	}
	if !strings.Contains(err.Error(), "RPC error") {
		t.Errorf("expected 'RPC error' in error message, got: %q", err.Error())
	}
}

// --- sendCommand error path ---

func TestRemoteIndexCommandRunWithUnreachableServerReturnsError(t *testing.T) {
	cmd := NewRemoteIndexCommand(NewSocketClient("/tmp/nonexistent-idx-remote-99.sock"), &fakeOutput{})
	if err := cmd.Run(); err == nil {
		t.Fatal("expected error for unreachable server")
	}
}

// --- RemoteReader writer error ---

type errorOutput struct{}

func (w *errorOutput) WriteLine(_ string) error { return fmt.Errorf("write failed") }

func TestRemoteReaderRunWithOptionsWriterErrorReturnsError(t *testing.T) {
	sock := fakeJSONRPCServer(t, func(_ string, _ []byte) any {
		return idxipc.ReadResponse{Lines: []string{"line1"}}
	})
	r := NewRemoteReader(NewSocketClient(sock), &errorOutput{})
	if err := r.RunWithOptions("/file.go", 0, 0); err == nil {
		t.Fatal("expected error from failing writer")
	}
}

func TestRemoteReaderRunWithOptionsCallErrorReturnsError(t *testing.T) {
	r := NewRemoteReader(NewSocketClient("/tmp/nonexistent-idx-reader-99.sock"), &fakeOutput{})
	if err := r.RunWithOptions("/file.go", 0, 0); err == nil {
		t.Fatal("expected error for unreachable server")
	}
}

func TestRemoteSearcherRunWithOptionsCallErrorReturnsError(t *testing.T) {
	s := NewRemoteSearcher(NewSocketClient("/tmp/nonexistent-idx-searcher-99.sock"), &fakeOutput{})
	if err := s.RunWithOptions("query", featsearch.Options{}); err == nil {
		t.Fatal("expected error for unreachable server")
	}
}

// writeAfterNOutput succeeds for the first maxWrites calls then returns an error.
type writeAfterNOutput struct {
	maxWrites int
	count     int
}

func (w *writeAfterNOutput) WriteLine(_ string) error {
	w.count++
	if w.count > w.maxWrites {
		return fmt.Errorf("write failed after %d writes", w.maxWrites)
	}
	return nil
}

func TestWriteSearchResultsTextPathWriteErrorPropagates(t *testing.T) {
	resp := idxipc.SearchResponse{
		Count: 1,
		Results: []idxipc.SearchResult{
			{Path: "/src/main.go", Matches: []idxipc.MatchedLine{{Line: 1, Content: "hello", Match: true}}},
		},
	}
	if err := writeSearchResultsText(resp, featsearch.Options{}, &errorOutput{}); err == nil {
		t.Fatal("expected error when path write fails")
	}
}

func TestWriteTextResultMatchLineWriteError(t *testing.T) {
	r := idxipc.SearchResult{
		Path:    "/src/foo.go",
		Matches: []idxipc.MatchedLine{{Line: 1, Content: "match line", Match: true}},
	}
	// maxWrites=1: path write succeeds, match line write fails.
	if err := writeTextResult(r, featsearch.Options{}, &writeAfterNOutput{maxWrites: 1}); err == nil {
		t.Fatal("expected error when match line write fails")
	}
}
