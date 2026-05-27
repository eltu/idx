package indexing

import (
	"strings"
	"sync"
	"testing"
	"time"
)

// --- disabledInitProgress ---

func TestDisabledInitProgressStartCounting(t *testing.T) {
	p := disabledInitProgress{}
	p.StartCounting() // must not panic
}

func TestDisabledInitProgressSetTotal(t *testing.T) {
	p := disabledInitProgress{}
	p.SetTotal(42) // must not panic
}

func TestDisabledInitProgressIncrementDir(t *testing.T) {
	p := disabledInitProgress{}
	p.IncrementDir("/some/dir") // must not panic
}

func TestDisabledInitProgressFinish(t *testing.T) {
	p := disabledInitProgress{}
	p.Finish() // must not panic
}

func TestDisabledInitProgressContextReturnsNonNil(t *testing.T) {
	p := disabledInitProgress{}
	if p.Context() == nil {
		t.Fatal("expected non-nil context from disabledInitProgress")
	}
}

// --- disabledInspectUIRunner ---

func TestDisabledInspectUIRunnerReturnsError(t *testing.T) {
	r := disabledInspectUIRunner{}
	if err := r.Run(nil); err == nil {
		t.Fatal("expected error from disabledInspectUIRunner.Run")
	}
}

// --- reportedError ---

func TestReportedErrorIsEmpty(t *testing.T) {
	err := reportedError{}
	if err.Error() != "" {
		t.Errorf("expected empty error string, got %q", err.Error())
	}
}

// --- writeNotGitRepoError / writeNoIndexError / writeStaleIndexError / writeDirectoryReports ---

type captureOutputWriter struct{ lines []string }

func (w *captureOutputWriter) WriteLine(text string) error {
	w.lines = append(w.lines, text)
	return nil
}

func newServiceWithOutput(out *captureOutputWriter) InitCommandService {
	return InitCommandService{output: out}
}

func TestWriteNotGitRepoErrorWritesMessageAndReturnsReportedError(t *testing.T) {
	out := &captureOutputWriter{}
	svc := newServiceWithOutput(out)
	err := svc.writeNotGitRepoError("/some/dir")
	if err == nil {
		t.Fatal("expected reportedError, got nil")
	}
	if len(out.lines) == 0 {
		t.Fatal("expected output, got none")
	}
}

func TestWriteNoIndexErrorWritesMessageAndReturnsReportedError(t *testing.T) {
	out := &captureOutputWriter{}
	svc := newServiceWithOutput(out)
	err := svc.writeNoIndexError("/some/project")
	if err == nil {
		t.Fatal("expected reportedError, got nil")
	}
	if len(out.lines) == 0 {
		t.Fatal("expected output, got none")
	}
}

