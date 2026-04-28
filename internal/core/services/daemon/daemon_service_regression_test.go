package daemon_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"idx/internal/core/services/daemon"
)

// TestDaemonRegressionEnableDisableEnableCycle testa ciclo Enable-Disable-Enable
func TestDaemonRegressionEnableDisableEnableCycle(t *testing.T) {
	p1 := t.TempDir()

	repo := &fakeDaemonRepository{}
	tree := &fakeProjectTree{
		currentDir: p1,
		existsMap: map[string]bool{
			p1:                                     true,
			filepath.Join(p1, ".idx", "index.gob"): true,
		},
	}
	output := &fakeTextOutput{}
	initCmd := &fakeInitCommand{}

	service := daemon.NewDaemonService(repo, tree, output, initCmd)

	// Enable
	if err := service.Enable(p1); err != nil {
		t.Fatalf("enable failed: %v", err)
	}

	state, _ := repo.ReadState()
	if len(state.Projects) != 1 || !state.Projects[0].Enabled {
		t.Fatal("expected 1 enabled project after enable")
	}

	// Disable
	if err := service.Disable(p1); err != nil {
		t.Fatalf("disable failed: %v", err)
	}

	state, _ = repo.ReadState()
	if len(state.Projects) != 0 {
		t.Fatal("expected 0 projects after disable")
	}

	// Enable novamente
	if err := service.Enable(p1); err != nil {
		t.Fatalf("second enable failed: %v", err)
	}

	state, _ = repo.ReadState()
	if len(state.Projects) != 1 {
		t.Fatal("expected 1 project after second enable")
	}
}

// TestDaemonRegressionMultipleProjectsSelective testa seleção em múltiplos projetos
func TestDaemonRegressionMultipleProjectsSelective(t *testing.T) {
	p1 := t.TempDir()
	p2 := t.TempDir()
	p3 := t.TempDir()

	repo := &fakeDaemonRepository{}
	tree := &fakeProjectTree{
		currentDir: p1,
		existsMap: map[string]bool{
			p1:                                     true,
			filepath.Join(p1, ".idx", "index.gob"): true,
			p2:                                     true,
			filepath.Join(p2, ".idx", "index.gob"): true,
			p3:                                     true,
			filepath.Join(p3, ".idx", "index.gob"): true,
		},
	}
	output := &fakeTextOutput{}
	initCmd := &fakeInitCommand{}

	service := daemon.NewDaemonService(repo, tree, output, initCmd)

	// Enable 3 projetos
	if err := service.Enable(p1); err != nil {
		t.Fatalf("enable p1 failed: %v", err)
	}
	if err := service.Enable(p2); err != nil {
		t.Fatalf("enable p2 failed: %v", err)
	}
	if err := service.Enable(p3); err != nil {
		t.Fatalf("enable p3 failed: %v", err)
	}

	state, _ := repo.ReadState()
	if len(state.Projects) != 3 {
		t.Fatalf("expected 3 projects, got %d", len(state.Projects))
	}

	// Disable projeto do meio
	if err := service.Disable(p2); err != nil {
		t.Fatalf("disable p2 failed: %v", err)
	}

	state, _ = repo.ReadState()
	if len(state.Projects) != 2 {
		t.Fatalf("expected 2 projects after disabling p2, got %d", len(state.Projects))
	}

	// Verificar que p1 e p3 ainda estão lá
	paths := map[string]bool{}
	for _, p := range state.Projects {
		paths[p.Path] = true
	}
	if !paths[p1] || !paths[p3] {
		t.Fatal("expected p1 and p3 to still be monitored")
	}
	if paths[p2] {
		t.Fatal("expected p2 to be removed")
	}
}

// TestDaemonRegressionInitCalledOnlyWhenNeeded testa que projetos com index não chamam init
func TestDaemonRegressionInitCalledOnlyWhenNeeded(t *testing.T) {
	p1 := t.TempDir()
	p2 := t.TempDir()

	// Criar índice em p2 para que não precise de init
	indexDir := filepath.Join(p2, ".idx")
	os.MkdirAll(indexDir, 0750)
	os.WriteFile(filepath.Join(indexDir, "index.gob"), []byte("fake index"), 0600)

	repo := &fakeDaemonRepository{}
	tree := &fakeProjectTree{
		currentDir: p1,
		existsMap: map[string]bool{
			p1: true,
			p2: true,
		},
	}
	output := &fakeTextOutput{}
	initCmd := &fakeInitCommand{}

	service := daemon.NewDaemonService(repo, tree, output, initCmd)

	// Enable p2 (com index real, não deve chamar init)
	if err := service.Enable(p2); err != nil {
		t.Fatalf("enable p2 failed: %v", err)
	}

	if initCmd.callCount != 0 {
		t.Fatalf("expected init not to be called for p2, but was called %d times", initCmd.callCount)
	}
}

