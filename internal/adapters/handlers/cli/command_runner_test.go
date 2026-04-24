package cli_test

import (
	"testing"

	"idx/internal/adapters/handlers/cli"
	"idx/internal/core/ports"
)

type fakeInitCommand struct {
	runCalls        int
	syncCalls       int
	inspectCalls    int
	lastInspectPath string
}

func (command *fakeInitCommand) Run() error {
	command.runCalls++
	return nil
}

func (command *fakeInitCommand) Sync() error {
	command.syncCalls++
	return nil
}

func (command *fakeInitCommand) Inspect(indexPath string) error {
	command.inspectCalls++
	command.lastInspectPath = indexPath
	return nil
}

type fakeDestroyCommand struct {
	runCalls int
}

func (command *fakeDestroyCommand) Run() error {
	command.runCalls++
	return nil
}

type fakeSearchCommand struct {
	runCalls    int
	lastQuery   string
	lastOptions ports.SearchOptions
}

func (command *fakeSearchCommand) Run(query string) error {
	command.runCalls++
	command.lastQuery = query
	return nil
}

func (command *fakeSearchCommand) RunWithOptions(query string, options ports.SearchOptions) error {
	command.runCalls++
	command.lastQuery = query
	command.lastOptions = options
	return nil
}

func TestCommandRunnerRunExecutesInitCommand(t *testing.T) {
	initCommand := &fakeInitCommand{}
	destroyCommand := &fakeDestroyCommand{}
	searchCommand := &fakeSearchCommand{}
	runner := cli.NewCommandRunner([]string{"idx", "init"}, initCommand, destroyCommand, searchCommand)

	err := runner.Run()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if initCommand.runCalls != 1 {
		t.Fatalf("expected 1 run call, got %d", initCommand.runCalls)
	}

	if initCommand.inspectCalls != 0 {
		t.Fatalf("expected 0 inspect calls, got %d", initCommand.inspectCalls)
	}

	if destroyCommand.runCalls != 0 {
		t.Fatalf("expected 0 destroy calls, got %d", destroyCommand.runCalls)
	}

	if searchCommand.runCalls != 0 {
		t.Fatalf("expected 0 search calls, got %d", searchCommand.runCalls)
	}
}

func TestCommandRunnerRunExecutesSyncCommand(t *testing.T) {
	initCommand := &fakeInitCommand{}
	destroyCommand := &fakeDestroyCommand{}
	searchCommand := &fakeSearchCommand{}
	runner := cli.NewCommandRunner([]string{"idx", "sync"}, initCommand, destroyCommand, searchCommand)

	err := runner.Run()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if initCommand.syncCalls != 1 {
		t.Fatalf("expected 1 sync call, got %d", initCommand.syncCalls)
	}

	if initCommand.runCalls != 0 {
		t.Fatalf("expected 0 init calls, got %d", initCommand.runCalls)
	}
}

func TestCommandRunnerRunExecutesInspectCommandWithPath(t *testing.T) {
	initCommand := &fakeInitCommand{}
	destroyCommand := &fakeDestroyCommand{}
	searchCommand := &fakeSearchCommand{}
	runner := cli.NewCommandRunner([]string{"idx", "inspect", "internal/"}, initCommand, destroyCommand, searchCommand)

	err := runner.Run()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if initCommand.inspectCalls != 1 {
		t.Fatalf("expected 1 inspect call, got %d", initCommand.inspectCalls)
	}

	if initCommand.lastInspectPath != "internal/" {
		t.Fatalf("expected inspect path %q, got %q", "internal/", initCommand.lastInspectPath)
	}
}

func TestCommandRunnerRunRejectsInspectWithoutPath(t *testing.T) {
	initCommand := &fakeInitCommand{}
	destroyCommand := &fakeDestroyCommand{}
	searchCommand := &fakeSearchCommand{}
	runner := cli.NewCommandRunner([]string{"idx", "inspect"}, initCommand, destroyCommand, searchCommand)

	err := runner.Run()
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if initCommand.inspectCalls != 0 {
		t.Fatalf("expected 0 inspect calls, got %d", initCommand.inspectCalls)
	}

	if initCommand.syncCalls != 0 {
		t.Fatalf("expected 0 sync calls, got %d", initCommand.syncCalls)
	}
}

