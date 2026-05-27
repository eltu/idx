package remote

import (
	idxipc "idx/internal/shared/ipc"
	sharedoutput "idx/internal/shared/output"
)

// RemoteReader implements cli.Reader by calling idx.read over the socket.
type RemoteReader struct {
	client *SocketClient
	output sharedoutput.Writer
}

// NewRemoteReader creates a Reader backed by the idx JSON-RPC server.
// Example: r := NewRemoteReader(client, writer).
func NewRemoteReader(client *SocketClient, output sharedoutput.Writer) *RemoteReader {
	return &RemoteReader{client: client, output: output}
}

// RunWithOptions sends a read request and writes the file lines to output.
func (r *RemoteReader) RunWithOptions(filePath string, fromLine, toLine int) error {
	req := idxipc.ReadRequest{FilePath: filePath, FromLine: fromLine, ToLine: toLine}
	var resp idxipc.ReadResponse
	if err := r.client.Call(idxipc.MethodRead, req, &resp); err != nil {
		return err
	}
	for _, line := range resp.Lines {
		if err := r.output.WriteLine(line); err != nil {
			return err
		}
	}
	return nil
}
