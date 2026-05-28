package daemon

import "time"

// ServerState is the per-project state persisted to ~/.idx/<project>.server.state.
type ServerState struct {
	PID         int       `json:"pid"`
	StartedAt   time.Time `json:"started_at"`
	SocketPath  string    `json:"socket"`
	ProjectPath string    `json:"project_path"`
}