func TestCommandRunnerRunRejectsLegacyIndexCommand(t *testing.T) {
	initCommand := &fakeInitCommand{}
	destroyCommand := &fakeDestroyCommand{}
	searchCommand := &fakeSearchCommand{}
	runner := cli.NewCommandRunner([]string{"idx", "index"}, initCommand, destroyCommand, searchCommand)

	err := runner.Run()
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if initCommand.runCalls != 0 {
		t.Fatalf("expected 0 init calls, got %d", initCommand.runCalls)
	}

	if initCommand.syncCalls != 0 {
		t.Fatalf("expected 0 sync calls, got %d", initCommand.syncCalls)
	}
}

func TestCommandRunnerRunExecutesDestroyCommand(t *testing.T) {
	initCommand := &fakeInitCommand{}
	destroyCommand := &fakeDestroyCommand{}
	searchCommand := &fakeSearchCommand{}
	runner := cli.NewCommandRunner([]string{"idx", "destroy"}, initCommand, destroyCommand, searchCommand)

	err := runner.Run()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if destroyCommand.runCalls != 1 {
		t.Fatalf("expected 1 destroy call, got %d", destroyCommand.runCalls)
	}

	if initCommand.runCalls != 0 {
		t.Fatalf("expected 0 init calls, got %d", initCommand.runCalls)
	}

	if searchCommand.runCalls != 0 {
		t.Fatalf("expected 0 search calls, got %d", searchCommand.runCalls)
	}
}

func TestCommandRunnerRunExecutesSearchCommand(t *testing.T) {
	initCommand := &fakeInitCommand{}
	destroyCommand := &fakeDestroyCommand{}
	searchCommand := &fakeSearchCommand{}
	runner := cli.NewCommandRunner([]string{"idx", "search", "needle", "term"}, initCommand, destroyCommand, searchCommand)

	err := runner.Run()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if searchCommand.runCalls != 1 {
		t.Fatalf("expected 1 search call, got %d", searchCommand.runCalls)
	}

	if searchCommand.lastQuery != "needle term" {
		t.Fatalf("expected query %q, got %q", "needle term", searchCommand.lastQuery)
	}

	if searchCommand.lastOptions.Format != ports.SearchOutputText {
		t.Fatalf("expected default format %q, got %q", ports.SearchOutputText, searchCommand.lastOptions.Format)
	}

	if searchCommand.lastOptions.Context != 0 {
		t.Fatalf("expected default context 0, got %d", searchCommand.lastOptions.Context)
	}

	if searchCommand.lastOptions.PrettyJSON {
		t.Fatal("expected default PrettyJSON false")
	}

	if searchCommand.lastOptions.MatchesOnly {
		t.Fatal("expected default MatchesOnly false")
	}

	if searchCommand.lastOptions.Limit != 0 {
		t.Fatalf("expected default limit 0, got %d", searchCommand.lastOptions.Limit)
	}
}

func TestCommandRunnerRunExecutesSearchCommandWithOptions(t *testing.T) {
	initCommand := &fakeInitCommand{}
	destroyCommand := &fakeDestroyCommand{}
	searchCommand := &fakeSearchCommand{}
	runner := cli.NewCommandRunner([]string{"idx", "search", "--format", "json", "--context", "2", "--json-pretty", "--matches-only", "--limit", "1", "needle", "term"}, initCommand, destroyCommand, searchCommand)

	err := runner.Run()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if searchCommand.runCalls != 1 {
		t.Fatalf("expected 1 search call, got %d", searchCommand.runCalls)
	}

	if searchCommand.lastQuery != "needle term" {
		t.Fatalf("expected query %q, got %q", "needle term", searchCommand.lastQuery)
	}

	if searchCommand.lastOptions.Format != ports.SearchOutputJSON {
		t.Fatalf("expected format %q, got %q", ports.SearchOutputJSON, searchCommand.lastOptions.Format)
	}

	if searchCommand.lastOptions.Context != 2 {
		t.Fatalf("expected context 2, got %d", searchCommand.lastOptions.Context)
	}

	if !searchCommand.lastOptions.PrettyJSON {
		t.Fatal("expected PrettyJSON true")
	}

	if !searchCommand.lastOptions.MatchesOnly {
		t.Fatal("expected MatchesOnly true")
	}

	if searchCommand.lastOptions.Limit != 1 {
		t.Fatalf("expected limit 1, got %d", searchCommand.lastOptions.Limit)
	}
}

