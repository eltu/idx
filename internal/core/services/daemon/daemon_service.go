package daemon

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"idx/internal/core/domain"
	"idx/internal/core/ports"
)

// DaemonService orchestrates the activation and management of watch processes for multiple projects.
type DaemonService struct {
	daemonRepo     ports.DaemonRepository
	projectTree    ports.ProjectTree
	output         ports.TextOutput
	initCommand    ports.InitCommandInterface
	processSpawner ports.ProcessSpawner
	processExists  func(int) bool
}

// NewDaemonService creates a new instance of the daemon service.
func NewDaemonService(
	daemonRepo ports.DaemonRepository,
	projectTree ports.ProjectTree,
	output ports.TextOutput,
	initCommand ports.InitCommandInterface,
	processSpawner ports.ProcessSpawner,
) *DaemonService {
	return NewDaemonServiceWithProcessChecker(daemonRepo, projectTree, output, initCommand, processSpawner, processRunning)
}

// NewDaemonServiceWithProcessChecker creates a new instance with injectable
// process liveness checks for deterministic tests.
func NewDaemonServiceWithProcessChecker(
	daemonRepo ports.DaemonRepository,
	projectTree ports.ProjectTree,
	output ports.TextOutput,
	initCommand ports.InitCommandInterface,
	processSpawner ports.ProcessSpawner,
	processExists func(int) bool,
) *DaemonService {
	if processExists == nil {
		processExists = processRunning
	}

	return &DaemonService{
		daemonRepo:     daemonRepo,
		projectTree:    projectTree,
		output:         output,
		initCommand:    initCommand,
		processSpawner: processSpawner,
		processExists:  processExists,
	}
}

func processRunning(pid int) bool {
	if pid <= 0 {
		return false
	}

	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	err = proc.Signal(syscall.Signal(0))
	return err == nil || errors.Is(err, syscall.EPERM)
}

// Enable activates the watch for a project. If the index does not exist, auto-init is executed.
// Idempotent: returns nil silently when the project is already being monitored with a live process.
// Returns an error only if it fails to initialize the project or start the watch.
func (s *DaemonService) Enable(projectPath string) error {
	// 1. Validate and resolve absolute path
	absPath, err := s.validateProjectPath(projectPath)
	if err != nil {
		return err
	}

	// 2. If already monitored with a running process, succeed silently (idempotent).
	state, _ := s.daemonRepo.ReadState()
	if s.isAlreadyMonitored(absPath, state) {
		return nil
	}

	// 3. Validate index existence; if missing, run auto-init
	if err := s.ensureIndexExists(absPath); err != nil {
		return err
	}

	// 4. Start watch process in background
	pid, err := s.processSpawner.SpawnWatchProcess(absPath)
	if err != nil {
		return fmt.Errorf("failed to start watch for %q: got error %v, expected process to start", absPath, err)
	}

	// 5. Register in state file
	if err := s.registerProject(absPath, pid); err != nil {
		// Kill the process if registration fails
		if proc, err := os.FindProcess(pid); err == nil {
			_ = proc.Kill()
		}
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

// validateProjectPath resolves and validates the absolute path of the project.
func (s *DaemonService) validateProjectPath(projectPath string) (string, error) {
	absPath, err := filepath.Abs(projectPath)
	if err != nil {
		return "", fmt.Errorf("invalid project path %q: got error %v, expected valid filesystem path", projectPath, err)
	}

	if _, err := os.Stat(absPath); err != nil {
		return "", fmt.Errorf("project path %q not found: got error %v, expected existing directory", absPath, err)
	}

	return absPath, nil
}

// isAlreadyMonitored reports whether the project path has a live monitoring process.
func (s *DaemonService) isAlreadyMonitored(absPath string, state *domain.DaemonState) bool {
	if state == nil {
		return false
	}

	for _, proj := range state.Projects {
		if proj.Path == absPath && proj.Enabled && s.processExists(proj.PID) {
			return true
		}
	}

	return false
}

// ensureIndexExists checks whether the index exists; if not, runs auto-init.
func (s *DaemonService) ensureIndexExists(absPath string) error {
	indexPath := filepath.Join(absPath, ".idx", "index.idx")
	if _, err := os.Stat(indexPath); err != nil {
		if err := s.output.WriteLine("ℹ️  Index not found. Creating initial index..."); err != nil {
			return err
		}
		// Run init on the project
		if err := s.initCommand.RunFromPath(absPath); err != nil {
			return err
		}
	}

	return nil
}

// Disable stops the watch for a project and removes it from the daemon state.
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

	filtered := make([]domain.MonitoredProject, 0, len(state.Projects))
	removedCount := 0
	for _, project := range state.Projects {
		if project.Path != absPath {
			filtered = append(filtered, project)
			continue
		}

		removedCount++
		if project.Enabled && project.PID > 0 {
			proc, findErr := os.FindProcess(project.PID)
			if findErr == nil {
				_ = proc.Kill()
			}
		}
	}

	if removedCount == 0 {
		return fmt.Errorf("project %q not being monitored", absPath)
	}

	state.Projects = filtered
	if err := s.daemonRepo.SaveState(state); err != nil {
		return fmt.Errorf("failed to remove project from daemon state: got error %v, expected writable state file", err)
	}

	return s.output.WriteLine(fmt.Sprintf("✅ Watch disabled for %q", absPath))
}

// Status shows all monitored projects and their status.
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
		if proj.Enabled && s.processExists(proj.PID) {
			status = "✅ running"
		}

		line := fmt.Sprintf("  %s %q (PID: %d, since %s)",
			status, proj.Path, proj.PID, proj.StartedAt.Format("15:04:05"))
		if err := s.output.WriteLine(line); err != nil {
			return err
		}
	}

	return nil
}

// registerProject adds a new monitored project to the daemon state.
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
