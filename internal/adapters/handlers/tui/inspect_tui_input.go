package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
)

func updateInspectDirectorySearchMode(model inspectModel, key tea.KeyMsg) (tea.Model, tea.Cmd) {
	keyText := key.String()
	switch keyText {
	case "enter":
		model.directorySearchMode = false
		model.commandMode = inspectCommandModeNone
		return model, nil
	case "backspace", "delete":
		model.directorySearchQuery = trimLastRune(model.directorySearchQuery)
		model = applyInspectDirectoryFilter(model)
		return model, nil
	case "ctrl+c", "q":
		model.quitting = true
		return model, tea.Quit
	}

	if keyPress, ok := key.(tea.KeyPressMsg); ok && keyPress.Text != "" {
		model.directorySearchQuery += keyPress.Text
		model = applyInspectDirectoryFilter(model)
		return model, nil
	}

	return model, nil
}

func updateInspectDocumentSearchMode(model inspectModel, key tea.KeyMsg) (tea.Model, tea.Cmd) {
	keyText := key.String()
	switch keyText {
	case "enter":
		model.documentSearchMode = false
		model.commandMode = inspectCommandModeNone
		return model, nil
	case "backspace", "delete":
		model.documentSearchQuery = trimLastRune(model.documentSearchQuery)
		model = applyInspectDocumentFilter(model)
		return model, nil
	case "ctrl+c", "q":
		model.quitting = true
		return model, tea.Quit
	}

	if keyPress, ok := key.(tea.KeyPressMsg); ok && keyPress.Text != "" {
		model.documentSearchQuery += keyPress.Text
		model = applyInspectDocumentFilter(model)
		return model, nil
	}

	return model, nil
}

func updateInspectLogSearchMode(model inspectModel, key tea.KeyMsg) (tea.Model, tea.Cmd) {
	keyText := key.String()
	switch keyText {
	case "enter":
		model.logSearchMode = false
		model.commandMode = inspectCommandModeNone
		return model, nil
	case "backspace", "delete":
		model.logSearchQuery = trimLastRune(model.logSearchQuery)
		model = applyInspectLogFilter(model)
		return model, nil
	case "ctrl+c", "q":
		model.quitting = true
		return model, tea.Quit
	}

	if keyPress, ok := key.(tea.KeyPressMsg); ok && keyPress.Text != "" {
		model.logSearchQuery += keyPress.Text
		model = applyInspectLogFilter(model)
		return model, nil
	}

	return model, nil
}

func updateInspectCommandInputMode(model inspectModel, key tea.KeyMsg) (tea.Model, tea.Cmd) {
	keyText := key.String()
	switch keyText {
	case "enter":
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
	case "tab":
		model.commandQuery = autocompleteInspectCommand(model.commandQuery)
		return model, nil
	case "backspace", "delete":
		model.commandQuery = trimLastRune(model.commandQuery)
		return model, nil
	case "ctrl+c", "q":
		model.quitting = true
		return model, tea.Quit
	}

	if keyPress, ok := key.(tea.KeyPressMsg); ok && keyPress.Text != "" {
		model.commandQuery += keyPress.Text
		return model, nil
	}

	return model, nil
}
