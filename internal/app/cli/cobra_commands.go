package cli

import (
	"strings"

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
	addCommandToGroup(root, groupIndexSync, runner.newSyncCommand(), runner.newStatusCommand())
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

func (runner CommandRunner) newSyncCommand() *cobra.Command {
	return &cobra.Command{
		Use:     "sync",
		Aliases: []string{"update"},
		Short:   "Synchronize project indices",
		Long: `Incrementally re-index changed directories using file checksums.
Only directories with modified content are re-processed — unchanged dirs are skipped.

Examples:
  idx sync
  idx update`,
		RunE: func(_ *cobra.Command, _ []string) error {
			return runner.indexCommand.Sync()
		},
	}
}

func (runner CommandRunner) newInitCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Initialize project index",
		Long: `Build the full BM25 index for the current project.
Safe to re-run: subsequent calls sync only changed directories.

Examples:
  idx init`,
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
		Short: "Browse the BM25 index (interactive TUI without args, JSON dump with path)",
		Long: `Without arguments: opens an interactive TUI to browse indexed directories, documents,
and transaction logs. Use / to filter, Enter to drill down, q to quit.

With a path argument: dumps the raw BM25 index for that directory as JSON,
useful for debugging ranking issues.

Examples:
  idx inspect
  idx inspect internal/features/search`,
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
		Long: `Remove all .idx index files and stop the server daemon.
Useful for a clean rebuild: run 'idx init' afterwards to re-index.

Examples:
  idx destroy`,
		RunE: func(_ *cobra.Command, _ []string) error {
			// Run destroy first so the RPC reaches the server while it is still alive.
			// stopServerForDestroy sends SIGTERM after, which is a no-op when the server
			// already exited or when serverManager is nil (standalone mode).
			if err := runner.destroyCommand.Run(); err != nil {
				return err
			}
			return runner.stopServerForDestroy()
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