func TestCommandRunnerRunAcceptsLegacyTypoMatchesOnlyOption(t *testing.T) {
	initCommand := &fakeInitCommand{}
	destroyCommand := &fakeDestroyCommand{}
	searchCommand := &fakeSearchCommand{}
	runner := cli.NewCommandRunner([]string{"idx", "search", "--macthes-only", "needle"}, initCommand, destroyCommand, searchCommand)

	err := runner.Run()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if !searchCommand.lastOptions.MatchesOnly {
		t.Fatal("expected MatchesOnly true")
	}
}

func TestCommandRunnerRunParsesFilesOnlyOption(t *testing.T) {
	initCommand := &fakeInitCommand{}
	destroyCommand := &fakeDestroyCommand{}
	searchCommand := &fakeSearchCommand{}
	runner := cli.NewCommandRunner([]string{"idx", "search", "--files-only", "needle"}, initCommand, destroyCommand, searchCommand)

	err := runner.Run()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if !searchCommand.lastOptions.FilesOnly {
		t.Fatal("expected FilesOnly true")
	}
}

func TestCommandRunnerRunParsesFileAndPathFilters(t *testing.T) {
	initCommand := &fakeInitCommand{}
	destroyCommand := &fakeDestroyCommand{}
	searchCommand := &fakeSearchCommand{}
	runner := cli.NewCommandRunner([]string{"idx", "search", "needle", "--file", "go.mod", "--path", "internal/core"}, initCommand, destroyCommand, searchCommand)

	err := runner.Run()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if searchCommand.lastOptions.FileQuery != "go.mod" {
		t.Fatalf("expected file filter %q, got %q", "go.mod", searchCommand.lastOptions.FileQuery)
	}

	if searchCommand.lastOptions.PathQuery != "internal/core" {
		t.Fatalf("expected path filter %q, got %q", "internal/core", searchCommand.lastOptions.PathQuery)
	}

	if len(searchCommand.lastOptions.FileQueries) != 1 || searchCommand.lastOptions.FileQueries[0] != "go.mod" {
		t.Fatalf("expected file queries [go.mod], got %v", searchCommand.lastOptions.FileQueries)
	}

	if len(searchCommand.lastOptions.PathQueries) != 1 || searchCommand.lastOptions.PathQueries[0] != "internal/core" {
		t.Fatalf("expected path queries [internal/core], got %v", searchCommand.lastOptions.PathQueries)
	}
}

func TestCommandRunnerRunAcceptsMetadataOnlySearch(t *testing.T) {
	initCommand := &fakeInitCommand{}
	destroyCommand := &fakeDestroyCommand{}
	searchCommand := &fakeSearchCommand{}
	runner := cli.NewCommandRunner([]string{"idx", "search", "--file", "go.mod"}, initCommand, destroyCommand, searchCommand)

	err := runner.Run()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if searchCommand.lastQuery != "" {
		t.Fatalf("expected empty content query, got %q", searchCommand.lastQuery)
	}
}

func TestCommandRunnerRunTreatsLeadingReservedWordAsLiteralQueryWhenOptionIsInvalid(t *testing.T) {
	initCommand := &fakeInitCommand{}
	destroyCommand := &fakeDestroyCommand{}
	searchCommand := &fakeSearchCommand{}
	runner := cli.NewCommandRunner([]string{"idx", "search", "--file", "--matches-only", "--format", "json"}, initCommand, destroyCommand, searchCommand)

	err := runner.Run()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if searchCommand.lastQuery != "--file" {
		t.Fatalf("expected literal query %q, got %q", "--file", searchCommand.lastQuery)
	}

	if !searchCommand.lastOptions.MatchesOnly {
		t.Fatal("expected MatchesOnly true")
	}

	if searchCommand.lastOptions.Format != ports.SearchOutputJSON {
		t.Fatalf("expected format %q, got %q", ports.SearchOutputJSON, searchCommand.lastOptions.Format)
	}
}

func TestCommandRunnerRunTreatsReservedPhraseAsLiteralQuery(t *testing.T) {
	initCommand := &fakeInitCommand{}
	destroyCommand := &fakeDestroyCommand{}
	searchCommand := &fakeSearchCommand{}
	runner := cli.NewCommandRunner([]string{"idx", "search", "--file --path", "--matches-only"}, initCommand, destroyCommand, searchCommand)

	err := runner.Run()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if searchCommand.lastQuery != "--file --path" {
		t.Fatalf("expected literal phrase query %q, got %q", "--file --path", searchCommand.lastQuery)
	}

	if !searchCommand.lastOptions.MatchesOnly {
		t.Fatal("expected MatchesOnly true")
	}
}

