package remote

import (
	idxipc "idx/internal/shared/ipc"
	sharedoutput "idx/internal/shared/output"
)

// RemoteConfigCommand implements cli.ConfigShower by forwarding idx.config to the server.
type RemoteConfigCommand struct {
	client *SocketClient
	output sharedoutput.Writer
}

// NewRemoteConfigCommand creates a config command backed by the idx JSON-RPC server.
// Example: cmd := NewRemoteConfigCommand(client, writer).
func NewRemoteConfigCommand(client *SocketClient, output sharedoutput.Writer) *RemoteConfigCommand {
	return &RemoteConfigCommand{client: client, output: output}
}

// Show sends idx.config to the server and writes the formatted output.
func (c *RemoteConfigCommand) Show() error {
	return sendCommandResponse(c.client, c.output, idxipc.MethodConfig)
}
