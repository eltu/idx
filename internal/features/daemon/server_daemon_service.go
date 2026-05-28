package daemon

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"idx/internal/features/indexing"
	"idx/internal/shared/output"
)

// ServerDaemonService manages the lifecycle of the idx server as a per-project daemon.
type ServerDaemonService struct {
	stateRepo     StateRepository
	spawner       ServerSpawner
	output        output.Writer
	processExists func(int) bool
}

// NewServerDaemonService creates a ServerDaemonService with real OS process checks.
// Example: svc := NewServerDaemonService(repo, spawner, writer); svc.Start(".").
func NewServerDaemonService(
	stateRepo StateRepository,
	spawner ServerSpawner,
	out output.Writer,
) *ServerDaemonService {
	return NewServerDaemonServiceWithProcessChecker(stateRepo, spawner, out, serverProcessRunning)
}

// NewServerDaemonServiceWithProcessChecker creates a service with an injectable liveness check.
func NewServerDaemonServiceWithProcessChecker(
	stateRepo StateRepository,
	spawner ServerSpawner,
	out output.Writer,
	processExists func(int) bool,
) *ServerDaemonService {
	if processExists == nil {
		processExists = serverProcessRunning
	}
	return &ServerDaemonService{
		stateRepo:     stateRepo,
		spawner:       spawner,
		output:        out,
		processExists: processExists,
	}
}

func serverProcessRunning(pid int) bool {
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

// Start spawns the idx server daemon for the given project path.
// Idempotent: returns nil if the server is already running.
func (s *ServerDaemonService) Start(projectPath string) error {
	absPath, err := resolveAbsPath(projectPath)
	if err != nil {
		return err
	}

	state, _ := s.stateRepo.ReadState(absPath)
	if state != nil && s.processExists(state.PID) {
		return s.output.WriteLine(fmt.Sprintf("⚡ Server already running (PID: %d)", state.PID))
	}

	pid, err := s.spawner.SpawnServerProcess(absPath)
	if err != nil {
		return fmt.Errorf("failed to start server for %q: got error %v, expected process to start", absPath, err)
	}

	socketPath := serverSocketPath(absPath)
	newState := &ServerState{
		PID:         pid,
		StartedAt:   time.Now(),
		SocketPath:  socketPath,
		ProjectPath: absPath,
	}
	if err := s.stateRepo.SaveState(absPath, newState); err != nil {
		if proc, findErr := os.FindProcess(pid); findErr == nil {
			_ = proc.Kill()
		}
		return err
	}

	if err := s.output.WriteLine(fmt.Sprintf("✅ Server started (PID: %d)", pid)); err != nil {
		return err
	}
	return s.output.WriteLine(fmt.Sprintf("🔌 Socket: %s", socketPath))
}

// Stop sends SIGTERM to the server daemon and removes the state file.
// Returns nil if the server is not running.
func (s *ServerDaemonService) Stop(projectPath string) error {
	absPath, err := resolveAbsPath(projectPath)
	if err != nil {
		return err
	}

	state, _ := s.stateRepo.ReadState(absPath)
	if state == nil || !s.processExists(state.PID) {
		return s.output.WriteLine("ℹ️  Server is not running")
	}

	if proc, err := os.FindProcess(state.PID); err == nil {
		_ = proc.Signal(syscall.SIGTERM)
	}

	if err := s.stateRepo.RemoveState(absPath); err != nil {
		return err
	}

	return s.output.WriteLine(fmt.Sprintf("🛑 Server stopped (was PID: %d)", state.PID))
}

// Status prints the current server status for the given project path.
func (s *ServerDaemonService) Status(projectPath string) error {
	absPath, err := resolveAbsPath(projectPath)
	if err != nil {
		return err
	}

	state, _ := s.stateRepo.ReadState(absPath)
	if state == nil || !s.processExists(state.PID) {
		return s.output.WriteLine("❌ Server not running")
	}

	uptime := time.Since(state.StartedAt).Round(time.Second)
	if err := s.output.WriteLine(fmt.Sprintf("✅ Server running (PID: %d, uptime: %s)", state.PID, uptime)); err != nil {
		return err
	}
	return s.output.WriteLine(fmt.Sprintf("🔌 Socket: %s", state.SocketPath))
}

// IsProjectMonitored implements indexing.ProjectMonitorChecker.
func (s *ServerDaemonService) IsProjectMonitored(projectRoot string) (bool, error) {
	state, err := s.stateRepo.ReadState(projectRoot)
	if err != nil || state == nil {
		return false, err
	}
	return s.processExists(state.PID), nil
}

// ProjectStatus implements indexing.ProjectMonitorChecker.
func (s *ServerDaemonService) ProjectStatus(projectRoot string) (*indexing.DaemonProjectStatus, error) {
	state, err := s.stateRepo.ReadState(projectRoot)
	if err != nil || state == nil {
		return nil, err
	}
	return &indexing.DaemonProjectStatus{
		Enabled:   s.processExists(state.PID),
		PID:       state.PID,
		StartedAt: state.StartedAt,
	}, nil
}

func resolveAbsPath(projectPath string) (string, error) {
	absPath, err := filepath.Abs(projectPath)
	if err != nil {
		return "", fmt.Errorf("invalid project path %q: got error %v, expected valid filesystem path", projectPath, err)
	}
	if _, err := os.Stat(absPath); err != nil {
		return "", fmt.Errorf("project path %q not found: got error %v, expected existing directory", absPath, err)
	}
	return absPath, nil
}

// serverSocketPath derives the socket path for a project; mirrors ipc.SocketPath
// but avoids a cross-package import cycle in tests.
func serverSocketPath(projectPath string) string {
	home, _ := os.UserHomeDir()
	name := sanitizeStateSegment(filepath.Base(projectPath))
	return filepath.Join(home, ".idx", name+".sock")
}
