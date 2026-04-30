package indexing

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"idx/internal/core/domain"
)

var runInspectTUI = runInspectTUIProgram

var inspectAvailableCommands = []string{"index", "tlog"}

var (
	inspectTitleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("39"))
	inspectHelpStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	inspectLabelStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("75"))
	inspectInfoStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))

	inspectSelectedRowStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("230")).Background(lipgloss.Color("25")).Bold(true)
	inspectRowStyle         = lipgloss.NewStyle().Foreground(lipgloss.Color("250"))

	inspectJSONKeyStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("81"))
	inspectJSONStringStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("114"))
	inspectJSONNumberStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("220"))
	inspectJSONKeywordStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("203")).Bold(true)
	inspectJSONPunctStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("246"))
	inspectJSONDefaultStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	inspectStatusLineStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("45"))
	inspectDividerStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("238"))
	inspectEmptyStateStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Bold(true)
	inspectQuitMessageStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	inspectDocumentPathStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("117")).Bold(true)
)

// SetRunInspectTUITestHook swaps the inspect TUI runner. Intended for tests.
func SetRunInspectTUITestHook(hook func(index *domain.InvertedIndex) error) {
	if hook == nil {
		runInspectTUI = runInspectTUIProgram
		return
	}

	runInspectTUI = hook
}

// RunInspectTUITestHook returns the current inspect TUI runner. Intended for tests.
func RunInspectTUITestHook() func(index *domain.InvertedIndex) error {
	return runInspectTUI
}

type inspectDocumentRow struct {
	key       string
	name      string
	directory string
	path      string
	length    int
	termCount int
}

type inspectDirectoryRow struct {
	path          string
	documentCount int
}

type inspectViewMode int

const (
	inspectViewModeDirectories inspectViewMode = iota
	inspectViewModeDocuments
	inspectViewModeLogs
	inspectViewModeJSON
)

type inspectCommandMode int

const (
	inspectCommandModeNone inspectCommandMode = iota
	inspectCommandModeSearch
	inspectCommandModeCommand
)

type inspectLogRow struct {
	indexedAt string
	path      string
	hash      string
	summary   string
	jsonRaw   string
}

type inspectRealtimeRefreshMsg struct{}

const inspectRealtimeRefreshInterval = time.Second

type inspectModel struct {
	index                *domain.InvertedIndex
	mode                 inspectViewMode
	width                int
	height               int
	quitting             bool
	directories          []inspectDirectoryRow
	filteredDirectories  []inspectDirectoryRow
	directorySearchQuery string
	directorySearchMode  bool
	documentsByDirectory map[string][]inspectDocumentRow
	directorySelected    int
	directoryStart       int
	activeDirectory      string
	documents            []inspectDocumentRow
	filteredDocuments    []inspectDocumentRow
	documentSearchQuery  string
	documentSearchMode   bool
	documentSelected     int
	documentStart        int
	logs                 []inspectLogRow
	filteredLogs         []inspectLogRow
	logSearchQuery       string
	logSearchMode        bool
	logSelected          int
	logStart             int
	logColumnOffset      int
	commandMode          inspectCommandMode
	commandQuery         string
	commandError         string
	jsonReturnMode       inspectViewMode
	jsonTitle            string
	jsonLines            []string
	jsonStart            int
}

func newInspectModel(index *domain.InvertedIndex) inspectModel {
	directories, byDirectory := inspectRowsFromIndex(index)
	return inspectModel{
		index:                index,
		mode:                 inspectViewModeDirectories,
		width:                100,
		height:               24,
		directories:          directories,
		filteredDirectories:  append([]inspectDirectoryRow(nil), directories...),
		directorySearchQuery: "",
		directorySearchMode:  false,
		documentsByDirectory: byDirectory,
		directorySelected:    0,
		directoryStart:       0,
		documents:            []inspectDocumentRow{},
		filteredDocuments:    []inspectDocumentRow{},
		documentSearchQuery:  "",
		documentSearchMode:   false,
		documentSelected:     0,
		documentStart:        0,
		logs:                 loadInspectTransactionLogs(),
		filteredLogs:         []inspectLogRow{},
		logSearchQuery:       "",
		logSearchMode:        false,
		logSelected:          0,
		logStart:             0,
		logColumnOffset:      0,
		commandMode:          inspectCommandModeNone,
		commandQuery:         "",
		commandError:         "",
		jsonReturnMode:       inspectViewModeDocuments,
	}
}

func loadInspectTransactionLogs() []inspectLogRow {
	projectRoot, err := resolveInspectProjectRoot()
	if err != nil {
		return []inspectLogRow{}
	}

	logFiles, err := discoverInspectTransactionLogFiles(projectRoot)
	if err != nil {
		return []inspectLogRow{}
	}

	rows := make([]inspectLogRow, 0)
	for _, logFile := range logFiles {
		fileRows, readErr := readInspectTransactionLogFile(logFile)
		if readErr != nil {
			continue
		}

		rows = append(rows, fileRows...)
	}

	sortInspectLogsNewestFirst(rows)

	return rows
}

func sortInspectLogsNewestFirst(rows []inspectLogRow) {
	sort.SliceStable(rows, func(i int, j int) bool {
		left := rows[i]
		right := rows[j]

		leftTime, leftOK := parseInspectLogTime(left.indexedAt)
		rightTime, rightOK := parseInspectLogTime(right.indexedAt)
		if leftOK && rightOK {
			return leftTime.After(rightTime)
		}

		if leftOK {
			return true
		}

		if rightOK {
			return false
		}

		return left.indexedAt > right.indexedAt
	})
}

func parseInspectLogTime(value string) (time.Time, bool) {
	if strings.TrimSpace(value) == "" || value == "-" {
		return time.Time{}, false
	}

	layouts := []string{time.RFC3339Nano, time.RFC3339, "2006-01-02 15:04:05", "2006-01-02 15:04:05Z07:00"}
	for _, layout := range layouts {
		parsed, err := time.Parse(layout, value)
		if err == nil {
			return parsed, true
		}
	}

	return time.Time{}, false
}

func resolveInspectProjectRoot() (string, error) {
	currentDir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	searchDir := currentDir
	for {
		gitPath := filepath.Join(searchDir, ".git")
		if info, statErr := os.Stat(gitPath); statErr == nil && info.IsDir() {
			return searchDir, nil
		}

		parentDir := filepath.Dir(searchDir)
		if parentDir == searchDir {
			return currentDir, nil
		}

		searchDir = parentDir
	}
}

