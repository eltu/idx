package ports

// ProcessSpawner encapsulates the logic for starting background processes.
// Allows mocking in tests.
type ProcessSpawner interface {
	// SpawnWatchProcess starts a background watch process for the given directory.
	// Returns the PID of the started process or an error if it fails to start.
	// Implementations must ensure the process is detached from the parent.
	SpawnWatchProcess(projectPath string) (int, error)
}
