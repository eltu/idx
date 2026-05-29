package tui

import (
	"context"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---- shortProgressDirName ----

func TestShortProgressDirName_ShallowPath_Unchanged(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "foo/bar", shortProgressDirName("foo/bar"))
}

func TestShortProgressDirName_SinglePart_Unchanged(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "single", shortProgressDirName("single"))
}

func TestShortProgressDirName_DeepPath_KeepsLastTwo(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "c/d", shortProgressDirName("a/b/c/d"))
}

func TestShortProgressDirName_WindowsSlash_Normalized(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "c/d", shortProgressDirName(`a\b\c\d`))
}

func TestShortProgressDirName_ExactlyThreeParts_KeepsLastTwo(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "y/z", shortProgressDirName("x/y/z"))
}

// ---- progressPercent ----

func TestProgressPercent_ZeroTotal_ReturnsZero(t *testing.T) {
	t.Parallel()

	m := initProgressModel{total: 0, current: 0}
	assert.Equal(t, 0.0, progressPercent(m))
}

func TestProgressPercent_HalfProgress_ReturnsApproximatelyHalf(t *testing.T) {
	t.Parallel()

	m := initProgressModel{total: 10, current: 5}
	assert.InDelta(t, 0.5, progressPercent(m), 0.01)
}

func TestProgressPercent_FullProgress_ReturnsOne(t *testing.T) {
	t.Parallel()

	m := initProgressModel{total: 10, current: 10}
	assert.Equal(t, 1.0, progressPercent(m))
}

func TestProgressPercent_OverFull_CapsAtOne(t *testing.T) {
	t.Parallel()

	m := initProgressModel{total: 5, current: 20}
	assert.Equal(t, 1.0, progressPercent(m))
}

// ---- renderGradientFilled ----

func TestRenderGradientFilled_Zero_ReturnsEmpty(t *testing.T) {
	t.Parallel()

	assert.Empty(t, renderGradientFilled(0))
}

func TestRenderGradientFilled_OneBlock_ContainsBlock(t *testing.T) {
	t.Parallel()

	assert.Contains(t, renderGradientFilled(1), "█")
}

func TestRenderGradientFilled_MultipleBlocks_ExactCount(t *testing.T) {
	t.Parallel()

	result := renderGradientFilled(5)
	count := 0
	for _, r := range result {
		if r == '█' {
			count++
		}
	}
	assert.Equal(t, 5, count)
}

// ---- renderInitProgressDirLine ----

func TestRenderInitProgressDirLine_EmptyDir_ReturnsEmpty(t *testing.T) {
	t.Parallel()

	assert.Empty(t, renderInitProgressDirLine(""))
}

func TestRenderInitProgressDirLine_NonEmpty_ContainsDirName(t *testing.T) {
	t.Parallel()

	assert.Contains(t, renderInitProgressDirLine("foo/bar"), "bar")
}

// ---- initProgressModel Update ----

func TestInitProgressModel_Update_WindowSize_UpdatesWidth(t *testing.T) {
	t.Parallel()

	// Arrange
	m := newInitProgressModel(nil, nil, func() {})

	// Act
	result, _ := m.Update(tea.WindowSizeMsg{Width: 120})

	// Assert
	assert.Equal(t, 120, result.(initProgressModel).width)
}

func TestInitProgressModel_Update_SpinnerMsg_AdvancesFrame(t *testing.T) {
	t.Parallel()

	// Arrange
	m := newInitProgressModel(nil, nil, func() {})
	m.phase = phaseCounting
	m.spinnerIdx = 0

	// Act
	result, _ := m.Update(progressSpinnerMsg{})

	// Assert
	assert.Equal(t, 1, result.(initProgressModel).spinnerIdx)
}

func TestInitProgressModel_Update_SpinnerMsg_WrapsAround(t *testing.T) {
	t.Parallel()

	// Arrange
	m := newInitProgressModel(nil, nil, func() {})
	m.phase = phaseCounting
	m.spinnerIdx = len(spinnerFrames) - 1

	// Act
	result, _ := m.Update(progressSpinnerMsg{})

	// Assert
	assert.Equal(t, 0, result.(initProgressModel).spinnerIdx)
}

