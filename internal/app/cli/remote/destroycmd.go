package remote

import (
	idxipc "idx/internal/shared/ipc"
	sharedoutput "idx/internal/shared/output"
)

// RemoteDestroyCommand implements cli.Runner by forwarding idx.destroy to the server.
type RemoteDestroyCommand struct {
	client *SocketClient
	output sharedoutput.Writer
}

// NewRemoteDestroyCommand creates a destroy command backed by the idx JSON-RPC server.
// Example: cmd := NewRemoteDestroyCommand(client, writer).
func NewRemoteDestroyCommand(client *SocketClient, output sharedoutput.Writer) *RemoteDestroyCommand {
	return &RemoteDestroyCommand{client: client, output: output}
}

// Run sends idx.destroy to the server and prints the output.
func (c *RemoteDestroyCommand) Run() error {
	var resp idxipc.CommandResponse
	if err := c.client.Call(idxipc.MethodDestroy, struct{}{}, &resp); err != nil {
		return err
	}
	if resp.Output != "" {
		return c.output.WriteLine(resp.Output)
	}
	return nil
}
