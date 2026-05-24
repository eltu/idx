package tui

import (
	"context"
	"idx/internal/features/indexing"

	tea "charm.land/bubbletea/v2"
)

// InitProgressRunner is the TUI adapter for indexing progress display during idx init.
// It implements indexing.Progress and can be silenced via SetQuiet.
type InitProgressRunner struct {
	quiet      bool
	program    *tea.Program
	progressCh chan string
	totalCh    chan int
	done       chan struct{}
	ctx        context.Context
	cancel     context.CancelFunc
}

// NewInitProgressRunner builds the init progress TUI adapter.
// Example: runner := tui.NewInitProgressRunner()
func NewInitProgressRunner() *InitProgressRunner {
	ctx, cancel := context.WithCancel(context.Background())
	return &InitProgressRunner{
		progressCh: make(chan string, 100),
		totalCh:    make(chan int, 1),
		ctx:        ctx,
		cancel:     cancel,
	}
}

func (r *InitProgressRunner) SetQuiet(quiet bool) { r.quiet = quiet }

func (r *InitProgressRunner) Context() context.Context { return r.ctx }

func (r *InitProgressRunner) StartCounting() {
	if r.quiet {
		return
	}
	r.done = make(chan struct{})
	model := newInitProgressModel(r.progressCh, r.totalCh, r.cancel)
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
	// Safe close: the model may have already quit via ctrl+c without draining the channel.
	select {
	case <-r.ctx.Done():
		// cancelled — program already quit, just wait for goroutine to exit
	default:
		close(r.progressCh)
	}
	<-r.done
}

var _ indexing.Progress = (*InitProgressRunner)(nil)