// TestDaemonRegressionStatusConsistency verifica consistência de status
func TestDaemonRegressionStatusConsistency(t *testing.T) {
	p1 := t.TempDir()

	repo := &fakeDaemonRepository{}
	tree := &fakeProjectTree{
		currentDir: p1,
		existsMap: map[string]bool{
			p1:                                     true,
			filepath.Join(p1, ".idx", "index.gob"): true,
		},
	}
	output := &fakeTextOutput{}
	initCmd := &fakeInitCommand{}

	service := daemon.NewDaemonService(repo, tree, output, initCmd)

	// Enable
	if err := service.Enable(p1); err != nil {
		t.Fatalf("enable failed: %v", err)
	}

	// Status após enable
	output.lines = []string{}
	service.Status()

	if len(output.lines) < 2 {
		t.Fatalf("expected at least 2 lines in status output, got %d", len(output.lines))
	}

	// Disable
	if err := service.Disable(p1); err != nil {
		t.Fatalf("disable failed: %v", err)
	}

	// Status após disable
	output.lines = []string{}
	service.Status()

	if len(output.lines) < 1 {
		t.Fatalf("expected at least 1 line in status output after disable, got %d", len(output.lines))
	}
}

// TestDaemonRegressionPathNormalization verifica normalização de paths
func TestDaemonRegressionPathNormalization(t *testing.T) {
	p1 := t.TempDir()

	repo := &fakeDaemonRepository{}
	tree := &fakeProjectTree{
		currentDir: p1,
		gitRoot:    p1,
		existsMap: map[string]bool{
			p1:                                     true,
			filepath.Join(p1, ".idx", "index.gob"): true,
		},
	}
	output := &fakeTextOutput{}
	initCmd := &fakeInitCommand{}

	service := daemon.NewDaemonService(repo, tree, output, initCmd)

	// Enable com caminho absoluto
	if err := service.Enable(p1); err != nil {
		t.Fatalf("enable failed: %v", err)
	}

	state, _ := repo.ReadState()
	if state.Projects[0].Path != p1 {
		t.Fatalf("expected path %s, got %s", p1, state.Projects[0].Path)
	}
}

