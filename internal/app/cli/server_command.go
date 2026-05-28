package cli

import (
	"context"
	"os/signal"
	"syscall"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/spf13/cobra"
)

// ServerRunner starts the JSON-RPC index server on a Unix socket.
type ServerRunner interface {
	Serve(ctx context.Context) error
}

// serverManagerCommand controls the server daemon lifecycle.
type serverManagerCommand interface {
	Start(projectPath string) error
	Stop(projectPath string) error
	Status(projectPath string) error
}

func (runner CommandRunner) newServerCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "server",
		Short: "Manage the idx JSON-RPC server daemon",
	}
	cmd.AddCommand(
		runner.newServerStartCommand(),
		runner.newServerStopCommand(),
		runner.newServerStatusCommand(),
		runner.newServerRunCommand(),
	)
	return cmd
}

func (runner CommandRunner) newServerStartCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "start",
		Short: "Start the idx server daemon in the background",
		RunE: func(_ *cobra.Command, _ []string) error {
			return runner.serverManager.Start(".")
		},
	}
}

func (runner CommandRunner) newServerStopCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "stop",
		Short: "Stop the idx server daemon",
		RunE: func(_ *cobra.Command, _ []string) error {
			return runner.serverManager.Stop(".")
		},
	}
}

func (runner CommandRunner) newServerStatusCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show the idx server daemon status",
		RunE: func(_ *cobra.Command, _ []string) error {
			return runner.serverManager.Status(".")
		},
	}
}

// newServerRunCommand is the hidden internal command spawned by OSServerSpawner.
// It runs the JSON-RPC listener and file-watch loop concurrently under one context.
func (runner CommandRunner) newServerRunCommand() *cobra.Command {
	return &cobra.Command{
		Use:    "run",
		Short:  "Run the idx server and watch loop (internal, spawned by 'server start')",
		Hidden: true,
		RunE: func(_ *cobra.Command, _ []string) error {
			ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
			defer stop()

			errg, ctx := errgroup.WithContext(ctx)
			errg.Go(func() error { return runner.indexServer.Serve(ctx) })
			errg.Go(func() error {
				return runner.indexCommand.WatchWithContext(ctx, time.Millisecond)
			})

			return errg.Wait()
		},
	}
}
