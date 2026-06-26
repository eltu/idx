package remote

import (
	"fmt"

	featrelated "idx/internal/features/related"
	idxipc "idx/internal/shared/ipc"
	sharedoutput "idx/internal/shared/output"
)

// RemoteRelatedCommand implements cli.RelatedRunner by calling idx.related over the socket.
type RemoteRelatedCommand struct {
	client *SocketClient
	output sharedoutput.Writer
}

// NewRemoteRelatedCommand creates a RelatedRunner backed by the idx JSON-RPC server.
// Example: r := NewRemoteRelatedCommand(client, writer).
func NewRemoteRelatedCommand(client *SocketClient, output sharedoutput.Writer) *RemoteRelatedCommand {
	return &RemoteRelatedCommand{client: client, output: output}
}

// Run sends a related request and writes the ranked results to output.
func (r *RemoteRelatedCommand) Run(filePath string, opts featrelated.Options) error {
	req := idxipc.RelatedRequest{FilePath: filePath, Size: opts.Size, Format: opts.Format}
	var resp idxipc.RelatedResponse
	if err := r.client.Call(idxipc.MethodRelated, req, &resp); err != nil {
		return err
	}

	if len(resp.Results) == 0 {
		return r.output.WriteLine("No related files found.")
	}

	for _, res := range resp.Results {
		line := fmt.Sprintf("  %-60s %-14s %.2f", res.Path, "("+res.Reason+")", res.Score)
		if err := r.output.WriteLine(line); err != nil {
			return err
		}
	}
	return nil
}
