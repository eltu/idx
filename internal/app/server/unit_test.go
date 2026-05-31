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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	featsearch "idx/internal/features/search"
	sharedconfig "idx/internal/shared/config"
	idxipc "idx/internal/shared/ipc"
	sharedjsonrpc "idx/internal/shared/jsonrpc"
)

// writeFailConn wraps a net.Conn so that all Write calls return an error.
type writeFailConn struct{ net.Conn }

func (w *writeFailConn) Write(_ []byte) (int, error) {
	return 0, fmt.Errorf("write deliberately failed")
}

// --- captureWriter ---

func TestCaptureWriter_WriteLineAndJoined(t *testing.T) {
	t.Parallel()

	// Arrange + Act
	w := &captureWriter{}
	require.NoError(t, w.WriteLine("line one"))
	require.NoError(t, w.WriteLine("line two"))

	// Assert
	assert.Equal(t, "line one\nline two", w.joined())
}

func TestCaptureWriter_JoinedEmpty_ReturnsEmptyString(t *testing.T) {
	t.Parallel()
	w := &captureWriter{}
	assert.Empty(t, w.joined())
}

func TestCaptureWriter_FirstLine_EmptyWhenNoLines(t *testing.T) {
	t.Parallel()
	w := &captureWriter{}
	assert.Empty(t, w.firstLine())
}

func TestCaptureWriter_FirstLine_ReturnsOnlyFirst(t *testing.T) {
	t.Parallel()
	w := &captureWriter{}
	_ = w.WriteLine("first")
	_ = w.WriteLine("second")
	assert.Equal(t, "first", w.firstLine())
}

// --- operatorOrDefault ---

func TestOperatorOrDefault_EmptyString_DefaultsToAND(t *testing.T) {
	t.Parallel()
	assert.Equal(t, featsearch.OperatorAND, operatorOrDefault(""))
}

func TestOperatorOrDefault_ExplicitOR_ReturnsOR(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "OR", operatorOrDefault("OR"))
}

// --- parseSearchJSON ---

func TestParseSearchJSON_EmptyLine_ReturnsEmpty(t *testing.T) {
	t.Parallel()
	resp, err := parseSearchJSON("")
	require.NoError(t, err)
	assert.Empty(t, resp.Results)
}

func TestParseSearchJSON_ValidResult_ParsesCorrectly(t *testing.T) {
	t.Parallel()
	line := `{"count":1,"results":[{"file":"foo.go","name":"foo.go","path":"/src/foo.go","matches":[{"line":5,"content":"hello","match":true}]}]}`
	resp, err := parseSearchJSON(line)
	require.NoError(t, err)
	assert.Equal(t, 1, resp.Count)
	require.Len(t, resp.Results, 1)
	assert.Equal(t, "foo.go", resp.Results[0].File)
	require.Len(t, resp.Results[0].Matches, 1)
	assert.True(t, resp.Results[0].Matches[0].Match)
}

func TestParseSearchJSON_SpecialFields_ParsedCorrectly(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		line  string
		check func(t *testing.T, resp idxipc.SearchResponse)
	}{
		{
			"score field",
			`{"count":1,"results":[{"file":"x.go","name":"x.go","path":"/x.go","score":1.5,"matches":[]}]}`,
			func(t *testing.T, resp idxipc.SearchResponse) {
				require.NotNil(t, resp.Results[0].Score)
				assert.Equal(t, 1.5, *resp.Results[0].Score)
			},
		},
		{
			"stale field",
			`{"count":1,"results":[{"file":"x.go","name":"x.go","path":"/x.go","stale":true,"matches":[]}]}`,
			func(t *testing.T, resp idxipc.SearchResponse) {
				assert.True(t, resp.Results[0].Stale)
			},
		},
		{
			"empty results",
			`{"count":0,"results":[]}`,
			func(t *testing.T, resp idxipc.SearchResponse) {
				assert.Equal(t, 0, resp.Count)
				assert.Empty(t, resp.Results)
			},
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			resp, err := parseSearchJSON(tc.line)
			require.NoError(t, err)
			tc.check(t, resp)
		})
	}
}

func TestParseSearchJSON_InvalidJSON_ReturnsError(t *testing.T) {
	t.Parallel()
	_, err := parseSearchJSON("not-valid-json")
	require.Error(t, err)
}

// --- searchOptionsFromRequest ---

func TestSearchOptionsFromRequest_MapsAllFields(t *testing.T) {
	t.Parallel()
	req := idxipc.SearchRequest{
		Size:              10,
		Operator:          "OR",
		Format:            "json",
		Context:           3,
		FilesOnly:         true, // intentionally not forwarded; applied client-side
		AgentCompact:      true,
		Explain:           true,
		From:              5,
		PopularityWeight:  0.5,
		ExtensionQueries:  []string{"go"},
		PathQueries:       []string{"internal"},
		RelaxationEnabled: true,
		RelaxationMin:     2,
	}
	opts := searchOptionsFromRequest(req)
	assert.Equal(t, 10, opts.Size)
	assert.Equal(t, "OR", opts.Operator)
	assert.Equal(t, featsearch.OutputJSON, opts.Format)
	assert.False(t, opts.FilesOnly, "FilesOnly is filtered client-side, not forwarded to server service")
	assert.True(t, opts.AgentCompact)
	assert.True(t, opts.Explain)
	assert.Equal(t, 5, opts.From)
	assert.True(t, opts.RelaxationEnabled)
	assert.Equal(t, 2, opts.RelaxationMinExclusive)
}

func TestSearchOptionsFromRequest_EmptyOperator_DefaultsToAND(t *testing.T) {
	t.Parallel()
	opts := searchOptionsFromRequest(idxipc.SearchRequest{})
	assert.Equal(t, featsearch.OperatorAND, opts.Operator)
}

