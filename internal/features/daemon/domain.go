package daemon

import "time"

// MonitoredProject represents a project being monitored in realtime.
type MonitoredProject struct {
	Path      string    `json:"path"`
	PID       int       `json:"pid"`
	Enabled   bool      `json:"enabled"`
	StartedAt time.Time `json:"started_at"`
	LastSync  time.Time `json:"last_sync"`
}

// State is the global state file ~/.idx/daemon.state that persists all monitored projects.
type State struct {
	Projects  []MonitoredProject `json:"projects"`
	UpdatedAt time.Time          `json:"updated_at"`
}
