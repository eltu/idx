package daemon

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// DaemonStateRepository persists the daemon state to ~/.idx/daemon.state.
type DaemonStateRepository struct{}

// NewDaemonStateRepository creates a new instance.
func NewDaemonStateRepository() *DaemonStateRepository {
	return &DaemonStateRepository{}
}

// ReadState reads the daemon state file.
// Returns nil if the file does not exist (first time the daemon is used).
func (r *DaemonStateRepository) ReadState() (*State, error) {
	statePath := r.stateFilePath()

	data, err := os.ReadFile(statePath) //nolint:gosec // path is derived from the project root, not user input
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // First run – file does not exist yet
		}
		return nil, fmt.Errorf("failed to read daemon state from %q: got error %v, expected readable file", statePath, err)
	}

	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("invalid daemon state file %q: got error %v, expected valid JSON", statePath, err)
	}

	return &state, nil
}

// SaveState persists the daemon state to disk.
// Creates ~/.idx/ if it does not exist.
func (r *DaemonStateRepository) SaveState(state *State) error {
	statePath := r.stateFilePath()

	// Create ~/.idx if it does not exist
	dir := filepath.Dir(statePath)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return fmt.Errorf("failed to create daemon config directory %q: got error %v, expected writable path", dir, err)
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal daemon state: got error %v, expected valid state", err)
	}

	if err := os.WriteFile(statePath, data, 0600); err != nil {
		return fmt.Errorf("failed to save daemon state to %q: got error %v, expected writable path", statePath, err)
	}

	return nil
}

// UpdateProjectPID updates the PID of a project in the state file.
func (r *DaemonStateRepository) UpdateProjectPID(projectPath string, pid int) error {
	state, _ := r.ReadState()
	if state == nil {
		state = &State{Projects: []MonitoredProject{}}
	}

	for i, proj := range state.Projects {
		if proj.Path == projectPath {
			state.Projects[i].PID = pid
			state.Projects[i].LastSync = time.Now()
			state.UpdatedAt = time.Now()
			break
		}
	}

	return r.SaveState(state)
}

// stateFilePath returns the absolute path to the daemon state file.
func (r *DaemonStateRepository) stateFilePath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".idx", "daemon.state")
}
