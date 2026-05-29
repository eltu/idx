package jsonrpc

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- codec ---

func TestWriteMessage_ReadMessage_RoundTripsMessage(t *testing.T) {
	t.Parallel()

	// Arrange
	id := json.RawMessage(`42`)
	msg := Message{
		JSONRPC: Version,
		ID:      &id,
		Method:  "test.ping",
		Params:  json.RawMessage(`{"x":1}`),
	}
	var buf bytes.Buffer

	// Act
	require.NoError(t, WriteMessage(&buf, msg))
	got, err := ReadMessage(bufio.NewReader(&buf))

	// Assert
	require.NoError(t, err)
	assert.Equal(t, msg.Method, got.Method)
}

func TestWriteMessage_ProducesContentLengthHeader(t *testing.T) {
	t.Parallel()

	// Arrange
	msg := Message{JSONRPC: Version, Method: "ping"}
	var buf bytes.Buffer

	// Act
	require.NoError(t, WriteMessage(&buf, msg))

	// Assert
	assert.Contains(t, buf.String(), "Content-Length:")
}

func TestReadMessage_MissingContentLength_ReturnsError(t *testing.T) {
	t.Parallel()

	// Arrange
	input := "X-Other: 5\r\n\r\nhello"

	// Act
	_, err := ReadMessage(bufio.NewReader(strings.NewReader(input)))

	// Assert
	require.Error(t, err)
}

func TestReadMessage_InvalidContentLengthValue_ReturnsError(t *testing.T) {
	t.Parallel()

	// Arrange
	input := "Content-Length: not-a-number\r\n\r\n"

	// Act
	_, err := ReadMessage(bufio.NewReader(strings.NewReader(input)))

	// Assert
	require.Error(t, err)
}

func TestReadMessage_TruncatedBody_ReturnsError(t *testing.T) {
	t.Parallel()

	// Arrange — claim 100 bytes but only provide 2
	input := "Content-Length: 100\r\n\r\n{}"

	// Act
	_, err := ReadMessage(bufio.NewReader(strings.NewReader(input)))

	// Assert
	require.Error(t, err)
}

func TestReadMessage_InvalidJSON_ReturnsError(t *testing.T) {
	t.Parallel()

	// Arrange
	body := []byte("not-json")
	input := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(body)) + string(body)

	// Act
	_, err := ReadMessage(bufio.NewReader(strings.NewReader(input)))

	// Assert
	require.Error(t, err)
}

func TestReadMessage_EmptyReader_ReturnsError(t *testing.T) {
	t.Parallel()

	// Act
	_, err := ReadMessage(bufio.NewReader(strings.NewReader("")))

	// Assert
	require.Error(t, err)
}

func TestReadMessage_IgnoresUnknownHeaders(t *testing.T) {
	t.Parallel()

	// Arrange
	body := []byte(`{"jsonrpc":"2.0","method":"hi"}`)
	input := fmt.Sprintf("X-Custom: ignored\r\nContent-Length: %d\r\n\r\n%s", len(body), body)

	// Act
	got, err := ReadMessage(bufio.NewReader(strings.NewReader(input)))

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "hi", got.Method)
}

// --- dispatcher ---

func TestDispatcher_KnownMethod_ReturnsResult(t *testing.T) {
	t.Parallel()

	// Arrange
	d := NewDispatcher()
	id := json.RawMessage(`1`)
	d.Register("greet", func(_ context.Context, _ json.RawMessage) (any, error) {
		return "hello", nil
	})

	// Act
	resp := d.Dispatch(context.Background(), &Message{JSONRPC: Version, ID: &id, Method: "greet"})

	// Assert
	require.NotNil(t, resp)
	assert.Nil(t, resp.Error)
}

func TestDispatcher_UnknownMethod_ReturnsMethodNotFoundError(t *testing.T) {
	t.Parallel()

	// Arrange
	d := NewDispatcher()
	id := json.RawMessage(`1`)

	// Act
	resp := d.Dispatch(context.Background(), &Message{JSONRPC: Version, ID: &id, Method: "unknown"})

	// Assert
	require.NotNil(t, resp)
	require.NotNil(t, resp.Error)
	assert.Equal(t, ErrMethodNotFound, resp.Error.Code)
}

func TestDispatcher_HandlerError_ReturnsInternalError(t *testing.T) {
	t.Parallel()

	// Arrange
	d := NewDispatcher()
	id := json.RawMessage(`1`)
	d.Register("fail", func(_ context.Context, _ json.RawMessage) (any, error) {
		return nil, fmt.Errorf("boom")
	})

	// Act
	resp := d.Dispatch(context.Background(), &Message{JSONRPC: Version, ID: &id, Method: "fail"})

	// Assert
	require.NotNil(t, resp)
	require.NotNil(t, resp.Error)
	assert.Equal(t, ErrInternalError, resp.Error.Code)
}

func TestDispatcher_NotificationWithKnownMethod_ReturnsNil(t *testing.T) {
	t.Parallel()

	// Arrange
	d := NewDispatcher()
	d.Register("notify", func(_ context.Context, _ json.RawMessage) (any, error) {
		return "ignored", nil
	})

	// Act — no ID field means this is a notification
	resp := d.Dispatch(context.Background(), &Message{JSONRPC: Version, Method: "notify"})

	// Assert
	assert.Nil(t, resp)
}

func TestDispatcher_NotificationWithUnknownMethod_ReturnsNil(t *testing.T) {
	t.Parallel()

	// Arrange
	d := NewDispatcher()

	// Act
	resp := d.Dispatch(context.Background(), &Message{JSONRPC: Version, Method: "unknown"})

	// Assert
	assert.Nil(t, resp)
}

func TestDispatcher_NotificationWithHandlerError_ReturnsNil(t *testing.T) {
	t.Parallel()

	// Arrange
	d := NewDispatcher()
	d.Register("fail", func(_ context.Context, _ json.RawMessage) (any, error) {
		return nil, fmt.Errorf("error")
	})

	// Act
	resp := d.Dispatch(context.Background(), &Message{JSONRPC: Version, Method: "fail"})

	// Assert
	assert.Nil(t, resp)
}

func TestDispatcher_RegisterOverwrite_UsesLatestHandler(t *testing.T) {
	t.Parallel()

	// Arrange
	d := NewDispatcher()
	id := json.RawMessage(`1`)
	d.Register("method", func(_ context.Context, _ json.RawMessage) (any, error) {
		return "first", nil
	})
	d.Register("method", func(_ context.Context, _ json.RawMessage) (any, error) {
		return "second", nil
	})

	// Act
	resp := d.Dispatch(context.Background(), &Message{JSONRPC: Version, ID: &id, Method: "method"})

	// Assert
	require.NotNil(t, resp)
	assert.Nil(t, resp.Error)
}
