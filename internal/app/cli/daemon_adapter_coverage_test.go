package cli

import (
	"path/filepath"
	"testing"
	"time"

	"go.uber.org/mock/gomock"

	"idx/internal/features/daemon"
	"idx/internal/features/daemon/mocks"
)

func newTestDaemonService(t *testing.T, repo *mocks.MockDaemonRepository, output *mocks.MockTextOutput) *daemon.DaemonService {
	t.Helper()
	return daemon.NewDaemonServiceWithProcessChecker(
		repo, nil, output, nil, nil,
		func(int) bool { return true },
	)
}

// ---- DaemonServiceAdapter.Status ----

func TestDaemonServiceAdapterStatusNoProjects(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := mocks.NewMockDaemonRepository(ctrl)
	out := mocks.NewMockTextOutput(ctrl)
	repo.EXPECT().ReadState().Return(nil, nil)
	out.EXPECT().WriteLine(gomock.Any()).Return(nil)

	adapter := NewDaemonServiceAdapter(newTestDaemonService(t, repo, out))
	if err := adapter.Status(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDaemonServiceAdapterStatusWithProjects(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := mocks.NewMockDaemonRepository(ctrl)
	out := mocks.NewMockTextOutput(ctrl)

	state := &daemon.State{
		Projects: []daemon.MonitoredProject{
			{Path: "/some/project", PID: 1234, Enabled: true, StartedAt: time.Now()},
		},
	}
	repo.EXPECT().ReadState().Return(state, nil)
	out.EXPECT().WriteLine(gomock.Any()).Return(nil).Times(2)

	adapter := NewDaemonServiceAdapter(newTestDaemonService(t, repo, out))
	if err := adapter.Status(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ---- DaemonServiceAdapter.Disable ----

func TestDaemonServiceAdapterDisableNoProjects(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := mocks.NewMockDaemonRepository(ctrl)
	out := mocks.NewMockTextOutput(ctrl)
	// nil state → "no projects active" → error
	repo.EXPECT().ReadState().Return(nil, nil)

	adapter := NewDaemonServiceAdapter(newTestDaemonService(t, repo, out))
	err := adapter.Disable(".")
	if err == nil {
		t.Fatal("expected error when no projects are monitored")
	}
}

func TestDaemonServiceAdapterDisableSuccess(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	projectDir := t.TempDir()
	absPath, _ := filepath.Abs(projectDir)

	repo := mocks.NewMockDaemonRepository(ctrl)
	out := mocks.NewMockTextOutput(ctrl)

	state := &daemon.State{
		Projects: []daemon.MonitoredProject{
			{Path: absPath, PID: 0, Enabled: false},
		},
	}
	repo.EXPECT().ReadState().Return(state, nil)
	repo.EXPECT().SaveState(gomock.Any()).Return(nil)
	out.EXPECT().WriteLine(gomock.Any()).Return(nil)

	adapter := NewDaemonServiceAdapter(newTestDaemonService(t, repo, out))
	if err := adapter.Disable(projectDir); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ---- DaemonServiceAdapter.Enable ----

func TestDaemonServiceAdapterEnableAlreadyMonitored(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	projectDir := t.TempDir()
	absPath, _ := filepath.Abs(projectDir)

	repo := mocks.NewMockDaemonRepository(ctrl)
	out := mocks.NewMockTextOutput(ctrl)

	state := &daemon.State{
		Projects: []daemon.MonitoredProject{
			{Path: absPath, PID: 9999, Enabled: true, StartedAt: time.Now()},
		},
	}
	repo.EXPECT().ReadState().Return(state, nil)

	adapter := NewDaemonServiceAdapter(newTestDaemonService(t, repo, out))
	if err := adapter.Enable(projectDir); err != nil {
		t.Fatalf("expected no error for already-monitored project, got %v", err)
	}
}

func TestDaemonServiceAdapterEnableInvalidPath(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := mocks.NewMockDaemonRepository(ctrl)
	out := mocks.NewMockTextOutput(ctrl)
	// validateProjectPath fails before ReadState is called — no expectations needed.

	adapter := NewDaemonServiceAdapter(newTestDaemonService(t, repo, out))
	err := adapter.Enable("/nonexistent/path/xyz/abc")
	if err == nil {
		t.Fatal("expected error for nonexistent path")
	}
}
