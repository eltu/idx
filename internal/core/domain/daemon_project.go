package domain

import "time"

// MonitoredProject representa um projeto sendo monitorado em realtime.
type MonitoredProject struct {
	Path      string    `json:"path"`
	PID       int       `json:"pid"`
	Enabled   bool      `json:"enabled"`
	StartedAt time.Time `json:"started_at"`
	LastSync  time.Time `json:"last_sync"`
}

// DaemonState é o arquivo global ~/.idx/daemon.state que persiste todos os projetos monitorados.
type DaemonState struct {
	Projects  []MonitoredProject `json:"projects"`
	UpdatedAt time.Time          `json:"updated_at"`
}
