package cli

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/spf13/cobra"

	"idx/internal/features/search"
)

const (
	searchCmdName        = "idx search"
	flagNameAgentCompact = "agent-compact"
	flagNameJSONPretty   = "json-pretty"
)

const searchLongDescription = `Search indexed files using BM25 full-text ranking.

Output flags:
  -j, --json           Output as JSON
      --pretty         Pretty-print JSON
  -c, --context N      Show N context lines around each match
  -l, --files-only     List only file paths
      --compact        Compact output for AI agents (fewer tokens)
      --count          Print only the number of matching files
      --explain        Include BM25 score per result
      --time           Show query execution time

Filtering flags:
  -e, --ext .go        Filter by file extension (repeatable)
  -p, --path internal  Filter by path prefix (repeatable)

Ranking flags:
      --any            Match any term (OR mode)
      --operator AND   Boolean mode: AND (default) or OR
      --relax N        Relax AND: require at least N terms to match
      --popularity-weight 0.3  Boost files read via 'idx read'

Pagination:
      --skip N         Skip first N results
  -n, --limit N        Show at most N results

Examples:
  idx search "error handling"
  idx find -e go -e ts "func main"
  idx search -p internal logger --any
  idx search -j --pretty "config"
  idx search --count "TODO"
  idx search --relax 2 "init sync destroy context"
  idx search -l -e md "installation"
  idx search --time --explain "BM25 scoring"`

