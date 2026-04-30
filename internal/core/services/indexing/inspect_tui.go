package indexing

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"idx/internal/core/domain"
)

var runInspectTUI = runInspectTUIProgram

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
	inspectViewModeJSON
)

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
	}
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
	return nil
}

func (model inspectModel) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := message.(type) {
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
		if model.mode == inspectViewModeJSON {
			return updateInspectJSONMode(model, msg)
		}

		if model.mode == inspectViewModeDocuments {
			return updateInspectDocumentsMode(model, msg)
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

func updateInspectDirectorySearchMode(model inspectModel, key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key.Type {
	case tea.KeyEsc:
		model.directorySearchMode = false
		model.directorySearchQuery = ""
		model = applyInspectDirectoryFilter(model)
		return model, nil
	case tea.KeyEnter:
		model.directorySearchMode = false
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
	case tea.KeyEsc:
		model.documentSearchMode = false
		model.documentSearchQuery = ""
		model = applyInspectDocumentFilter(model)
		return model, nil
	case tea.KeyEnter:
		model.documentSearchMode = false
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
	builder.WriteString(inspectSearchLine(model.directorySearchMode, model.directorySearchQuery))
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
		return "\n" + inspectEmptyStateStyle.Render("No indexed documents in this directory.") + "\n" + inspectHelpStyle.Render("Press esc to go back.") + "\n"
	}

	var builder strings.Builder
	builder.WriteString(inspectTitleStyle.Render("idx inspect - Directory documents"))
	builder.WriteString("\n")
	builder.WriteString(inspectHelpStyle.Render("Navigate: up/down (k/j, pgup/pgdown) | Search: / | Open JSON: enter | Back: esc | Quit: q, ctrl+c"))
	builder.WriteString("\n\n")
	builder.WriteString(inspectDocumentPathStyle.Render(inspectTruncateLine("Directory: "+model.activeDirectory, model.width)))
	builder.WriteString("\n")
	builder.WriteString(inspectLabelStyle.Render(fmt.Sprintf("Documents (%d shown of %d)", len(model.filteredDocuments), len(model.documents))))
	builder.WriteString("\n")
	builder.WriteString(inspectSearchLine(model.documentSearchMode, model.documentSearchQuery))
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

func updateInspectJSONMode(model inspectModel, key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key.String() {
	case "ctrl+c", "q":
		model.quitting = true
		return model, tea.Quit
	case "esc":
		model.mode = inspectViewModeDocuments
		model = adjustInspectDocumentsViewport(model)
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
	builder.WriteString(inspectHelpStyle.Render("Navigate JSON: up/down (or k/j, pgup/pgdown) | Back: esc | Quit: q, ctrl+c"))
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

func inspectSearchLine(active bool, query string) string {
	prefix := "/"
	if active {
		return inspectStatusLineStyle.Render(prefix + query + "_")
	}

	if strings.TrimSpace(query) == "" {
		return inspectHelpStyle.Render("/: quick filter")
	}

	return inspectStatusLineStyle.Render(prefix + query)
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
