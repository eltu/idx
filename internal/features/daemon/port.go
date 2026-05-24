package daemon

// Repository defines persistence operations for the daemon state.
type Repository interface {
	// ReadState reads the global state file (~/.idx/daemon.state).
	// Returns nil if the file does not exist (first use).
	ReadState() (*State, error)

	// SaveState persists the global state to disk.
	SaveState(state *State) error

	// UpdateProjectPID updates the PID of a specific project in the state.
	UpdateProjectPID(projectPath string, pid int) error
}

// ProcessSpawner encapsulates the logic for starting background processes.
type ProcessSpawner interface {
	// SpawnWatchProcess starts a background watch process for the given directory.
	// Returns the PID of the started process or an error if it fails to start.
	// Implementations must ensure the process is detached from the parent.
	SpawnWatchProcess(projectPath string) (int, error)
}
