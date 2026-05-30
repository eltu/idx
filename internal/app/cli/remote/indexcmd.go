package remote

import (
	"context"
	"time"

	featindexing "idx/internal/features/indexing"
	idxipc "idx/internal/shared/ipc"
	sharedoutput "idx/internal/shared/output"
)

// RemoteIndexCommand implements cli.indexableCommand (Run/Sync/Status/Inspect/Watch)
// by forwarding to the idx JSON-RPC server where applicable.
type RemoteIndexCommand struct {
	client    *SocketClient
	output    sharedoutput.Writer
	inspectUI featindexing.InspectUIRunner
}

// NewRemoteIndexCommand creates an index command backed by the idx JSON-RPC server.
// Example: cmd := NewRemoteIndexCommand(client, writer, inspectRunner).
func NewRemoteIndexCommand(client *SocketClient, output sharedoutput.Writer, inspectUI featindexing.InspectUIRunner) *RemoteIndexCommand {
	return &RemoteIndexCommand{client: client, output: output, inspectUI: inspectUI}
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

// Inspect fetches the merged InvertedIndex from the server and renders the TUI locally.
// Example: err := cmd.Inspect("").
func (c *RemoteIndexCommand) Inspect(indexPath string) error {
	var index featindexing.InvertedIndex
	if err := c.client.Call(idxipc.MethodInspect, idxipc.InspectRequest{IndexPath: indexPath}, &index); err != nil {
		return err
	}
	return c.inspectUI.Run(&index)
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
