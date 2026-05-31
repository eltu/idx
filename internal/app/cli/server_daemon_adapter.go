package cli

import (
	"encoding/json"
	"math"
	"path/filepath"
	"time"

	"idx/internal/features/daemon"
	"idx/internal/shared/ipc"
)

// ServerDaemonAdapter adapts ServerDaemonService to the serverManagerCommand interface.
type ServerDaemonAdapter struct {
	svc *daemon.ServerDaemonService
}

// NewServerDaemonAdapter creates a ServerDaemonAdapter.
// Example: adapter := NewServerDaemonAdapter(serverDaemonService).
func NewServerDaemonAdapter(svc *daemon.ServerDaemonService) *ServerDaemonAdapter {
	return &ServerDaemonAdapter{svc: svc}
}

func (a *ServerDaemonAdapter) Start(projectPath string) error {
	return a.svc.Start(projectPath)
}

func (a *ServerDaemonAdapter) Stop(projectPath string) error {
	return a.svc.Stop(projectPath)
}

func (a *ServerDaemonAdapter) Status(projectPath string) error {
	return a.svc.Status(projectPath)
}

// serverStatusJSON is the JSON payload for 'idx server status --json'.
type serverStatusJSON struct {
	Running       bool   `json:"running"`
	PID           int    `json:"pid,omitempty"`
	UptimeSeconds int    `json:"uptime_seconds,omitempty"`
	SocketPath    string `json:"socket_path"`
}

// StatusJSON implements serverStatusJSONProvider for machine-readable output.
// Example: data, err := adapter.StatusJSON("/home/user/myproject").
func (a *ServerDaemonAdapter) StatusJSON(projectPath string) ([]byte, error) {
	absPath, err := filepath.Abs(projectPath)
	if err != nil {
		return nil, err
	}

	socketPath := ipc.SocketPath(absPath)
	status, err := a.svc.ProjectStatus(absPath)

	payload := serverStatusJSON{SocketPath: socketPath}
	if err == nil && status != nil && status.Enabled {
		payload.Running = true
		payload.PID = status.PID
		payload.UptimeSeconds = int(math.Round(time.Since(status.StartedAt).Seconds()))
	}

	return json.Marshal(payload)
}
