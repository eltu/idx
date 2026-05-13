package cli

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/spf13/cobra"

	"idx/internal/core/ports"
)

const (
	groupIndexSetup = "index-setup"
	groupIndexSync  = "index-sync"
	groupSearch     = "search"
	groupAbout      = "about"
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
	)

	addCommandToGroup(root, groupIndexSetup, runner.newInitCommand(), runner.newDestroyCommand())
	addCommandToGroup(root, groupIndexSync, runner.newSyncCommand(), runner.newWatchCommand(), runner.newDaemonCommand(), runner.newStatusCommand())
	addCommandToGroup(root, groupSearch, runner.newSearchCommand(), runner.newInspectCommand())
	addCommandToGroup(root, groupAbout, runner.newVersionCommand())

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

func (runner CommandRunner) newStatusCommand() *cobra.Command {
	var profile bool

	statusCommand := &cobra.Command{
		Use:   "status",
		Short: "Check whether indexed files are up to date",
		RunE: func(_ *cobra.Command, _ []string) error {
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
			if err := runner.disableDaemonForDestroy(); err != nil {
				return err
			}

			return runner.destroyCommand.Run()
		},
	}
}

func (runner CommandRunner) disableDaemonForDestroy() error {
	err := runner.daemonService.Disable(".")
	if err == nil {
		return nil
	}

	if isIgnorableDestroyDaemonDisableError(err) {
		return nil
	}

	return err
}

func isIgnorableDestroyDaemonDisableError(err error) bool {
	message := err.Error()
	if strings.Contains(message, "daemon not initialized") {
		return true
	}

	if strings.Contains(message, "not being monitored") {
		return true
	}

	if strings.Contains(message, "no projects active") {
		return true
	}

	return false
}

func (runner CommandRunner) newSearchCommand() *cobra.Command {
	config := &searchCommandConfig{}
	var searchCommand *cobra.Command
	searchCommand = &cobra.Command{
		Use:   "search [query terms]",
		Short: "Search indexed content",
		RunE: func(_ *cobra.Command, args []string) error {
			return runner.runSearchCommand(searchCommand, args, config)
		},
	}

	configureSearchFlags(searchCommand, config)

	return searchCommand
}

type searchCommandConfig struct {
	format            string
	contextLines      int
	prettyJSON        bool
	explain           bool
	agentCompact      bool
	matchesOnly       bool
	legacyMatchesOnly bool
	filesOnly         bool
	pathQueries       []string
	extensionQueries  []string
	from              int
	size              int
	operator          string
	relaxation        string
	relaxationMin     int
	relaxationEnabled bool
}

func (runner CommandRunner) runSearchCommand(searchCommand *cobra.Command, args []string, config *searchCommandConfig) error {
	if err := validateSearchConfig(config, searchCommand.Flags().Changed("size")); err != nil {
		return err
	}

	query := strings.Join(args, " ")
	if err := validateSearchInput(query, config.pathQueries, config.extensionQueries, runner.arguments); err != nil {
		writeSearchMissingQueryError(searchCommand)
		return fmt.Errorf("")
	}

	return runner.searchCommand.RunWithOptions(query, config.options())
}

func configureSearchFlags(searchCommand *cobra.Command, config *searchCommandConfig) {
	searchCommand.Flags().StringVar(&config.format, "format", ports.SearchOutputText, "Output format: text|json")
	searchCommand.Flags().IntVar(&config.contextLines, "context", 0, "Number of context lines around matches")
	searchCommand.Flags().BoolVar(&config.prettyJSON, "json-pretty", false, "Pretty-print JSON output")
	searchCommand.Flags().BoolVar(&config.explain, "explain", false, "Include ranking metadata such as score")
	searchCommand.Flags().BoolVar(&config.agentCompact, "agent-compact", false, "Use compact text output optimized for agents (fewer tokens)")
	searchCommand.Flags().BoolVar(&config.matchesOnly, "matches-only", false, "Show only directly matched lines")
	searchCommand.Flags().BoolVar(&config.legacyMatchesOnly, "macthes-only", false, "Legacy typo alias for matches-only")
	searchCommand.Flags().MarkHidden("macthes-only")
	searchCommand.Flags().BoolVar(&config.filesOnly, "files-only", false, "Show only matched file paths")
	searchCommand.Flags().StringArrayVar(&config.pathQueries, "path", []string{}, "Filter results by metadata path (repeatable)")
	searchCommand.Flags().StringArrayVar(&config.extensionQueries, "ext", []string{}, "Filter results by file extension (repeatable). Accepts go or .go")
	searchCommand.Flags().IntVar(&config.from, "from", 0, "Skip the first N ranked files")
	searchCommand.Flags().IntVar(&config.size, "size", 0, "Limit results to top N files")
	searchCommand.Flags().StringVar(&config.operator, "operator", ports.SearchOperatorAND, "Boolean operator for multi-term queries: AND|OR")
	searchCommand.Flags().StringVar(&config.relaxation, "relaxation", "", "Relax AND query with trailing-term fallback. Format: >N")
}

