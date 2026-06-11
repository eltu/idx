package daemon

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"idx/internal/features/indexing"
	"idx/internal/shared/ipc"
	"idx/internal/shared/output"
)

const (
	defaultReadinessTimeout = 5 * time.Second
	socketPollInterval      = 100 * time.Millisecond
	socketDialTimeout       = 300 * time.Millisecond
	readinessTimeoutMsEnv   = "IDX_READINESS_TIMEOUT_MS"
)

// readinessTimeoutFromEnv returns the readiness timeout from IDX_READINESS_TIMEOUT_MS
// if set, otherwise returns defaultReadinessTimeout. Allows tests to reduce wait time.
func readinessTimeoutFromEnv() time.Duration {
	val := os.Getenv(readinessTimeoutMsEnv)
	if val == "" {
		return defaultReadinessTimeout
	}
	ms, err := strconv.Atoi(val)
	if err != nil || ms <= 0 {
		return defaultReadinessTimeout
	}
	return time.Duration(ms) * time.Millisecond
}

// ServerDaemonServiceDeps holds injectable dependencies for testing.
type ServerDaemonServiceDeps struct {
	StateRepo        StateRepository
	Spawner          ServerSpawner
	Output           output.Writer
	ProcessExists    func(int) bool
	IsSocketAlive    func(string) bool
	ReadinessTimeout time.Duration
}

// ServerDaemonService manages the lifecycle of the idx server as a per-project daemon.
type ServerDaemonService struct {
	stateRepo        StateRepository
	spawner          ServerSpawner
	output           output.Writer
	processExists    func(int) bool
	isSocketAlive    func(string) bool
	readinessTimeout time.Duration
}

// NewServerDaemonService creates a ServerDaemonService with real OS checks.
// Example: svc := NewServerDaemonService(repo, spawner, writer); svc.Start(".").
func NewServerDaemonService(
	stateRepo StateRepository,
	spawner ServerSpawner,
	out output.Writer,
) *ServerDaemonService {
	return newServerDaemonService(ServerDaemonServiceDeps{
		StateRepo: stateRepo,
		Spawner:   spawner,
		Output:    out,
	})
}

// NewServerDaemonServiceWithDeps creates a service with fully injectable deps for testing.
func NewServerDaemonServiceWithDeps(deps ServerDaemonServiceDeps) *ServerDaemonService {
	return newServerDaemonService(deps)
}