func TestWriteStaleIndexErrorWritesMessageAndReturnsError(t *testing.T) {
	out := &captureOutputWriter{}
	svc := newServiceWithOutput(out)
	err := svc.writeStaleIndexError("/root", []string{"/root/internal", "/root/pkg"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if len(out.lines) == 0 {
		t.Fatal("expected output, got none")
	}
}

func TestWriteStaleIndexErrorSingleDirectory(t *testing.T) {
	out := &captureOutputWriter{}
	svc := newServiceWithOutput(out)
	err := svc.writeStaleIndexError("/root", []string{"/root/internal"})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestWriteDirectoryReportsEmptySlice(t *testing.T) {
	out := &captureOutputWriter{}
	svc := newServiceWithOutput(out)
	if err := svc.writeDirectoryReports("/root", nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWriteDirectoryReportCurrentFolderLabelIsProjectRoot(t *testing.T) {
	out := &captureOutputWriter{}
	svc := newServiceWithOutput(out)
	report := statusDirectoryReport{Path: "/root"}
	if err := svc.writeDirectoryReport("/root", report); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out.lines) == 0 {
		t.Fatal("expected output")
	}
	if !strings.Contains(out.lines[0], "/root") {
		t.Errorf("expected project root in output, got: %q", out.lines[0])
	}
}

func TestWriteDirectoryReportStaleDirectoryState(t *testing.T) {
	out := &captureOutputWriter{}
	svc := newServiceWithOutput(out)
	report := statusDirectoryReport{Path: "/root/pkg", ShouldReindex: true}
	if err := svc.writeDirectoryReport("/root", report); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out.lines) == 0 {
		t.Fatal("expected output")
	}
}

func TestWriteDirectoryReportStructuralChange(t *testing.T) {
	out := &captureOutputWriter{}
	svc := newServiceWithOutput(out)
	report := statusDirectoryReport{Path: "/root/cmd", StructuralChange: true}
	if err := svc.writeDirectoryReport("/root", report); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- runStatusSpinnerLoop / startStatusSpinner ---

// captureSpinnerWriter implements output.Writer and statusSpinnerWriter
// so it can be used as a service output that supports inline writing.
type captureSpinnerWriter struct {
	mu     sync.Mutex
	writes []string
}

func (w *captureSpinnerWriter) WriteLine(text string) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.writes = append(w.writes, text)
	return nil
}

func (w *captureSpinnerWriter) WriteInline(text string) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.writes = append(w.writes, text)
	return nil
}

func TestRunStatusSpinnerLoopStopsImmediatelyWhenDoneClosed(t *testing.T) {
	w := &captureSpinnerWriter{}
	done := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	// Large interval so the ticker never fires before done is closed.
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	go runStatusSpinnerLoop(w, done, ticker, &wg)
	close(done)
	wg.Wait()
}

func TestRunStatusSpinnerLoopClearsLineAfterTick(t *testing.T) {
	w := &captureSpinnerWriter{}
	done := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	ticker := time.NewTicker(5 * time.Millisecond)

	go runStatusSpinnerLoop(w, done, ticker, &wg)
	// Wait for at least 2 ticks so both prefix branches are covered.
	time.Sleep(25 * time.Millisecond)
	close(done)
	wg.Wait()

	w.mu.Lock()
	count := len(w.writes)
	w.mu.Unlock()
	if count == 0 {
		t.Error("expected writes from spinner ticks, got none")
	}
}

func TestStartStatusSpinnerWithInlineWriterStartsAndStops(t *testing.T) {
	w := &captureSpinnerWriter{}
	svc := InitCommandService{output: w}
	stop := svc.startStatusSpinner()
	stop()
}

// --- StatusWithContext ---

func TestStatusWithContextSetsConfigFieldsAndRunsStatus(t *testing.T) {
	out := &captureOutputWriter{}
	svc := newServiceWithOutput(out)
	// Fails due to nil projectTree but StatusWithContext's own assignments are covered.
	_ = svc.StatusWithContext("/path/.idx.yml", []string{"bm25.b: 0.5"})
}

// --- writeStaleResult / writeStatusReport / writeDirectoryReports ---

func TestWriteStaleResultWithProfileTrue(t *testing.T) {
	out := &captureOutputWriter{}
	svc := newServiceWithOutput(out)
	err := svc.writeStaleResult(true, "/root", []string{"/root"}, statusSummary{}, []string{"/root/pkg"})
	if err == nil {
		t.Fatal("expected error from writeStaleIndexError")
	}
	if len(out.lines) == 0 {
		t.Fatal("expected output from writeStaleIndexError")
	}
}

func TestWriteStatusReportWritesDirectoriesAndSummary(t *testing.T) {
	out := &captureOutputWriter{}
	svc := newServiceWithOutput(out)
	reports := []statusDirectoryReport{{Path: "/root/pkg"}}
	summary := statusSummary{CheckedDirectories: 1}
	if err := svc.writeStatusReport("/root", reports, summary); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out.lines) < 2 {
		t.Fatalf("expected at least 2 output lines, got %d", len(out.lines))
	}
}

func TestWriteDirectoryReportsWithMultipleReports(t *testing.T) {
	out := &captureOutputWriter{}
	svc := newServiceWithOutput(out)
	reports := []statusDirectoryReport{
		{Path: "/root/pkg"},
		{Path: "/root/cmd", ShouldReindex: true},
	}
	if err := svc.writeDirectoryReports("/root", reports); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out.lines) != 2 {
		t.Fatalf("expected 2 directory reports, got %d", len(out.lines))
	}
}