func (runner CommandRunner) newSearchCommand() *cobra.Command {
	config := &searchCommandConfig{}
	var searchCommand *cobra.Command
	searchCommand = &cobra.Command{
		Use:     "search [query terms]",
		Aliases: []string{"find"},
		Short:   "Search indexed content",
		Long:    searchLongDescription,
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
	pretty            bool
	jsonShorthand     bool
	explain           bool
	agentCompact      bool
	compact           bool
	filesOnly         bool
	countOnly         bool
	timing            bool
	pathQueries       []string
	extensionQueries  []string
	from              int
	skip              int
	size              int
	limit             int
	operator          string
	anyMode           bool
	relaxation        int
	relaxInt          int
	relaxIntSet       bool
	relaxationMin     int
	relaxationEnabled bool
	popularityWeight  float64
}

func (runner CommandRunner) runSearchCommand(searchCommand *cobra.Command, args []string, config *searchCommandConfig) error {
	applySearchAliasFlags(config)
	config.relaxIntSet = searchCommand.Flags().Changed("relax")
	// --size is deprecated; when explicitly set, forward its value to --limit so that
	// options() can use a single field. Only override when --limit was not also set.
	if searchCommand.Flags().Changed("size") && !searchCommand.Flags().Changed("limit") {
		config.limit = config.size
	}

	if err := validateSearchConfig(config, searchCommand.Flags().Changed("size"), searchCommand.Flags().Changed("limit")); err != nil {
		return err
	}

	query := strings.Join(args, " ")
	if err := validateSearchInput(query, config.pathQueries, config.extensionQueries, runner.arguments); err != nil {
		writeSearchMissingQueryError(searchCommand)
		return fmt.Errorf("")
	}

	return runner.searchCommand.RunWithOptions(query, config.options())
}

// applySearchAliasFlags resolves alias flags into their canonical counterparts before validation.
func applySearchAliasFlags(config *searchCommandConfig) {
	if config.jsonShorthand {
		config.format = search.OutputJSON
	}
	if config.anyMode {
		config.operator = search.OperatorOR
	}
}

func (runner CommandRunner) configureSearchFlags(searchCommand *cobra.Command, config *searchCommandConfig) {
	cfg := runner.config.Search

	searchCommand.Flags().StringVar(&config.format, "format", cfg.Format, "Output format: text|json")
	searchCommand.Flags().BoolVarP(&config.jsonShorthand, "json", "j", false, "Output results as JSON (shorthand for --format json)")
	searchCommand.Flags().IntVarP(&config.contextLines, "context", "c", cfg.Context, "Number of context lines around matches")
	searchCommand.Flags().BoolVar(&config.pretty, "pretty", false, "Pretty-print JSON output (requires --json or --format json)")
	searchCommand.Flags().BoolVar(&config.explain, "explain", false, "Include ranking metadata such as score")
	searchCommand.Flags().BoolVar(&config.compact, "compact", false, "Compact output with fewer tokens (good for AI agents)")
	searchCommand.Flags().BoolVarP(&config.filesOnly, "files-only", "l", false, "Show only matched file paths")
	searchCommand.Flags().BoolVar(&config.countOnly, "count", false, "Print only the number of matching files")
	searchCommand.Flags().BoolVar(&config.timing, "time", false, "Show query execution time")
	searchCommand.Flags().StringArrayVarP(&config.pathQueries, "path", "p", []string{}, "Filter results by metadata path (repeatable)")
	searchCommand.Flags().StringArrayVarP(&config.extensionQueries, "ext", "e", []string{}, "Filter results by file extension (repeatable). Accepts go or .go")
	searchCommand.Flags().IntVar(&config.skip, "skip", 0, "Skip the first N ranked results")
	searchCommand.Flags().IntVarP(&config.limit, "limit", "n", cfg.Limit, "Limit results to top N files")
	searchCommand.Flags().StringVar(&config.operator, "operator", cfg.Operator, "Boolean operator for multi-term queries: AND|OR")
	searchCommand.Flags().BoolVar(&config.anyMode, "any", false, "Match files containing ANY query term (shorthand for --operator OR)")
	searchCommand.Flags().IntVar(&config.relaxInt, "relax", 0, "Relax AND: require at least N matching terms (e.g. --relax 2)")
	config.relaxation = cfg.Relaxation
	searchCommand.Flags().Float64Var(&config.popularityWeight, "popularity-weight", runner.config.BM25.PopularityWeight, "Boost weight for files frequently read via 'idx read' (0 disables, default 0.3)")

	// Deprecated aliases kept for backward compatibility.
	searchCommand.Flags().BoolVar(&config.agentCompact, flagNameAgentCompact, false, "")
	_ = searchCommand.Flags().MarkHidden(flagNameAgentCompact)
	_ = searchCommand.Flags().MarkDeprecated(flagNameAgentCompact, "use --compact instead")
	searchCommand.Flags().BoolVar(&config.prettyJSON, flagNameJSONPretty, false, "")
	_ = searchCommand.Flags().MarkHidden(flagNameJSONPretty)
	_ = searchCommand.Flags().MarkDeprecated(flagNameJSONPretty, "use --pretty instead")
	searchCommand.Flags().IntVar(&config.size, "size", cfg.Limit, "")
	_ = searchCommand.Flags().MarkHidden("size")
	_ = searchCommand.Flags().MarkDeprecated("size", "use --limit/-n instead")
	searchCommand.Flags().IntVar(&config.from, "from", 0, "")
	_ = searchCommand.Flags().MarkHidden("from")
	_ = searchCommand.Flags().MarkDeprecated("from", "use --skip instead")
}

func validateSearchConfig(config *searchCommandConfig, sizeChanged, limitChanged bool) error {
	if err := validateSearchFlagValues(config.contextLines, config.skip, config.from, config.size, config.limit, sizeChanged, limitChanged); err != nil {
		return err
	}

	prettyActive := config.prettyJSON || config.pretty
	if err := validateSearchFormat(config.format, prettyActive); err != nil {
		return err
	}

	if err := validateSearchOperator(config.operator); err != nil {
		return err
	}

	return validateSearchRelaxation(config)
}

func validateSearchFlagValues(contextLines, skip, from, size, limit int, sizeChanged, limitChanged bool) error {
	if contextLines < 0 {
		return fmt.Errorf("invalid --context value %d: expected a non-negative integer", contextLines)
	}

	if skip < 0 {
		return fmt.Errorf("invalid --skip value %d: expected a non-negative integer", skip)
	}

	if from < 0 {
		return fmt.Errorf("invalid --from value %d: expected a non-negative integer", from)
	}

	if size < 0 || (size == 0 && sizeChanged) {
		return fmt.Errorf("invalid --size value %d: expected a positive integer", size)
	}

	if limit < 0 || (limit == 0 && limitChanged) {
		return fmt.Errorf("invalid --limit value %d: expected a positive integer", limit)
	}

	return nil
}

func validateSearchFormat(format string, prettyActive bool) error {
	if format != search.OutputText && format != search.OutputJSON {
		return fmt.Errorf("unsupported --format value %q: expected one of [%s %s]", format, search.OutputText, search.OutputJSON)
	}

	if prettyActive && format != search.OutputJSON {
		return fmt.Errorf("--pretty requires --format %s (or -j): got format %q", search.OutputJSON, format)
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

	if config.relaxIntSet {
		if config.operator != search.OperatorAND {
			return fmt.Errorf("--relax requires --operator %s, got %q", search.OperatorAND, config.operator)
		}
		config.relaxationEnabled = true
		config.relaxationMin = config.relaxInt
		return nil
	}

	if config.relaxation == 0 {
		return nil
	}

	if config.operator != search.OperatorAND {
		return fmt.Errorf("invalid search.relaxation with --operator %q: expected %q", config.operator, search.OperatorAND)
	}

	config.relaxationEnabled = true
	config.relaxationMin = config.relaxation
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
		Format:           config.format,
		Context:          config.contextLines,
		PrettyJSON:       config.prettyJSON || config.pretty,
		Explain:          config.explain,
		AgentCompact:     config.agentCompact || config.compact,
		FilesOnly:        config.filesOnly || config.countOnly,
		CountOnly:        config.countOnly,
		Timing:           config.timing,
		PathQueries:      config.pathQueries,
		ExtensionQueries: config.extensionQueries,
		From:             config.from + config.skip,
		// runSearchCommand forwards --size into config.limit before this is called.
		Size:                   config.limit,
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
		searchErrorMutedStyle.Render(searchCmdName) + " " + searchErrorPathStyle.Render("-e go func main"),
		searchErrorMutedStyle.Render(searchCmdName) + " " + searchErrorPathStyle.Render("-p internal logger"),
	}

	msg := fmt.Sprintf("\n%s\n\n  Usage:  %s\n\n  Examples:\n    %s\n",
		searchErrorWarningStyle.Render("⚠  Missing search query"),
		usage,
		strings.Join(examples, "\n    "),
	)

	_, _ = fmt.Fprintln(cmd.OutOrStdout(), msg)
}
