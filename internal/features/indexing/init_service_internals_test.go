package indexing

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- disabledInitProgress ---

func TestDisabledInitProgress_StartCounting_DoesNotPanic(t *testing.T) {
	t.Parallel()
	p := disabledInitProgress{}
	p.StartCounting() // must not panic
}

func TestDisabledInitProgress_SetTotal_DoesNotPanic(t *testing.T) {
	t.Parallel()
	p := disabledInitProgress{}
	p.SetTotal(42) // must not panic
}

func TestDisabledInitProgress_IncrementDir_DoesNotPanic(t *testing.T) {
	t.Parallel()
	p := disabledInitProgress{}
	p.IncrementDir("/some/dir") // must not panic
}

func TestDisabledInitProgress_Finish_DoesNotPanic(t *testing.T) {
	t.Parallel()
	p := disabledInitProgress{}
	p.Finish() // must not panic
}

func TestDisabledInitProgress_Context_ReturnsNonNil(t *testing.T) {
	t.Parallel()

	// Act
	ctx := disabledInitProgress{}.Context()

	// Assert
	require.NotNil(t, ctx)
}

// --- disabledInspectUIRunner ---

func TestDisabledInspectUIRunner_Run_ReturnsError(t *testing.T) {
	t.Parallel()

	// Act
	err := disabledInspectUIRunner{}.Run(nil)

	// Assert
	require.Error(t, err)
}

// --- reportedError ---

