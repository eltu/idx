package cli_test

import (
	"testing"
	"time"

	"idx/internal/adapters/handlers/cli"
	"idx/internal/core/ports"
)

type fakeInitCommand struct {
	runCalls        int
	syncCalls       int
	inspectCalls    int
	watchCalls      int
	watchShowFiles  bool
	watchDebounce   time.Duration
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

func (command *fakeInitCommand) Watch(showUpdatedFiles bool, debounce time.Duration) error {
	command.watchCalls++
	command.watchShowFiles = showUpdatedFiles
	command.watchDebounce = debounce
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

func (command *fakeSearchCommand) RunWithOptions(query string, options ports.SearchOptions) error {
	command.runCalls++
	command.lastQuery = query
	command.lastOptions = options
	return nil
}

type fakeDaemonCommand struct {
	enableCalls  int
	disableCalls int
	statusCalls  int
}

func (command *fakeDaemonCommand) Enable(string) error {
	command.enableCalls++
	return nil
}

func (command *fakeDaemonCommand) Disable(string) error {
	command.disableCalls++
	return nil
}

func (command *fakeDaemonCommand) Status() error {
	command.statusCalls++
	return nil
}

func TestCommandRunnerRunExecutesInitCommand(t *testing.T) {
	initCommand := &fakeInitCommand{}
	destroyCommand := &fakeDestroyCommand{}
	searchCommand := &fakeSearchCommand{}
	daemonCommand := &fakeDaemonCommand{}
	runner := cli.NewCommandRunner([]string{"idx", "init"}, initCommand, destroyCommand, searchCommand, daemonCommand)

	if err := runner.Run(); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if initCommand.runCalls != 1 || initCommand.syncCalls != 0 {
		t.Fatalf("expected init run=1 and sync=0, got run=%d sync=%d", initCommand.runCalls, initCommand.syncCalls)
	}
}

func TestCommandRunnerRunExecutesSyncCommand(t *testing.T) {
	initCommand := &fakeInitCommand{}
	destroyCommand := &fakeDestroyCommand{}
	searchCommand := &fakeSearchCommand{}
	daemonCommand := &fakeDaemonCommand{}
	runner := cli.NewCommandRunner([]string{"idx", "sync"}, initCommand, destroyCommand, searchCommand, daemonCommand)

	if err := runner.Run(); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if initCommand.syncCalls != 1 || initCommand.runCalls != 0 {
		t.Fatalf("expected sync=1 and init run=0, got sync=%d run=%d", initCommand.syncCalls, initCommand.runCalls)
	}
}

func TestCommandRunnerRunExecutesInspectCommandWithPath(t *testing.T) {
	initCommand := &fakeInitCommand{}
	destroyCommand := &fakeDestroyCommand{}
	searchCommand := &fakeSearchCommand{}
	daemonCommand := &fakeDaemonCommand{}
	runner := cli.NewCommandRunner([]string{"idx", "inspect", "internal/"}, initCommand, destroyCommand, searchCommand, daemonCommand)

	if err := runner.Run(); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if initCommand.inspectCalls != 1 || initCommand.lastInspectPath != "internal/" {
		t.Fatalf("expected inspect call with path %q, got calls=%d path=%q", "internal/", initCommand.inspectCalls, initCommand.lastInspectPath)
	}
}

func TestCommandRunnerRunExecutesInspectCommandWithoutPath(t *testing.T) {
	initCommand := &fakeInitCommand{}
	destroyCommand := &fakeDestroyCommand{}
	searchCommand := &fakeSearchCommand{}
	daemonCommand := &fakeDaemonCommand{}
	runner := cli.NewCommandRunner([]string{"idx", "inspect"}, initCommand, destroyCommand, searchCommand, daemonCommand)

	if err := runner.Run(); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if initCommand.inspectCalls != 1 {
		t.Fatalf("expected 1 inspect call, got %d", initCommand.inspectCalls)
	}

	if initCommand.lastInspectPath != "" {
		t.Fatalf("expected empty inspect path, got %q", initCommand.lastInspectPath)
	}
}

func TestCommandRunnerRunExecutesDestroyCommand(t *testing.T) {
	initCommand := &fakeInitCommand{}
	destroyCommand := &fakeDestroyCommand{}
	searchCommand := &fakeSearchCommand{}
	daemonCommand := &fakeDaemonCommand{}
	runner := cli.NewCommandRunner([]string{"idx", "destroy"}, initCommand, destroyCommand, searchCommand, daemonCommand)

	if err := runner.Run(); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if destroyCommand.runCalls != 1 {
		t.Fatalf("expected 1 destroy call, got %d", destroyCommand.runCalls)
	}
}

func TestCommandRunnerRunExecutesWatchCommand(t *testing.T) {
	initCommand := &fakeInitCommand{}
	destroyCommand := &fakeDestroyCommand{}
	searchCommand := &fakeSearchCommand{}
	daemonCommand := &fakeDaemonCommand{}
	runner := cli.NewCommandRunner([]string{"idx", "watch"}, initCommand, destroyCommand, searchCommand, daemonCommand)

	if err := runner.Run(); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if initCommand.watchCalls != 1 {
		t.Fatalf("expected 1 watch call, got %d", initCommand.watchCalls)
	}

	if initCommand.watchShowFiles {
		t.Fatal("expected watch show files disabled by default")
	}

	if initCommand.watchDebounce != 750*time.Millisecond {
		t.Fatalf("expected default debounce 750ms, got %s", initCommand.watchDebounce)
	}
}

func TestCommandRunnerRunExecutesWatchCommandWithShowUpdatedFilesFlag(t *testing.T) {
	initCommand := &fakeInitCommand{}
	destroyCommand := &fakeDestroyCommand{}
	searchCommand := &fakeSearchCommand{}
	daemonCommand := &fakeDaemonCommand{}
	runner := cli.NewCommandRunner([]string{"idx", "watch", "--show-updated-files"}, initCommand, destroyCommand, searchCommand, daemonCommand)

	if err := runner.Run(); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if !initCommand.watchShowFiles {
		t.Fatal("expected watch show files enabled with --show-updated-files")
	}
}

func TestCommandRunnerRunExecutesWatchCommandWithDebounceFlag(t *testing.T) {
	initCommand := &fakeInitCommand{}
	destroyCommand := &fakeDestroyCommand{}
	searchCommand := &fakeSearchCommand{}
	daemonCommand := &fakeDaemonCommand{}
	runner := cli.NewCommandRunner([]string{"idx", "watch", "--debounce", "250ms"}, initCommand, destroyCommand, searchCommand, daemonCommand)

	if err := runner.Run(); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if initCommand.watchDebounce != 250*time.Millisecond {
		t.Fatalf("expected watch debounce 250ms, got %s", initCommand.watchDebounce)
	}
}

func TestCommandRunnerRunRejectsWatchWithInvalidDebounce(t *testing.T) {
	initCommand := &fakeInitCommand{}
	destroyCommand := &fakeDestroyCommand{}
	searchCommand := &fakeSearchCommand{}
	daemonCommand := &fakeDaemonCommand{}
	runner := cli.NewCommandRunner([]string{"idx", "watch", "--debounce", "0s"}, initCommand, destroyCommand, searchCommand, daemonCommand)

	if err := runner.Run(); err == nil {
		t.Fatal("expected error for non-positive debounce")
	}
}

func TestCommandRunnerRunExecutesSearchCommand(t *testing.T) {
	initCommand := &fakeInitCommand{}
	destroyCommand := &fakeDestroyCommand{}
	searchCommand := &fakeSearchCommand{}
	daemonCommand := &fakeDaemonCommand{}
	runner := cli.NewCommandRunner([]string{"idx", "search", "needle", "term"}, initCommand, destroyCommand, searchCommand, daemonCommand)

	if err := runner.Run(); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if searchCommand.runCalls != 1 {
		t.Fatalf("expected 1 search call, got %d", searchCommand.runCalls)
	}

	if searchCommand.lastQuery != "needle term" {
		t.Fatalf("expected query %q, got %q", "needle term", searchCommand.lastQuery)
	}

	if searchCommand.lastOptions.Format != ports.SearchOutputText {
		t.Fatalf("expected format %q, got %q", ports.SearchOutputText, searchCommand.lastOptions.Format)
	}
}

func TestCommandRunnerRunExecutesSearchWithNativeCobraFlags(t *testing.T) {
	initCommand := &fakeInitCommand{}
	destroyCommand := &fakeDestroyCommand{}
	searchCommand := &fakeSearchCommand{}
	daemonCommand := &fakeDaemonCommand{}
	runner := cli.NewCommandRunner([]string{
		"idx", "search", "needle", "term",
		"--format", "json",
		"--context", "2",
		"--json-pretty",
		"--explain",
		"--matches-only",
		"--files-only",
		"--path", "internal/core",
		"--path", "cmd/idx",
		"--from", "1",
		"--size", "5",
	}, initCommand, destroyCommand, searchCommand, daemonCommand)

	if err := runner.Run(); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if searchCommand.lastQuery != "needle term" {
		t.Fatalf("expected query %q, got %q", "needle term", searchCommand.lastQuery)
	}

	if searchCommand.lastOptions.Format != ports.SearchOutputJSON {
		t.Fatalf("expected format %q, got %q", ports.SearchOutputJSON, searchCommand.lastOptions.Format)
	}

	if !searchCommand.lastOptions.PrettyJSON || !searchCommand.lastOptions.MatchesOnly || !searchCommand.lastOptions.FilesOnly {
		t.Fatalf("expected pretty/matches-only/files-only true, got %+v", searchCommand.lastOptions)
	}

	if !searchCommand.lastOptions.Explain {
		t.Fatal("expected Explain true when --explain is provided")
	}

	if searchCommand.lastOptions.Context != 2 || searchCommand.lastOptions.Size != 5 {
		t.Fatalf("expected context=2 and size=5, got context=%d size=%d", searchCommand.lastOptions.Context, searchCommand.lastOptions.Size)
	}

	if searchCommand.lastOptions.From != 1 {
		t.Fatalf("expected from=1, got from=%d", searchCommand.lastOptions.From)
	}

	if len(searchCommand.lastOptions.PathQueries) != 2 {
		t.Fatalf("expected two path filters, got %v", searchCommand.lastOptions.PathQueries)
	}

	if searchCommand.lastOptions.PathQuery != "internal/core" {
		t.Fatalf("expected first path as PathQuery, got %q", searchCommand.lastOptions.PathQuery)
	}
}

func TestCommandRunnerRunAcceptsMetadataOnlySearch(t *testing.T) {
	initCommand := &fakeInitCommand{}
	destroyCommand := &fakeDestroyCommand{}
	searchCommand := &fakeSearchCommand{}
	daemonCommand := &fakeDaemonCommand{}
	runner := cli.NewCommandRunner([]string{"idx", "search", "--path", "internal/core"}, initCommand, destroyCommand, searchCommand, daemonCommand)

	if err := runner.Run(); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if searchCommand.lastQuery != "" {
		t.Fatalf("expected empty query, got %q", searchCommand.lastQuery)
	}

	if len(searchCommand.lastOptions.PathQueries) != 1 || searchCommand.lastOptions.PathQueries[0] != "internal/core" {
		t.Fatalf("expected path filter [internal/core], got %v", searchCommand.lastOptions.PathQueries)
	}
}

func TestCommandRunnerRunAcceptsLegacyTypoMatchesOnlyOption(t *testing.T) {
	initCommand := &fakeInitCommand{}
	destroyCommand := &fakeDestroyCommand{}
	searchCommand := &fakeSearchCommand{}
	daemonCommand := &fakeDaemonCommand{}
	runner := cli.NewCommandRunner([]string{"idx", "search", "--macthes-only", "needle"}, initCommand, destroyCommand, searchCommand, daemonCommand)

	if err := runner.Run(); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if !searchCommand.lastOptions.MatchesOnly {
		t.Fatal("expected MatchesOnly true")
	}
}

func TestCommandRunnerRunRejectsSearchWithoutInput(t *testing.T) {
	initCommand := &fakeInitCommand{}
	destroyCommand := &fakeDestroyCommand{}
	searchCommand := &fakeSearchCommand{}
	daemonCommand := &fakeDaemonCommand{}
	runner := cli.NewCommandRunner([]string{"idx", "search"}, initCommand, destroyCommand, searchCommand, daemonCommand)

	if err := runner.Run(); err == nil {
		t.Fatal("expected an error, got nil")
	}

	if searchCommand.runCalls != 0 {
		t.Fatalf("expected 0 search calls, got %d", searchCommand.runCalls)
	}
}

func TestCommandRunnerRunRejectsJsonPrettyWithoutJsonFormat(t *testing.T) {
	initCommand := &fakeInitCommand{}
	destroyCommand := &fakeDestroyCommand{}
	searchCommand := &fakeSearchCommand{}
	daemonCommand := &fakeDaemonCommand{}
	runner := cli.NewCommandRunner([]string{"idx", "search", "--json-pretty", "needle"}, initCommand, destroyCommand, searchCommand, daemonCommand)

	if err := runner.Run(); err == nil {
		t.Fatal("expected an error, got nil")
	}
}

func TestCommandRunnerRunRejectsUnsupportedFormat(t *testing.T) {
	initCommand := &fakeInitCommand{}
	destroyCommand := &fakeDestroyCommand{}
	searchCommand := &fakeSearchCommand{}
	daemonCommand := &fakeDaemonCommand{}
	runner := cli.NewCommandRunner([]string{"idx", "search", "needle", "--format", "xml"}, initCommand, destroyCommand, searchCommand, daemonCommand)

	if err := runner.Run(); err == nil {
		t.Fatal("expected an error, got nil")
	}
}

func TestCommandRunnerRunRejectsInvalidContext(t *testing.T) {
	initCommand := &fakeInitCommand{}
	destroyCommand := &fakeDestroyCommand{}
	searchCommand := &fakeSearchCommand{}
	daemonCommand := &fakeDaemonCommand{}
	runner := cli.NewCommandRunner([]string{"idx", "search", "needle", "--context", "-1"}, initCommand, destroyCommand, searchCommand, daemonCommand)

	if err := runner.Run(); err == nil {
		t.Fatal("expected an error, got nil")
	}
}

func TestCommandRunnerRunRejectsInvalidSize(t *testing.T) {
	initCommand := &fakeInitCommand{}
	destroyCommand := &fakeDestroyCommand{}
	searchCommand := &fakeSearchCommand{}
	daemonCommand := &fakeDaemonCommand{}

	runnerZero := cli.NewCommandRunner([]string{"idx", "search", "needle", "--size", "0"}, initCommand, destroyCommand, searchCommand, daemonCommand)
	if err := runnerZero.Run(); err == nil {
		t.Fatal("expected error for --size 0, got nil")
	}

	runnerNegative := cli.NewCommandRunner([]string{"idx", "search", "needle", "--size", "-2"}, initCommand, destroyCommand, searchCommand, daemonCommand)
	if err := runnerNegative.Run(); err == nil {
		t.Fatal("expected error for negative --size, got nil")
	}
}

func TestCommandRunnerRunRejectsInvalidFrom(t *testing.T) {
	initCommand := &fakeInitCommand{}
	destroyCommand := &fakeDestroyCommand{}
	searchCommand := &fakeSearchCommand{}
	daemonCommand := &fakeDaemonCommand{}

	runner := cli.NewCommandRunner([]string{"idx", "search", "needle", "--from", "-1"}, initCommand, destroyCommand, searchCommand, daemonCommand)
	if err := runner.Run(); err == nil {
		t.Fatal("expected error for negative --from, got nil")
	}
}

func TestCommandRunnerRunRejectsUnknownSearchFlag(t *testing.T) {
	initCommand := &fakeInitCommand{}
	destroyCommand := &fakeDestroyCommand{}
	searchCommand := &fakeSearchCommand{}
	daemonCommand := &fakeDaemonCommand{}
	runner := cli.NewCommandRunner([]string{"idx", "search", "needle", "--unknown"}, initCommand, destroyCommand, searchCommand, daemonCommand)

	if err := runner.Run(); err == nil {
		t.Fatal("expected an error, got nil")
	}
}

func TestCommandRunnerRunRejectsUnsupportedCommand(t *testing.T) {
	initCommand := &fakeInitCommand{}
	destroyCommand := &fakeDestroyCommand{}
	searchCommand := &fakeSearchCommand{}
	daemonCommand := &fakeDaemonCommand{}
	runner := cli.NewCommandRunner([]string{"idx", "other"}, initCommand, destroyCommand, searchCommand, daemonCommand)

	if err := runner.Run(); err == nil {
		t.Fatal("expected an error, got nil")
	}
}

func TestCommandRunnerRunAllowsHelpCommand(t *testing.T) {
	initCommand := &fakeInitCommand{}
	destroyCommand := &fakeDestroyCommand{}
	searchCommand := &fakeSearchCommand{}
	daemonCommand := &fakeDaemonCommand{}
	runner := cli.NewCommandRunner([]string{"idx", "help"}, initCommand, destroyCommand, searchCommand, daemonCommand)

	if err := runner.Run(); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if initCommand.runCalls != 0 || initCommand.syncCalls != 0 || initCommand.inspectCalls != 0 || destroyCommand.runCalls != 0 || searchCommand.runCalls != 0 {
		t.Fatalf("expected no business command calls on help, got init=%+v destroy=%d search=%d", initCommand, destroyCommand.runCalls, searchCommand.runCalls)
	}
}

func TestCommandRunnerRunAllowsHelpFlag(t *testing.T) {
	initCommand := &fakeInitCommand{}
	destroyCommand := &fakeDestroyCommand{}
	searchCommand := &fakeSearchCommand{}
	daemonCommand := &fakeDaemonCommand{}
	runner := cli.NewCommandRunner([]string{"idx", "--help"}, initCommand, destroyCommand, searchCommand, daemonCommand)

	if err := runner.Run(); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}