func TestInitProgressModel_Update_SpinnerMsg_IgnoredDuringIndexing(t *testing.T) {
	t.Parallel()

	// Arrange
	progressCh := make(chan string)
	totalCh := make(chan int)
	m := newInitProgressModel(progressCh, totalCh, func() {})
	m.phase = phaseIndexing
	m.spinnerIdx = 2

	// Act
	result, _ := m.Update(progressSpinnerMsg{})

	// Assert — spinner index must not change during indexing phase
	assert.Equal(t, 2, result.(initProgressModel).spinnerIdx)
}

func TestInitProgressModel_Update_SetTotal_SwitchesToIndexingPhase(t *testing.T) {
	t.Parallel()

	// Arrange
	progressCh := make(chan string, 1)
	totalCh := make(chan int, 1)
	m := newInitProgressModel(progressCh, totalCh, func() {})
	m.phase = phaseCounting

	// Act
	result, _ := m.Update(progressSetTotalMsg{total: 42})
	updated := result.(initProgressModel)

	// Assert
	assert.Equal(t, phaseIndexing, updated.phase)
	assert.Equal(t, 42, updated.total)
}

func TestInitProgressModel_Update_ProgressTick_IncrementsCurrentAndSetsDir(t *testing.T) {
	t.Parallel()

	// Arrange
	progressCh := make(chan string, 1)
	totalCh := make(chan int, 1)
	m := newInitProgressModel(progressCh, totalCh, func() {})
	m.phase = phaseIndexing
	m.total = 10
	m.current = 3

	// Act
	result, _ := m.Update(progressTickMsg{dir: "some/dir"})
	updated := result.(initProgressModel)

	// Assert
	assert.Equal(t, 4, updated.current)
	assert.Equal(t, "some/dir", updated.lastDir)
}

func TestInitProgressModel_Update_Done_SetsTotalAsCurrent(t *testing.T) {
	t.Parallel()

	// Arrange
	m := newInitProgressModel(nil, nil, func() {})
	m.total = 10
	m.current = 7

	// Act
	result, _ := m.Update(progressDoneMsg{})

	// Assert
	assert.Equal(t, 10, result.(initProgressModel).current)
}

func TestInitProgressModel_Update_CtrlC_CallsCancel(t *testing.T) {
	t.Parallel()

	// Arrange
	canceled := false
	m := newInitProgressModel(nil, nil, func() { canceled = true })

	// Act
	m.Update(tea.KeyPressMsg{Text: "ctrl+c"})

	// Assert
	assert.True(t, canceled)
}

// ---- initProgressModel View ----

func TestInitProgressModel_View_CountingPhase_DoesNotPanic(t *testing.T) {
	t.Parallel()

	m := newInitProgressModel(nil, nil, func() {})
	m.phase = phaseCounting
	_ = m.View()
}

func TestInitProgressModel_View_IndexingPhaseZeroTotal_DoesNotPanic(t *testing.T) {
	t.Parallel()

	m := newInitProgressModel(nil, nil, func() {})
	m.phase = phaseIndexing
	m.total = 0
	_ = m.View()
}

func TestInitProgressModel_View_IndexingWithProgress_DoesNotPanic(t *testing.T) {
	t.Parallel()

	m := newInitProgressModel(nil, nil, func() {})
	m.phase = phaseIndexing
	m.total = 10
	m.current = 5
	m.width = 80
	_ = m.View()
}

// ---- newInitProgressModel ----

func TestNewInitProgressModel_SetsAllChannels(t *testing.T) {
	t.Parallel()

	// Arrange
	progressCh := make(chan string)
	totalCh := make(chan int)
	_, cancel := context.WithCancel(context.Background())

	// Act
	m := newInitProgressModel(progressCh, totalCh, cancel)

	// Assert
	require.NotNil(t, m.progressCh)
	require.NotNil(t, m.totalCh)
	require.NotNil(t, m.cancelFunc)
}

// ---- renderInitProgressBar ----

func TestRenderInitProgressBar_NarrowWidth_UsesMinimum(t *testing.T) {
	t.Parallel()

	m := initProgressModel{total: 10, current: 5, width: 5}
	assert.NotEmpty(t, renderInitProgressBar(m))
}

func TestRenderInitProgressBar_ZeroWidth_DefaultsTo80(t *testing.T) {
	t.Parallel()

	m := initProgressModel{total: 10, current: 5, width: 0}
	assert.NotEmpty(t, renderInitProgressBar(m))
}
