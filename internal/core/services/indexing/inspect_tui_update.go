package indexing

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

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

	keyString := key.String()
	if keyString == "ctrl+c" || keyString == "q" {
		model.quitting = true
		return model, tea.Quit
	}

	if updatedModel, handled := handleInspectDocumentsViewAction(model, keyString); handled {
		return updatedModel, nil
	}

	model = updateInspectDocumentsSelection(model, keyString)

	model = adjustInspectDocumentsViewport(model)
	return model, nil
}

func handleInspectDocumentsViewAction(model inspectModel, key string) (inspectModel, bool) {
	switch key {
	case "/":
		model.documentSearchMode = true
		model.documentSearchQuery = ""
		model.commandMode = inspectCommandModeSearch
		model = applyInspectDocumentFilter(model)
		return model, true
	case "esc":
		model.mode = inspectViewModeDirectories
		model.documentSearchMode = false
		model.documentSearchQuery = ""
		model.filteredDocuments = append([]inspectDocumentRow(nil), model.documents...)
		model = adjustInspectDirectoriesViewport(model)
		return model, true
	case "enter":
		return openInspectSelectedDocumentJSON(model), true
	default:
		return model, false
	}
}

func openInspectSelectedDocumentJSON(model inspectModel) inspectModel {
	if len(model.filteredDocuments) == 0 {
		return model
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

	return adjustInspectJSONViewport(model)
}

func updateInspectDocumentsSelection(model inspectModel, key string) inspectModel {
	switch key {
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

	return model
}

func updateInspectLogsMode(model inspectModel, key tea.KeyMsg) (tea.Model, tea.Cmd) {
	if model.logSearchMode {
		return updateInspectLogSearchMode(model, key)
	}

	keyString := key.String()
	if keyString == "ctrl+c" || keyString == "q" {
		model.quitting = true
		return model, tea.Quit
	}

	if updatedModel, handled := handleInspectLogsViewAction(model, keyString); handled {
		return updatedModel, nil
	}

	model = updateInspectLogsSelection(model, keyString)
	model = updateInspectLogsHorizontalOffset(model, keyString)

	model = adjustInspectLogsViewport(model)
	return model, nil
}

func handleInspectLogsViewAction(model inspectModel, key string) (inspectModel, bool) {
	switch key {
	case "/":
		model.logSearchMode = true
		model.logSearchQuery = ""
		model.commandMode = inspectCommandModeSearch
		model = applyInspectLogFilter(model)
		return model, true
	case "enter":
		// Logs list is the final data view; keep selection unchanged on Enter.
		return model, true
	default:
		return model, false
	}
}

func updateInspectLogsSelection(model inspectModel, key string) inspectModel {
	switch key {
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
	}

	return model
}

func updateInspectLogsHorizontalOffset(model inspectModel, key string) inspectModel {
	switch key {
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

	return model
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
