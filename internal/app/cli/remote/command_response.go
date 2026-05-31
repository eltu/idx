package remote

import (
	idxipc "idx/internal/shared/ipc"
	sharedoutput "idx/internal/shared/output"
)

// sendCommandResponse calls method on client and writes any non-empty output to out.
// Shared by RemoteDestroyCommand, RemoteConfigCommand, and similar single-shot commands.
func sendCommandResponse(client *SocketClient, out sharedoutput.Writer, method string) error {
	var resp idxipc.CommandResponse
	if err := client.Call(method, struct{}{}, &resp); err != nil {
		return err
	}
	if resp.Output != "" {
		return out.WriteLine(resp.Output)
	}
	return nil
}
