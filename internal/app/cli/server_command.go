package cli

import (
	"context"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
)

// ServerRunner starts the JSON-RPC index server on a Unix socket.
type ServerRunner interface {
	Serve(ctx context.Context) error
}

func (runner CommandRunner) newServerCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "server",
		Short: "Start the idx JSON-RPC server on a Unix socket",
		RunE: func(_ *cobra.Command, _ []string) error {
			ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
			defer stop()
			return runner.indexServer.Serve(ctx)
		},
	}
}
