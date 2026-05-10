package tui

import "strings"

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
