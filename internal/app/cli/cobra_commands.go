package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

const (
	groupIndexSetup = "index-setup"
	groupIndexSync  = "index-sync"
	groupSearch     = "search"
	groupAbout      = "about"
	groupTools      = "tools"
	groupConfig     = "config"
)

func (runner CommandRunner) newRootCommand() *cobra.Command {
	var quiet bool

	root := &cobra.Command{
		Use:           "idx",
		SilenceUsage:  true,
		SilenceErrors: true,
		CompletionOptions: cobra.CompletionOptions{
			DisableDefaultCmd: true,
		},
		Version: runner.buildInfo.Version,
		PersistentPreRun: func(_ *cobra.Command, _ []string) {
			if quiet && runner.quietToggle != nil {
				runner.quietToggle.SetQuiet(true)
			}
		},
	}

	// Customize --version / -v output to use the logo renderer.
	root.SetVersionTemplate(renderVersionOutput(runner.buildInfo.Version, runner.buildInfo.BuildDate) + "\n")

	// --quiet suppresses informational messages so automated/scripted callers
	// (e.g. benchmark pre-steps) do not pollute the agent context window.
	// Errors are always written to stderr via the returned error value.
	root.PersistentFlags().BoolVarP(&quiet, "quiet", "q", false, "Suppress informational output (errors are still written to stderr)")

	root.AddGroup(
		&cobra.Group{ID: groupIndexSetup, Title: "Index Setup:"},
		&cobra.Group{ID: groupIndexSync, Title: "Index Sync:"},
		&cobra.Group{ID: groupSearch, Title: "Search:"},
		&cobra.Group{ID: groupAbout, Title: "About:"},
		&cobra.Group{ID: groupTools, Title: "Tools:"},
		&cobra.Group{ID: groupConfig, Title: "Config:"},
	)

	addCommandToGroup(root, groupIndexSetup, runner.newInitCommand(), runner.newDestroyCommand())
	addCommandToGroup(root, groupIndexSync, runner.newSyncCommand(), runner.newWatchCommand(), runner.newStatusCommand())
	addCommandToGroup(root, groupSearch, runner.newSearchCommand(), runner.newInspectCommand(), runner.newReadCommand())
	addCommandToGroup(root, groupAbout, runner.newVersionCommand())
	addCommandToGroup(root, groupTools, runner.newSkillsCommand(), runner.newServerCommand())
	addCommandToGroup(root, groupConfig, runner.newConfigCommand())

	return root
}

func addCommandToGroup(parent *cobra.Command, groupID string, cmds ...*cobra.Command) {
	for _, cmd := range cmds {
		cmd.GroupID = groupID
		parent.AddCommand(cmd)
	}
}

func (runner CommandRunner) newWatchCommand() *cobra.Command {
	var showUpdatedFiles bool
	var debounce time.Duration

	watchCommand := &cobra.Command{
		Use:   "watch",
		Short: "Watch project files and keep indices synchronized in real time",
		RunE: func(_ *cobra.Command, _ []string) error {
			if debounce <= 0 {
				return fmt.Errorf("invalid --debounce value %s: expected a duration greater than 0", debounce)
			}

			return runner.indexCommand.Watch(showUpdatedFiles, debounce)
		},
	}

	watchCommand.Flags().BoolVar(&showUpdatedFiles, "show-updated-files", false, "Print updated files in each synchronized batch")
	watchCommand.Flags().DurationVar(&debounce, "debounce", runner.config.Watch.Debounce, "Debounce window for batching file events (e.g. 250ms, 1s)")

	return watchCommand
}

func (runner CommandRunner) newSyncCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "sync",
		Short: "Synchronize project indices",
		RunE: func(_ *cobra.Command, _ []string) error {
			return runner.indexCommand.Sync()
		},
	}
}

func (runner CommandRunner) newInitCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Initialize project index",
		RunE: func(_ *cobra.Command, _ []string) error {
			return runner.indexCommand.Run()
		},
	}
}

func (runner CommandRunner) newStatusCommand() *cobra.Command {
	var profile bool

	statusCommand := &cobra.Command{
		Use:   "status",
		Short: "Check whether indexed files are up to date",
		RunE: func(_ *cobra.Command, _ []string) error {
			if !profile {
				if sc, ok := runner.indexCommand.(interface {
					StatusWithContext(string, []string) error
				}); ok {
					return sc.StatusWithContext(runner.configFilePath, runner.configOverrides)
				}
			}

			runner.showConfigBanner()
			if profile {
				if profileCommand, ok := runner.indexCommand.(interface{ StatusWithProfile(bool) error }); ok {
					return profileCommand.StatusWithProfile(true)
				}
			}
			return runner.indexCommand.Status()
		},
	}

	statusCommand.Flags().BoolVar(&profile, "profile", false, "Show detailed per-directory profile report")
	return statusCommand
}

func (runner CommandRunner) newInspectCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "inspect [path]",
		Short: "Inspect an index payload (JSON with path, interactive without path)",
		Args: func(_ *cobra.Command, args []string) error {
			_, err := parseInspectArguments(args)
			return err
		},
		RunE: func(_ *cobra.Command, args []string) error {
			inspectPath, err := parseInspectArguments(args)
			if err != nil {
				return err
			}

			return runner.indexCommand.Inspect(inspectPath)
		},
	}
}

func (runner CommandRunner) newDestroyCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "destroy",
		Short: "Destroy index metadata",
		RunE: func(_ *cobra.Command, _ []string) error {
			if err := runner.stopServerForDestroy(); err != nil {
				return err
			}

			return runner.destroyCommand.Run()
		},
	}
}

func (runner CommandRunner) stopServerForDestroy() error {
	if runner.serverManager == nil {
		return nil
	}
	err := runner.serverManager.Stop(".")
	if err == nil || isIgnorableServerStopError(err) {
		return nil
	}
	return err
}

func isIgnorableServerStopError(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "not running") ||
		strings.Contains(msg, "state not found") ||
		strings.Contains(msg, "not found")
}
