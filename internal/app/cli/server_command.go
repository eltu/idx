package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"charm.land/lipgloss/v2"
	"go.uber.org/zap"

	"github.com/spf13/cobra"
)

var (
	serverErrTitleStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#EF4444"))
	serverErrActionStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#6366F1"))
	serverErrHintStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#64748B"))
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

// serverStatusJSONProvider is an optional enhancement — implementations that support
// structured JSON output for 'idx server status --json' can satisfy this interface.
type serverStatusJSONProvider interface {
	StatusJSON(projectPath string) ([]byte, error)
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
		RunE: func(cmd *cobra.Command, _ []string) error {
			root, err := runner.resolveServerRoot()
			if err != nil {
				_, _ = fmt.Fprintln(cmd.ErrOrStderr(), serverNotInProjectMessage())
				return fmt.Errorf("")
			}
			return runner.serverManager.Start(root)
		},
	}
}

func (runner CommandRunner) newServerStopCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "stop",
		Short: "Stop the idx server daemon",
		RunE: func(cmd *cobra.Command, _ []string) error {
			root, err := runner.resolveServerRoot()
			if err != nil {
				_, _ = fmt.Fprintln(cmd.ErrOrStderr(), serverNotInProjectMessage())
				return fmt.Errorf("")
			}
			return runner.serverManager.Stop(root)
		},
	}
}

func (runner CommandRunner) newServerStatusCommand() *cobra.Command {
	var jsonOut bool

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show the idx server daemon status",
		RunE: func(cmd *cobra.Command, _ []string) error {
			root, err := runner.resolveServerRoot()
			if err != nil {
				_, _ = fmt.Fprintln(cmd.ErrOrStderr(), serverNotInProjectMessage())
				return fmt.Errorf("")
			}
			if jsonOut {
				return runner.runServerStatusJSON(cmd, root)
			}
			return runner.serverManager.Status(root)
		},
	}

	cmd.Flags().BoolVarP(&jsonOut, "json", "j", false, "Output status as JSON")
	return cmd
}

func (runner CommandRunner) runServerStatusJSON(cmd *cobra.Command, root string) error {
	jp, ok := runner.serverManager.(serverStatusJSONProvider)
	if !ok {
		// Fall back to text output when the impl does not support JSON.
		return runner.serverManager.Status(root)
	}
	data, err := jp.StatusJSON(root)
	if err != nil {
		return err
	}
	_, _ = fmt.Fprintln(cmd.OutOrStdout(), string(data))
	return nil
}

// resolveServerRoot returns the pre-resolved project root when available,
// falling back to a filesystem walk looking for a .idx directory.
// The pre-resolved root (set via WithProjectRoot) is preferred so that CLI
// commands stay consistent with the path already computed by sharedDeps and
// passed to the server via IDX_PROJECT_PATH.
func (runner CommandRunner) resolveServerRoot() (string, error) {
	if runner.projectRoot != "" {
		return runner.projectRoot, nil
	}
	return findProjectRoot(".")
}

// newServerRunCommand is the hidden internal command spawned by OSServerSpawner.
// It runs the JSON-RPC listener and file-watch loop concurrently under one context.
// The server is the primary component: its exit drives shutdown. Watch errors are
// logged but do not stop the server — search and read continue working regardless.
func (runner CommandRunner) newServerRunCommand() *cobra.Command {
	return &cobra.Command{
		Use:    "run",
		Short:  "Run the idx server and watch loop (internal, spawned by 'server start')",
		Hidden: true,
		RunE: func(_ *cobra.Command, _ []string) error {
			ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
			defer stop()

			go func() {
				if err := runner.indexCommand.WatchWithContext(ctx, defaultServerWatchDebounce); err != nil && ctx.Err() == nil {
					zap.L().Error("watch loop exited with error", zap.Error(err))
				}
			}()

			return runner.indexServer.Serve(ctx)
		},
	}
}

const defaultServerWatchDebounce = 750 * time.Millisecond

// findProjectRoot walks up the directory tree from startDir looking for a .idx
// directory, which marks the root of an idx-managed project.
// Example: findProjectRoot("/home/user/myproject/internal/core") → "/home/user/myproject".
func findProjectRoot(startDir string) (string, error) {
	dir, err := filepath.Abs(startDir)
	if err != nil {
		return "", fmt.Errorf("cannot resolve directory: %w", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, ".idx")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("not inside an idx project: no .idx directory found from %q", startDir)
		}
		dir = parent
	}
}

func serverNotInProjectMessage() string {
	return fmt.Sprintf("\n%s\n  %s\n  %s\n",
		serverErrTitleStyle.Render("✗ Not inside an idx project"),
		serverErrActionStyle.Render("cd <project-root>"),
		serverErrHintStyle.Render("then: idx server start"),
	)
}
