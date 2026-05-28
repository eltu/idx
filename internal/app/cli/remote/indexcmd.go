package remote

import (
	"context"
	"time"

	idxipc "idx/internal/shared/ipc"
	sharedoutput "idx/internal/shared/output"
)

// RemoteIndexCommand implements cli.indexableCommand (Run/Sync/Status/Inspect/Watch)
// by forwarding to the idx JSON-RPC server where applicable.
type RemoteIndexCommand struct {
	client *SocketClient
	output sharedoutput.Writer
}

// NewRemoteIndexCommand creates an index command backed by the idx JSON-RPC server.
// Example: cmd := NewRemoteIndexCommand(client, writer).
func NewRemoteIndexCommand(client *SocketClient, output sharedoutput.Writer) *RemoteIndexCommand {
	return &RemoteIndexCommand{client: client, output: output}
}

// Run sends idx.init to the server and prints the output.
func (c *RemoteIndexCommand) Run() error {
	return c.sendCommand(idxipc.MethodInit)
}

// Sync sends idx.sync to the server and prints the output.
func (c *RemoteIndexCommand) Sync() error {
	return c.sendCommand(idxipc.MethodSync)
}

// Status sends idx.status to the server and prints the output.
func (c *RemoteIndexCommand) Status() error {
	return c.sendCommand(idxipc.MethodStatus)
}

// Inspect is not supported over RPC — the TUI requires a local process.
func (c *RemoteIndexCommand) Inspect(_ string) error {
	return c.output.WriteLine("idx inspect is not available in server mode — run idx server locally and use idx inspect directly")
}

// Watch is not supported over RPC — the server handles watching internally.
func (c *RemoteIndexCommand) Watch(_ bool, _ time.Duration) error {
	return c.output.WriteLine("idx watch is not available in server mode — the server watches the project automatically")
}

// WatchWithContext is not supported over RPC — the server handles watching internally.
func (c *RemoteIndexCommand) WatchWithContext(_ context.Context, _ time.Duration) error {
	return c.output.WriteLine("idx watch is not available in server mode — the server watches the project automatically")
}

func (c *RemoteIndexCommand) sendCommand(method string) error {
	var resp idxipc.CommandResponse
	if err := c.client.Call(method, struct{}{}, &resp); err != nil {
		return err
	}
	if resp.Output != "" {
		return c.output.WriteLine(resp.Output)
	}
	return nil
}
