package daemon

import (
	"idx/internal/features/indexing"
	"idx/internal/shared/filesystem"
	"idx/internal/shared/output"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"

	
	
)

// DaemonService orchestrates the activation and management of watch processes for multiple projects.
type DaemonService struct {
	daemonRepo     Repository
	projectTree    filesystem.ProjectTree
	output         output.Writer
	initCommand    indexing.CommandInterface
	processSpawner ProcessSpawner
	processExists  func(int) bool
}

// NewDaemonService creates a new instance of the daemon service.
func NewDaemonService(
	daemonRepo Repository,
	projectTree filesystem.ProjectTree,
	output output.Writer,
	initCommand indexing.CommandInterface,
	processSpawner ProcessSpawner,
) *DaemonService {
	return NewDaemonServiceWithProcessChecker(daemonRepo, projectTree, output, initCommand, processSpawner, processRunning)
}

// NewDaemonServiceWithProcessChecker creates a new instance with injectable
// process liveness checks for deterministic tests.
func NewDaemonServiceWithProcessChecker(
	daemonRepo Repository,
	projectTree filesystem.ProjectTree,
	output output.Writer,
	initCommand indexing.CommandInterface,
	processSpawner ProcessSpawner,
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
	absPath, err := s.validateProjectPath(projectPath)
	if err != nil {
		return err
	}
	state, _ := s.daemonRepo.ReadState()
	if s.isAlreadyMonitored(absPath, state) {
		return nil
	}
	if err := s.ensureIndexExists(absPath); err != nil {
		return err
	}
	pid, err := s.spawnAndRegister(absPath)
	if err != nil {
		return err
	}
	return s.outputEnableSuccess(absPath, pid)
}

// spawnAndRegister starts the watch process and persists it to the state file.
// Kills the process if state registration fails to avoid orphaned processes.
func (s *DaemonService) spawnAndRegister(absPath string) (int, error) {
	pid, err := s.processSpawner.SpawnWatchProcess(absPath)
	if err != nil {
		return 0, fmt.Errorf("failed to start watch for %q: got error %v, expected process to start", absPath, err)
	}
	if err := s.registerProject(absPath, pid); err != nil {
		if proc, findErr := os.FindProcess(pid); findErr == nil {
			_ = proc.Kill()
		}
		return 0, err
	}
	return pid, nil
}

func (s *DaemonService) outputEnableSuccess(absPath string, pid int) error {
	if err := s.output.WriteLine(fmt.Sprintf("✅ Watch enabled for %q", absPath)); err != nil {
		return err
	}
	return s.output.WriteLine(fmt.Sprintf("👀 Monitoring in realtime (PID: %d)", pid))
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
func (s *DaemonService) isAlreadyMonitored(absPath string, state *State) bool {
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
	state, err := s.loadNonEmptyState(absPath)
	if err != nil {
		return err
	}
	filtered, removed := removeProjectFromState(state.Projects, absPath)
	if removed == 0 {
		return fmt.Errorf("project %q not being monitored", absPath)
	}
	state.Projects = filtered
	if err := s.daemonRepo.SaveState(state); err != nil {
		return fmt.Errorf("failed to remove project from daemon state: got error %v, expected writable state file", err)
	}
	return s.output.WriteLine(fmt.Sprintf("✅ Watch disabled for %q", absPath))
}

func (s *DaemonService) loadNonEmptyState(absPath string) (*State, error) {
	state, err := s.daemonRepo.ReadState()
	if err != nil {
		return nil, fmt.Errorf("daemon not initialized: got error %v, expected valid state file", err)
	}
	if state == nil || len(state.Projects) == 0 {
		return nil, fmt.Errorf("project %q not being monitored: no projects active", absPath)
	}
	return state, nil
}

func removeProjectFromState(projects []MonitoredProject, absPath string) ([]MonitoredProject, int) {
	filtered := make([]MonitoredProject, 0, len(projects))
	removed := 0
	for _, project := range projects {
		if project.Path != absPath {
			filtered = append(filtered, project)
			continue
		}
		removed++
		killProjectProcess(project)
	}
	return filtered, removed
}

func killProjectProcess(project MonitoredProject) {
	if !project.Enabled || project.PID <= 0 {
		return
	}
	if proc, err := os.FindProcess(project.PID); err == nil {
		_ = proc.Kill()
	}
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
		state = &State{Projects: []MonitoredProject{}}
	}

	state.Projects = append(state.Projects, MonitoredProject{
		Path:      projectPath,
		PID:       pid,
		Enabled:   true,
		StartedAt: time.Now(),
		LastSync:  time.Now(),
	})

	state.UpdatedAt = time.Now()
	return s.daemonRepo.SaveState(state)
}

// SetInitCommand wires the init command after construction, breaking the DI cycle:
// DaemonService → InitCommand → DaemonService (as ProjectMonitorChecker).
func (s *DaemonService) SetInitCommand(cmd indexing.CommandInterface) {
	s.initCommand = cmd
}

// IsProjectMonitored implements indexing.ProjectMonitorChecker.
func (s *DaemonService) IsProjectMonitored(projectRoot string) (bool, error) {
	state, err := s.daemonRepo.ReadState()
	if err != nil || state == nil {
		return false, err
	}
	for _, p := range state.Projects {
		if p.Enabled && p.Path == projectRoot {
			return true, nil
		}
	}
	return false, nil
}

// ProjectStatus implements indexing.ProjectMonitorChecker for the status panel.
func (s *DaemonService) ProjectStatus(projectRoot string) (*indexing.DaemonProjectStatus, error) {
	state, err := s.daemonRepo.ReadState()
	if err != nil || state == nil {
		return nil, err
	}
	for _, p := range state.Projects {
		if p.Path == projectRoot {
			return &indexing.DaemonProjectStatus{
				Enabled:   p.Enabled,
				PID:       p.PID,
				StartedAt: p.StartedAt,
			}, nil
		}
	}
	return nil, nil
}
