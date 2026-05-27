package jsonrpc

import (
	"context"
	"encoding/json"
)

// HandlerFunc handles a JSON-RPC method call and returns a result or error.
type HandlerFunc func(ctx context.Context, params json.RawMessage) (any, error)

// Dispatcher routes JSON-RPC messages to registered handlers.
type Dispatcher struct {
	handlers map[string]HandlerFunc
}

// NewDispatcher creates an empty Dispatcher.
func NewDispatcher() *Dispatcher {
	return &Dispatcher{handlers: make(map[string]HandlerFunc)}
}

// Register binds a handler to a method name.
func (d *Dispatcher) Register(method string, h HandlerFunc) {
	d.handlers[method] = h
}

// Dispatch routes msg to its handler and returns the response Message.
// Returns nil for notifications (no ID field) — caller must not write a response.
func (d *Dispatcher) Dispatch(ctx context.Context, msg *Message) *Message {
	h, ok := d.handlers[msg.Method]
	if !ok {
		if msg.ID == nil {
			return nil
		}
		return errorResponse(msg.ID, ErrMethodNotFound, "method not found: "+msg.Method)
	}

	result, err := h(ctx, msg.Params)
	if err != nil {
		if msg.ID == nil {
			return nil
		}
		return errorResponse(msg.ID, ErrInternalError, err.Error())
	}

	if msg.ID == nil {
		return nil
	}

	return &Message{JSONRPC: Version, ID: msg.ID, Result: result}
}

func errorResponse(id *json.RawMessage, code int, message string) *Message {
	return &Message{
		JSONRPC: Version,
		ID:      id,
		Error:   &RPCError{Code: code, Message: message},
	}
}
