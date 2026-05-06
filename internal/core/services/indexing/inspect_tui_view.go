package indexing

import (
	"fmt"
	"strings"
)

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
	builder.WriteString("\n\n")

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
	builder.WriteString("\n\n")

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
	start := maxInt(model.directoryStart, 0)
	end := minInt(start+inspectDirectoriesListHeight(model), len(model.filteredDirectories))
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
	start := maxInt(model.documentStart, 0)
	end := minInt(start+inspectDocumentsListHeight(model), len(model.filteredDocuments))
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
	start := maxInt(model.logStart, 0)
	end := minInt(start+inspectLogsListHeight(model), len(model.filteredLogs))
	if start >= end {
		start = len(model.filteredLogs) - 1
		end = len(model.filteredLogs)
	}
	return start, end
}

func inspectDirectoriesListHeight(model inspectModel) int { return maxInt(model.height-11, 1) }
func inspectDocumentsListHeight(model inspectModel) int   { return maxInt(model.height-12, 1) }
func inspectLogsListHeight(model inspectModel) int        { return maxInt(model.height-12, 1) }
func inspectDirectoriesPageStep(model inspectModel) int {
	return maxInt(inspectDirectoriesListHeight(model)-1, 1)
}
func inspectDocumentsPageStep(model inspectModel) int {
	return maxInt(inspectDocumentsListHeight(model)-1, 1)
}
func inspectLogsPageStep(model inspectModel) int { return maxInt(inspectLogsListHeight(model)-1, 1) }

func adjustInspectDirectoriesViewport(model inspectModel) inspectModel {
	if len(model.filteredDirectories) == 0 {
		model.directoryStart, model.directorySelected = 0, 0
		return model
	}
	model.directorySelected = clampInt(model.directorySelected, 0, len(model.filteredDirectories)-1)
	listHeight := inspectDirectoriesListHeight(model)
	if model.directorySelected < model.directoryStart {
		model.directoryStart = model.directorySelected
	}
	if model.directorySelected >= model.directoryStart+listHeight {
		model.directoryStart = model.directorySelected - listHeight + 1
	}
	model.directoryStart = clampInt(model.directoryStart, 0, maxInt(len(model.filteredDirectories)-listHeight, 0))
	return model
}

func adjustInspectDocumentsViewport(model inspectModel) inspectModel {
	if len(model.filteredDocuments) == 0 {
		model.documentStart, model.documentSelected = 0, 0
		return model
	}
	model.documentSelected = clampInt(model.documentSelected, 0, len(model.filteredDocuments)-1)
	listHeight := inspectDocumentsListHeight(model)
	if model.documentSelected < model.documentStart {
		model.documentStart = model.documentSelected
	}
	if model.documentSelected >= model.documentStart+listHeight {
		model.documentStart = model.documentSelected - listHeight + 1
	}
	model.documentStart = clampInt(model.documentStart, 0, maxInt(len(model.filteredDocuments)-listHeight, 0))
	return model
}

func adjustInspectLogsViewport(model inspectModel) inspectModel {
	model.logColumnOffset = clampInt(model.logColumnOffset, 0, inspectMaxLogColumnOffset(model))
	if len(model.filteredLogs) == 0 {
		model.logStart, model.logSelected = 0, 0
		return model
	}
	model.logSelected = clampInt(model.logSelected, 0, len(model.filteredLogs)-1)
	listHeight := inspectLogsListHeight(model)
	if model.logSelected < model.logStart {
		model.logStart = model.logSelected
	}
	if model.logSelected >= model.logStart+listHeight {
		model.logStart = model.logSelected - listHeight + 1
	}
	model.logStart = clampInt(model.logStart, 0, maxInt(len(model.filteredLogs)-listHeight, 0))
	return model
}

func inspectMaxLogColumnOffset(model inspectModel) int {
	maxWidth := len([]rune(inspectLogTableHeader()))
	for _, row := range model.filteredLogs {
		maxWidth = maxInt(maxWidth, len([]rune(inspectLogTableRow(row))))
	}
	availableWidth := maxInt(model.width, 1)
	return maxInt(maxWidth-availableWidth, 0)
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
	offset = maxInt(offset, 0)
	runes := []rune(text)
	if offset >= len(runes) {
		return ""
	}
	end := minInt(offset+width, len(runes))
	window := string(runes[offset:end])
	if len([]rune(window)) < width {
		window += strings.Repeat(" ", width-len([]rune(window)))
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

func inspectDividerWidth(width int) int {
	if width < 8 {
		return 8
	}
	if width > 120 {
		return 120
	}
	return width
}

func minInt(a int, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a int, b int) int {
	if a > b {
		return a
	}
	return b
}

func clampInt(value int, low int, high int) int {
	if value < low {
		return low
	}
	if value > high {
		return high
	}
	return value
}