// TestDaemonRegressionProjectPreservation verifica preservação de projetos
func TestDaemonRegressionProjectPreservation(t *testing.T) {
	p1 := t.TempDir()
	p2 := t.TempDir()

	repo := &fakeDaemonRepository{}
	tree := &fakeProjectTree{
		currentDir: p1,
		existsMap: map[string]bool{
			p1:                                     true,
			filepath.Join(p1, ".idx", "index.gob"): true,
			p2:                                     true,
			filepath.Join(p2, ".idx", "index.gob"): true,
		},
	}
	output := &fakeTextOutput{}
	initCmd := &fakeInitCommand{}

	service := daemon.NewDaemonService(repo, tree, output, initCmd)

	// Enable p1
	if err := service.Enable(p1); err != nil {
		t.Fatalf("enable p1 failed: %v", err)
	}

	// Enable p2 (não deve afetar p1)
	if err := service.Enable(p2); err != nil {
		t.Fatalf("enable p2 failed: %v", err)
	}

	// Verificar que p1 ainda existe
	state2, _ := repo.ReadState()
	if len(state2.Projects) != 2 {
		t.Fatalf("expected 2 projects, got %d", len(state2.Projects))
	}

	// Verificar que p1 está no novo estado
	found := false
	for _, p := range state2.Projects {
		if p.Path == p1 {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected p1 to be preserved in state")
	}
}

// TestDaemonRegressionRepositoryIntegration verifica integração com repository
func TestDaemonRegressionRepositoryIntegration(t *testing.T) {
	p1 := t.TempDir()

	// Primeiro repository
	repo1 := &fakeDaemonRepository{}
	tree := &fakeProjectTree{
		currentDir: p1,
		existsMap: map[string]bool{
			p1:                                     true,
			filepath.Join(p1, ".idx", "index.gob"): true,
		},
	}
	output := &fakeTextOutput{}
	initCmd := &fakeInitCommand{}

	service1 := daemon.NewDaemonService(repo1, tree, output, initCmd)

	if err := service1.Enable(p1); err != nil {
		t.Fatalf("enable failed: %v", err)
	}

	// Criar novo service com mesmo repository
	service2 := daemon.NewDaemonService(repo1, tree, output, initCmd)

	// Novo service deve ver o estado salvo
	state, _ := repo1.ReadState()
	if len(state.Projects) != 1 {
		t.Fatalf("expected 1 project in repository, got %d", len(state.Projects))
	}

	// Status do novo service deve refletir projeto existente
	output.lines = []string{}
	service2.Status()

	if len(output.lines) < 2 {
		t.Fatalf("expected status output to show existing project, got %d lines", len(output.lines))
	}
}

// TestDaemonRegressionInitFailureRecovery verifica que projeto com índice funciona mesmo com init falhando
func TestDaemonRegressionInitFailureRecovery(t *testing.T) {
	p1 := t.TempDir()
	p2 := t.TempDir()

	// Criar índice em p2 para que não preciseaprendiz de init
	indexDir := filepath.Join(p2, ".idx")
	os.MkdirAll(indexDir, 0750)
	os.WriteFile(filepath.Join(indexDir, "index.gob"), []byte("fake index"), 0600)

	repo := &fakeDaemonRepository{}
	tree := &fakeProjectTree{
		currentDir: p1,
		existsMap: map[string]bool{
			p1: true,
			p2: true,
		},
	}
	output := &fakeTextOutput{}

	// Init command que falha (não deve afetar p2 que tem index)
	initCmd := &fakeInitCommand{returnError: fmt.Errorf("init failed")}

	service := daemon.NewDaemonService(repo, tree, output, initCmd)

	// Enable p2 (com index real, init não é chamado)
	if err := service.Enable(p2); err != nil {
		t.Fatalf("enable p2 with existing index should succeed, got %v", err)
	}

	if initCmd.callCount != 0 {
		t.Fatalf("expected init not to be called for p2, got %d calls", initCmd.callCount)
	}

	state, _ := repo.ReadState()
	if state == nil || len(state.Projects) != 1 {
		t.Fatalf("expected 1 project after enable")
	}
}

// TestDaemonRegressionConcurrentStateUpdates verifica atualizações de estado
func TestDaemonRegressionConcurrentStateUpdates(t *testing.T) {
	p1 := t.TempDir()
	p2 := t.TempDir()

	repo := &fakeDaemonRepository{}
	tree := &fakeProjectTree{
		currentDir: p1,
		existsMap: map[string]bool{
			p1:                                     true,
			filepath.Join(p1, ".idx", "index.gob"): true,
			p2:                                     true,
			filepath.Join(p2, ".idx", "index.gob"): true,
		},
	}
	output := &fakeTextOutput{}
	initCmd := &fakeInitCommand{}

	service := daemon.NewDaemonService(repo, tree, output, initCmd)

	// Sequência de operações
	if err := service.Enable(p1); err != nil {
		t.Fatalf("enable p1 failed: %v", err)
	}

	if err := service.Enable(p2); err != nil {
		t.Fatalf("enable p2 failed: %v", err)
	}

	state1, _ := repo.ReadState()
	if len(state1.Projects) != 2 {
		t.Fatalf("expected 2 projects after two enables, got %d", len(state1.Projects))
	}

	if err := service.Disable(p1); err != nil {
		t.Fatalf("disable p1 failed: %v", err)
	}

	state2, _ := repo.ReadState()
	if len(state2.Projects) != 1 || state2.Projects[0].Path != p2 {
		t.Fatalf("expected only p2 after disabling p1")
	}
}

// TestDaemonRegressionStatusFormatting verifica formatação de status
func TestDaemonRegressionStatusFormatting(t *testing.T) {
	repo := &fakeDaemonRepository{}
	tree := &fakeProjectTree{currentDir: "/tmp"}
	output := &fakeTextOutput{}
	initCmd := &fakeInitCommand{}

	service := daemon.NewDaemonService(repo, tree, output, initCmd)

	// Status com zero projetos
	service.Status()
	if len(output.lines) < 1 {
		t.Fatal("expected at least one line in empty status")
	}

	emptyMsg := output.lines[0]
	if emptyMsg == "" {
		t.Fatal("expected non-empty message for empty status")
	}
}

// TestDaemonRegressionErrorMessages verifica mensagens de erro
func TestDaemonRegressionErrorMessages(t *testing.T) {
	p1 := t.TempDir()

	repo := &fakeDaemonRepository{}
	tree := &fakeProjectTree{
		currentDir: p1,
		existsMap: map[string]bool{
			p1:                                     true,
			filepath.Join(p1, ".idx", "index.gob"): true,
		},
	}
	output := &fakeTextOutput{}
	initCmd := &fakeInitCommand{}

	service := daemon.NewDaemonService(repo, tree, output, initCmd)

	// Enable primeiro projeto
	if err := service.Enable(p1); err != nil {
		t.Fatalf("enable failed: %v", err)
	}

	// Tentar enable novamente
	err := service.Enable(p1)
	if err == nil {
		t.Fatal("expected error when enabling already monitored project")
	}

	if len(err.Error()) == 0 {
		t.Fatal("expected error message not to be empty")
	}

	// Tentar disable de projeto não monitorado
	err = service.Disable("/nonexistent")
	if err == nil {
		t.Fatal("expected error when disabling non-existent project")
	}
}
