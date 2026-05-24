package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
)

// View implements tea.Model.
// Example: view := model.View()
func (model inspectModel) View() tea.View {
	if model.quitting {
		return tea.NewView("\n" + inspectQuitMessageStyle.Render("Leaving inspect mode...") + "\n")
	}
	if model.mode == inspectViewModeJSON {
		return tea.NewView(inspectJSONView(model))
	}
	if len(model.directories) == 0 {
		return tea.NewView("\n" + inspectEmptyStateStyle.Render("No indexed documents available.") + "\n" + inspectHelpStyle.Render("Press q to quit.") + "\n")
	}
	if model.mode == inspectViewModeDocuments {
		return tea.NewView(inspectDocumentsView(model))
	}
	if model.mode == inspectViewModeLogs {
		return tea.NewView(inspectLogsView(model))
	}
	return tea.NewView(inspectDirectoriesView(model))
}

// --- shared row rendering helpers ---

// inspectRowCursor returns the cursor prefix for a list row.
func inspectRowCursor(selected bool) string {
	if selected {
		return "> "
	}
	return "  "
}

// inspectWriteRow appends a styled row line to b, applying selection highlight.
func inspectWriteRow(b *strings.Builder, line string, selected bool) {
	if selected {
		b.WriteString(inspectSelectedRowStyle.Render(line))
	} else {
		b.WriteString(inspectRowStyle.Render(line))
	}
	b.WriteString("\n")
}

// inspectPadToCapacity fills b with blank lines until written reaches capacity,
// keeping the list area height stable when fewer items are visible.
func inspectPadToCapacity(b *strings.Builder, written, capacity int) {
	if capacity < 1 {
		capacity = 1
	}
	for written < capacity {
		b.WriteString("\n")
		written++
	}
}

// inspectStatusAndDivider renders the "Showing X-Y of Z" line and horizontal divider.
func inspectStatusAndDivider(start, end, total, width int) string {
	var b strings.Builder
	b.WriteString(inspectStatusLineStyle.Render(fmt.Sprintf("Showing %d-%d of %d", start+1, end, total)))
	b.WriteString("\n")
	b.WriteString(inspectDividerStyle.Render(strings.Repeat("-", inspectDividerWidth(width))))
	b.WriteString("\n")
	return b.String()
}

// --- directories view ---

func inspectDirectoriesView(model inspectModel) string {
	var b strings.Builder
	b.WriteString(inspectDirectoriesHeader(model))
	start, end, rows := inspectDirectoriesRows(model)
	b.WriteString(rows)
	b.WriteString(inspectStatusAndDivider(start, end, len(model.filteredDirectories), model.width))
	if len(model.filteredDirectories) == 0 {
		b.WriteString(inspectEmptyStateStyle.Render("No directories match current search."))
		return b.String()
	}
	b.WriteString(inspectDirectoriesDetails(model))
	return b.String()
}

func inspectDirectoriesHeader(model inspectModel) string {
	var b strings.Builder
	b.WriteString(inspectTitleStyle.Render("idx inspect - Indexed directories"))
	b.WriteString("\n")
	b.WriteString(inspectHelpStyle.Render("Navigate: up/down (k/j, pgup/pgdown) | Search: / | Open directory: enter | Quit: q, ctrl+c"))
	b.WriteString("\n\n")
	b.WriteString(inspectLabelStyle.Render(fmt.Sprintf("Directories (%d shown of %d)", len(model.filteredDirectories), len(model.directories))))
	b.WriteString("\n")
	b.WriteString(inspectInputLine(model, model.directorySearchMode, model.directorySearchQuery))
	b.WriteString("\n\n")
	return b.String()
}

