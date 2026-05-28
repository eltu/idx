package daemon

// StateRepository reads, writes, and removes per-project server state.
type StateRepository interface {
	ReadState(projectPath string) (*ServerState, error)
	SaveState(projectPath string, state *ServerState) error
	RemoveState(projectPath string) error
}

// ServerSpawner starts the idx server process in the background.
type ServerSpawner interface {
	SpawnServerProcess(projectPath string) (int, error)
}
