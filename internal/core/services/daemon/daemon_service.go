package daemon

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"idx/internal/core/domain"
	"idx/internal/core/ports"
)

// DaemonService orquestra a ativação e gerenciamento de watch processes para múltiplos projetos.
type DaemonService struct {
	daemonRepo  ports.DaemonRepository
	projectTree ports.ProjectTree
	output      ports.TextOutput
	initCommand ports.InitCommandInterface
}

// NewDaemonService cria uma nova instância do serviço daemon.
func NewDaemonService(
	daemonRepo ports.DaemonRepository,
	projectTree ports.ProjectTree,
	output ports.TextOutput,
	initCommand ports.InitCommandInterface,
) *DaemonService {
	return &DaemonService{
		daemonRepo:  daemonRepo,
		projectTree: projectTree,
		output:      output,
		initCommand: initCommand,
	}
}

// Enable ativa o watch para um projeto. Se o index não existir, auto-init é executado.
// Retorna erro apenas se não conseguir inicializar o projeto ou iniciar o watch.
func (s *DaemonService) Enable(projectPath string) error {
	// 1. Valida e resolve path absoluto
	absPath, err := filepath.Abs(projectPath)
	if err != nil {
		return fmt.Errorf("invalid project path %q: got error %v, expected valid filesystem path", projectPath, err)
	}

	if _, err := os.Stat(absPath); err != nil {
		return fmt.Errorf("project path %q not found: got error %v, expected existing directory", absPath, err)
	}

	// 2. Verifica se já está ativado
	state, _ := s.daemonRepo.ReadState()
	if state != nil {
		for _, proj := range state.Projects {
			if proj.Path == absPath && proj.Enabled {
				// Verifica se PID ainda está vivo
				if _, err := os.FindProcess(proj.PID); err == nil {
					return fmt.Errorf("project %q is already being monitored (PID: %d)", absPath, proj.PID)
				}
			}
		}
	}

	// 3. Valida se tem index, se não tem = auto-init
	indexPath := filepath.Join(absPath, ".idx", "index.gob")
	if _, err := os.Stat(indexPath); err != nil {
		if err := s.output.WriteLine("ℹ️  Index not found. Creating initial index..."); err != nil {
			return err
		}
		// Executa init no projeto
		if err := s.initCommand.RunFromPath(absPath); err != nil {
			return err
		}
	}

	// 4. Inicia processo watch em background
	cmd := exec.Command("idx", "watch", "--debounce", "750ms")
	cmd.Dir = absPath
	cmd.Stdout = nil
	cmd.Stderr = nil

	// Desacopla do terminal pai para não ser terminado junto
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start watch for %q: got error %v, expected process to start", absPath, err)
	}

	pid := cmd.Process.Pid

	// 5. Registra no state file
	if err := s.registerProject(absPath, pid); err != nil {
		_ = cmd.Process.Kill()
		return err
	}

	// 6. Feedback
	if err := s.output.WriteLine(fmt.Sprintf("✅ Watch enabled for %q", absPath)); err != nil {
		return err
	}
	if err := s.output.WriteLine(fmt.Sprintf("👀 Monitoring in realtime (PID: %d)", pid)); err != nil {
		return err
	}

	return nil
}

// Disable para o watch de um projeto e o remove do daemon state.
func (s *DaemonService) Disable(projectPath string) error {
	absPath, err := filepath.Abs(projectPath)
	if err != nil {
		return fmt.Errorf("invalid project path: got error %v, expected valid filesystem path", err)
	}

	state, err := s.daemonRepo.ReadState()
	if err != nil {
		return fmt.Errorf("daemon not initialized: got error %v, expected valid state file", err)
	}

	if state == nil || len(state.Projects) == 0 {
		return fmt.Errorf("project %q not being monitored: no projects active", absPath)
	}

	for i, proj := range state.Projects {
		if proj.Path == absPath {
			// Kill do PID
			if proj.Enabled && proj.PID > 0 {
				proc, err := os.FindProcess(proj.PID)
				if err == nil {
					_ = proc.Kill()
				}
			}

			// Remove do state
			state.Projects = append(state.Projects[:i], state.Projects[i+1:]...)
			if err := s.daemonRepo.SaveState(state); err != nil {
				return fmt.Errorf("failed to remove project from daemon state: got error %v, expected writable state file", err)
			}

			return s.output.WriteLine(fmt.Sprintf("✅ Watch disabled for %q", absPath))
		}
	}

	return fmt.Errorf("project %q not being monitored", absPath)
}

// Status mostra todos os projetos sendo monitorados e seus status.
func (s *DaemonService) Status() error {
	state, _ := s.daemonRepo.ReadState()

	if state == nil || len(state.Projects) == 0 {
		return s.output.WriteLine("❌ No projects being monitored")
	}

	if err := s.output.WriteLine("📊 Monitored Projects:"); err != nil {
		return err
	}

	for _, proj := range state.Projects {
		status := "❌ stopped"
		if proj.Enabled {
			// Verifica se PID ainda existe
			if _, err := os.FindProcess(proj.PID); err == nil {
				status = "✅ running"
			}
		}

		line := fmt.Sprintf("  %s %q (PID: %d, since %s)",
			status, proj.Path, proj.PID, proj.StartedAt.Format("15:04:05"))
		if err := s.output.WriteLine(line); err != nil {
			return err
		}
	}

	return nil
}

// registerProject adiciona um novo projeto monitorado ao daemon state.
func (s *DaemonService) registerProject(projectPath string, pid int) error {
	state, _ := s.daemonRepo.ReadState()
	if state == nil {
		state = &domain.DaemonState{Projects: []domain.MonitoredProject{}}
	}

	state.Projects = append(state.Projects, domain.MonitoredProject{
		Path:      projectPath,
		PID:       pid,
		Enabled:   true,
		StartedAt: time.Now(),
		LastSync:  time.Now(),
	})

	state.UpdatedAt = time.Now()
	return s.daemonRepo.SaveState(state)
}
