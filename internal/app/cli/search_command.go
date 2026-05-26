package cli

import (
	"fmt"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/spf13/cobra"

	"idx/internal/features/search"
)

const (
	searchCmdName          = "idx search"
	errMsgRelaxationFormat = "invalid --relaxation value %q: expected format >N where N is a non-negative integer"
)

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

	runner.configureSearchFlags(searchCommand, config)

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
	popularityWeight  float64
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

func (runner CommandRunner) configureSearchFlags(searchCommand *cobra.Command, config *searchCommandConfig) {
	cfg := runner.config.Search
	searchCommand.Flags().StringVar(&config.format, "format", cfg.Format, "Output format: text|json")
	searchCommand.Flags().IntVar(&config.contextLines, "context", cfg.Context, "Number of context lines around matches")
	searchCommand.Flags().BoolVar(&config.prettyJSON, "json-pretty", false, "Pretty-print JSON output")
	searchCommand.Flags().BoolVar(&config.explain, "explain", false, "Include ranking metadata such as score")
	searchCommand.Flags().BoolVar(&config.agentCompact, "agent-compact", false, "Use compact text output optimized for agents (fewer tokens)")
	searchCommand.Flags().BoolVar(&config.matchesOnly, "matches-only", false, "Show only directly matched lines")
	searchCommand.Flags().BoolVar(&config.legacyMatchesOnly, "macthes-only", false, "Legacy typo alias for matches-only")
	_ = searchCommand.Flags().MarkHidden("macthes-only")
	searchCommand.Flags().BoolVar(&config.filesOnly, "files-only", false, "Show only matched file paths")
	searchCommand.Flags().StringArrayVar(&config.pathQueries, "path", []string{}, "Filter results by metadata path (repeatable)")
	searchCommand.Flags().StringArrayVar(&config.extensionQueries, "ext", []string{}, "Filter results by file extension (repeatable). Accepts go or .go")
	searchCommand.Flags().IntVar(&config.from, "from", 0, "Skip the first N ranked files")
	searchCommand.Flags().IntVar(&config.size, "size", cfg.Size, "Limit results to top N files")
	searchCommand.Flags().StringVar(&config.operator, "operator", cfg.Operator, "Boolean operator for multi-term queries: AND|OR")
	searchCommand.Flags().StringVar(&config.relaxation, "relaxation", cfg.Relaxation, "Relax AND query with trailing-term fallback. Format: >N")
	searchCommand.Flags().Float64Var(&config.popularityWeight, "popularity-weight", runner.config.BM25.PopularityWeight, "Boost weight for files frequently read via 'idx read' (0 disables, default 0.3)")
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

func validateSearchFlagValues(contextLines, from, size int, sizeChanged bool) error {
	if contextLines < 0 {
		return fmt.Errorf("invalid --context value %d: expected a non-negative integer", contextLines)
	}

	if from < 0 {
		return fmt.Errorf("invalid --from value %d: expected a non-negative integer", from)
	}

	if size < 0 || (size == 0 && sizeChanged) {
		return fmt.Errorf("invalid --size value %d: expected a positive integer", size)
	}

	return nil
}

func validateSearchFormat(format string, prettyJSON bool) error {
	if format != search.OutputText && format != search.OutputJSON {
		return fmt.Errorf("unsupported --format value %q: expected one of [%s %s]", format, search.OutputText, search.OutputJSON)
	}

	if prettyJSON && format != search.OutputJSON {
		return fmt.Errorf("--json-pretty requires --format %s: got format %q", search.OutputJSON, format)
	}

	return nil
}

func validateSearchOperator(operator string) error {
	if operator != search.OperatorAND && operator != search.OperatorOR {
		return fmt.Errorf("unsupported --operator value %q: expected one of [%s %s]", operator, search.OperatorAND, search.OperatorOR)
	}

	return nil
}

func validateSearchRelaxation(config *searchCommandConfig) error {
	config.relaxationEnabled = false
	config.relaxationMin = 0

	if strings.TrimSpace(config.relaxation) == "" {
		return nil
	}

	if config.operator != search.OperatorAND {
		return fmt.Errorf("invalid --relaxation with --operator %q: expected %q", config.operator, search.OperatorAND)
	}

	value := strings.TrimSpace(config.relaxation)
	if !strings.HasPrefix(value, ">") || len(value) == 1 {
		return fmt.Errorf(errMsgRelaxationFormat, config.relaxation)
	}

	parsed, err := strconv.Atoi(value[1:])
	if err != nil || parsed < 0 {
		return fmt.Errorf(errMsgRelaxationFormat, config.relaxation)
	}

	config.relaxationEnabled = true
	config.relaxationMin = parsed
	return nil
}

func validateSearchInput(query string, pathQueries, extensionQueries, arguments []string) error {
	if query == "" && len(pathQueries) == 0 && len(extensionQueries) == 0 {
		return fmt.Errorf("missing search query: got %v, expected idx search <terms>", arguments)
	}

	return nil
}

func (config searchCommandConfig) options() search.Options {
	options := search.Options{
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
		PopularityWeight:       config.popularityWeight,
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
	usage := searchErrorActionStyle.Render(searchCmdName) + " " + searchErrorPathStyle.Render("<terms>")
	examples := []string{
		searchErrorMutedStyle.Render(searchCmdName) + " " + searchErrorPathStyle.Render("error handling"),
		searchErrorMutedStyle.Render(searchCmdName) + " " + searchErrorPathStyle.Render("--ext go func main"),
		searchErrorMutedStyle.Render(searchCmdName) + " " + searchErrorPathStyle.Render("--path internal logger"),
	}

	msg := fmt.Sprintf("\n%s\n\n  Usage:  %s\n\n  Examples:\n    %s\n",
		searchErrorWarningStyle.Render("⚠  Missing search query"),
		usage,
		strings.Join(examples, "\n    "),
	)

	_, _ = fmt.Fprintln(cmd.OutOrStdout(), msg)
}