func inspectDirectoriesRows(model inspectModel) (start, end int, content string) {
	var b strings.Builder
	start, end = inspectDirectoriesVisibleRange(model)
	written := 0
	for i := start; i < end; i++ {
		row := model.filteredDirectories[i]
		label := fmt.Sprintf("%s%s (%d docs)", inspectRowCursor(i == model.directorySelected), row.path, row.documentCount)
		inspectWriteRow(&b, inspectTruncateLine(label, model.width), i == model.directorySelected)
		written++
	}
	inspectPadToCapacity(&b, written, inspectDirectoriesListHeight(model))
	return start, end, b.String()
}

func inspectDirectoriesDetails(model inspectModel) string {
	selected := model.filteredDirectories[model.directorySelected]
	var b strings.Builder
	b.WriteString(inspectLabelStyle.Render("Details"))
	b.WriteString("\n")
	b.WriteString(inspectDocumentPathStyle.Render(inspectTruncateLine(fmt.Sprintf("Directory: %s", selected.path), model.width)))
	b.WriteString("\n")
	b.WriteString(inspectInfoStyle.Render(fmt.Sprintf("Indexed documents: %d", selected.documentCount)))
	return b.String()
}

// --- documents view ---

func inspectDocumentsView(model inspectModel) string {
	if len(model.filteredDocuments) == 0 && strings.TrimSpace(model.documentSearchQuery) == "" {
		return "\n" + inspectEmptyStateStyle.Render("No indexed documents in this directory.") + "\n" + inspectHelpStyle.Render("Press esc to go back to directories.") + "\n"
	}
	var b strings.Builder
	b.WriteString(inspectDocumentsHeader(model))
	start, end, rows := inspectDocumentsRows(model)
	b.WriteString(rows)
	b.WriteString(inspectStatusAndDivider(start, end, len(model.filteredDocuments), model.width))
	if len(model.filteredDocuments) == 0 {
		b.WriteString(inspectEmptyStateStyle.Render("No documents match current search."))
		return b.String()
	}
	b.WriteString(inspectDocumentsDetails(model))
	return b.String()
}

func inspectDocumentsHeader(model inspectModel) string {
	var b strings.Builder
	b.WriteString(inspectTitleStyle.Render("idx inspect - Directory documents"))
	b.WriteString("\n")
	b.WriteString(inspectHelpStyle.Render("Navigate: up/down (k/j, pgup/pgdown) | Search: / | Open JSON: enter | Back: esc | Commands: : | Quit: q, ctrl+c"))
	b.WriteString("\n\n")
	b.WriteString(inspectDocumentPathStyle.Render(inspectTruncateLine("Directory: "+model.activeDirectory, model.width)))
	b.WriteString("\n")
	b.WriteString(inspectLabelStyle.Render(fmt.Sprintf("Documents (%d shown of %d)", len(model.filteredDocuments), len(model.documents))))
	b.WriteString("\n")
	b.WriteString(inspectInputLine(model, model.documentSearchMode, model.documentSearchQuery))
	b.WriteString("\n\n")
	return b.String()
}

func inspectDocumentsRows(model inspectModel) (start, end int, content string) {
	var b strings.Builder
	start, end = inspectDocumentsVisibleRange(model)
	written := 0
	for i := start; i < end; i++ {
		row := model.filteredDocuments[i]
		label := fmt.Sprintf("%s%s", inspectRowCursor(i == model.documentSelected), row.path)
		inspectWriteRow(&b, inspectTruncateLine(label, model.width), i == model.documentSelected)
		written++
	}
	inspectPadToCapacity(&b, written, inspectDocumentsListHeight(model))
	return start, end, b.String()
}

func inspectDocumentsDetails(model inspectModel) string {
	selected := model.filteredDocuments[model.documentSelected]
	var b strings.Builder
	b.WriteString(inspectLabelStyle.Render("Details"))
	b.WriteString("\n")
	b.WriteString(inspectInfoStyle.Render(inspectTruncateLine(fmt.Sprintf("Name: %s", selected.name), model.width)))
	b.WriteString("\n")
	b.WriteString(inspectDocumentPathStyle.Render(inspectTruncateLine(fmt.Sprintf("Path: %s", selected.path), model.width)))
	b.WriteString("\n")
	b.WriteString(inspectInfoStyle.Render(fmt.Sprintf("Length (tokens): %d", selected.length)))
	b.WriteString("\n")
	b.WriteString(inspectInfoStyle.Render(fmt.Sprintf("Unique terms in document: %d", selected.termCount)))
	return b.String()
}

