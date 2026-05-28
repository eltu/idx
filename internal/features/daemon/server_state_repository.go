package daemon

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const serverStateFileName = "server.state"

type serverStateRepository struct{}

// NewServerStateRepository creates a StateRepository backed by <project>/.idx/server.state.
// Example: NewServerStateRepository().ReadState("/home/user/myproject").
func NewServerStateRepository() StateRepository {
	return &serverStateRepository{}
}

// ReadState reads <project>/.idx/server.state; returns nil if it does not exist.
func (r *serverStateRepository) ReadState(projectPath string) (*ServerState, error) {
	statePath := r.stateFilePath(projectPath)

	data, err := os.ReadFile(statePath) //nolint:gosec // path is derived from the project root
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read server state from %q: got error %v, expected readable file", statePath, err)
	}

	var state ServerState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("invalid server state file %q: got error %v, expected valid JSON", statePath, err)
	}

	return &state, nil
}

// SaveState writes the server state to <project>/.idx/server.state.
func (r *serverStateRepository) SaveState(projectPath string, state *ServerState) error {
	statePath := r.stateFilePath(projectPath)

	dir := filepath.Dir(statePath)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return fmt.Errorf("failed to create state directory %q: got error %v, expected writable path", dir, err)
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal server state: got error %v, expected valid state", err)
	}

	if err := os.WriteFile(statePath, data, 0600); err != nil {
		return fmt.Errorf("failed to save server state to %q: got error %v, expected writable path", statePath, err)
	}

	return nil
}

// RemoveState deletes <project>/.idx/server.state.
func (r *serverStateRepository) RemoveState(projectPath string) error {
	statePath := r.stateFilePath(projectPath)

	if err := os.Remove(statePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove server state %q: got error %v, expected removable file", statePath, err)
	}

	return nil
}

// stateFilePath derives <project>/.idx/server.state from projectPath.
func (r *serverStateRepository) stateFilePath(projectPath string) string {
	return filepath.Join(projectPath, ".idx", serverStateFileName)
}
