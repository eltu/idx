package indexing

import (
	"strings"
	"testing"
	"time"

	"idx/internal/core/domain"
	"idx/internal/core/ports"
)

func TestHumanAgeJustNow(t *testing.T) {
	got := humanAge(time.Now().Add(-10 * time.Second))
	if got != "just now" {
		t.Fatalf("expected 'just now' for <1m, got %q", got)
	}
}

func TestHumanAgeMinutes(t *testing.T) {
	got := humanAge(time.Now().Add(-5 * time.Minute))
	if !strings.Contains(got, "minute") {
		t.Fatalf("expected minutes label, got %q", got)
	}
	if !strings.Contains(got, "5") {
		t.Fatalf("expected 5 in minutes output, got %q", got)
	}
}

func TestHumanAgeHours(t *testing.T) {
	got := humanAge(time.Now().Add(-3 * time.Hour))
	if !strings.Contains(got, "hour") {
		t.Fatalf("expected hours label, got %q", got)
	}
}

func TestHumanAgeDays(t *testing.T) {
	got := humanAge(time.Now().Add(-48 * time.Hour))
	if !strings.Contains(got, "day") {
		t.Fatalf("expected days label, got %q", got)
	}
}

func TestHumanAgeOldDateReturnsFormatted(t *testing.T) {
	old := time.Now().Add(-30 * 24 * time.Hour)
	got := humanAge(old)
	if strings.Contains(got, "ago") {
		t.Fatalf("expected formatted date for >7 days, got %q", got)
	}
}

func TestFormatBytesBytes(t *testing.T) {
	got := formatBytes(512)
	if got != "512 B" {
		t.Fatalf("expected '512 B', got %q", got)
	}
}

func TestFormatBytesKB(t *testing.T) {
	got := formatBytes(2048)
	if got != "2.0 KB" {
		t.Fatalf("expected '2.0 KB', got %q", got)
	}
}

func TestFormatBytesMB(t *testing.T) {
	got := formatBytes(2*1024*1024 + 100*1024)
	if !strings.Contains(got, "MB") {
		t.Fatalf("expected MB suffix, got %q", got)
	}
}

func TestFormatBytesGB(t *testing.T) {
	got := formatBytes(2 * 1024 * 1024 * 1024)
	if got != "2.0 GB" {
		t.Fatalf("expected '2.0 GB', got %q", got)
	}
}

func TestDaemonStatusLineNilRepo(t *testing.T) {
	got := daemonStatusLine(nil, "/some/project")
	if !strings.Contains(got, "not configured") {
		t.Fatalf("expected 'not configured' for nil repo, got %q", got)
	}
}

func TestDaemonStatusLineNoMatchingProject(t *testing.T) {
	repo := &fakeDaemonRepo{state: &domain.DaemonState{
		Projects: []domain.MonitoredProject{
			{Path: "/other/project", Enabled: true, PID: 1234},
		},
	}}
	got := daemonStatusLine(repo, "/my/project")
	if !strings.Contains(got, "not configured") {
		t.Fatalf("expected 'not configured' for no matching project, got %q", got)
	}
}

func TestDaemonStatusLineDisabledProject(t *testing.T) {
	repo := &fakeDaemonRepo{state: &domain.DaemonState{
		Projects: []domain.MonitoredProject{
			{Path: "/my/project", Enabled: false, PID: 1234},
		},
	}}
	got := daemonStatusLine(repo, "/my/project")
	if !strings.Contains(got, "disabled") {
		t.Fatalf("expected 'disabled' for disabled project, got %q", got)
	}
}

func TestDaemonStatusLineActiveProject(t *testing.T) {
	repo := &fakeDaemonRepo{state: &domain.DaemonState{
		Projects: []domain.MonitoredProject{
			{Path: "/my/project", Enabled: true, PID: 4821, StartedAt: time.Now()},
		},
	}}
	got := daemonStatusLine(repo, "/my/project")
	if !strings.Contains(got, "watching") {
		t.Fatalf("expected 'watching' for active project, got %q", got)
	}
	if !strings.Contains(got, "4821") {
		t.Fatalf("expected PID 4821 in output, got %q", got)
	}
}

func TestBuildStatusPanelContentContainsProjectName(t *testing.T) {
	data := statusPanelData{
		projectRoot: "/home/user/my-project",
		summary:     statusSummary{CheckedFiles: 10, CheckedDirectories: 2, HasLatest: false},
		directories: []string{},
		indexStatus: "✅ up to date",
	}
	got := buildStatusPanelContent(nil, data)
	if !strings.Contains(got, "my-project") {
		t.Fatalf("expected project name in panel, got %q", got)
	}
	if !strings.Contains(got, "up to date") {
		t.Fatalf("expected index status in panel, got %q", got)
	}
}

func TestBuildStatusPanelContentIncludesConfigWhenProvided(t *testing.T) {
	data := statusPanelData{
		projectRoot:     "/home/user/proj",
		summary:         statusSummary{},
		directories:     []string{},
		indexStatus:     "✅ up to date",
		configFilePath:  "/home/user/proj/.idx.yml",
		configOverrides: []string{"search.format", "bm25.k1"},
	}
	got := buildStatusPanelContent(nil, data)
	if !strings.Contains(got, ".idx.yml") {
		t.Fatalf("expected config file name in panel, got %q", got)
	}
	if !strings.Contains(got, "2 overrides") {
		t.Fatalf("expected override count in panel, got %q", got)
	}
}

func TestBuildStatusPanelContentOmitsConfigWhenAbsent(t *testing.T) {
	data := statusPanelData{
		projectRoot: "/home/user/proj",
		summary:     statusSummary{},
		directories: []string{},
		indexStatus: "✅ up to date",
	}
	got := buildStatusPanelContent(nil, data)
	if strings.Contains(got, "Config") {
		t.Fatalf("expected no Config row when configFilePath is empty, got %q", got)
	}
}

func TestIndexTotalSizeBytesEmptyDirectories(t *testing.T) {
	got := indexTotalSizeBytes([]string{})
	if got != 0 {
		t.Fatalf("expected 0 for empty directories, got %d", got)
	}
}

func TestIndexTotalSizeBytesSkipsMissingFiles(t *testing.T) {
	got := indexTotalSizeBytes([]string{"/nonexistent/path"})
	if got != 0 {
		t.Fatalf("expected 0 for missing index file, got %d", got)
	}
}

// fakeDaemonRepo implements ports.DaemonRepository for tests.
type fakeDaemonRepo struct {
	state *domain.DaemonState
	err   error
}

func (f *fakeDaemonRepo) ReadState() (*domain.DaemonState, error) {
	return f.state, f.err
}

func (f *fakeDaemonRepo) SaveState(_ *domain.DaemonState) error { return nil }

func (f *fakeDaemonRepo) UpdateProjectPID(_ string, _ int) error { return nil }

var _ ports.DaemonRepository = (*fakeDaemonRepo)(nil)
