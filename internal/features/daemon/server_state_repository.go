package daemon

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const serverStateFileSuffix = ".server.state"

type serverStateRepository struct{}

// NewServerStateRepository creates a StateRepository backed by ~/.idx/<project>.server.state.
// Example: NewServerStateRepository().ReadState("/home/user/myproject").
func NewServerStateRepository() StateRepository {
	return &serverStateRepository{}
}

// ReadState reads ~/.idx/<project>.server.state; returns nil if it does not exist.
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

// SaveState writes the server state to ~/.idx/<project>.server.state.
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

// RemoveState deletes ~/.idx/<project>.server.state.
func (r *serverStateRepository) RemoveState(projectPath string) error {
	statePath := r.stateFilePath(projectPath)

	if err := os.Remove(statePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove server state %q: got error %v, expected removable file", statePath, err)
	}

	return nil
}

// stateFilePath derives ~/.idx/<sanitized-project-name>.server.state from projectPath.
func (r *serverStateRepository) stateFilePath(projectPath string) string {
	home, _ := os.UserHomeDir()
	name := sanitizeStateSegment(filepath.Base(projectPath))
	return filepath.Join(home, ".idx", name+serverStateFileSuffix)
}

const unknownStateProject = "unknown-project"

func sanitizeStateSegment(name string) string {
	if name == "" || name == "." || name == string(filepath.Separator) {
		return unknownStateProject
	}

	b := strings.Builder{}
	b.Grow(len(name))
	for i := range len(name) {
		ch := name[i]
		if isStateSafeChar(ch) {
			b.WriteByte(ch)
			continue
		}
		b.WriteByte('_')
	}

	clean := strings.Trim(b.String(), "._-")
	if clean == "" {
		return unknownStateProject
	}

	return clean
}

func isStateSafeChar(ch byte) bool {
	return (ch >= 'a' && ch <= 'z') ||
		(ch >= 'A' && ch <= 'Z') ||
		(ch >= '0' && ch <= '9') ||
		ch == '-' || ch == '_' || ch == '.'
}