// --- logs view ---

func inspectLogsView(model inspectModel) string {
	var b strings.Builder
	b.WriteString(inspectLogsHeader(model))
	start, end, rows := inspectLogsRows(model)
	b.WriteString(rows)
	b.WriteString(inspectLogsStatusAndDivider(model, start, end))
	if len(model.filteredLogs) == 0 {
		b.WriteString(inspectEmptyStateStyle.Render("No transactions match current filter."))
		return b.String()
	}
	b.WriteString(inspectLogsDetails(model))
	return b.String()
}

func inspectLogsHeader(model inspectModel) string {
	var b strings.Builder
	b.WriteString(inspectTitleStyle.Render("idx inspect - Index transaction logs"))
	b.WriteString("\n")
	b.WriteString(inspectHelpStyle.Render("Navigate: up/down (k/j, pgup/pgdown, left/right) | Search: / | Commands: : | Quit: q, ctrl+c"))
	b.WriteString("\n\n")
	b.WriteString(inspectLabelStyle.Render(fmt.Sprintf("Transactions (%d shown of %d)", len(model.filteredLogs), len(model.logs))))
	b.WriteString("\n")
	b.WriteString(inspectInputLine(model, model.logSearchMode, model.logSearchQuery))
	b.WriteString("\n\n")
	b.WriteString(inspectLabelStyle.Render(inspectHorizontalWindow(inspectLogTableHeader(), model.width, model.logColumnOffset)))
	b.WriteString("\n")
	return b.String()
}

func inspectLogsRows(model inspectModel) (start, end int, content string) {
	var b strings.Builder
	start, end = inspectLogsVisibleRange(model)
	written := 0
	for i := start; i < end; i++ {
		row := model.filteredLogs[i]
		label := fmt.Sprintf("%s%s", inspectRowCursor(i == model.logSelected), inspectLogTableRow(row))
		inspectWriteRow(&b, inspectHorizontalWindow(label, model.width, model.logColumnOffset), i == model.logSelected)
		written++
	}
	inspectPadToCapacity(&b, written, inspectLogsListHeight(model))
	return start, end, b.String()
}

// inspectLogsStatusAndDivider renders two status lines (count + column offset) and the divider.
// Logs need the column offset indicator because they support horizontal scrolling.
func inspectLogsStatusAndDivider(model inspectModel, start, end int) string {
	var b strings.Builder
	b.WriteString(inspectStatusLineStyle.Render(fmt.Sprintf("Showing %d-%d of %d", start+1, end, len(model.filteredLogs))))
	b.WriteString("\n")
	b.WriteString(inspectStatusLineStyle.Render(fmt.Sprintf("Column offset: %d", model.logColumnOffset)))
	b.WriteString("\n")
	b.WriteString(inspectDividerStyle.Render(strings.Repeat("-", inspectDividerWidth(model.width))))
	b.WriteString("\n")
	return b.String()
}

func inspectLogsDetails(model inspectModel) string {
	selected := model.filteredLogs[model.logSelected]
	var b strings.Builder
	b.WriteString(inspectLabelStyle.Render("Details"))
	b.WriteString("\n")
	b.WriteString(inspectInfoStyle.Render(inspectTruncateLine("indexed_at: "+selected.indexedAt, model.width)))
	b.WriteString("\n")
	b.WriteString(inspectInfoStyle.Render(inspectTruncateLine("path: "+selected.path, model.width)))
	b.WriteString("\n")
	b.WriteString(inspectInfoStyle.Render(inspectTruncateLine("hash: "+selected.hash, model.width)))
	return b.String()
}

// --- utility functions ---

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