func TestCommandRunnerRunTreatsLeadingStandaloneFlagsAsLiteralQuery(t *testing.T) {
	testCases := []struct {
		name          string
		leadingToken  string
		arguments     []string
		expectedQuery string
	}{
		{name: "matches only", leadingToken: "--matches-only", arguments: []string{"idx", "search", "--matches-only", "--matches-only"}, expectedQuery: "--matches-only"},
		{name: "legacy matches typo", leadingToken: "--macthes-only", arguments: []string{"idx", "search", "--macthes-only", "--matches-only"}, expectedQuery: "--macthes-only"},
		{name: "files only", leadingToken: "--files-only", arguments: []string{"idx", "search", "--files-only", "--matches-only"}, expectedQuery: "--files-only"},
		{name: "json pretty", leadingToken: "--json-pretty", arguments: []string{"idx", "search", "--json-pretty", "--format", "json"}, expectedQuery: "--json-pretty"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			initCommand := &fakeInitCommand{}
			destroyCommand := &fakeDestroyCommand{}
			searchCommand := &fakeSearchCommand{}
			runner := cli.NewCommandRunner(testCase.arguments, initCommand, destroyCommand, searchCommand)

			err := runner.Run()
			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}

			if searchCommand.lastQuery != testCase.expectedQuery {
				t.Fatalf("expected literal query %q, got %q", testCase.expectedQuery, searchCommand.lastQuery)
			}
		})
	}
}

func TestCommandRunnerRunStillParsesLeadingPathFilter(t *testing.T) {
	initCommand := &fakeInitCommand{}
	destroyCommand := &fakeDestroyCommand{}
	searchCommand := &fakeSearchCommand{}
	runner := cli.NewCommandRunner([]string{"idx", "search", "--path", "internal/core", "--matches-only"}, initCommand, destroyCommand, searchCommand)

	err := runner.Run()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(searchCommand.lastOptions.PathQueries) != 1 || searchCommand.lastOptions.PathQueries[0] != "internal/core" {
		t.Fatalf("expected path filter [internal/core], got %v", searchCommand.lastOptions.PathQueries)
	}

	if searchCommand.lastQuery != "" {
		t.Fatalf("expected empty content query, got %q", searchCommand.lastQuery)
	}
}

func TestCommandRunnerRunParsesExpandedPathValuesWithoutQuotes(t *testing.T) {
	initCommand := &fakeInitCommand{}
	destroyCommand := &fakeDestroyCommand{}
	searchCommand := &fakeSearchCommand{}
	runner := cli.NewCommandRunner([]string{"idx", "search", "--path", "internal/core/services/search_command_service.go", "internal/core/ports/search_options.go"}, initCommand, destroyCommand, searchCommand)

	err := runner.Run()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if searchCommand.lastQuery != "" {
		t.Fatalf("expected empty query with metadata-only expanded values, got %q", searchCommand.lastQuery)
	}

	if len(searchCommand.lastOptions.PathQueries) != 2 {
		t.Fatalf("expected 2 path filters, got %v", searchCommand.lastOptions.PathQueries)
	}
}

func TestCommandRunnerRunRejectsSearchWithoutQuery(t *testing.T) {
	initCommand := &fakeInitCommand{}
	destroyCommand := &fakeDestroyCommand{}
	searchCommand := &fakeSearchCommand{}
	runner := cli.NewCommandRunner([]string{"idx", "search"}, initCommand, destroyCommand, searchCommand)

	err := runner.Run()
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if initCommand.runCalls != 0 {
		t.Fatalf("expected 0 init calls, got %d", initCommand.runCalls)
	}

	if destroyCommand.runCalls != 0 {
		t.Fatalf("expected 0 destroy calls, got %d", destroyCommand.runCalls)
	}

	if searchCommand.runCalls != 0 {
		t.Fatalf("expected 0 search calls, got %d", searchCommand.runCalls)
	}
}