func discoverInspectTransactionLogFiles(projectRoot string) ([]string, error) {
	paths := make([]string, 0)
	err := filepath.WalkDir(projectRoot, func(path string, directoryEntry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		if directoryEntry.IsDir() {
			if directoryEntry.Name() == ".git" {
				return filepath.SkipDir
			}

			return nil
		}

		if directoryEntry.Name() != "tlog.idx" {
			return nil
		}

		if !isInspectTransactionLogPath(path) {
			return nil
		}

		paths = append(paths, path)
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Strings(paths)
	return paths, nil
}

func isInspectTransactionLogPath(path string) bool {
	relativePath := filepath.ToSlash(path)
	return strings.HasSuffix(relativePath, "/.idx/logs/tlog.idx") || strings.HasSuffix(relativePath, "/idx/logs/tlog.idx")
}

func readInspectTransactionLogFile(filePath string) ([]inspectLogRow, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	lines := strings.Split(strings.TrimSpace(string(content)), "\n")
	rows := make([]inspectLogRow, 0, len(lines))
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		rows = append(rows, inspectBuildLogRow(trimmed, i+1, filePath))
	}

	// Show latest transactions first.
	for left, right := 0, len(rows)-1; left < right; left, right = left+1, right-1 {
		rows[left], rows[right] = rows[right], rows[left]
	}

	return rows, nil
}

func inspectBuildLogRow(line string, position int, filePath string) inspectLogRow {
	row := inspectLogRow{
		indexedAt: "-",
		path:      filepath.Dir(filepath.Dir(filePath)),
		hash:      "-",
		summary:   line,
		jsonRaw:   line,
	}

	jsonRaw := line

	var parsed map[string]any
	if err := json.Unmarshal([]byte(line), &parsed); err == nil {
		if value, ok := parsed["summary"].(string); ok && strings.TrimSpace(value) != "" {
			row.summary = value
		}

		row.indexedAt = inspectStringField(parsed, "indexed_at", "indexedAt", "timestamp", "time", "updated")
		if row.indexedAt == "" {
			row.indexedAt = "-"
		}

		parsedPath := inspectStringField(parsed, "path", "file_path", "filePath", "directory")
		if parsedPath != "" {
			row.path = parsedPath
		}

		row.hash = inspectStringField(parsed, "hash", "checksum", "sha", "sha256")
		if row.hash == "" {
			row.hash = "-"
		}

		pretty, marshalErr := json.MarshalIndent(parsed, "", "  ")
		if marshalErr == nil {
			jsonRaw = string(pretty)
		}
	}

	indexedAt, pathValue, hash := parseInspectSummaryFields(row.summary)
	if indexedAt != "" {
		row.indexedAt = indexedAt
	}
	if pathValue != "" {
		row.path = pathValue
	}
	if hash != "" {
		row.hash = hash
	}

	row.jsonRaw = jsonRaw
	return row
}

func inspectStringField(fields map[string]any, keys ...string) string {
	for _, key := range keys {
		value, ok := fields[key]
		if !ok {
			continue
		}

		stringValue, isString := value.(string)
		if !isString {
			continue
		}

		trimmed := strings.TrimSpace(stringValue)
		if trimmed != "" {
			return trimmed
		}
	}

	return ""
}

func parseInspectSummaryFields(summary string) (string, string, string) {
	if strings.TrimSpace(summary) == "" {
		return "", "", ""
	}

	normalized := strings.NewReplacer(",", " ",
		";", " ",
		"|", " ",
	).Replace(summary)

	parsed := map[string]string{}
	for _, token := range strings.Fields(normalized) {
		if strings.Contains(token, "=") {
			parts := strings.SplitN(token, "=", 2)
			if len(parts) == 2 {
				parsed[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
			}
			continue
		}

		if strings.Contains(token, ":") {
			parts := strings.SplitN(token, ":", 2)
			if len(parts) == 2 {
				parsed[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
			}
		}
	}

	indexedAt := parsed["indexed_at"]
	pathValue := parsed["path"]
	hash := parsed["hash"]

	if indexedAt == "" {
		indexedAt = extractSummaryValue(summary, "indexed_at")
	}
	if pathValue == "" {
		pathValue = extractSummaryValue(summary, "path")
	}
	if hash == "" {
		hash = extractSummaryValue(summary, "hash")
	}

	return indexedAt, pathValue, hash
}

func extractSummaryValue(summary string, key string) string {
	patterns := []string{key + "=", key + ":"}
	for _, pattern := range patterns {
		start := strings.Index(summary, pattern)
		if start < 0 {
			continue
		}

		value := strings.TrimSpace(summary[start+len(pattern):])
		if value == "" {
			continue
		}

		for index, runeValue := range value {
			if runeValue == ',' || runeValue == ';' || runeValue == '|' || runeValue == ' ' || runeValue == '\t' {
				return strings.TrimSpace(value[:index])
			}
		}

		return strings.TrimSpace(value)
	}

	return ""
}

func inspectRowsFromIndex(index *domain.InvertedIndex) ([]inspectDirectoryRow, map[string][]inspectDocumentRow) {
	if index == nil {
		return []inspectDirectoryRow{}, map[string][]inspectDocumentRow{}
	}

	byDirectory := make(map[string][]inspectDocumentRow)
	for documentName, stats := range index.Documents {
		if stats == nil {
			continue
		}

		displayName := stats.Name
		if displayName == "" {
			displayName = documentName
		}

		directory := inspectDocumentDirectory(documentName, stats.Path)
		byDirectory[directory] = append(byDirectory[directory], inspectDocumentRow{
			key:       documentName,
			name:      displayName,
			directory: directory,
			path:      stats.Path,
			length:    stats.Length,
			termCount: documentTermCount(index, documentName),
		})
	}

	directories := make([]inspectDirectoryRow, 0, len(byDirectory))
	for directoryPath, rows := range byDirectory {
		sort.Slice(rows, func(i int, j int) bool {
			if rows[i].path == rows[j].path {
				return rows[i].name < rows[j].name
			}

			return rows[i].path < rows[j].path
		})

		directories = append(directories, inspectDirectoryRow{
			path:          directoryPath,
			documentCount: len(rows),
		})
	}

	sort.Slice(directories, func(i int, j int) bool {
		return directories[i].path < directories[j].path
	})

	return directories, byDirectory
}

func inspectDocumentDirectory(documentKey string, documentPath string) string {
	const separator = "::"
	if separatorIndex := strings.Index(documentKey, separator); separatorIndex > 0 {
		return documentKey[:separatorIndex]
	}

	lastSlash := strings.LastIndex(documentPath, "/")
	lastBackslash := strings.LastIndex(documentPath, "\\")
	separatorIndex := lastSlash
	if lastBackslash > separatorIndex {
		separatorIndex = lastBackslash
	}

	if separatorIndex <= 0 {
		if documentPath != "" {
			return documentPath
		}

		return "."
	}

	return documentPath[:separatorIndex]
}

func documentTermCount(index *domain.InvertedIndex, documentName string) int {
	count := 0
	for _, termStats := range index.Terms {
		if termStats == nil {
			continue
		}

		if _, exists := termStats.Docs[documentName]; exists {
			count++
		}
	}

	return count
}

func (model inspectModel) Init() tea.Cmd {
	return inspectRealtimeRefreshCmd()
}

func inspectRealtimeRefreshCmd() tea.Cmd {
	return tea.Tick(inspectRealtimeRefreshInterval, func(time.Time) tea.Msg {
		return inspectRealtimeRefreshMsg{}
	})
}

func (model inspectModel) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := message.(type) {
	case inspectRealtimeRefreshMsg:
		if model.mode == inspectViewModeLogs {
			model = refreshInspectLogs(model)
		}

		return model, inspectRealtimeRefreshCmd()
	case tea.WindowSizeMsg:
		model.width = msg.Width
		model.height = msg.Height
		if model.mode == inspectViewModeJSON {
			model = adjustInspectJSONViewport(model)
		} else if model.mode == inspectViewModeDocuments {
			model = adjustInspectDocumentsViewport(model)
		} else {
			model = adjustInspectDirectoriesViewport(model)
		}
		return model, nil
	case tea.KeyMsg:
		if model.commandMode == inspectCommandModeCommand {
			return updateInspectCommandInputMode(model, msg)
		}

		if msg.String() == ":" {
			model.commandMode = inspectCommandModeCommand
			model.commandQuery = ""
			model.commandError = ""
			return model, nil
		}

		if model.mode == inspectViewModeJSON {
			return updateInspectJSONMode(model, msg)
		}

		if model.mode == inspectViewModeDocuments {
			return updateInspectDocumentsMode(model, msg)
		}

		if model.mode == inspectViewModeLogs {
			return updateInspectLogsMode(model, msg)
		}

		return updateInspectDirectoriesMode(model, msg)
	}

	return model, nil
}

func updateInspectDirectoriesMode(model inspectModel, key tea.KeyMsg) (tea.Model, tea.Cmd) {
	if model.directorySearchMode {
		return updateInspectDirectorySearchMode(model, key)
	}

	switch key.String() {
	case "ctrl+c", "q":
		model.quitting = true
		return model, tea.Quit
	case "/":
		model.directorySearchMode = true
		model.directorySearchQuery = ""
		model.commandMode = inspectCommandModeSearch
		model = applyInspectDirectoryFilter(model)
		return model, nil
	case "enter":
		if len(model.filteredDirectories) == 0 {
			return model, nil
		}

		directory := model.filteredDirectories[model.directorySelected]
		model.activeDirectory = directory.path
		model.documents = model.documentsByDirectory[directory.path]
		model.filteredDocuments = append([]inspectDocumentRow(nil), model.documents...)
		model.mode = inspectViewModeDocuments
		model.documentSearchMode = false
		model.documentSearchQuery = ""
		model.documentSelected = 0
		model.documentStart = 0
		model = adjustInspectDocumentsViewport(model)
		return model, nil
	case "up", "k":
		if model.directorySelected > 0 {
			model.directorySelected--
		}
	case "down", "j":
		if model.directorySelected < len(model.filteredDirectories)-1 {
			model.directorySelected++
		}
	case "pgup":
		model.directorySelected -= inspectDirectoriesPageStep(model)
		if model.directorySelected < 0 {
			model.directorySelected = 0
		}
	case "pgdown":
		model.directorySelected += inspectDirectoriesPageStep(model)
		if model.directorySelected >= len(model.filteredDirectories) {
			model.directorySelected = len(model.filteredDirectories) - 1
		}
	}

	model = adjustInspectDirectoriesViewport(model)
	return model, nil
}

func updateInspectDocumentsMode(model inspectModel, key tea.KeyMsg) (tea.Model, tea.Cmd) {
	if model.documentSearchMode {
		return updateInspectDocumentSearchMode(model, key)
	}

	switch key.String() {
	case "ctrl+c", "q":
		model.quitting = true
		return model, tea.Quit
	case "/":
		model.documentSearchMode = true
		model.documentSearchQuery = ""
		model.commandMode = inspectCommandModeSearch
		model = applyInspectDocumentFilter(model)
		return model, nil
	case "esc":
		model.mode = inspectViewModeDirectories
		model.documentSearchMode = false
		model.documentSearchQuery = ""
		model.filteredDocuments = append([]inspectDocumentRow(nil), model.documents...)
		model = adjustInspectDirectoriesViewport(model)
		return model, nil
	case "enter":
		if len(model.filteredDocuments) == 0 {
			return model, nil
		}

		selected := model.filteredDocuments[model.documentSelected]
		jsonText, err := inspectDocumentJSON(model.index, selected)
		if err != nil {
			jsonText = fmt.Sprintf("{\n  \"error\": %q\n}", err.Error())
		}

		model.mode = inspectViewModeJSON
		model.jsonReturnMode = inspectViewModeDocuments
		model.jsonTitle = selected.path
		model.jsonLines = strings.Split(jsonText, "\n")
		model.jsonStart = 0
		model = adjustInspectJSONViewport(model)
		return model, nil
	case "up", "k":
		if model.documentSelected > 0 {
			model.documentSelected--
		}
	case "down", "j":
		if model.documentSelected < len(model.filteredDocuments)-1 {
			model.documentSelected++
		}
	case "pgup":
		model.documentSelected -= inspectDocumentsPageStep(model)
		if model.documentSelected < 0 {
			model.documentSelected = 0
		}
	case "pgdown":
		model.documentSelected += inspectDocumentsPageStep(model)
		if model.documentSelected >= len(model.filteredDocuments) {
			model.documentSelected = len(model.filteredDocuments) - 1
		}
	}

	model = adjustInspectDocumentsViewport(model)
	return model, nil
}

func updateInspectLogsMode(model inspectModel, key tea.KeyMsg) (tea.Model, tea.Cmd) {
	if model.logSearchMode {
		return updateInspectLogSearchMode(model, key)
	}

	switch key.String() {
	case "ctrl+c", "q":
		model.quitting = true
		return model, tea.Quit
	case "/":
		model.logSearchMode = true
		model.logSearchQuery = ""
		model.commandMode = inspectCommandModeSearch
		model = applyInspectLogFilter(model)
		return model, nil
	case "enter":
		// Logs list is the final data view; keep selection unchanged on Enter.
		return model, nil
	case "up", "k":
		if model.logSelected > 0 {
			model.logSelected--
		}
	case "down", "j":
		if model.logSelected < len(model.filteredLogs)-1 {
			model.logSelected++
		}
	case "pgup":
		model.logSelected -= inspectLogsPageStep(model)
		if model.logSelected < 0 {
			model.logSelected = 0
		}
	case "pgdown":
		model.logSelected += inspectLogsPageStep(model)
		if model.logSelected >= len(model.filteredLogs) {
			model.logSelected = len(model.filteredLogs) - 1
		}
	case "left":
		if model.logColumnOffset > 0 {
			model.logColumnOffset -= 4
			if model.logColumnOffset < 0 {
				model.logColumnOffset = 0
			}
		}
	case "right":
		model.logColumnOffset += 4
	}

	model = adjustInspectLogsViewport(model)
	return model, nil
}

func updateInspectDirectorySearchMode(model inspectModel, key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key.Type {
	case tea.KeyEnter:
		model.directorySearchMode = false
		model.commandMode = inspectCommandModeNone
		return model, nil
	case tea.KeyBackspace, tea.KeyDelete:
		model.directorySearchQuery = trimLastRune(model.directorySearchQuery)
		model = applyInspectDirectoryFilter(model)
		return model, nil
	case tea.KeyRunes:
		model.directorySearchQuery += string(key.Runes)
		model = applyInspectDirectoryFilter(model)
		return model, nil
	}

	if key.String() == "ctrl+c" || key.String() == "q" {
		model.quitting = true
		return model, tea.Quit
	}

	return model, nil
}

func updateInspectDocumentSearchMode(model inspectModel, key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key.Type {
	case tea.KeyEnter:
		model.documentSearchMode = false
		model.commandMode = inspectCommandModeNone
		return model, nil
	case tea.KeyBackspace, tea.KeyDelete:
		model.documentSearchQuery = trimLastRune(model.documentSearchQuery)
		model = applyInspectDocumentFilter(model)
		return model, nil
	case tea.KeyRunes:
		model.documentSearchQuery += string(key.Runes)
		model = applyInspectDocumentFilter(model)
		return model, nil
	}

	if key.String() == "ctrl+c" || key.String() == "q" {
		model.quitting = true
		return model, tea.Quit
	}

	return model, nil
}

func updateInspectLogSearchMode(model inspectModel, key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key.Type {
	case tea.KeyEnter:
		model.logSearchMode = false
		model.commandMode = inspectCommandModeNone
		return model, nil
	case tea.KeyBackspace, tea.KeyDelete:
		model.logSearchQuery = trimLastRune(model.logSearchQuery)
		model = applyInspectLogFilter(model)
		return model, nil
	case tea.KeyRunes:
		model.logSearchQuery += string(key.Runes)
		model = applyInspectLogFilter(model)
		return model, nil
	}

	if key.String() == "ctrl+c" || key.String() == "q" {
		model.quitting = true
		return model, tea.Quit
	}

	return model, nil
}

func updateInspectCommandInputMode(model inspectModel, key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key.Type {
	case tea.KeyEnter:
		command := strings.TrimSpace(strings.TrimPrefix(model.commandQuery, ":"))
		model.commandMode = inspectCommandModeNone
		model.commandQuery = ""
		switch command {
		case "index":
			model.mode = inspectViewModeDirectories
			model.commandError = ""
			model = adjustInspectDirectoriesViewport(model)
		case "tlog":
			model.logs = loadInspectTransactionLogs()
			model.filteredLogs = append([]inspectLogRow(nil), model.logs...)
			model.logSearchQuery = ""
			model.logSearchMode = false
			model.mode = inspectViewModeLogs
			model.logSelected = 0
			model.logStart = 0
			model.commandError = ""
			model = adjustInspectLogsViewport(model)
		default:
			model.commandError = fmt.Sprintf("unknown command %q, use :index or :tlog", command)
		}

		return model, nil
	case tea.KeyTab:
		model.commandQuery = autocompleteInspectCommand(model.commandQuery)
		return model, nil
	case tea.KeyBackspace, tea.KeyDelete:
		model.commandQuery = trimLastRune(model.commandQuery)
		return model, nil
	case tea.KeyRunes:
		model.commandQuery += string(key.Runes)
		return model, nil
	}

	if key.String() == "ctrl+c" || key.String() == "q" {
		model.quitting = true
		return model, tea.Quit
	}

	return model, nil
}

func applyInspectDirectoryFilter(model inspectModel) inspectModel {
	query := strings.ToLower(strings.TrimSpace(model.directorySearchQuery))
	filtered := make([]inspectDirectoryRow, 0, len(model.directories))
	for _, row := range model.directories {
		if query == "" || strings.Contains(strings.ToLower(row.path), query) {
			filtered = append(filtered, row)
		}
	}

	model.filteredDirectories = filtered
	if model.directorySelected >= len(model.filteredDirectories) {
		model.directorySelected = len(model.filteredDirectories) - 1
	}
	if model.directorySelected < 0 {
		model.directorySelected = 0
	}

	return adjustInspectDirectoriesViewport(model)
}

func applyInspectDocumentFilter(model inspectModel) inspectModel {
	query := strings.ToLower(strings.TrimSpace(model.documentSearchQuery))
	filtered := make([]inspectDocumentRow, 0, len(model.documents))
	for _, row := range model.documents {
		path := strings.ToLower(row.path)
		name := strings.ToLower(row.name)
		if query == "" || strings.Contains(path, query) || strings.Contains(name, query) {
			filtered = append(filtered, row)
		}
	}

	model.filteredDocuments = filtered
	if model.documentSelected >= len(model.filteredDocuments) {
		model.documentSelected = len(model.filteredDocuments) - 1
	}
	if model.documentSelected < 0 {
		model.documentSelected = 0
	}

	return adjustInspectDocumentsViewport(model)
}

func applyInspectLogFilter(model inspectModel) inspectModel {
	query := strings.ToLower(strings.TrimSpace(model.logSearchQuery))
	filtered := make([]inspectLogRow, 0, len(model.logs))
	for _, row := range model.logs {
		if query == "" || strings.Contains(strings.ToLower(row.indexedAt), query) || strings.Contains(strings.ToLower(row.path), query) || strings.Contains(strings.ToLower(row.hash), query) || strings.Contains(strings.ToLower(row.summary), query) {
			filtered = append(filtered, row)
		}
	}

	model.filteredLogs = filtered
	if model.logSelected >= len(model.filteredLogs) {
		model.logSelected = len(model.filteredLogs) - 1
	}
	if model.logSelected < 0 {
		model.logSelected = 0
	}

	return adjustInspectLogsViewport(model)
}

func refreshInspectLogs(model inspectModel) inspectModel {
	latest := loadInspectTransactionLogs()
	return replaceInspectLogs(model, latest)
}

func replaceInspectLogs(model inspectModel, logs []inspectLogRow) inspectModel {
	selectedRaw := ""
	if len(model.filteredLogs) > 0 && model.logSelected >= 0 && model.logSelected < len(model.filteredLogs) {
		selectedRaw = model.filteredLogs[model.logSelected].jsonRaw
	}

	model.logs = logs
	model = applyInspectLogFilter(model)

	if selectedRaw != "" {
		for i, row := range model.filteredLogs {
			if row.jsonRaw == selectedRaw {
				model.logSelected = i
				break
			}
		}
	}

	return adjustInspectLogsViewport(model)
}

func trimLastRune(value string) string {
	runes := []rune(value)
	if len(runes) == 0 {
		return ""
	}

	return string(runes[:len(runes)-1])
}

func (model inspectModel) View() string {
	if model.quitting {
		return "\n" + inspectQuitMessageStyle.Render("Leaving inspect mode...") + "\n"
	}

	if model.mode == inspectViewModeJSON {
		return inspectJSONView(model)
	}

	if len(model.directories) == 0 {
		return "\n" + inspectEmptyStateStyle.Render("No indexed documents available.") + "\n" + inspectHelpStyle.Render("Press q to quit.") + "\n"
	}

	if model.mode == inspectViewModeDocuments {
		return inspectDocumentsView(model)
	}

	if model.mode == inspectViewModeLogs {
		return inspectLogsView(model)
	}

	return inspectDirectoriesView(model)
}

func inspectDirectoriesView(model inspectModel) string {
	var builder strings.Builder
	builder.WriteString(inspectTitleStyle.Render("idx inspect - Indexed directories"))
	builder.WriteString("\n")
	builder.WriteString(inspectHelpStyle.Render("Navigate: up/down (k/j, pgup/pgdown) | Search: / | Open directory: enter | Quit: q, ctrl+c"))
	builder.WriteString("\n\n")
	builder.WriteString(inspectLabelStyle.Render(fmt.Sprintf("Directories (%d shown of %d)", len(model.filteredDirectories), len(model.directories))))
	builder.WriteString("\n")
	builder.WriteString(inspectInputLine(model, model.directorySearchMode, model.directorySearchQuery))
	builder.WriteString("\n")
	builder.WriteString("\n")

	start, end := inspectDirectoriesVisibleRange(model)
	rowsWritten := 0
	for i := start; i < end; i++ {
		row := model.filteredDirectories[i]
		cursor := "  "
		if i == model.directorySelected {
			cursor = "> "
		}

		line := inspectTruncateLine(fmt.Sprintf("%s%s (%d docs)", cursor, row.path, row.documentCount), model.width)
		if i == model.directorySelected {
			builder.WriteString(inspectSelectedRowStyle.Render(line))
		} else {
			builder.WriteString(inspectRowStyle.Render(line))
		}
		builder.WriteString("\n")
		rowsWritten++
	}

	rowsCapacity := inspectDirectoriesListHeight(model)
	if rowsCapacity < 1 {
		rowsCapacity = 1
	}
	for rowsWritten < rowsCapacity {
		builder.WriteString("\n")
		rowsWritten++
	}

	builder.WriteString(inspectStatusLineStyle.Render(fmt.Sprintf("Showing %d-%d of %d", start+1, end, len(model.filteredDirectories))))
	builder.WriteString("\n")
	builder.WriteString(inspectDividerStyle.Render(strings.Repeat("-", inspectDividerWidth(model.width))))
	builder.WriteString("\n")

	if len(model.filteredDirectories) == 0 {
		builder.WriteString(inspectEmptyStateStyle.Render("No directories match current search."))
		return builder.String()
	}

	selected := model.filteredDirectories[model.directorySelected]
	builder.WriteString(inspectLabelStyle.Render("Details"))
	builder.WriteString("\n")
	builder.WriteString(inspectDocumentPathStyle.Render(inspectTruncateLine(fmt.Sprintf("Directory: %s", selected.path), model.width)))
	builder.WriteString("\n")
	builder.WriteString(inspectInfoStyle.Render(fmt.Sprintf("Indexed documents: %d", selected.documentCount)))

	return builder.String()
}

func inspectDocumentsView(model inspectModel) string {
	if len(model.filteredDocuments) == 0 && strings.TrimSpace(model.documentSearchQuery) == "" {
		return "\n" + inspectEmptyStateStyle.Render("No indexed documents in this directory.") + "\n" + inspectHelpStyle.Render("Press esc to go back to directories.") + "\n"
	}

	var builder strings.Builder
	builder.WriteString(inspectTitleStyle.Render("idx inspect - Directory documents"))
	builder.WriteString("\n")
	builder.WriteString(inspectHelpStyle.Render("Navigate: up/down (k/j, pgup/pgdown) | Search: / | Open JSON: enter | Back: esc | Commands: : | Quit: q, ctrl+c"))
	builder.WriteString("\n\n")
	builder.WriteString(inspectDocumentPathStyle.Render(inspectTruncateLine("Directory: "+model.activeDirectory, model.width)))
	builder.WriteString("\n")
	builder.WriteString(inspectLabelStyle.Render(fmt.Sprintf("Documents (%d shown of %d)", len(model.filteredDocuments), len(model.documents))))
	builder.WriteString("\n")
	builder.WriteString(inspectInputLine(model, model.documentSearchMode, model.documentSearchQuery))
	builder.WriteString("\n")
	builder.WriteString("\n")

	start, end := inspectDocumentsVisibleRange(model)
	rowsWritten := 0
	for i := start; i < end; i++ {
		row := model.filteredDocuments[i]
		cursor := "  "
		if i == model.documentSelected {
			cursor = "> "
		}

		line := inspectTruncateLine(fmt.Sprintf("%s%s", cursor, row.path), model.width)
		if i == model.documentSelected {
			builder.WriteString(inspectSelectedRowStyle.Render(line))
		} else {
			builder.WriteString(inspectRowStyle.Render(line))
		}
		builder.WriteString("\n")
		rowsWritten++
	}

	rowsCapacity := inspectDocumentsListHeight(model)
	if rowsCapacity < 1 {
		rowsCapacity = 1
	}
	for rowsWritten < rowsCapacity {
		builder.WriteString("\n")
		rowsWritten++
	}

	builder.WriteString(inspectStatusLineStyle.Render(fmt.Sprintf("Showing %d-%d of %d", start+1, end, len(model.filteredDocuments))))
	builder.WriteString("\n")
	builder.WriteString(inspectDividerStyle.Render(strings.Repeat("-", inspectDividerWidth(model.width))))
	builder.WriteString("\n")

	if len(model.filteredDocuments) == 0 {
		builder.WriteString(inspectEmptyStateStyle.Render("No documents match current search."))
		return builder.String()
	}

	selected := model.filteredDocuments[model.documentSelected]
	builder.WriteString(inspectLabelStyle.Render("Details"))
	builder.WriteString("\n")
	builder.WriteString(inspectInfoStyle.Render(inspectTruncateLine(fmt.Sprintf("Name: %s", selected.name), model.width)))
	builder.WriteString("\n")
	builder.WriteString(inspectDocumentPathStyle.Render(inspectTruncateLine(fmt.Sprintf("Path: %s", selected.path), model.width)))
	builder.WriteString("\n")
	builder.WriteString(inspectInfoStyle.Render(fmt.Sprintf("Length (tokens): %d", selected.length)))
	builder.WriteString("\n")
	builder.WriteString(inspectInfoStyle.Render(fmt.Sprintf("Unique terms in document: %d", selected.termCount)))

	return builder.String()
}

func inspectLogsView(model inspectModel) string {
	var builder strings.Builder
	builder.WriteString(inspectTitleStyle.Render("idx inspect - Index transaction logs"))
	builder.WriteString("\n")
	builder.WriteString(inspectHelpStyle.Render("Navigate: up/down (k/j, pgup/pgdown, left/right) | Search: / | Commands: : | Quit: q, ctrl+c"))
	builder.WriteString("\n\n")
	builder.WriteString(inspectLabelStyle.Render(fmt.Sprintf("Transactions (%d shown of %d)", len(model.filteredLogs), len(model.logs))))
	builder.WriteString("\n")
	builder.WriteString(inspectInputLine(model, model.logSearchMode, model.logSearchQuery))
	builder.WriteString("\n\n")
	builder.WriteString(inspectLabelStyle.Render(inspectHorizontalWindow(inspectLogTableHeader(), model.width, model.logColumnOffset)))
	builder.WriteString("\n")

	start, end := inspectLogsVisibleRange(model)
	rowsWritten := 0
	for i := start; i < end; i++ {
		row := model.filteredLogs[i]
		cursor := "  "
		if i == model.logSelected {
			cursor = "> "
		}

		line := inspectHorizontalWindow(fmt.Sprintf("%s%s", cursor, inspectLogTableRow(row)), model.width, model.logColumnOffset)
		if i == model.logSelected {
			builder.WriteString(inspectSelectedRowStyle.Render(line))
		} else {
			builder.WriteString(inspectRowStyle.Render(line))
		}
		builder.WriteString("\n")
		rowsWritten++
	}

	rowsCapacity := inspectLogsListHeight(model)
	if rowsCapacity < 1 {
		rowsCapacity = 1
	}
	for rowsWritten < rowsCapacity {
		builder.WriteString("\n")
		rowsWritten++
	}

	builder.WriteString(inspectStatusLineStyle.Render(fmt.Sprintf("Showing %d-%d of %d", start+1, end, len(model.filteredLogs))))
	builder.WriteString("\n")
	builder.WriteString(inspectStatusLineStyle.Render(fmt.Sprintf("Column offset: %d", model.logColumnOffset)))
	builder.WriteString("\n")
	builder.WriteString(inspectDividerStyle.Render(strings.Repeat("-", inspectDividerWidth(model.width))))
	builder.WriteString("\n")

	if len(model.filteredLogs) == 0 {
		builder.WriteString(inspectEmptyStateStyle.Render("No transactions match current filter."))
		return builder.String()
	}

	selected := model.filteredLogs[model.logSelected]
	builder.WriteString(inspectLabelStyle.Render("Details"))
	builder.WriteString("\n")
	builder.WriteString(inspectInfoStyle.Render(inspectTruncateLine("indexed_at: "+selected.indexedAt, model.width)))
	builder.WriteString("\n")
	builder.WriteString(inspectInfoStyle.Render(inspectTruncateLine("path: "+selected.path, model.width)))
	builder.WriteString("\n")
	builder.WriteString(inspectInfoStyle.Render(inspectTruncateLine("hash: "+selected.hash, model.width)))

	return builder.String()
}

func inspectDirectoriesVisibleRange(model inspectModel) (int, int) {
	if len(model.filteredDirectories) == 0 {
		return 0, 0
	}

	start := model.directoryStart
	if start < 0 {
		start = 0
	}

	end := start + inspectDirectoriesListHeight(model)
	if end > len(model.filteredDirectories) {
		end = len(model.filteredDirectories)
	}

	if start >= end {
		start = len(model.filteredDirectories) - 1
		end = len(model.filteredDirectories)
	}

	return start, end
}

func inspectDocumentsVisibleRange(model inspectModel) (int, int) {
	if len(model.filteredDocuments) == 0 {
		return 0, 0
	}

	start := model.documentStart
	if start < 0 {
		start = 0
	}

	end := start + inspectDocumentsListHeight(model)
	if end > len(model.filteredDocuments) {
		end = len(model.filteredDocuments)
	}

	if start >= end {
		start = len(model.filteredDocuments) - 1
		end = len(model.filteredDocuments)
	}

	return start, end
}

func inspectLogsVisibleRange(model inspectModel) (int, int) {
	if len(model.filteredLogs) == 0 {
		return 0, 0
	}

	start := model.logStart
	if start < 0 {
		start = 0
	}

	end := start + inspectLogsListHeight(model)
	if end > len(model.filteredLogs) {
		end = len(model.filteredLogs)
	}

	if start >= end {
		start = len(model.filteredLogs) - 1
		end = len(model.filteredLogs)
	}

	return start, end
}

func inspectDirectoriesListHeight(model inspectModel) int {
	const reservedLines = 11
	listHeight := model.height - reservedLines
	if listHeight < 1 {
		return 1
	}

	return listHeight
}

func inspectDocumentsListHeight(model inspectModel) int {
	const reservedLines = 12
	listHeight := model.height - reservedLines
	if listHeight < 1 {
		return 1
	}

	return listHeight
}

func inspectLogsListHeight(model inspectModel) int {
	const reservedLines = 12
	listHeight := model.height - reservedLines
	if listHeight < 1 {
		return 1
	}

	return listHeight
}

func inspectDirectoriesPageStep(model inspectModel) int {
	step := inspectDirectoriesListHeight(model) - 1
	if step < 1 {
		return 1
	}

	return step
}

func inspectDocumentsPageStep(model inspectModel) int {
	step := inspectDocumentsListHeight(model) - 1
	if step < 1 {
		return 1
	}

	return step
}

func inspectLogsPageStep(model inspectModel) int {
	step := inspectLogsListHeight(model) - 1
	if step < 1 {
		return 1
	}

	return step
}

func updateInspectJSONMode(model inspectModel, key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key.String() {
	case "ctrl+c", "q":
		model.quitting = true
		return model, tea.Quit
	case "esc":
		model.mode = model.jsonReturnMode
		if model.mode == inspectViewModeLogs {
			model = adjustInspectLogsViewport(model)
		} else {
			model = adjustInspectDocumentsViewport(model)
		}
		return model, nil
	case "up", "k":
		if model.jsonStart > 0 {
			model.jsonStart--
		}
	case "down", "j":
		maxStart := len(model.jsonLines) - inspectJSONHeight(model)
		if maxStart < 0 {
			maxStart = 0
		}
		if model.jsonStart < maxStart {
			model.jsonStart++
		}
	case "pgup":
		model.jsonStart -= inspectJSONHeight(model)
		if model.jsonStart < 0 {
			model.jsonStart = 0
		}
	case "pgdown":
		model.jsonStart += inspectJSONHeight(model)
		model = adjustInspectJSONViewport(model)
		return model, nil
	}

	model = adjustInspectJSONViewport(model)
	return model, nil
}

func inspectJSONView(model inspectModel) string {
	var builder strings.Builder
	builder.WriteString(inspectTitleStyle.Render("idx inspect - Document JSON (read-only)"))
	builder.WriteString("\n")
	builder.WriteString(inspectHelpStyle.Render("Navigate JSON: up/down (or k/j, pgup/pgdown) | Back: esc | Commands: : | Quit: q, ctrl+c"))
	builder.WriteString("\n\n")
	builder.WriteString(inspectDocumentPathStyle.Render(inspectTruncateLine("Document: "+model.jsonTitle, model.width)))
	builder.WriteString("\n\n")

	start, end := inspectJSONRange(model)
	for i := start; i < end; i++ {
		truncated := inspectTruncateLine(model.jsonLines[i], model.width)
		builder.WriteString(inspectHighlightJSONLine(truncated))
		builder.WriteString("\n")
	}

	builder.WriteString("\n")
	builder.WriteString(inspectStatusLineStyle.Render(fmt.Sprintf("Showing lines %d-%d of %d", start+1, end, len(model.jsonLines))))
	return builder.String()
}

func inspectDividerWidth(width int) int {
	if width < 8 {
		return 8
	}

	if width > 120 {
		return 120
	}

	return width
}

func inspectHighlightJSONLine(line string) string {
	var builder strings.Builder
	for i := 0; i < len(line); {
		if line[i] == '"' {
			token, end := inspectReadJSONString(line, i)
			if inspectJSONStringIsKey(line, end) {
				builder.WriteString(inspectJSONKeyStyle.Render(token))
			} else {
				builder.WriteString(inspectJSONStringStyle.Render(token))
			}
			i = end
			continue
		}

		if inspectStartsJSONNumber(line, i) {
			token, end := inspectReadJSONNumber(line, i)
			builder.WriteString(inspectJSONNumberStyle.Render(token))
			i = end
			continue
		}

		if token, end, ok := inspectReadJSONKeyword(line, i); ok {
			builder.WriteString(inspectJSONKeywordStyle.Render(token))
			i = end
			continue
		}

		ch := line[i]
		if strings.ContainsRune("{}[]:,", rune(ch)) {
			builder.WriteString(inspectJSONPunctStyle.Render(string(ch)))
		} else {
			builder.WriteString(inspectJSONDefaultStyle.Render(string(ch)))
		}

		i++
	}

	return builder.String()
}

func inspectReadJSONString(line string, start int) (string, int) {
	for i := start + 1; i < len(line); i++ {
		if line[i] == '\\' {
			i++
			continue
		}

		if line[i] == '"' {
			return line[start : i+1], i + 1
		}
	}

	return line[start:], len(line)
}

func inspectJSONStringIsKey(line string, from int) bool {
	for i := from; i < len(line); i++ {
		if line[i] == ' ' || line[i] == '\t' {
			continue
		}

		return line[i] == ':'
	}

	return false
}

func inspectStartsJSONNumber(line string, start int) bool {
	ch := line[start]
	if ch >= '0' && ch <= '9' {
		return true
	}

	return ch == '-' && start+1 < len(line) && line[start+1] >= '0' && line[start+1] <= '9'
}

func inspectReadJSONNumber(line string, start int) (string, int) {
	i := start
	if line[i] == '-' {
		i++
	}

	for i < len(line) && line[i] >= '0' && line[i] <= '9' {
		i++
	}

	if i < len(line) && line[i] == '.' {
		i++
		for i < len(line) && line[i] >= '0' && line[i] <= '9' {
			i++
		}
	}

	if i < len(line) && (line[i] == 'e' || line[i] == 'E') {
		i++
		if i < len(line) && (line[i] == '+' || line[i] == '-') {
			i++
		}

		for i < len(line) && line[i] >= '0' && line[i] <= '9' {
			i++
		}
	}

	return line[start:i], i
}

func inspectReadJSONKeyword(line string, start int) (string, int, bool) {
	literals := []string{"true", "false", "null"}
	for _, literal := range literals {
		if strings.HasPrefix(line[start:], literal) {
			end := start + len(literal)
			if end < len(line) {
				next := line[end]
				if (next >= 'a' && next <= 'z') || (next >= 'A' && next <= 'Z') || (next >= '0' && next <= '9') || next == '_' {
					continue
				}
			}

			return literal, end, true
		}
	}

	return "", start, false
}

func inspectJSONHeight(model inspectModel) int {
	const reservedLines = 7
	jsonHeight := model.height - reservedLines
	if jsonHeight < 1 {
		return 1
	}

	return jsonHeight
}

func inspectJSONRange(model inspectModel) (int, int) {
	if len(model.jsonLines) == 0 {
		return 0, 0
	}

	start := model.jsonStart
	if start < 0 {
		start = 0
	}

	end := start + inspectJSONHeight(model)
	if end > len(model.jsonLines) {
		end = len(model.jsonLines)
	}

	if start >= end {
		start = len(model.jsonLines) - 1
		end = len(model.jsonLines)
	}

	return start, end
}

func adjustInspectJSONViewport(model inspectModel) inspectModel {
	if len(model.jsonLines) == 0 {
		model.jsonStart = 0
		return model
	}

	maxStart := len(model.jsonLines) - inspectJSONHeight(model)
	if maxStart < 0 {
		maxStart = 0
	}

	if model.jsonStart < 0 {
		model.jsonStart = 0
	}
	if model.jsonStart > maxStart {
		model.jsonStart = maxStart
	}

	return model
}

func inspectDocumentJSON(index *domain.InvertedIndex, row inspectDocumentRow) (string, error) {
	if index == nil {
		return "", fmt.Errorf("index is nil")
	}

	docStats, ok := index.Documents[row.key]
	if !ok || docStats == nil {
		return "", fmt.Errorf("document %q not found in index", row.key)
	}

	type inspectTermEntry struct {
		Term      string `json:"term"`
		TF        int    `json:"tf"`
		Positions []int  `json:"positions"`
	}

	type inspectDocumentPayload struct {
		Name        string             `json:"name"`
		Path        string             `json:"path"`
		Length      int                `json:"length"`
		UniqueTerms int                `json:"uniqueTerms"`
		Terms       []inspectTermEntry `json:"terms"`
	}

	terms := make([]inspectTermEntry, 0)
	for term, termStats := range index.Terms {
		if termStats == nil {
			continue
		}

		docTerm, exists := termStats.Docs[row.key]
		if !exists || docTerm == nil {
			continue
		}

		terms = append(terms, inspectTermEntry{
			Term:      term,
			TF:        docTerm.TF,
			Positions: append([]int(nil), docTerm.Positions...),
		})
	}

	sort.Slice(terms, func(i int, j int) bool {
		return terms[i].Term < terms[j].Term
	})

	payload := inspectDocumentPayload{
		Name:        docStats.Name,
		Path:        docStats.Path,
		Length:      docStats.Length,
		UniqueTerms: len(terms),
		Terms:       terms,
	}

	bytes, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to encode inspect document payload: %w", err)
	}

	return string(bytes), nil
}

func adjustInspectDirectoriesViewport(model inspectModel) inspectModel {
	if len(model.filteredDirectories) == 0 {
		model.directoryStart = 0
		model.directorySelected = 0
		return model
	}

	if model.directorySelected < 0 {
		model.directorySelected = 0
	}
	if model.directorySelected >= len(model.filteredDirectories) {
		model.directorySelected = len(model.filteredDirectories) - 1
	}

	listHeight := inspectDirectoriesListHeight(model)
	if model.directorySelected < model.directoryStart {
		model.directoryStart = model.directorySelected
	}
	if model.directorySelected >= model.directoryStart+listHeight {
		model.directoryStart = model.directorySelected - listHeight + 1
	}

	maxStart := len(model.filteredDirectories) - listHeight
	if maxStart < 0 {
		maxStart = 0
	}
	if model.directoryStart > maxStart {
		model.directoryStart = maxStart
	}
	if model.directoryStart < 0 {
		model.directoryStart = 0
	}

	return model
}

func adjustInspectDocumentsViewport(model inspectModel) inspectModel {
	if len(model.filteredDocuments) == 0 {
		model.documentStart = 0
		model.documentSelected = 0
		return model
	}

	if model.documentSelected < 0 {
		model.documentSelected = 0
	}
	if model.documentSelected >= len(model.filteredDocuments) {
		model.documentSelected = len(model.filteredDocuments) - 1
	}

	listHeight := inspectDocumentsListHeight(model)
	if model.documentSelected < model.documentStart {
		model.documentStart = model.documentSelected
	}
	if model.documentSelected >= model.documentStart+listHeight {
		model.documentStart = model.documentSelected - listHeight + 1
	}

	maxStart := len(model.filteredDocuments) - listHeight
	if maxStart < 0 {
		maxStart = 0
	}
	if model.documentStart > maxStart {
		model.documentStart = maxStart
	}
	if model.documentStart < 0 {
		model.documentStart = 0
	}

	return model
}

func adjustInspectLogsViewport(model inspectModel) inspectModel {
	maxOffset := inspectMaxLogColumnOffset(model)
	if model.logColumnOffset < 0 {
		model.logColumnOffset = 0
	}
	if model.logColumnOffset > maxOffset {
		model.logColumnOffset = maxOffset
	}

	if len(model.filteredLogs) == 0 {
		model.logStart = 0
		model.logSelected = 0
		return model
	}

	if model.logSelected < 0 {
		model.logSelected = 0
	}
	if model.logSelected >= len(model.filteredLogs) {
		model.logSelected = len(model.filteredLogs) - 1
	}

	listHeight := inspectLogsListHeight(model)
	if model.logSelected < model.logStart {
		model.logStart = model.logSelected
	}
	if model.logSelected >= model.logStart+listHeight {
		model.logStart = model.logSelected - listHeight + 1
	}

	maxStart := len(model.filteredLogs) - listHeight
	if maxStart < 0 {
		maxStart = 0
	}
	if model.logStart > maxStart {
		model.logStart = maxStart
	}
	if model.logStart < 0 {
		model.logStart = 0
	}

	return model
}

func inspectMaxLogColumnOffset(model inspectModel) int {
	maxWidth := len([]rune(inspectLogTableHeader()))
	for _, row := range model.filteredLogs {
		rowWidth := len([]rune(inspectLogTableRow(row)))
		if rowWidth > maxWidth {
			maxWidth = rowWidth
		}
	}

	availableWidth := model.width
	if availableWidth < 1 {
		availableWidth = 1
	}

	maxOffset := maxWidth - availableWidth
	if maxOffset < 0 {
		return 0
	}

	return maxOffset
}

func inspectInputLine(model inspectModel, searchActive bool, searchQuery string) string {
	if model.commandMode == inspectCommandModeCommand {
		suggestions := inspectCommandSuggestions(model.commandQuery)
		if len(suggestions) == 0 {
			return inspectStatusLineStyle.Render(":" + model.commandQuery + "_")
		}

		return inspectStatusLineStyle.Render(fmt.Sprintf(":%s_  [%s]", model.commandQuery, strings.Join(suggestions, " | ")))
	}

	if searchActive {
		return inspectStatusLineStyle.Render("/" + searchQuery + "_")
	}

	if model.commandError != "" {
		return inspectEmptyStateStyle.Render(model.commandError)
	}

	if strings.TrimSpace(searchQuery) != "" {
		return inspectStatusLineStyle.Render("/" + searchQuery)
	}

	return inspectHelpStyle.Render("/ quick filter | :index | :tlog | tab autocomplete")
}

func autocompleteInspectCommand(query string) string {
	normalized := strings.TrimSpace(strings.TrimPrefix(query, ":"))
	if normalized == "" {
		return query
	}

	matches := inspectCommandSuggestions(normalized)
	if len(matches) == 1 {
		return matches[0]
	}

	if len(matches) <= 1 {
		return query
	}

	prefix := matches[0]
	for _, match := range matches[1:] {
		prefix = inspectCommonPrefix(prefix, match)
		if prefix == "" {
			break
		}
	}

	if len(prefix) > len(normalized) {
		return prefix
	}

	return query
}

func inspectCommandSuggestions(query string) []string {
	normalized := strings.TrimSpace(strings.TrimPrefix(query, ":"))
	if normalized == "" {
		return append([]string(nil), inspectAvailableCommands...)
	}

	suggestions := make([]string, 0, len(inspectAvailableCommands))
	for _, command := range inspectAvailableCommands {
		if strings.HasPrefix(command, normalized) {
			suggestions = append(suggestions, command)
		}
	}

	return suggestions
}

func inspectCommonPrefix(left string, right string) string {
	leftRunes := []rune(left)
	rightRunes := []rune(right)
	maxLen := len(leftRunes)
	if len(rightRunes) < maxLen {
		maxLen = len(rightRunes)
	}

	index := 0
	for index < maxLen && leftRunes[index] == rightRunes[index] {
		index++
	}

	return string(leftRunes[:index])
}

func inspectLogTableHeader() string {
	return fmt.Sprintf("%-24s | %-52s | %-24s", "INDEXED_AT", "PATH", "HASH")
}

func inspectLogTableRow(row inspectLogRow) string {
	return fmt.Sprintf("%-24s | %-52s | %-24s", row.indexedAt, row.path, row.hash)
}

func inspectHorizontalWindow(text string, width int, offset int) string {
	if width <= 0 {
		return ""
	}

	if offset < 0 {
		offset = 0
	}

	runes := []rune(text)
	if offset >= len(runes) {
		return ""
	}

	end := offset + width
	if end > len(runes) {
		end = len(runes)
	}

	window := string(runes[offset:end])
	if len([]rune(window)) < width {
		window = window + strings.Repeat(" ", width-len([]rune(window)))
	}

	return window
}

func inspectTruncateLine(text string, width int) string {
	if width <= 0 {
		return ""
	}

	runes := []rune(text)
	if len(runes) <= width {
		return text
	}

	if width <= 3 {
		return string(runes[:width])
	}

	return string(runes[:width-3]) + "..."
}

func runInspectTUIProgram(index *domain.InvertedIndex) error {
	program := tea.NewProgram(newInspectModel(index))
	if _, err := program.Run(); err != nil {
		return fmt.Errorf("failed to run inspect TUI: %w", err)
	}

	return nil
}
