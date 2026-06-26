package remote

import (
	"encoding/json"
	"fmt"

	featrelated "idx/internal/features/related"
	idxipc "idx/internal/shared/ipc"
	sharedoutput "idx/internal/shared/output"
)

const msgNoRelatedFound = "No related files found."

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
	req := idxipc.RelatedRequest{
		FilePath: filePath,
		Size:     opts.Size,
		Skip:     opts.Skip,
		Format:   opts.Format,
		Since:    opts.Since,
		Ext:      opts.Ext,
		Compact:  opts.Compact,
	}
	var resp idxipc.RelatedResponse
	if err := r.client.Call(idxipc.MethodRelated, req, &resp); err != nil {
		return err
	}

	if opts.Format == featrelated.OutputJSON {
		return writeRelatedRespJSON(resp.Results, r.output)
	}

	if len(resp.Results) == 0 {
		return r.output.WriteLine(msgNoRelatedFound)
	}

	for _, res := range resp.Results {
		line := formatRelatedResult(res, opts)
		if err := r.output.WriteLine(line); err != nil {
			return err
		}
	}
	return nil
}

func writeRelatedRespJSON(results []idxipc.RelatedResult, out sharedoutput.Writer) error {
	encoded, err := json.Marshal(results)
	if err != nil {
		return fmt.Errorf("failed to encode related results as JSON: %w", err)
	}
	return out.WriteLine(string(encoded))
}

func formatRelatedResult(res idxipc.RelatedResult, opts featrelated.Options) string {
	if opts.Compact {
		return res.Path
	}
	return fmt.Sprintf("  %-60s %-14s %.2f", res.Path, "("+res.Reason+")", res.Score)
}