func validateSearchConfig(config *searchCommandConfig, sizeChanged bool) error {
	if err := validateSearchFlagValues(config.contextLines, config.from, config.size, sizeChanged); err != nil {
		return err
	}

	if err := validateSearchFormat(config.format, config.prettyJSON); err != nil {
		return err
	}

	if err := validateSearchOperator(config.operator); err != nil {
		return err
	}

	return validateSearchRelaxation(config)
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

func validateSearchOperator(operator string) error {
	if operator != ports.SearchOperatorAND && operator != ports.SearchOperatorOR {
		return fmt.Errorf("unsupported --operator value %q: expected one of [%s %s]", operator, ports.SearchOperatorAND, ports.SearchOperatorOR)
	}

	return nil
}

func validateSearchRelaxation(config *searchCommandConfig) error {
	config.relaxationEnabled = false
	config.relaxationMin = 0

	if strings.TrimSpace(config.relaxation) == "" {
		return nil
	}

	if config.operator != ports.SearchOperatorAND {
		return fmt.Errorf("invalid --relaxation with --operator %q: expected %q", config.operator, ports.SearchOperatorAND)
	}

	value := strings.TrimSpace(config.relaxation)
	if !strings.HasPrefix(value, ">") || len(value) == 1 {
		return fmt.Errorf("invalid --relaxation value %q: expected format >N where N is a non-negative integer", config.relaxation)
	}

	parsed, err := strconv.Atoi(value[1:])
	if err != nil || parsed < 0 {
		return fmt.Errorf("invalid --relaxation value %q: expected format >N where N is a non-negative integer", config.relaxation)
	}

	config.relaxationEnabled = true
	config.relaxationMin = parsed
	return nil
}

func validateSearchInput(query string, pathQueries []string, extensionQueries []string, arguments []string) error {
	if query == "" && len(pathQueries) == 0 && len(extensionQueries) == 0 {
		return fmt.Errorf("missing search query: got %v, expected idx search <terms>", arguments)
	}

	return nil
}

func (config searchCommandConfig) options() ports.SearchOptions {
	options := ports.SearchOptions{
		Format:                 config.format,
		Context:                config.contextLines,
		PrettyJSON:             config.prettyJSON,
		Explain:                config.explain,
		AgentCompact:           config.agentCompact,
		MatchesOnly:            config.matchesOnly || config.legacyMatchesOnly,
		FilesOnly:              config.filesOnly,
		PathQueries:            config.pathQueries,
		ExtensionQueries:       config.extensionQueries,
		From:                   config.from,
		Size:                   config.size,
		Operator:               config.operator,
		RelaxationEnabled:      config.relaxationEnabled,
		RelaxationMinExclusive: config.relaxationMin,
	}

	if len(config.pathQueries) > 0 {
		options.PathQuery = config.pathQueries[0]
	}

	if len(config.extensionQueries) > 0 {
		options.ExtensionQuery = config.extensionQueries[0]
	}

	return options
}

var (
	searchErrorWarningStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FBBF24"))
	searchErrorMutedStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#64748B"))
	searchErrorActionStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#6366F1"))
	searchErrorPathStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#94A3B8"))
)

func writeSearchMissingQueryError(cmd *cobra.Command) {
	usage := searchErrorActionStyle.Render("idx search") + " " + searchErrorPathStyle.Render("<terms>")
	examples := []string{
		searchErrorMutedStyle.Render("idx search") + " " + searchErrorPathStyle.Render("error handling"),
		searchErrorMutedStyle.Render("idx search") + " " + searchErrorPathStyle.Render("--ext go func main"),
		searchErrorMutedStyle.Render("idx search") + " " + searchErrorPathStyle.Render("--path internal logger"),
	}

	msg := fmt.Sprintf("\n%s\n\n  Usage:  %s\n\n  Examples:\n    %s\n",
		searchErrorWarningStyle.Render("⚠  Missing search query"),
		usage,
		strings.Join(examples, "\n    "),
	)

	fmt.Fprintln(cmd.OutOrStdout(), msg)
}
