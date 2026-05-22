package cli_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"go.uber.org/mock/gomock"

	"idx/internal/adapters/handlers/cli"
	"idx/internal/core/services/daemon"
	daemonmocks "idx/internal/core/services/daemon/mocks"
)

func newDaemonAdapterEnv(t *testing.T) (*cli.DaemonServiceAdapter, *daemonmocks.MockDaemonRepository, *daemonmocks.MockProcessSpawner, *daemonmocks.MockTextOutput) {
	t.Helper()
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	repo := daemonmocks.NewMockDaemonRepository(ctrl)
	tree := daemonmocks.NewMockProjectTree(ctrl)
	output := daemonmocks.NewMockTextOutput(ctrl)
	initCmd := daemonmocks.NewMockInitCommandInterface(ctrl)
	spawner := daemonmocks.NewMockProcessSpawner(ctrl)

	svc := daemon.NewDaemonService(repo, tree, output, initCmd, spawner)
	return cli.NewDaemonServiceAdapter(svc), repo, spawner, output
}

func createAdapterIndexFile(t *testing.T, projectDir string) {
	t.Helper()
	indexDir := filepath.Join(projectDir, ".idx")
	if err := os.MkdirAll(indexDir, 0o750); err != nil {
		t.Fatalf("failed to create index directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(indexDir, "index.idx"), []byte("fake"), 0o600); err != nil {
		t.Fatalf("failed to create index file: %v", err)
	}
}

func TestDaemonAdapterEnableDelegatesToService(t *testing.T) {
	projectDir := t.TempDir()
	createAdapterIndexFile(t, projectDir)

	adapter, repo, spawner, _ := newDaemonAdapterEnv(t)
	repo.EXPECT().ReadState().Return(nil, nil).AnyTimes()
	spawner.EXPECT().SpawnWatchProcess(gomock.Any()).Return(0, errors.New("spawn failed"))

	err := adapter.Enable(projectDir)
	if err == nil {
		t.Fatal("expected error from delegated service, got nil")
	}
}

func TestDaemonAdapterDisableDelegatesToService(t *testing.T) {
	adapter, repo, _, _ := newDaemonAdapterEnv(t)
	repo.EXPECT().ReadState().Return(nil, errors.New("read error"))

	err := adapter.Disable(t.TempDir())
	if err == nil {
		t.Fatal("expected error from delegated service, got nil")
	}
}

func TestDaemonAdapterStatusDelegatesToService(t *testing.T) {
	adapter, repo, _, output := newDaemonAdapterEnv(t)
	repo.EXPECT().ReadState().Return(nil, nil).AnyTimes()
	output.EXPECT().WriteLine(gomock.Any()).AnyTimes()

	// Status surfaces results via output, not by returning errors.
	_ = adapter.Status()
}
