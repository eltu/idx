package tui

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
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
		return inspectHandleRefresh(model)
	case tea.WindowSizeMsg:
		return inspectHandleResize(model, msg), nil
	case tea.KeyMsg:
		return inspectHandleKey(model, msg)
	}
	return model, nil
}

func inspectHandleRefresh(model inspectModel) (tea.Model, tea.Cmd) {
	if model.mode == inspectViewModeLogs {
		model = refreshInspectLogs(model)
	}
	return model, inspectRealtimeRefreshCmd()
}

func inspectHandleResize(model inspectModel, msg tea.WindowSizeMsg) inspectModel {
	model.width = msg.Width
	model.height = msg.Height
	if model.mode == inspectViewModeJSON {
		return adjustInspectJSONViewport(model)
	}
	if model.mode == inspectViewModeDocuments {
		return adjustInspectDocumentsViewport(model)
	}
	return adjustInspectDirectoriesViewport(model)
}

func inspectHandleKey(model inspectModel, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
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

func updateInspectDirectoriesMode(model inspectModel, key tea.KeyMsg) (tea.Model, tea.Cmd) {
	if model.directorySearchMode {
		return updateInspectDirectorySearchMode(model, key)
	}
	switch key.String() {
	case "ctrl+c", "q":
		model.quitting = true
		return model, tea.Quit
	case "/":
		return enterInspectDirectorySearch(model), nil
	case "enter":
		return openInspectDirectory(model)
	}
	model = adjustInspectDirectorySelection(model, key.String())
	model = adjustInspectDirectoriesViewport(model)
	return model, nil
}

func enterInspectDirectorySearch(model inspectModel) inspectModel {
	model.directorySearchMode = true
	model.directorySearchQuery = ""
	model.commandMode = inspectCommandModeSearch
	return applyInspectDirectoryFilter(model)
}

func openInspectDirectory(model inspectModel) (tea.Model, tea.Cmd) {
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
	return adjustInspectDocumentsViewport(model), nil
}

func adjustInspectDirectorySelection(model inspectModel, key string) inspectModel {
	switch key {
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
	return model
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
