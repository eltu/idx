package ports

// InitCommandInterface defines the contract for initializing a project.
type InitCommandInterface interface {
	// RunFromPath runs the index initialization from a specific directory.
	RunFromPath(projectPath string) error
}