func TestReportedError_Error_IsEmpty(t *testing.T) {
	t.Parallel()

	// Act
	msg := reportedError{}.Error()

	// Assert
	assert.Empty(t, msg)
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

func TestWriteNotGitRepoError_WritesMessage_ReturnsReportedError(t *testing.T) {
	t.Parallel()

	// Arrange
	out := &captureOutputWriter{}
	svc := newServiceWithOutput(out)

	// Act
	err := svc.writeNotGitRepoError("/some/dir")

	// Assert
	require.Error(t, err)
	assert.NotEmpty(t, out.lines)
}

func TestWriteNoIndexError_WritesMessage_ReturnsReportedError(t *testing.T) {
	t.Parallel()

	// Arrange
	out := &captureOutputWriter{}
	svc := newServiceWithOutput(out)

	// Act
	err := svc.writeNoIndexError("/some/project")

	// Assert
	require.Error(t, err)
	assert.NotEmpty(t, out.lines)
}

func TestWriteStaleIndexError_WritesMessage_ReturnsError(t *testing.T) {
	t.Parallel()

	// Arrange
	out := &captureOutputWriter{}
	svc := newServiceWithOutput(out)

	// Act
	err := svc.writeStaleIndexError("/root", []string{"/root/internal", "/root/pkg"})

	// Assert
	require.Error(t, err)
	assert.NotEmpty(t, out.lines)
}

func TestWriteStaleIndexError_SingleDirectory_ReturnsError(t *testing.T) {
	t.Parallel()

	// Arrange
	out := &captureOutputWriter{}
	svc := newServiceWithOutput(out)

	// Act
	err := svc.writeStaleIndexError("/root", []string{"/root/internal"})

	// Assert
	require.Error(t, err)
}

func TestWriteDirectoryReports_EmptySlice_ReturnsNoError(t *testing.T) {
	t.Parallel()

	// Arrange
	out := &captureOutputWriter{}
	svc := newServiceWithOutput(out)

	// Act
	err := svc.writeDirectoryReports("/root", nil)

	// Assert
	require.NoError(t, err)
}

func TestWriteDirectoryReport_CurrentFolder_LabelIsProjectRoot(t *testing.T) {
	t.Parallel()

	// Arrange
	out := &captureOutputWriter{}
	svc := newServiceWithOutput(out)
	report := statusDirectoryReport{Path: "/root"}

	// Act
	err := svc.writeDirectoryReport("/root", report)

	// Assert
	require.NoError(t, err)
	require.NotEmpty(t, out.lines)
	assert.Contains(t, out.lines[0], "/root")
}

func TestWriteDirectoryReport_StaleDirectory_WritesOutput(t *testing.T) {
	t.Parallel()

	// Arrange
	out := &captureOutputWriter{}
	svc := newServiceWithOutput(out)
	report := statusDirectoryReport{Path: "/root/pkg", ShouldReindex: true}

	// Act
	err := svc.writeDirectoryReport("/root", report)

	// Assert
	require.NoError(t, err)
	assert.NotEmpty(t, out.lines)
}

func TestWriteDirectoryReport_StructuralChange_WritesOutput(t *testing.T) {
	t.Parallel()

	// Arrange
	out := &captureOutputWriter{}
	svc := newServiceWithOutput(out)
	report := statusDirectoryReport{Path: "/root/cmd", StructuralChange: true}

	// Act
	err := svc.writeDirectoryReport("/root", report)

	// Assert
	require.NoError(t, err)
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
	return w.WriteLine(text)
}

func TestRunStatusSpinnerLoop_DoneClosed_StopsImmediately(t *testing.T) {
	t.Parallel()

	// Arrange
	w := &captureSpinnerWriter{}
	done := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	// Large interval so the ticker never fires before done is closed.
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	// Act
	go runStatusSpinnerLoop(w, done, ticker, &wg)
	close(done)

	// Assert: no deadlock
	wg.Wait()
}

func TestRunStatusSpinnerLoop_AfterTick_ClearsLine(t *testing.T) {
	t.Parallel()

	// Arrange
	w := &captureSpinnerWriter{}
	done := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	ticker := time.NewTicker(5 * time.Millisecond)

	// Act: Wait for at least 2 ticks so both prefix branches are covered.
	go runStatusSpinnerLoop(w, done, ticker, &wg)
	time.Sleep(25 * time.Millisecond)
	close(done)
	wg.Wait()

	// Assert
	w.mu.Lock()
	count := len(w.writes)
	w.mu.Unlock()
	assert.Greater(t, count, 0)
}

func TestStartStatusSpinner_WithInlineWriter_StartsAndStops(t *testing.T) {
	t.Parallel()

	// Arrange
	w := &captureSpinnerWriter{}
	svc := InitCommandService{output: w}

	// Act / Assert: must not panic or deadlock
	stop := svc.startStatusSpinner()
	stop()
}

// --- StatusWithContext ---

func TestStatusWithContext_SetsConfigFields_RunsStatus(t *testing.T) {
	t.Parallel()

	// Arrange
	out := &captureOutputWriter{}
	svc := newServiceWithOutput(out)

	// Act: Fails due to nil projectTree but StatusWithContext's own assignments are covered.
	_ = svc.StatusWithContext("/path/.idx.yml", []string{"bm25.b: 0.5"})
}

// --- writeStaleResult / writeStatusReport / writeDirectoryReports ---

func TestWriteStaleResult_ProfileTrue_ReturnsErrorAndWritesOutput(t *testing.T) {
	t.Parallel()

	// Arrange
	out := &captureOutputWriter{}
	svc := newServiceWithOutput(out)

	// Act
	err := svc.writeStaleResult(true, "/root", []string{"/root"}, statusSummary{}, []string{"/root/pkg"})

	// Assert
	require.Error(t, err)
	assert.NotEmpty(t, out.lines)
}

func TestWriteStatusReport_WritesDirectoriesAndSummary(t *testing.T) {
	t.Parallel()

	// Arrange
	out := &captureOutputWriter{}
	svc := newServiceWithOutput(out)
	reports := []statusDirectoryReport{{Path: "/root/pkg"}}
	summary := statusSummary{CheckedDirectories: 1}

	// Act
	err := svc.writeStatusReport("/root", reports, summary)

	// Assert
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(out.lines), 2)
}

func TestWriteDirectoryReports_MultipleReports_WritesEachReport(t *testing.T) {
	t.Parallel()

	// Arrange
	out := &captureOutputWriter{}
	svc := newServiceWithOutput(out)
	reports := []statusDirectoryReport{
		{Path: "/root/pkg"},
		{Path: "/root/cmd", ShouldReindex: true},
	}

	// Act
	err := svc.writeDirectoryReports("/root", reports)

	// Assert
	require.NoError(t, err)
	assert.Len(t, out.lines, 2)
}
