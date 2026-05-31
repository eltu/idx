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
	return sendCommandResponse(c.client, c.output, idxipc.MethodDestroy)
}
