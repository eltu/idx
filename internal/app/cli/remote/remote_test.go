package remote

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	featindexing "idx/internal/features/indexing"
	featsearch "idx/internal/features/search"
	idxipc "idx/internal/shared/ipc"
	sharedjsonrpc "idx/internal/shared/jsonrpc"
)

// fakeInspectUIRunner captures the index passed to it without rendering a TUI.
type fakeInspectUIRunner struct {
	captured *featindexing.InvertedIndex
}

func (f *fakeInspectUIRunner) Run(index *featindexing.InvertedIndex) error {
	f.captured = index
	return nil
}

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
	require.NoError(t, err, "mkdirtemp")
	t.Cleanup(func() { os.RemoveAll(dir) })
	sockPath := fmt.Sprintf("%s/t.sock", dir)

	ln, listenErr := net.Listen("unix", sockPath)
	require.NoError(t, listenErr, "listen")
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

func TestNewSocketClient_StoresPath(t *testing.T) {
	t.Parallel()
	c := NewSocketClient("/tmp/test.sock")
	require.NotNil(t, c)
	assert.Equal(t, "/tmp/test.sock", c.socketPath)
}

func TestSocketClientCall_ServerNotReachable_ReturnsError(t *testing.T) {
	t.Parallel()
	c := NewSocketClient("/tmp/nonexistent-idx-test.sock")
	var resp any
	err := c.Call("test", struct{}{}, &resp)
	require.Error(t, err)
	assert.Contains(t, err.Error(), errNoServerMsg)
}

func TestSocketClientCall_Success_ReturnsResponse(t *testing.T) {
	t.Parallel()

	// Arrange
	sock := fakeJSONRPCServer(t, func(_ string, _ []byte) any {
		return idxipc.CommandResponse{Success: true, Output: "done"}
	})

	// Act
	c := NewSocketClient(sock)
	var resp idxipc.CommandResponse
	err := c.Call(idxipc.MethodInit, struct{}{}, &resp)

	// Assert
	require.NoError(t, err)
	assert.True(t, resp.Success)
	assert.Equal(t, "done", resp.Output)
}

func TestSocketClientCall_RPCError_ReturnsError(t *testing.T) {
	t.Parallel()
	sock := fakeJSONRPCServerWithRPCError(t, sharedjsonrpc.ErrInternalError, "internal error")
	c := NewSocketClient(sock)
	var resp idxipc.CommandResponse
	err := c.Call(idxipc.MethodInit, struct{}{}, &resp)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "RPC error")
}

func TestServerNotRunningError_ContainsHints(t *testing.T) {
	t.Parallel()
	err := serverNotRunningError()
	require.NotNil(t, err)
	assert.Contains(t, err.Error(), errNoServerStartMsg)
}

// --- RemoteIndexCommand ---

func TestRemoteIndexCommand_Run_SendsInit(t *testing.T) {
	t.Parallel()

	// Arrange
	var calledMethod string
	sock := fakeJSONRPCServer(t, func(method string, _ []byte) any {
		calledMethod = method
		return idxipc.CommandResponse{Success: true, Output: "indexed"}
	})
	out := &fakeOutput{}

	// Act
	cmd := NewRemoteIndexCommand(NewSocketClient(sock), out, &fakeInspectUIRunner{})
	err := cmd.Run()

	// Assert
	require.NoError(t, err)
	assert.Equal(t, idxipc.MethodInit, calledMethod)
	require.Len(t, out.lines, 1)
	assert.Equal(t, "indexed", out.lines[0])
}

func TestRemoteIndexCommand_Sync_SendsSync(t *testing.T) {
	t.Parallel()
	var calledMethod string
	sock := fakeJSONRPCServer(t, func(method string, _ []byte) any {
		calledMethod = method
		return idxipc.CommandResponse{Success: true}
	})
	cmd := NewRemoteIndexCommand(NewSocketClient(sock), &fakeOutput{}, &fakeInspectUIRunner{})
	require.NoError(t, cmd.Sync())
	assert.Equal(t, idxipc.MethodSync, calledMethod)
}

func TestRemoteIndexCommand_Status_SendsStatus(t *testing.T) {
	t.Parallel()
	var calledMethod string
	sock := fakeJSONRPCServer(t, func(method string, _ []byte) any {
		calledMethod = method
		return idxipc.CommandResponse{Success: true}
	})
	cmd := NewRemoteIndexCommand(NewSocketClient(sock), &fakeOutput{}, &fakeInspectUIRunner{})
	require.NoError(t, cmd.Status())
	assert.Equal(t, idxipc.MethodStatus, calledMethod)
}

