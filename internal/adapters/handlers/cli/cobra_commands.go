package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"idx/internal/core/ports"
)

func (runner CommandRunner) newRootCommand() *cobra.Command {
	root := &cobra.Command{
		Use:           "idx",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	root.AddCommand(runner.newSyncCommand())
	root.AddCommand(runner.newInitCommand())
	root.AddCommand(runner.newInspectCommand())
	root.AddCommand(runner.newWatchCommand())
	root.AddCommand(runner.newDestroyCommand())
	root.AddCommand(runner.newSearchCommand())

	return root
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
	watchCommand.Flags().DurationVar(&debounce, "debounce", 750*time.Millisecond, "Debounce window for batching file events (e.g. 250ms, 1s)")

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

func (runner CommandRunner) newInspectCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "inspect <path>",
		Short: "Inspect an index payload",
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
			return runner.destroyCommand.Run()
		},
	}
}

func (runner CommandRunner) newSearchCommand() *cobra.Command {
	var format string
	var contextLines int
	var prettyJSON bool
	var matchesOnly bool
	var legacyMatchesOnly bool
	var filesOnly bool
	var pathQueries []string
	var from int
	var size int

	var searchCommand *cobra.Command
	searchCommand = &cobra.Command{
		Use:   "search [query terms]",
		Short: "Search indexed content",
		RunE: func(_ *cobra.Command, args []string) error {
			if err := validateSearchFlagValues(contextLines, from, size, searchCommand.Flags().Changed("size")); err != nil {
				return err
			}

			if err := validateSearchFormat(format, prettyJSON); err != nil {
				return err
			}

			query := strings.Join(args, " ")
			if err := validateSearchInput(query, pathQueries, runner.arguments); err != nil {
				return err
			}

			options := buildSearchOptions(format, contextLines, prettyJSON, matchesOnly, legacyMatchesOnly, filesOnly, pathQueries, from, size)
			return runner.searchCommand.RunWithOptions(query, options)
		},
	}

	searchCommand.Flags().StringVar(&format, "format", ports.SearchOutputText, "Output format: text|json")
	searchCommand.Flags().IntVar(&contextLines, "context", 0, "Number of context lines around matches")
	searchCommand.Flags().BoolVar(&prettyJSON, "json-pretty", false, "Pretty-print JSON output")
	searchCommand.Flags().BoolVar(&matchesOnly, "matches-only", false, "Show only directly matched lines")
	searchCommand.Flags().BoolVar(&legacyMatchesOnly, "macthes-only", false, "Legacy typo alias for matches-only")
	searchCommand.Flags().MarkHidden("macthes-only")
	searchCommand.Flags().BoolVar(&filesOnly, "files-only", false, "Show only matched file paths")
	searchCommand.Flags().StringArrayVar(&pathQueries, "path", []string{}, "Filter results by metadata path (repeatable)")
	searchCommand.Flags().IntVar(&from, "from", 0, "Skip the first N ranked files")
	searchCommand.Flags().IntVar(&size, "size", 0, "Limit results to top N files")

	return searchCommand
}

func validateSearchFlagValues(contextLines int, from int, size int, sizeChanged bool) error {
	if contextLines < 0 {
		return fmt.Errorf("invalid --context value %d: expected a non-negative integer", contextLines)
	}

	if from < 0 {
		return fmt.Errorf("invalid --from value %d: expected a non-negative integer", from)
	}

	if size < 0 {
		return fmt.Errorf("invalid --size value %d: expected a positive integer", size)
	}

	if size == 0 && sizeChanged {
		return fmt.Errorf("invalid --size value %d: expected a positive integer", size)
	}

	return nil
}

func validateSearchFormat(format string, prettyJSON bool) error {
	if format != ports.SearchOutputText && format != ports.SearchOutputJSON {
		return fmt.Errorf("unsupported --format value %q: expected one of [%s %s]", format, ports.SearchOutputText, ports.SearchOutputJSON)
	}

	if prettyJSON && format != ports.SearchOutputJSON {
		return fmt.Errorf("--json-pretty requires --format %s: got format %q", ports.SearchOutputJSON, format)
	}

	return nil
}

func validateSearchInput(query string, pathQueries []string, arguments []string) error {
	if query == "" && len(pathQueries) == 0 {
		return fmt.Errorf("missing search query: got %v, expected idx search <terms>", arguments)
	}

	return nil
}

func buildSearchOptions(
	format string,
	contextLines int,
	prettyJSON bool,
	matchesOnly bool,
	legacyMatchesOnly bool,
	filesOnly bool,
	pathQueries []string,
	from int,
	size int,
) ports.SearchOptions {
	options := ports.SearchOptions{
		Format:      format,
		Context:     contextLines,
		PrettyJSON:  prettyJSON,
		MatchesOnly: matchesOnly || legacyMatchesOnly,
		FilesOnly:   filesOnly,
		PathQueries: pathQueries,
		From:        from,
		Size:        size,
	}

	if len(pathQueries) > 0 {
		options.PathQuery = pathQueries[0]
	}

	return options
}
