package tui

import (
	"idx/internal/core/ports"

	tea "charm.land/bubbletea/v2"
)

// InitProgressRunner is the TUI adapter for indexing progress display during idx init.
// It implements ports.InitProgress and can be silenced via SetQuiet.
type InitProgressRunner struct {
	quiet      bool
	program    *tea.Program
	progressCh chan string
	totalCh    chan int
	done       chan struct{}
}

// NewInitProgressRunner builds the init progress TUI adapter.
// Example: runner := tui.NewInitProgressRunner()
func NewInitProgressRunner() *InitProgressRunner {
	return &InitProgressRunner{
		progressCh: make(chan string, 100),
		totalCh:    make(chan int, 1),
	}
}

func (r *InitProgressRunner) SetQuiet(quiet bool) { r.quiet = quiet }

func (r *InitProgressRunner) StartCounting() {
	if r.quiet {
		return
	}
	r.done = make(chan struct{})
	model := newInitProgressModel(r.progressCh, r.totalCh)
	r.program = tea.NewProgram(model)
	go func() {
		defer close(r.done)
		_, _ = r.program.Run()
	}()
}

func (r *InitProgressRunner) SetTotal(total int) {
	if r.quiet || r.program == nil {
		return
	}
	r.totalCh <- total
}

func (r *InitProgressRunner) IncrementDir(dirPath string) {
	if r.quiet || r.program == nil {
		return
	}
	r.progressCh <- dirPath
}

func (r *InitProgressRunner) Finish() {
	if r.quiet || r.program == nil {
		return
	}
	close(r.progressCh)
	<-r.done
}

var _ ports.InitProgress = (*InitProgressRunner)(nil)