func TestRemoteIndexCommand_Inspect_FetchesIndexAndRunsTUI(t *testing.T) {
	t.Parallel()

	// Arrange
	index := featindexing.InvertedIndex{DocumentCount: 3}
	sock := fakeJSONRPCServer(t, func(method string, _ []byte) any {
		assert.Equal(t, idxipc.MethodInspect, method)
		return index
	})
	runner := &fakeInspectUIRunner{}
	cmd := NewRemoteIndexCommand(NewSocketClient(sock), &fakeOutput{}, runner)

	// Act
	err := cmd.Inspect("")

	// Assert
	require.NoError(t, err)
	require.NotNil(t, runner.captured)
	assert.Equal(t, 3, runner.captured.DocumentCount)
}

func TestRemoteIndexCommand_Inspect_ServerError_ReturnsError(t *testing.T) {
	t.Parallel()
	cmd := NewRemoteIndexCommand(NewSocketClient("/tmp/nonexistent-idx-inspect-99.sock"), &fakeOutput{}, &fakeInspectUIRunner{})
	require.Error(t, cmd.Inspect(""))
}

func TestRemoteIndexCommand_Run_EmptyOutput_IsOK(t *testing.T) {
	t.Parallel()
	sock := fakeJSONRPCServer(t, func(_ string, _ []byte) any {
		return idxipc.CommandResponse{Success: true, Output: ""}
	})
	out := &fakeOutput{}
	cmd := NewRemoteIndexCommand(NewSocketClient(sock), out, &fakeInspectUIRunner{})
	require.NoError(t, cmd.Run())
	assert.Empty(t, out.lines)
}

func TestRemoteIndexCommand_Run_UnreachableServer_ReturnsError(t *testing.T) {
	t.Parallel()
	cmd := NewRemoteIndexCommand(NewSocketClient("/tmp/nonexistent-idx-remote-99.sock"), &fakeOutput{}, &fakeInspectUIRunner{})
	require.Error(t, cmd.Run())
}

// --- RemoteDestroyCommand ---

func TestRemoteDestroyCommand_Run_SendsDestroy(t *testing.T) {
	t.Parallel()

	// Arrange
	var calledMethod string
	sock := fakeJSONRPCServer(t, func(method string, _ []byte) any {
		calledMethod = method
		return idxipc.CommandResponse{Success: true, Output: "🧹 Index metadata removed from project."}
	})
	out := &fakeOutput{}

	// Act
	cmd := NewRemoteDestroyCommand(NewSocketClient(sock), out)
	err := cmd.Run()

	// Assert
	require.NoError(t, err)
	assert.Equal(t, idxipc.MethodDestroy, calledMethod)
	require.Len(t, out.lines, 1)
	assert.Contains(t, out.lines[0], "Index metadata removed")
}

func TestRemoteDestroyCommand_Run_EmptyOutput_IsOK(t *testing.T) {
	t.Parallel()
	sock := fakeJSONRPCServer(t, func(_ string, _ []byte) any {
		return idxipc.CommandResponse{Success: true}
	})
	cmd := NewRemoteDestroyCommand(NewSocketClient(sock), &fakeOutput{})
	require.NoError(t, cmd.Run())
}

func TestRemoteDestroyCommand_Run_UnreachableServer_ReturnsError(t *testing.T) {
	t.Parallel()
	cmd := NewRemoteDestroyCommand(NewSocketClient("/tmp/nonexistent-idx-destroy-99.sock"), &fakeOutput{})
	require.Error(t, cmd.Run())
}

// --- RemoteReader ---

func TestRemoteReader_RunWithOptions_WritesLines(t *testing.T) {
	t.Parallel()

	// Arrange
	sock := fakeJSONRPCServer(t, func(_ string, _ []byte) any {
		return idxipc.ReadResponse{Lines: []string{"line1", "line2"}}
	})
	out := &fakeOutput{}

	// Act
	r := NewRemoteReader(NewSocketClient(sock), out)
	err := r.RunWithOptions("/some/file.go", 0, 0)

	// Assert
	require.NoError(t, err)
	require.Len(t, out.lines, 2)
	assert.Equal(t, "line1", out.lines[0])
}

