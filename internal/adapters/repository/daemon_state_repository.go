package repository

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"idx/internal/core/domain"
)

// DaemonStateRepository persiste o estado do daemon em ~/.idx/daemon.state.
type DaemonStateRepository struct{}

// NewDaemonStateRepository cria uma nova instância.
func NewDaemonStateRepository() *DaemonStateRepository {
	return &DaemonStateRepository{}
}

// ReadState lê o arquivo de estado do daemon.
// Retorna nil se o arquivo não existe (primeira vez que o daemon é usado).
func (r *DaemonStateRepository) ReadState() (*domain.DaemonState, error) {
	statePath := r.stateFilePath()

	data, err := os.ReadFile(statePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // First run - arquivo não existe ainda
		}
		return nil, fmt.Errorf("failed to read daemon state from %q: got error %v, expected readable file", statePath, err)
	}

	var state domain.DaemonState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("invalid daemon state file %q: got error %v, expected valid JSON", statePath, err)
	}

	return &state, nil
}

// SaveState persiste o estado do daemon para disco.
// Cria ~/.idx/ se não existir.
func (r *DaemonStateRepository) SaveState(state *domain.DaemonState) error {
	statePath := r.stateFilePath()

	// Cria ~/.idx se não existir
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

// UpdateProjectPID atualiza o PID de um projeto no state file.
func (r *DaemonStateRepository) UpdateProjectPID(projectPath string, pid int) error {
	state, _ := r.ReadState()
	if state == nil {
		state = &domain.DaemonState{Projects: []domain.MonitoredProject{}}
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

// stateFilePath retorna o caminho absoluto para o arquivo de estado do daemon.
func (r *DaemonStateRepository) stateFilePath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".idx", "daemon.state")
}