// --- handleConfig ---

func TestHandleConfig_NoFile_OutputContainsNoFileMessage(t *testing.T) {
	t.Parallel()

	// Arrange — deps with empty ConfigFilePath
	s := NewServer(ServerDeps{}).(*indexServer)

	// Act
	resp := dispatchToServer(t, s, idxipc.MethodConfig, struct{}{})

	// Assert
	require.NotNil(t, resp)
	assert.Nil(t, resp.Error)
	resultBytes, err := json.Marshal(resp.Result)
	require.NoError(t, err)
	var cr idxipc.CommandResponse
	require.NoError(t, json.Unmarshal(resultBytes, &cr))
	assert.True(t, cr.Success)
	assert.Contains(t, cr.Output, "No .idx.yml")
}

func TestHandleConfig_WithFile_OutputContainsAllKeys(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := sharedconfig.DefaultIdxConfig()
	s := NewServer(ServerDeps{
		Config:         cfg,
		ConfigFilePath: "/project/.idx.yml",
	}).(*indexServer)

	// Act
	resp := dispatchToServer(t, s, idxipc.MethodConfig, struct{}{})

	// Assert
	require.NotNil(t, resp)
	assert.Nil(t, resp.Error)
	resultBytes, err := json.Marshal(resp.Result)
	require.NoError(t, err)
	var cr idxipc.CommandResponse
	require.NoError(t, json.Unmarshal(resultBytes, &cr))
	assert.True(t, cr.Success)
	for _, key := range sharedconfig.AllKeys() {
		assert.Contains(t, cr.Output, key, "expected key %q in config output", key)
	}
}

func TestHandleConfig_WithOverride_OutputContainsSourceMarker(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := sharedconfig.DefaultIdxConfig()
	cfg.Search.Format = "json"
	s := NewServer(ServerDeps{
		Config:          cfg,
		ConfigFilePath:  "/project/.idx.yml",
		ConfigOverrides: []string{sharedconfig.KeySearchFormat},
	}).(*indexServer)

	// Act
	resp := dispatchToServer(t, s, idxipc.MethodConfig, struct{}{})

	// Assert
	require.NotNil(t, resp)
	resultBytes, err := json.Marshal(resp.Result)
	require.NoError(t, err)
	var cr idxipc.CommandResponse
	require.NoError(t, json.Unmarshal(resultBytes, &cr))
	assert.Contains(t, cr.Output, "← .idx.yml")
	assert.Contains(t, cr.Output, "· default")
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

func TestHandleConn_DispatchesRequest(t *testing.T) {
	t.Parallel()
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
	require.NoError(t, sharedjsonrpc.WriteMessage(&buf, msg))

	done := make(chan struct{})
	go func() {
		defer close(done)
		s.handleConn(context.Background(), serverConn)
	}()

	_, err := clientConn.Write(buf.Bytes())
	require.NoError(t, err)

	resp, err := sharedjsonrpc.ReadMessage(bufio.NewReader(clientConn))
	clientConn.Close()
	<-done

	require.NoError(t, err)
	assert.Nil(t, resp.Error)
}

func TestHandleConn_UnknownMethod_ReturnsError(t *testing.T) {
	t.Parallel()
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
	require.NoError(t, sharedjsonrpc.WriteMessage(&buf, msg))

	done := make(chan struct{})
	go func() {
		defer close(done)
		s.handleConn(context.Background(), serverConn)
	}()

	_, err := clientConn.Write(buf.Bytes())
	require.NoError(t, err)

	resp, err := sharedjsonrpc.ReadMessage(bufio.NewReader(clientConn))
	clientConn.Close()
	<-done

	require.NoError(t, err)
	require.NotNil(t, resp.Error)
	assert.Equal(t, sharedjsonrpc.ErrMethodNotFound, resp.Error.Code)
}

func TestHandleConn_EOF_Ignored(t *testing.T) {
	t.Parallel()
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

func TestHandleConn_NonEOFReadError_IsHandled(t *testing.T) {
	t.Parallel()
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

func TestHandleConn_NotificationNoID_SkipsWrite(t *testing.T) {
	t.Parallel()
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

func TestHandleConn_WriteError_IsHandledGracefully(t *testing.T) {
	t.Parallel()
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

func TestNewServer_RegistersHandlers(t *testing.T) {
	t.Parallel()
	// NewServer must not panic with valid deps structure
	srv := NewServer(ServerDeps{})
	require.NotNil(t, srv)
}

func TestServe_ListenError_ReturnsError(t *testing.T) {
	t.Parallel()
	srv := NewServer(ServerDeps{SocketPath: "/tmp/nonexistent-dir-abc123xyz/s.sock"})
	err := srv.Serve(context.Background())
	require.Error(t, err)
}

func TestServe_AcceptsConnection_SpawnsGoroutine(t *testing.T) {
	// Arrange
	dir, err := os.MkdirTemp("/tmp", "idx")
	require.NoError(t, err, "mkdirtemp")
	defer os.RemoveAll(dir)

	sockPath := dir + "/s.sock"
	srv := NewServer(ServerDeps{SocketPath: sockPath})

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- srv.Serve(ctx) }()

	// Act: wait for the socket to be ready
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

	// Give the spawned goroutine a moment to start before canceling.
	// Use a short sleep rather than a race-prone time.After approach since we are
	// just ensuring the goroutine is schedulable, not awaiting a result.
	time.Sleep(20 * time.Millisecond)

	cancel()

	// Assert
	assert.NoError(t, <-done, "Serve returned unexpected error")
}