func TestRemoteReader_RunWithOptions_EmptyLines_IsOK(t *testing.T) {
	t.Parallel()
	sock := fakeJSONRPCServer(t, func(_ string, _ []byte) any {
		return idxipc.ReadResponse{Lines: []string{}}
	})
	out := &fakeOutput{}
	r := NewRemoteReader(NewSocketClient(sock), out)
	require.NoError(t, r.RunWithOptions("/some/file.go", 0, 0))
	assert.Empty(t, out.lines)
}

func TestRemoteReader_RunWithOptions_WriterError_ReturnsError(t *testing.T) {
	t.Parallel()
	sock := fakeJSONRPCServer(t, func(_ string, _ []byte) any {
		return idxipc.ReadResponse{Lines: []string{"line1"}}
	})
	r := NewRemoteReader(NewSocketClient(sock), &errorOutput{})
	require.Error(t, r.RunWithOptions("/file.go", 0, 0))
}

func TestRemoteReader_RunWithOptions_CallError_ReturnsError(t *testing.T) {
	t.Parallel()
	r := NewRemoteReader(NewSocketClient("/tmp/nonexistent-idx-reader-99.sock"), &fakeOutput{})
	require.Error(t, r.RunWithOptions("/file.go", 0, 0))
}

// --- searchRequestFromOptions ---

func TestSearchRequestFromOptions_MapsAllFields(t *testing.T) {
	t.Parallel()
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
	assert.Equal(t, "hello", req.Query)
	assert.Equal(t, 5, req.Size)
	assert.Equal(t, "OR", req.Operator)
	assert.Equal(t, "json", req.Format)
	assert.True(t, req.FilesOnly)
	assert.True(t, req.AgentCompact)
	assert.True(t, req.Explain)
	assert.Equal(t, 3, req.From)
	assert.Equal(t, 0.7, req.PopularityWeight)
}

// --- writeSearchResultsJSON ---

func TestWriteSearchResultsJSON_Compact_WritesResult(t *testing.T) {
	t.Parallel()
	resp := idxipc.SearchResponse{Count: 1, Results: []idxipc.SearchResult{{File: "x.go", Name: "x.go", Path: "/x.go"}}}
	out := &fakeOutput{}
	require.NoError(t, writeSearchResultsJSON(resp, featsearch.Options{Format: featsearch.OutputJSON}, out))
	require.NotEmpty(t, out.lines)
	assert.Contains(t, out.lines[0], "x.go")
}

func TestWriteSearchResultsJSON_Pretty_WritesResult(t *testing.T) {
	t.Parallel()
	resp := idxipc.SearchResponse{Count: 0, Results: []idxipc.SearchResult{}}
	out := &fakeOutput{}
	require.NoError(t, writeSearchResultsJSON(resp, featsearch.Options{Format: featsearch.OutputJSON, PrettyJSON: true}, out))
	assert.NotEmpty(t, out.lines)
}

func TestWriteSearchResultsJSON_FilesOnly_WritesStringArray(t *testing.T) {
	t.Parallel()
	resp := idxipc.SearchResponse{Count: 2, Results: []idxipc.SearchResult{
		{File: "a.go", Name: "a.go", Path: "pkg/a.go"},
		{File: "b.go", Name: "b.go", Path: "pkg/b.go"},
	}}
	out := &fakeOutput{}
	require.NoError(t, writeSearchResultsJSON(resp, featsearch.Options{Format: featsearch.OutputJSON, FilesOnly: true}, out))
	require.NotEmpty(t, out.lines)
	var paths []string
	require.NoError(t, json.Unmarshal([]byte(out.lines[0]), &paths))
	assert.Equal(t, []string{"pkg/a.go", "pkg/b.go"}, paths)
}

// --- writeSearchResultsText ---

func TestWriteSearchResultsText_NoResults_WritesNoResultsMessage(t *testing.T) {
	t.Parallel()
	out := &fakeOutput{}
	require.NoError(t, writeSearchResultsText(idxipc.SearchResponse{}, featsearch.Options{}, out))
	require.NotEmpty(t, out.lines)
	assert.Contains(t, out.lines[0], "No results")
}

func TestWriteSearchResultsText_WithResults_WritesOutput(t *testing.T) {
	t.Parallel()
	resp := idxipc.SearchResponse{
		Count: 1,
		Results: []idxipc.SearchResult{
			{File: "main.go", Name: "main.go", Path: "/src/main.go", Matches: []idxipc.MatchedLine{
				{Line: 1, Content: "package main", Match: true},
			}},
		},
	}
	out := &fakeOutput{}
	require.NoError(t, writeSearchResultsText(resp, featsearch.Options{}, out))
	assert.NotEmpty(t, out.lines)
}

