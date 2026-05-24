package tui

import (
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"idx/internal/features/indexing"
)

var runInspectTUI = runInspectTUIProgram

var (
	inspectTitleStyle = lipgloss.NewStyle().Bold(true).Foreground(colorPrimary)
	inspectHelpStyle  = lipgloss.NewStyle().Foreground(colorMuted)
	inspectLabelStyle = lipgloss.NewStyle().Bold(true).Foreground(colorSecondary)
	inspectInfoStyle  = lipgloss.NewStyle().Foreground(colorText)

	inspectSelectedRowStyle = lipgloss.NewStyle().Foreground(colorSelectedFG).Background(colorSelectedBG).Bold(true)
	inspectRowStyle         = lipgloss.NewStyle().Foreground(colorText)

	inspectJSONKeyStyle      = lipgloss.NewStyle().Foreground(colorSecondary)
	inspectJSONStringStyle   = lipgloss.NewStyle().Foreground(colorSuccess)
	inspectJSONNumberStyle   = lipgloss.NewStyle().Foreground(colorNumber)
	inspectJSONKeywordStyle  = lipgloss.NewStyle().Foreground(colorError).Bold(true)
	inspectJSONPunctStyle    = lipgloss.NewStyle().Foreground(colorTextDim)
	inspectJSONDefaultStyle  = lipgloss.NewStyle().Foreground(colorText)
	inspectStatusLineStyle   = lipgloss.NewStyle().Foreground(colorSecondary)
	inspectDividerStyle      = lipgloss.NewStyle().Foreground(colorSurface)
	inspectEmptyStateStyle   = lipgloss.NewStyle().Foreground(colorAccent).Bold(true)
	inspectQuitMessageStyle  = lipgloss.NewStyle().Foreground(colorMuted)
	inspectDocumentPathStyle = lipgloss.NewStyle().Foreground(colorPath).Bold(true)
)

// SetRunInspectTUITestHook replaces the TUI runner for tests. Pass nil to restore the default.
func SetRunInspectTUITestHook(hook func(index *indexing.InvertedIndex) error) {
	if hook == nil {
		runInspectTUI = runInspectTUIProgram
		return
	}

	runInspectTUI = hook
}

// RunInspectTUITestHook returns the current TUI runner (default or test hook).
func RunInspectTUITestHook() func(index *indexing.InvertedIndex) error {
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
	index                *indexing.InvertedIndex
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

func newInspectModel(index *indexing.InvertedIndex) inspectModel {
	directories, byDirectory := inspectRowsFromIndex(index)
	return inspectModel{
		index:                index,
		mode:                 inspectViewModeDirectories,
		width:                100,
		height:               24,
		directories:          directories,
		filteredDirectories:  append([]inspectDirectoryRow(nil), directories...),
		documentsByDirectory: byDirectory,
		logs:                 loadInspectTransactionLogs(),
		filteredLogs:         []inspectLogRow{},
		jsonReturnMode:       inspectViewModeDocuments,
	}
}

func runInspectTUIProgram(index *indexing.InvertedIndex) error {
	program := tea.NewProgram(newInspectModel(index))
	if _, err := program.Run(); err != nil {
		return fmt.Errorf("failed to run inspect TUI: %w", err)
	}

	return nil
}
