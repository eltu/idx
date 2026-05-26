package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

var (
	progressTitleStyle   = lipgloss.NewStyle().Bold(true).Foreground(colorSecondary)
	progressEmptyStyle   = lipgloss.NewStyle().Foreground(colorSurface)
	progressPercentStyle = lipgloss.NewStyle().Bold(true).Foreground(colorText)
	progressCountStyle   = lipgloss.NewStyle().Foreground(colorMuted)
	progressDirStyle     = lipgloss.NewStyle().Foreground(colorPath)
	progressSpinnerStyle = lipgloss.NewStyle().Bold(true).Foreground(colorPrimary)
)

var spinnerFrames = []string{"|", "/", "-", "\\"}

type (
	progressTickMsg     struct{ dir string }
	progressDoneMsg     struct{}
	progressSetTotalMsg struct{ total int }
	progressSpinnerMsg  struct{}
)

type progressPhase int

const (
	phaseCounting progressPhase = iota
	phaseIndexing
)

type initProgressModel struct {
	phase      progressPhase
	spinnerIdx int
	total      int
	current    int
	lastDir    string
	width      int
	progressCh <-chan string
	totalCh    <-chan int
	cancelFunc context.CancelFunc
}

func newInitProgressModel(progressCh <-chan string, totalCh <-chan int, cancelFunc context.CancelFunc) initProgressModel {
	return initProgressModel{progressCh: progressCh, totalCh: totalCh, cancelFunc: cancelFunc}
}

func (m initProgressModel) Init() tea.Cmd {
	return tea.Batch(spinnerTickCmd(), waitForTotal(m.totalCh))
}

func spinnerTickCmd() tea.Cmd {
	return tea.Tick(120*time.Millisecond, func(time.Time) tea.Msg {
		return progressSpinnerMsg{}
	})
}

func waitForTotal(ch <-chan int) tea.Cmd {
	return func() tea.Msg {
		return progressSetTotalMsg{total: <-ch}
	}
}

func waitForProgressDir(ch <-chan string) tea.Cmd {
	return func() tea.Msg {
		dir, ok := <-ch
		if !ok {
			return progressDoneMsg{}
		}
		return progressTickMsg{dir: dir}
	}
}

func (m initProgressModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == inspectQuitKey {
			m.cancelFunc()
			return m, tea.Quit
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		return m, nil
	case progressSpinnerMsg:
		if m.phase == phaseCounting {
			m.spinnerIdx = (m.spinnerIdx + 1) % len(spinnerFrames)
			return m, spinnerTickCmd()
		}
		return m, nil
	case progressSetTotalMsg:
		m.phase = phaseIndexing
		m.total = msg.total
		return m, waitForProgressDir(m.progressCh)
	case progressTickMsg:
		m.current++
		m.lastDir = msg.dir
		return m, waitForProgressDir(m.progressCh)
	case progressDoneMsg:
		m.current = m.total
		return m, tea.Quit
	}
	return m, nil
}

func (m initProgressModel) View() tea.View {
	if m.phase == phaseCounting {
		return tea.NewView(renderCountingView(m))
	}
	if m.total == 0 {
		return tea.NewView("")
	}
	return tea.NewView(renderInitProgress(m))
}

func renderCountingView(m initProgressModel) string {
	spinner := progressSpinnerStyle.Render(spinnerFrames[m.spinnerIdx])
	label := progressTitleStyle.Render("Calculating files for indexing...")
	return fmt.Sprintf("\n  %s %s\n", label, spinner)
}

func renderInitProgress(m initProgressModel) string {
	bar := renderInitProgressBar(m)
	pct := progressPercentStyle.Render(fmt.Sprintf("%3.0f%%", progressPercent(m)*100))
	count := progressCountStyle.Render(fmt.Sprintf("  %d/%d dirs", m.current, m.total))
	title := progressTitleStyle.Render("Indexing project...")
	dirLine := renderInitProgressDirLine(m.lastDir)
	return fmt.Sprintf("\n  %s\n\n  %s  %s%s%s\n", title, bar, pct, count, dirLine)
}

func renderInitProgressBar(m initProgressModel) string {
	width := m.width
	if width < 20 {
		width = 80
	}
	barWidth := width - 16
	if barWidth < 10 {
		barWidth = 10
	}
	if barWidth > 56 {
		barWidth = 56
	}
	filled := int(progressPercent(m) * float64(barWidth))
	return renderGradientFilled(filled) +
		progressEmptyStyle.Render(strings.Repeat("░", barWidth-filled))
}

func renderGradientFilled(count int) string {
	if count == 0 {
		return ""
	}
	b := strings.Builder{}
	for i := range count {
		idx := 0
		if count > 1 {
			idx = (i * (len(progressGradientHex) - 1)) / (count - 1)
		}
		b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color(progressGradientHex[idx])).Render("█"))
	}
	return b.String()
}

func renderInitProgressDirLine(lastDir string) string {
	if lastDir == "" {
		return ""
	}
	return "\n  " + progressDirStyle.Render(shortProgressDirName(lastDir))
}

func shortProgressDirName(dirPath string) string {
	parts := strings.Split(strings.ReplaceAll(dirPath, "\\", "/"), "/")
	if len(parts) <= 2 {
		return dirPath
	}
	return strings.Join(parts[len(parts)-2:], "/")
}

func progressPercent(m initProgressModel) float64 {
	if m.total == 0 {
		return 0
	}
	p := float64(m.current) / float64(m.total)
	if p > 1.0 {
		return 1.0
	}
	return p
}