func TestWriteSearchResultsText_PathWriteError_Propagates(t *testing.T) {
	t.Parallel()
	resp := idxipc.SearchResponse{
		Count: 1,
		Results: []idxipc.SearchResult{
			{Path: "/src/main.go", Matches: []idxipc.MatchedLine{{Line: 1, Content: "hello", Match: true}}},
		},
	}
	require.Error(t, writeSearchResultsText(resp, featsearch.Options{}, &errorOutput{}))
}

// --- writeTextResult ---

func TestWriteTextResult_FilesOnly_WritesPathOnly(t *testing.T) {
	t.Parallel()
	r := idxipc.SearchResult{Path: "/src/main.go"}
	out := &fakeOutput{}
	require.NoError(t, writeTextResult(r, featsearch.Options{FilesOnly: true}, out))
	require.Len(t, out.lines, 1)
	assert.Equal(t, "/src/main.go", out.lines[0])
}

func TestWriteTextResult_MatchesOnly_FiltersContextLines(t *testing.T) {
	t.Parallel()
	r := idxipc.SearchResult{
		Path: "/src/foo.go",
		Matches: []idxipc.MatchedLine{
			{Line: 1, Content: "context line", Match: false},
			{Line: 2, Content: "match line", Match: true},
		},
	}
	out := &fakeOutput{}
	require.NoError(t, writeTextResult(r, featsearch.Options{MatchesOnly: true}, out))
	// path line + only matched lines
	assert.Len(t, out.lines, 2)
}

func TestWriteTextResult_AllMatches_WritesAll(t *testing.T) {
	t.Parallel()
	r := idxipc.SearchResult{
		Path: "/src/bar.go",
		Matches: []idxipc.MatchedLine{
			{Line: 1, Content: "line one\r\n", Match: true},
			{Line: 2, Content: "line two", Match: false},
		},
	}
	out := &fakeOutput{}
	require.NoError(t, writeTextResult(r, featsearch.Options{}, out))
	// path + 2 match lines
	assert.Len(t, out.lines, 3)
}

func TestWriteTextResult_MatchLineWriteError_Propagates(t *testing.T) {
	t.Parallel()
	r := idxipc.SearchResult{
		Path:    "/src/foo.go",
		Matches: []idxipc.MatchedLine{{Line: 1, Content: "match line", Match: true}},
	}
	// maxWrites=1: path write succeeds, match line write fails.
	require.Error(t, writeTextResult(r, featsearch.Options{}, &writeAfterNOutput{maxWrites: 1}))
}

// --- RemoteSearcher ---

func TestRemoteSearcher_RunWithOptions_JSON_WritesOutput(t *testing.T) {
	t.Parallel()
	sock := fakeJSONRPCServer(t, func(_ string, _ []byte) any {
		return idxipc.SearchResponse{Count: 1, Results: []idxipc.SearchResult{{File: "a.go", Name: "a.go", Path: "/a.go", Matches: []idxipc.MatchedLine{}}}}
	})
	out := &fakeOutput{}
	s := NewRemoteSearcher(NewSocketClient(sock), out)
	require.NoError(t, s.RunWithOptions("query", featsearch.Options{Format: featsearch.OutputJSON}))
	assert.NotEmpty(t, out.lines)
}

func TestRemoteSearcher_RunWithOptions_Text_WritesOutput(t *testing.T) {
	t.Parallel()
	sock := fakeJSONRPCServer(t, func(_ string, _ []byte) any {
		return idxipc.SearchResponse{Count: 0, Results: []idxipc.SearchResult{}}
	})
	out := &fakeOutput{}
	s := NewRemoteSearcher(NewSocketClient(sock), out)
	require.NoError(t, s.RunWithOptions("query", featsearch.Options{Format: featsearch.OutputText}))
	assert.NotEmpty(t, out.lines)
}

func TestRemoteSearcher_RunWithOptions_CallError_ReturnsError(t *testing.T) {
	t.Parallel()
	s := NewRemoteSearcher(NewSocketClient("/tmp/nonexistent-idx-searcher-99.sock"), &fakeOutput{})
	require.Error(t, s.RunWithOptions("query", featsearch.Options{}))
}

