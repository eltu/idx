package jsonrpc

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// --- codec ---

func TestWriteMessageReadMessageRoundTrip(t *testing.T) {
	id := json.RawMessage(`42`)
	msg := Message{
		JSONRPC: Version,
		ID:      &id,
		Method:  "test.ping",
		Params:  json.RawMessage(`{"x":1}`),
	}

	var buf bytes.Buffer
	if err := WriteMessage(&buf, msg); err != nil {
		t.Fatalf("WriteMessage: %v", err)
	}

	got, err := ReadMessage(bufio.NewReader(&buf))
	if err != nil {
		t.Fatalf("ReadMessage: %v", err)
	}
	if got.Method != msg.Method {
		t.Errorf("method: want %q, got %q", msg.Method, got.Method)
	}
}

func TestWriteMessageProducesContentLengthHeader(t *testing.T) {
	msg := Message{JSONRPC: Version, Method: "ping"}
	var buf bytes.Buffer
	if err := WriteMessage(&buf, msg); err != nil {
		t.Fatalf("WriteMessage: %v", err)
	}
	if !strings.Contains(buf.String(), "Content-Length:") {
		t.Errorf("expected Content-Length header in output, got: %q", buf.String())
	}
}

func TestReadMessageMissingContentLength(t *testing.T) {
	input := "X-Other: 5\r\n\r\nhello"
	_, err := ReadMessage(bufio.NewReader(strings.NewReader(input)))
	if err == nil {
		t.Fatal("expected error for missing Content-Length")
	}
}

func TestReadMessageInvalidContentLengthValue(t *testing.T) {
	input := "Content-Length: not-a-number\r\n\r\n"
	_, err := ReadMessage(bufio.NewReader(strings.NewReader(input)))
	if err == nil {
		t.Fatal("expected error for non-numeric Content-Length")
	}
}

func TestReadMessageTruncatedBody(t *testing.T) {
	// Claim 100 bytes but only provide 2
	input := "Content-Length: 100\r\n\r\n{}"
	_, err := ReadMessage(bufio.NewReader(strings.NewReader(input)))
	if err == nil {
		t.Fatal("expected error for truncated body")
	}
}

func TestReadMessageInvalidJSON(t *testing.T) {
	body := []byte("not-json")
	header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(body))
	input := header + string(body)
	_, err := ReadMessage(bufio.NewReader(strings.NewReader(input)))
	if err == nil {
		t.Fatal("expected error for invalid JSON body")
	}
}

func TestReadMessageEOFReturnsError(t *testing.T) {
	_, err := ReadMessage(bufio.NewReader(strings.NewReader("")))
	if err == nil {
		t.Fatal("expected error for empty reader (EOF)")
	}
}

func TestReadMessageIgnoresUnknownHeaders(t *testing.T) {
	body := []byte(`{"jsonrpc":"2.0","method":"hi"}`)
	input := fmt.Sprintf("X-Custom: ignored\r\nContent-Length: %d\r\n\r\n%s", len(body), body)
	got, err := ReadMessage(bufio.NewReader(strings.NewReader(input)))
	if err != nil {
		t.Fatalf("ReadMessage: %v", err)
	}
	if got.Method != "hi" {
		t.Errorf("unexpected method: %q", got.Method)
	}
}

// --- dispatcher ---

func TestDispatcherKnownMethodReturnsResult(t *testing.T) {
	d := NewDispatcher()
	id := json.RawMessage(`1`)
	d.Register("greet", func(_ context.Context, _ json.RawMessage) (any, error) {
		return "hello", nil
	})

	resp := d.Dispatch(context.Background(), &Message{JSONRPC: Version, ID: &id, Method: "greet"})
	if resp == nil || resp.Error != nil {
		t.Fatalf("expected success response, got: %+v", resp)
	}
}

func TestDispatcherUnknownMethodReturnsError(t *testing.T) {
	d := NewDispatcher()
	id := json.RawMessage(`1`)

	resp := d.Dispatch(context.Background(), &Message{JSONRPC: Version, ID: &id, Method: "unknown"})
	if resp == nil || resp.Error == nil || resp.Error.Code != ErrMethodNotFound {
		t.Errorf("expected method-not-found error, got: %+v", resp)
	}
}

func TestDispatcherHandlerErrorReturnsInternalError(t *testing.T) {
	d := NewDispatcher()
	id := json.RawMessage(`1`)
	d.Register("fail", func(_ context.Context, _ json.RawMessage) (any, error) {
		return nil, fmt.Errorf("boom")
	})

	resp := d.Dispatch(context.Background(), &Message{JSONRPC: Version, ID: &id, Method: "fail"})
	if resp == nil || resp.Error == nil || resp.Error.Code != ErrInternalError {
		t.Errorf("expected internal-error response, got: %+v", resp)
	}
}

func TestDispatcherNotificationWithKnownMethodReturnsNil(t *testing.T) {
	d := NewDispatcher()
	d.Register("notify", func(_ context.Context, _ json.RawMessage) (any, error) {
		return "ignored", nil
	})

	// No ID field = notification
	resp := d.Dispatch(context.Background(), &Message{JSONRPC: Version, Method: "notify"})
	if resp != nil {
		t.Errorf("expected nil for notification, got: %+v", resp)
	}
}

func TestDispatcherNotificationWithUnknownMethodReturnsNil(t *testing.T) {
	d := NewDispatcher()
	resp := d.Dispatch(context.Background(), &Message{JSONRPC: Version, Method: "unknown"})
	if resp != nil {
		t.Errorf("expected nil for unknown-method notification, got: %+v", resp)
	}
}

func TestDispatcherNotificationWithHandlerErrorReturnsNil(t *testing.T) {
	d := NewDispatcher()
	d.Register("fail", func(_ context.Context, _ json.RawMessage) (any, error) {
		return nil, fmt.Errorf("error")
	})
	resp := d.Dispatch(context.Background(), &Message{JSONRPC: Version, Method: "fail"})
	if resp != nil {
		t.Errorf("expected nil for error notification, got: %+v", resp)
	}
}

func TestDispatcherRegisterOverwritesHandler(t *testing.T) {
	d := NewDispatcher()
	id := json.RawMessage(`1`)
	d.Register("method", func(_ context.Context, _ json.RawMessage) (any, error) {
		return "first", nil
	})
	d.Register("method", func(_ context.Context, _ json.RawMessage) (any, error) {
		return "second", nil
	})

	resp := d.Dispatch(context.Background(), &Message{JSONRPC: Version, ID: &id, Method: "method"})
	if resp == nil || resp.Error != nil {
		t.Fatalf("expected success, got: %+v", resp)
	}
}