func newServerDaemonService(deps ServerDaemonServiceDeps) *ServerDaemonService {
	if deps.ProcessExists == nil {
		deps.ProcessExists = serverProcessRunning
	}
	if deps.IsSocketAlive == nil {
		deps.IsSocketAlive = defaultSocketAlive
	}
	if deps.ReadinessTimeout == 0 {
		deps.ReadinessTimeout = readinessTimeoutFromEnv()
	}
	return &ServerDaemonService{
		stateRepo:        deps.StateRepo,
		spawner:          deps.Spawner,
		output:           deps.Output,
		processExists:    deps.ProcessExists,
		isSocketAlive:    deps.IsSocketAlive,
		readinessTimeout: deps.ReadinessTimeout,
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

func defaultSocketAlive(socketPath string) bool {
	d := &net.Dialer{Timeout: socketDialTimeout}
	conn, err := d.DialContext(context.Background(), "unix", socketPath)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

// Start spawns the idx server daemon for the given project path.
// Uses socket connectivity as the source of truth for liveness.
// The server self-initializes via ensureRootIndex on first watch event when no index exists.
func (s *ServerDaemonService) Start(projectPath string) error {
	absPath, err := resolveAbsPath(projectPath)
	if err != nil {
		return err
	}

	socketPath := ipc.SocketPath(absPath)
	if s.isSocketAlive(socketPath) {
		return s.output.WriteLine("⚡ Agent is already running")
	}

	s.killStaleProcess(absPath)

	pid, err := s.spawner.SpawnServerProcess(absPath)
	if err != nil {
		return fmt.Errorf("failed to start server for %q: got error %v, expected process to start", absPath, err)
	}

	if !s.waitForSocket(socketPath) {
		if proc, findErr := os.FindProcess(pid); findErr == nil {
			_ = proc.Kill()
		}
		return fmt.Errorf("server process (PID: %d) did not become ready within %s", pid, s.readinessTimeout)
	}

	newState := &ServerState{
		PID:         pid,
		StartedAt:   time.Now(),
		SocketPath:  socketPath,
		ProjectPath: absPath,
	}
	if err := s.stateRepo.SaveState(absPath, newState); err != nil {
		if proc, findErr := os.FindProcess(pid); findErr == nil {
			_ = proc.Signal(syscall.SIGTERM)
		}
		return err
	}

	return s.output.WriteLine(fmt.Sprintf("✅ Agent started (PID: %d)", pid))
}

// Stop sends SIGTERM to the server daemon and removes the state file.
// Returns nil if the server is not running.
func (s *ServerDaemonService) Stop(projectPath string) error {
	absPath, err := resolveAbsPath(projectPath)
	if err != nil {
		return err
	}

	socketPath := ipc.SocketPath(absPath)
	state, _ := s.stateRepo.ReadState(absPath)

	if !s.isSocketAlive(socketPath) {
		s.removeStaleStateIfDead(absPath, state)
		return s.output.WriteLine("ℹ️  Agent is not running")
	}

	if state != nil {
		if proc, err := os.FindProcess(state.PID); err == nil {
			_ = proc.Signal(syscall.SIGTERM)
		}
		_ = s.stateRepo.RemoveState(absPath)
		return s.output.WriteLine("🛑 Agent stopped")
	}

	return s.output.WriteLine("🛑 Agent stopped")
}

// Status prints the current server status for the given project path.
func (s *ServerDaemonService) Status(projectPath string) error {
	absPath, err := resolveAbsPath(projectPath)
	if err != nil {
		return err
	}

	socketPath := ipc.SocketPath(absPath)
	state, _ := s.stateRepo.ReadState(absPath)

	if !s.isSocketAlive(socketPath) {
		s.removeStaleStateIfDead(absPath, state)
		return s.output.WriteLine("❌ Agent is not running")
	}

	if state == nil {
		return s.output.WriteLine("✅ Agent running")
	}

	uptime := time.Since(state.StartedAt).Round(time.Second)
	return s.output.WriteLine(fmt.Sprintf("✅ Agent running (uptime: %s)", uptime))
}

// IsProjectMonitored implements indexing.ProjectMonitorChecker.
func (s *ServerDaemonService) IsProjectMonitored(projectRoot string) (bool, error) {
	socketPath := ipc.SocketPath(projectRoot)
	return s.isSocketAlive(socketPath), nil
}

// ProjectStatus implements indexing.ProjectMonitorChecker.
func (s *ServerDaemonService) ProjectStatus(projectRoot string) (*indexing.DaemonProjectStatus, error) {
	state, err := s.stateRepo.ReadState(projectRoot)
	if err != nil || state == nil {
		return nil, err
	}
	socketPath := ipc.SocketPath(projectRoot)
	return &indexing.DaemonProjectStatus{
		Enabled:   s.isSocketAlive(socketPath),
		PID:       state.PID,
		StartedAt: state.StartedAt,
	}, nil
}

// killStaleProcess terminates a process from a stale state file whose socket is already dead.
func (s *ServerDaemonService) killStaleProcess(absPath string) {
	state, _ := s.stateRepo.ReadState(absPath)
	if state == nil {
		return
	}
	if s.processExists(state.PID) {
		if proc, err := os.FindProcess(state.PID); err == nil {
			_ = proc.Signal(syscall.SIGTERM)
		}
	}
	_ = s.stateRepo.RemoveState(absPath)
}

// removeStaleStateIfDead removes the state file when the process is no longer alive.
func (s *ServerDaemonService) removeStaleStateIfDead(absPath string, state *ServerState) {
	if state == nil || s.processExists(state.PID) {
		return
	}
	_ = s.stateRepo.RemoveState(absPath)
}

func (s *ServerDaemonService) waitForSocket(socketPath string) bool {
	deadline := time.Now().Add(s.readinessTimeout)
	for time.Now().Before(deadline) {
		if s.isSocketAlive(socketPath) {
			return true
		}
		time.Sleep(socketPollInterval)
	}
	return false
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
