package ports

import "idx/internal/core/domain"

// DaemonRepository defines persistence operations for the daemon state.
type DaemonRepository interface {
	// ReadState reads the global state file (~/.idx/daemon.state).
	// Returns nil if the file does not exist (first use).
	ReadState() (*domain.DaemonState, error)

	// SaveState persists the global state to disk.
	SaveState(state *domain.DaemonState) error

	// UpdateProjectPID updates the PID of a specific project in the state.
	UpdateProjectPID(projectPath string, pid int) error
}
