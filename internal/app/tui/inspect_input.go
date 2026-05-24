package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
)

// inspectSearchHandlers groups field accessors so the three pane-specific search
// mode handlers can share identical key-dispatch logic without duplication.
type inspectSearchHandlers struct {
	clearMode   func(inspectModel) inspectModel
	getQuery    func(inspectModel) string
	setQuery    func(inspectModel, string) inspectModel
	applyFilter func(inspectModel) inspectModel
}

func updateInspectSearchMode(model inspectModel, key tea.KeyMsg, h inspectSearchHandlers) (tea.Model, tea.Cmd) {
	switch key.String() {
	case "enter":
		return h.clearMode(model), nil
	case "backspace", "delete":
		m := h.setQuery(model, trimLastRune(h.getQuery(model)))
		return h.applyFilter(m), nil
	case "ctrl+c", "q":
		model.quitting = true
		return model, tea.Quit
	}
	if kp, ok := key.(tea.KeyPressMsg); ok && kp.Text != "" {
		m := h.setQuery(model, h.getQuery(model)+kp.Text)
		return h.applyFilter(m), nil
	}
	return model, nil
}

func updateInspectDirectorySearchMode(model inspectModel, key tea.KeyMsg) (tea.Model, tea.Cmd) {
	return updateInspectSearchMode(model, key, inspectSearchHandlers{
		clearMode:   func(m inspectModel) inspectModel { m.directorySearchMode = false; m.commandMode = inspectCommandModeNone; return m },
		getQuery:    func(m inspectModel) string { return m.directorySearchQuery },
		setQuery:    func(m inspectModel, q string) inspectModel { m.directorySearchQuery = q; return m },
		applyFilter: applyInspectDirectoryFilter,
	})
}

func updateInspectDocumentSearchMode(model inspectModel, key tea.KeyMsg) (tea.Model, tea.Cmd) {
	return updateInspectSearchMode(model, key, inspectSearchHandlers{
		clearMode:   func(m inspectModel) inspectModel { m.documentSearchMode = false; m.commandMode = inspectCommandModeNone; return m },
		getQuery:    func(m inspectModel) string { return m.documentSearchQuery },
		setQuery:    func(m inspectModel, q string) inspectModel { m.documentSearchQuery = q; return m },
		applyFilter: applyInspectDocumentFilter,
	})
}

func updateInspectLogSearchMode(model inspectModel, key tea.KeyMsg) (tea.Model, tea.Cmd) {
	return updateInspectSearchMode(model, key, inspectSearchHandlers{
		clearMode:   func(m inspectModel) inspectModel { m.logSearchMode = false; m.commandMode = inspectCommandModeNone; return m },
		getQuery:    func(m inspectModel) string { return m.logSearchQuery },
		setQuery:    func(m inspectModel, q string) inspectModel { m.logSearchQuery = q; return m },
		applyFilter: applyInspectLogFilter,
	})
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
