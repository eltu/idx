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

type searchableCommand interface {
	Run(query string) error
	RunWithOptions(query string, options ports.SearchOptions) error
}

type CommandRunner struct {
	arguments      []string
	initCommand    runnableCommand
	destroyCommand runnableCommand
	searchCommand  searchableCommand
}

// NewCommandRunner wires CLI arguments to command execution.
// Example: runner := NewCommandRunner(os.Args, initCommand, destroyCommand, searchCommand)
func NewCommandRunner(arguments []string, initCommand runnableCommand, destroyCommand runnableCommand, searchCommand searchableCommand) CommandRunner {
	return CommandRunner{
		arguments:      arguments,
		initCommand:    initCommand,
		destroyCommand: destroyCommand,
		searchCommand:  searchCommand,
	}
}

// Run dispatches the CLI command based on the first argument.
// Example: err := runner.Run()
func (runner CommandRunner) Run() error {
	if len(runner.arguments) < 2 {
		return fmt.Errorf("missing command: got %v, expected one of [init destroy search]", runner.arguments)
	}

	switch runner.arguments[1] {
	case "init":
		return runner.initCommand.Run()
	case "destroy":
		return runner.destroyCommand.Run()
	case "search":
		return runner.runSearch()
	default:
		return fmt.Errorf("unsupported command %q: expected one of [init destroy search]", runner.arguments[1])
	}
}

func (runner CommandRunner) runSearch() error {
	if len(runner.arguments) < 3 {
		return fmt.Errorf("missing search query: got %v, expected idx search <terms>", runner.arguments)
	}

	query, options, err := parseSearchArguments(runner.arguments[2:])
	if err != nil {
		return err
	}

	if query == "" {
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

func validateSearchOption(argument string) error {
	if strings.HasPrefix(argument, "--") {
		return fmt.Errorf("unsupported search option %q: expected --format <text|json>, --context <n>, or --json-pretty", argument)
	}

	return nil
}

func validatePrettyJSONOption(options ports.SearchOptions) error {
	if options.PrettyJSON && options.Format != ports.SearchOutputJSON {
		return fmt.Errorf("--json-pretty requires --format %s: got format %q", ports.SearchOutputJSON, options.Format)
	}

	return nil
}
