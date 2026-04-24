package cli

import (
	"fmt"
	"strconv"
	"strings"

	"idx/internal/core/ports"
)

type runnableCommand interface {
	Run() error
}

type indexableCommand interface {
	Run() error
	Sync() error
	Inspect(indexPath string) error
}

type searchableCommand interface {
	Run(query string) error
	RunWithOptions(query string, options ports.SearchOptions) error
}

type CommandRunner struct {
	arguments      []string
	indexCommand   indexableCommand
	destroyCommand runnableCommand
	searchCommand  searchableCommand
}

// NewCommandRunner wires CLI arguments to command execution.
// Example: runner := NewCommandRunner(os.Args, initCommand, destroyCommand, searchCommand).
func NewCommandRunner(arguments []string, indexCommand indexableCommand, destroyCommand runnableCommand, searchCommand searchableCommand) CommandRunner {
	return CommandRunner{
		arguments:      arguments,
		indexCommand:   indexCommand,
		destroyCommand: destroyCommand,
		searchCommand:  searchCommand,
	}
}

// Run dispatches the CLI command based on the first argument.
// Example: err := runner.Run().
func (runner CommandRunner) Run() error {
	if len(runner.arguments) < 2 {
		return fmt.Errorf("missing command: got %v, expected one of [sync init inspect destroy search]", runner.arguments)
	}

	switch runner.arguments[1] {
	case "sync":
		return runner.indexCommand.Sync()
	case "init":
		return runner.indexCommand.Run()
	case "inspect":
		return runner.runInspect()
	case "destroy":
		return runner.destroyCommand.Run()
	case "search":
		return runner.runSearch()
	default:
		return fmt.Errorf("unsupported command %q: expected one of [sync init inspect destroy search]", runner.arguments[1])
	}
}

func (runner CommandRunner) runInspect() error {
	inspectPath, err := parseInspectArguments(runner.arguments[2:])
	if err != nil {
		return err
	}

	return runner.indexCommand.Inspect(inspectPath)
}

func (runner CommandRunner) runSearch() error {
	if len(runner.arguments) < 3 {
		return fmt.Errorf("missing search query: got %v, expected idx search <terms>", runner.arguments)
	}

	query, options, err := parseSearchArguments(runner.arguments[2:])
	if err != nil {
		return err
	}

	if !hasSearchInput(query, options) {
		return fmt.Errorf("missing search query: got %v, expected idx search <terms>", runner.arguments)
	}

	return runner.searchCommand.RunWithOptions(query, options)
}

func parseSearchArguments(arguments []string) (string, ports.SearchOptions, error) {
	queryTerms := make([]string, 0, len(arguments))
	options := ports.SearchOptions{Format: ports.SearchOutputText}

	for index := 0; index < len(arguments); index++ {
		argument := arguments[index]
		switch argument {
		case "--format":
			selectedFormat, err := parseFormatOption(arguments, index)
			if err != nil {
				return "", options, err
			}

			options.Format = selectedFormat
			index++
		case "--context":
			parsedContext, err := parseContextOption(arguments, index)
			if err != nil {
				return "", options, err
			}

			options.Context = parsedContext
			index++
		case "--json-pretty":
			options.PrettyJSON = true
		case "--matches-only", "--macthes-only":
			options.MatchesOnly = true
		case "--files-only":
			options.FilesOnly = true
		case "--file":
			fileQuery, err := parseTextOption(arguments, index, argument)
			if err != nil {
				return "", options, err
			}

			options.FileQuery = fileQuery
			index++
		case "--path":
			pathQuery, err := parseTextOption(arguments, index, argument)
			if err != nil {
				return "", options, err
			}

			options.PathQuery = pathQuery
			index++
		case "--limit":
			parsedLimit, err := parseLimitOption(arguments, index)
			if err != nil {
				return "", options, err
			}

			options.Limit = parsedLimit
			index++
		default:
			if err := validateSearchOption(argument); err != nil {
				return "", options, err
			}

			queryTerms = append(queryTerms, argument)
		}
	}

	if err := validatePrettyJSONOption(options); err != nil {
		return "", options, err
	}

	return strings.Join(queryTerms, " "), options, nil
}

func parseInspectArguments(arguments []string) (string, error) {
	if len(arguments) != 1 {
		return "", fmt.Errorf("inspect requires exactly one path: got %v, expected idx inspect <path>", arguments)
	}

	inspectPath := strings.TrimSpace(arguments[0])
	if inspectPath == "" || strings.HasPrefix(inspectPath, "--") {
		return "", fmt.Errorf("invalid inspect path %q: expected idx inspect <path>", arguments[0])
	}

	return inspectPath, nil
}

func parseFormatOption(arguments []string, index int) (string, error) {
	if index+1 >= len(arguments) {
		return "", fmt.Errorf("missing --format value: got %q, expected one of [%s %s]", arguments[index], ports.SearchOutputText, ports.SearchOutputJSON)
	}

	selectedFormat := arguments[index+1]
	if selectedFormat != ports.SearchOutputText && selectedFormat != ports.SearchOutputJSON {
		return "", fmt.Errorf("unsupported --format value %q: expected one of [%s %s]", selectedFormat, ports.SearchOutputText, ports.SearchOutputJSON)
	}

	return selectedFormat, nil
}

func parseContextOption(arguments []string, index int) (int, error) {
	if index+1 >= len(arguments) {
		return 0, fmt.Errorf("missing --context value: got %q, expected a non-negative integer", arguments[index])
	}

	contextValue := arguments[index+1]
	parsedContext, err := strconv.Atoi(contextValue)
	if err != nil || parsedContext < 0 {
		return 0, fmt.Errorf("invalid --context value %q: expected a non-negative integer", contextValue)
	}

	return parsedContext, nil
}

func parseLimitOption(arguments []string, index int) (int, error) {
	if index+1 >= len(arguments) {
		return 0, fmt.Errorf("missing --limit value: got %q, expected a positive integer", arguments[index])
	}

	limitValue := arguments[index+1]
	parsedLimit, err := strconv.Atoi(limitValue)
	if err != nil || parsedLimit <= 0 {
		return 0, fmt.Errorf("invalid --limit value %q: expected a positive integer", limitValue)
	}

	return parsedLimit, nil
}

func parseTextOption(arguments []string, index int, option string) (string, error) {
	if index+1 >= len(arguments) {
		return "", fmt.Errorf("missing %s value: got %q, expected non-empty text", option, arguments[index])
	}

	value := strings.TrimSpace(arguments[index+1])
	if value == "" || strings.HasPrefix(value, "--") {
		return "", fmt.Errorf("invalid %s value %q: expected non-empty text", option, arguments[index+1])
	}

	return value, nil
}

func validateSearchOption(argument string) error {
	if strings.HasPrefix(argument, "--") {
		return fmt.Errorf("unsupported search option %q: expected --format <text|json>, --json-pretty, --context <n>, --matches-only, --files-only, --file <text>, --path <text>, or --limit <n>", argument)
	}

	return nil
}

func hasSearchInput(query string, options ports.SearchOptions) bool {
	return query != "" || options.FileQuery != "" || options.PathQuery != ""
}

func validatePrettyJSONOption(options ports.SearchOptions) error {
	if options.PrettyJSON && options.Format != ports.SearchOutputJSON {
		return fmt.Errorf("--json-pretty requires --format %s: got format %q", ports.SearchOutputJSON, options.Format)
	}

	return nil
}
