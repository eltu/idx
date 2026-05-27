package remote

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"

	"charm.land/lipgloss/v2"

	sharedjsonrpc "idx/internal/shared/jsonrpc"
)

const (
	errNoServerMsg      = "idx server not running"
	errNoServerStartMsg = "start with: idx server"
	errNoServerAltMsg   = "or:          idx daemon enable ."
)

var (
	errServerStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#EF4444"))
	errHintStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#64748B"))
	errActionStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#6366F1"))
)

// SocketClient makes JSON-RPC calls to an idx server over a Unix socket.
type SocketClient struct {
	socketPath string
}

// NewSocketClient creates a SocketClient for the given socket path.
// Example: c := NewSocketClient("/home/user/.idx/myproject.sock").
func NewSocketClient(socketPath string) *SocketClient {
	return &SocketClient{socketPath: socketPath}
}

// Call sends a JSON-RPC request and decodes the result into resp.
// Returns a styled error if the server is not reachable.
func (c *SocketClient) Call(method string, req, resp any) error {
	d := net.Dialer{}
	conn, err := d.DialContext(context.Background(), "unix", c.socketPath)
	if err != nil {
		return serverNotRunningError()
	}
	defer func() { _ = conn.Close() }()

	id := json.RawMessage(`1`)
	params, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to encode RPC request: %w", err)
	}

	msg := sharedjsonrpc.Message{
		JSONRPC: sharedjsonrpc.Version,
		ID:      &id,
		Method:  method,
		Params:  params,
	}
	if err := sharedjsonrpc.WriteMessage(conn, msg); err != nil {
		return fmt.Errorf("failed to send RPC request: %w", err)
	}

	reply, err := sharedjsonrpc.ReadMessage(bufio.NewReader(conn))
	if err != nil {
		return fmt.Errorf("failed to read RPC response: %w", err)
	}

	if reply.Error != nil {
		return fmt.Errorf("RPC error %d: %s", reply.Error.Code, reply.Error.Message)
	}

	raw, err := json.Marshal(reply.Result)
	if err != nil {
		return fmt.Errorf("failed to re-encode RPC result: %w", err)
	}

	return json.Unmarshal(raw, resp)
}

func serverNotRunningError() error {
	msg := fmt.Sprintf("\n%s\n  %s\n  %s\n",
		errServerStyle.Render("✗ "+errNoServerMsg),
		errActionStyle.Render(errNoServerStartMsg),
		errHintStyle.Render(errNoServerAltMsg),
	)
	return fmt.Errorf("%s", msg)
}
