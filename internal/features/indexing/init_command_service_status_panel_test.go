package indexing

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestHumanAge_RecentTime_ReturnsJustNow(t *testing.T) {
	t.Parallel()

	// Act
	got := humanAge(time.Now().Add(-10 * time.Second))

	// Assert
	assert.Equal(t, "just now", got)
}

func TestHumanAge_FiveMinutesAgo_ReturnsMinutesLabel(t *testing.T) {
	t.Parallel()

	// Act
	got := humanAge(time.Now().Add(-5 * time.Minute))

	// Assert
	assert.Contains(t, got, "minute")
	assert.Contains(t, got, "5")
}

func TestHumanAge_ThreeHoursAgo_ReturnsHoursLabel(t *testing.T) {
	t.Parallel()

	// Act
	got := humanAge(time.Now().Add(-3 * time.Hour))

	// Assert
	assert.Contains(t, got, "hour")
}

func TestHumanAge_TwoDaysAgo_ReturnsDaysLabel(t *testing.T) {
	t.Parallel()

	// Act
	got := humanAge(time.Now().Add(-48 * time.Hour))

	// Assert
	assert.Contains(t, got, "day")
}

func TestHumanAge_OldDate_ReturnsFormattedDate(t *testing.T) {
	t.Parallel()

	// Arrange
	old := time.Now().Add(-30 * 24 * time.Hour)

	// Act
	got := humanAge(old)

	// Assert
	assert.False(t, strings.Contains(got, "ago"), "expected formatted date for >7 days, got %q", got)
}

func TestFormatBytes_Bytes_ReturnsBytes(t *testing.T) {
	t.Parallel()

	// Act
	got := formatBytes(512)

	// Assert
	assert.Equal(t, "512 B", got)
}

func TestFormatBytes_Kilobytes_ReturnsKB(t *testing.T) {
	t.Parallel()

	// Act
	got := formatBytes(2048)

	// Assert
	assert.Equal(t, "2.0 KB", got)
}

func TestFormatBytes_Megabytes_ReturnsMB(t *testing.T) {
	t.Parallel()

	// Act
	got := formatBytes(2*1024*1024 + 100*1024)

	// Assert
	assert.Contains(t, got, "MB")
}

func TestFormatBytes_Gigabytes_ReturnsGB(t *testing.T) {
	t.Parallel()

	// Act
	got := formatBytes(2 * 1024 * 1024 * 1024)

	// Assert
	assert.Equal(t, "2.0 GB", got)
}

func TestDaemonStatusLine_NilRepo_ReturnsNotConfigured(t *testing.T) {
	t.Parallel()

	// Act
	got := daemonStatusLine(nil, "/some/project")

	// Assert
	assert.Contains(t, got, "not configured")
}

func TestDaemonStatusLine_NoMatchingProject_ReturnsNotConfigured(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := &panelFakeDaemonRepo{statuses: map[string]*DaemonProjectStatus{
		"/other/project": {Enabled: true, PID: 1234},
	}}

	// Act
	got := daemonStatusLine(repo, "/my/project")

	// Assert
	assert.Contains(t, got, "not configured")
}

func TestDaemonStatusLine_DisabledProject_ReturnsDisabled(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := &panelFakeDaemonRepo{statuses: map[string]*DaemonProjectStatus{
		"/my/project": {Enabled: false, PID: 1234},
	}}

	// Act
	got := daemonStatusLine(repo, "/my/project")

	// Assert
	assert.Contains(t, got, "disabled")
}

func TestDaemonStatusLine_ActiveProject_ReturnsWatchingAndPID(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := &panelFakeDaemonRepo{statuses: map[string]*DaemonProjectStatus{
		"/my/project": {Enabled: true, PID: 4821, StartedAt: time.Now()},
	}}

	// Act
	got := daemonStatusLine(repo, "/my/project")

	// Assert
	assert.Contains(t, got, "watching")
	assert.Contains(t, got, "4821")
}

func TestBuildStatusPanelContent_ContainsProjectName(t *testing.T) {
	t.Parallel()

	// Arrange
	data := statusPanelData{
		projectRoot: "/home/user/my-project",
		summary:     statusSummary{CheckedFiles: 10, CheckedDirectories: 2, HasLatest: false},
		directories: []string{},
		indexStatus: "✅ up to date",
	}

	// Act
	got := buildStatusPanelContent(nil, data)

	// Assert
	assert.Contains(t, got, "my-project")
	assert.Contains(t, got, "up to date")
}

func TestBuildStatusPanelContent_IncludesConfigWhenProvided(t *testing.T) {
	t.Parallel()

	// Arrange
	data := statusPanelData{
		projectRoot:     "/home/user/proj",
		summary:         statusSummary{},
		directories:     []string{},
		indexStatus:     "✅ up to date",
		configFilePath:  "/home/user/proj/.idx.yml",
		configOverrides: []string{"search.format", "bm25.k1"},
	}

	// Act
	got := buildStatusPanelContent(nil, data)

	// Assert
	assert.Contains(t, got, ".idx.yml")
	assert.Contains(t, got, "2 overrides")
}

func TestBuildStatusPanelContent_OmitsConfigWhenAbsent(t *testing.T) {
	t.Parallel()

	// Arrange
	data := statusPanelData{
		projectRoot: "/home/user/proj",
		summary:     statusSummary{},
		directories: []string{},
		indexStatus: "✅ up to date",
	}

	// Act
	got := buildStatusPanelContent(nil, data)

	// Assert
	assert.NotContains(t, got, "Config")
}

func TestIndexTotalSizeBytes_EmptyDirectories_ReturnsZero(t *testing.T) {
	t.Parallel()

	// Act
	got := indexTotalSizeBytes([]string{})

	// Assert
	assert.Equal(t, int64(0), got)
}

func TestIndexTotalSizeBytes_MissingFiles_ReturnsZero(t *testing.T) {
	t.Parallel()

	// Act
	got := indexTotalSizeBytes([]string{"/nonexistent/path"})

	// Assert
	assert.Equal(t, int64(0), got)
}

// panelFakeDaemonRepo implements ProjectMonitorChecker for status panel tests.
type panelFakeDaemonRepo struct {
	statuses map[string]*DaemonProjectStatus
	err      error
}

func (f *panelFakeDaemonRepo) IsProjectMonitored(projectRoot string) (bool, error) {
	if f.err != nil {
		return false, f.err
	}
	status, ok := f.statuses[projectRoot]
	return ok && status != nil && status.Enabled, nil
}

func (f *panelFakeDaemonRepo) ProjectStatus(projectRoot string) (*DaemonProjectStatus, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.statuses[projectRoot], nil
}

var _ ProjectMonitorChecker = (*panelFakeDaemonRepo)(nil)
