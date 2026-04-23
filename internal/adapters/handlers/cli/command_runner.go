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
			if index+1 >= len(arguments) {
				return "", options, fmt.Errorf("missing --format value: got %q, expected one of [%s %s]", argument, ports.SearchOutputText, ports.SearchOutputJSON)
			}

			selectedFormat := arguments[index+1]
			if selectedFormat != ports.SearchOutputText && selectedFormat != ports.SearchOutputJSON {
				return "", options, fmt.Errorf("unsupported --format value %q: expected one of [%s %s]", selectedFormat, ports.SearchOutputText, ports.SearchOutputJSON)
			}

			options.Format = selectedFormat
			index++
		case "--context":
			if index+1 >= len(arguments) {
				return "", options, fmt.Errorf("missing --context value: got %q, expected a non-negative integer", argument)
			}

			contextValue := arguments[index+1]
			parsedContext, err := strconv.Atoi(contextValue)
			if err != nil || parsedContext < 0 {
				return "", options, fmt.Errorf("invalid --context value %q: expected a non-negative integer", contextValue)
			}

			options.Context = parsedContext
			index++
		case "--json-pretty":
			options.PrettyJSON = true
		default:
			if strings.HasPrefix(argument, "--") {
				return "", options, fmt.Errorf("unsupported search option %q: expected --format <text|json>, --context <n>, or --json-pretty", argument)
			}

			queryTerms = append(queryTerms, argument)
		}
	}

	if options.PrettyJSON && options.Format != ports.SearchOutputJSON {
		return "", options, fmt.Errorf("--json-pretty requires --format %s: got format %q", ports.SearchOutputJSON, options.Format)
	}

	return strings.Join(queryTerms, " "), options, nil
}
