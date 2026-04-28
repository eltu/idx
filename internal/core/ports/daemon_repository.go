package ports

import "idx/internal/core/domain"

// DaemonRepository define operações de persistência para o estado do daemon.
type DaemonRepository interface {
	// ReadState lê o arquivo de estado global (~/.idx/daemon.state).
	// Retorna nil se o arquivo não existe (primeiro uso).
	ReadState() (*domain.DaemonState, error)

	// SaveState persiste o estado global para disco.
	SaveState(state *domain.DaemonState) error

	// UpdateProjectPID atualiza o PID de um projeto específico no state.
	UpdateProjectPID(projectPath string, pid int) error
}
