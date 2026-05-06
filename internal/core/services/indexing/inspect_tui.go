package indexing

import (
	"fmt"
	"time"

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

func SetRunInspectTUITestHook(hook func(index *domain.InvertedIndex) error) {
	if hook == nil {
		runInspectTUI = runInspectTUIProgram
		return
	}

	runInspectTUI = hook
}

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
		documentsByDirectory: byDirectory,
		logs:                 loadInspectTransactionLogs(),
		filteredLogs:         []inspectLogRow{},
		jsonReturnMode:       inspectViewModeDocuments,
	}
}

func runInspectTUIProgram(index *domain.InvertedIndex) error {
	program := tea.NewProgram(newInspectModel(index))
	if _, err := program.Run(); err != nil {
		return fmt.Errorf("failed to run inspect TUI: %w", err)
	}

	return nil
}

func RunInspectUI(index *domain.InvertedIndex) error {
	return runInspectTUI(index)
}