// --- helper server/outputs for error cases ---

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
	require.NoError(t, err, "mkdirtemp")
	t.Cleanup(func() { os.RemoveAll(dir) })
	sockPath := fmt.Sprintf("%s/t.sock", dir)

	ln, listenErr := net.Listen("unix", sockPath)
	require.NoError(t, listenErr, "listen")
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

// errorOutput always returns an error from WriteLine.
type errorOutput struct{}

func (w *errorOutput) WriteLine(_ string) error { return fmt.Errorf("write failed") }

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

// Ensure strings is used (referenced in TestSocketClientCall_ServerNotReachable_ReturnsError).
var _ = strings.Contains

// --- RemoteConfigCommand ---

func TestRemoteConfigCommand_Show_WritesOutput(t *testing.T) {
	t.Parallel()

	// Arrange — serve a CommandResponse with a config text payload
	sockPath := fakeJSONRPCServer(t, func(_ string, _ []byte) any {
		return idxipc.CommandResponse{Success: true, Output: "  search.format  text  · default\n"}
	})
	client := NewSocketClient(sockPath)
	out := &fakeOutput{}

	// Act
	err := NewRemoteConfigCommand(client, out).Show()

	// Assert
	require.NoError(t, err)
	require.NotEmpty(t, out.lines)
	assert.Contains(t, out.lines[0], "search.format")
}

func TestRemoteConfigCommand_Show_EmptyOutput_NoWrite(t *testing.T) {
	t.Parallel()

	// Arrange — server returns empty output (no .idx.yml scenario)
	sockPath := fakeJSONRPCServer(t, func(_ string, _ []byte) any {
		return idxipc.CommandResponse{Success: true, Output: ""}
	})
	client := NewSocketClient(sockPath)
	out := &fakeOutput{}

	// Act
	err := NewRemoteConfigCommand(client, out).Show()

	// Assert
	require.NoError(t, err)
	assert.Empty(t, out.lines)
}

func TestRemoteConfigCommand_Show_ServerNotRunning_ReturnsError(t *testing.T) {
	t.Parallel()
	client := NewSocketClient("/tmp/idx-nonexistent-config-test.sock")
	out := &fakeOutput{}
	err := NewRemoteConfigCommand(client, out).Show()
	require.Error(t, err)
	assert.ErrorContains(t, err, "server not running")
}

// ---- RemoteSearcher --count ----

func TestRemoteSearcher_CountOnly_PrintsOnlyCount(t *testing.T) {
	t.Parallel()

	// Arrange — server returns 7 results
	const wantCount = 7
	sockPath := fakeJSONRPCServer(t, func(_ string, _ []byte) any {
		results := make([]idxipc.SearchResult, wantCount)
		for i := range results {
			results[i] = idxipc.SearchResult{Path: fmt.Sprintf("file%d.go", i)}
		}
		return idxipc.SearchResponse{Count: wantCount, Results: results}
	})
	client := NewSocketClient(sockPath)
	out := &fakeOutput{}

	// Act
	err := NewRemoteSearcher(client, out).RunWithOptions("query", featsearch.Options{
		Format:    featsearch.OutputText,
		Operator:  "AND",
		CountOnly: true,
		FilesOnly: true, // set by CLI when --count is used
	})

	// Assert
	require.NoError(t, err)
	require.Len(t, out.lines, 1)
	assert.Equal(t, "7", out.lines[0])
}

// ---- RemoteSearcher --time ----

func TestRemoteSearcher_TimingEnabled_AppendsTimingLine(t *testing.T) {
	t.Parallel()

	// Arrange — server returns one result
	sockPath := fakeJSONRPCServer(t, func(_ string, _ []byte) any {
		return idxipc.SearchResponse{
			Count: 1,
			Results: []idxipc.SearchResult{
				{Path: "service.go", Matches: []idxipc.MatchedLine{{Line: 1, Content: "func", Match: true}}},
			},
		}
	})
	client := NewSocketClient(sockPath)
	out := &fakeOutput{}

	// Act
	err := NewRemoteSearcher(client, out).RunWithOptions("func", featsearch.Options{
		Format:   featsearch.OutputText,
		Operator: "AND",
		Timing:   true,
	})

	// Assert
	require.NoError(t, err)
	require.NotEmpty(t, out.lines)
	// Timing line is the last line and contains "ms".
	last := out.lines[len(out.lines)-1]
	assert.Contains(t, last, "ms")
}