func TestCommandRunnerRunTreatsInvalidLeadingFormatAsLiteralQuery(t *testing.T) {
	initCommand := &fakeInitCommand{}
	destroyCommand := &fakeDestroyCommand{}
	searchCommand := &fakeSearchCommand{}
	runner := cli.NewCommandRunner([]string{"idx", "search", "--format", "xml", "needle"}, initCommand, destroyCommand, searchCommand)

	err := runner.Run()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if searchCommand.lastQuery != "--format xml needle" {
		t.Fatalf("expected literal query %q, got %q", "--format xml needle", searchCommand.lastQuery)
	}
}

func TestCommandRunnerRunTreatsInvalidLeadingContextAsLiteralQuery(t *testing.T) {
	initCommand := &fakeInitCommand{}
	destroyCommand := &fakeDestroyCommand{}
	searchCommand := &fakeSearchCommand{}
	runner := cli.NewCommandRunner([]string{"idx", "search", "--context", "-1", "needle"}, initCommand, destroyCommand, searchCommand)

	err := runner.Run()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if searchCommand.lastQuery != "--context -1 needle" {
		t.Fatalf("expected literal query %q, got %q", "--context -1 needle", searchCommand.lastQuery)
	}
}

func TestCommandRunnerRunTreatsInvalidLeadingLimitAsLiteralQuery(t *testing.T) {
	initCommand := &fakeInitCommand{}
	destroyCommand := &fakeDestroyCommand{}
	searchCommand := &fakeSearchCommand{}
	runner := cli.NewCommandRunner([]string{"idx", "search", "--limit", "0", "needle"}, initCommand, destroyCommand, searchCommand)

	err := runner.Run()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if searchCommand.lastQuery != "--limit 0 needle" {
		t.Fatalf("expected literal query %q, got %q", "--limit 0 needle", searchCommand.lastQuery)
	}
}

func TestCommandRunnerRunTreatsUnsupportedLeadingOptionAsLiteralQuery(t *testing.T) {
	initCommand := &fakeInitCommand{}
	destroyCommand := &fakeDestroyCommand{}
	searchCommand := &fakeSearchCommand{}
	runner := cli.NewCommandRunner([]string{"idx", "search", "--unknown", "needle"}, initCommand, destroyCommand, searchCommand)

	err := runner.Run()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if searchCommand.lastQuery != "--unknown needle" {
		t.Fatalf("expected literal query %q, got %q", "--unknown needle", searchCommand.lastQuery)
	}
}

func TestCommandRunnerRunRejectsJsonPrettyWithoutJsonFormat(t *testing.T) {
	initCommand := &fakeInitCommand{}
	destroyCommand := &fakeDestroyCommand{}
	searchCommand := &fakeSearchCommand{}
	runner := cli.NewCommandRunner([]string{"idx", "search", "--json-pretty", "needle"}, initCommand, destroyCommand, searchCommand)

	err := runner.Run()
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if searchCommand.runCalls != 0 {
		t.Fatalf("expected 0 search calls, got %d", searchCommand.runCalls)
	}
}

func TestCommandRunnerRunRejectsUnsupportedCommand(t *testing.T) {
	initCommand := &fakeInitCommand{}
	destroyCommand := &fakeDestroyCommand{}
	searchCommand := &fakeSearchCommand{}
	runner := cli.NewCommandRunner([]string{"idx", "other"}, initCommand, destroyCommand, searchCommand)

	err := runner.Run()
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if initCommand.runCalls != 0 {
		t.Fatalf("expected 0 init calls, got %d", initCommand.runCalls)
	}

	if destroyCommand.runCalls != 0 {
		t.Fatalf("expected 0 destroy calls, got %d", destroyCommand.runCalls)
	}

	if searchCommand.runCalls != 0 {
		t.Fatalf("expected 0 search calls, got %d", searchCommand.runCalls)
	}
}

func TestCommandRunnerRunRejectsInspectWithOptionInsteadOfPath(t *testing.T) {
	initCommand := &fakeInitCommand{}
	destroyCommand := &fakeDestroyCommand{}
	searchCommand := &fakeSearchCommand{}
	runner := cli.NewCommandRunner([]string{"idx", "inspect", "--json"}, initCommand, destroyCommand, searchCommand)

	err := runner.Run()
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if initCommand.inspectCalls != 0 {
		t.Fatalf("expected 0 inspect calls, got %d", initCommand.inspectCalls)
	}
}
