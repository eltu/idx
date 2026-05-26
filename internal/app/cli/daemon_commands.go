package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// newDaemonCommand creates the root 'idx daemon' command.
func (runner CommandRunner) newDaemonCommand() *cobra.Command {
	daemonCommand := &cobra.Command{
		Use:   "daemon",
		Short: "Manage realtime project monitoring daemon",
	}

	daemonCommand.AddCommand(runner.newDaemonEnableCommand())
	daemonCommand.AddCommand(runner.newDaemonDisableCommand())
	daemonCommand.AddCommand(runner.newDaemonStatusCommand())

	return daemonCommand
}

// newDaemonEnableCommand creates the 'idx daemon enable <path>' command.
func (runner CommandRunner) newDaemonEnableCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "enable <path>",
		Short: "Enable realtime watch for a project",
		Long:  "Activates realtime file monitoring for a project. If the project index doesn't exist, it will be created automatically.",
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) != 1 {
				return fmt.Errorf("expected project path argument")
			}
			return nil
		},
		RunE: func(_ *cobra.Command, args []string) error {
			projectPath := args[0]
			return runner.daemonService.Enable(projectPath)
		},
	}
}

// newDaemonDisableCommand creates the 'idx daemon disable <path>' command.
func (runner CommandRunner) newDaemonDisableCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "disable <path>",
		Short: "Disable realtime watch for a project",
		Long:  "Stops monitoring a project and removes it from the daemon state.",
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) != 1 {
				return fmt.Errorf("expected project path argument")
			}
			return nil
		},
		RunE: func(_ *cobra.Command, args []string) error {
			projectPath := args[0]
			return runner.daemonService.Disable(projectPath)
		},
	}
}

// newDaemonStatusCommand creates the 'idx daemon status' command.
func (runner CommandRunner) newDaemonStatusCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show all monitored projects",
		Long:  "Lists all projects currently being monitored by the daemon and their status.",
		RunE: func(_ *cobra.Command, _ []string) error {
			return runner.daemonService.Status()
		},
	}
}
