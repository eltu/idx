package cli

import "idx/internal/features/daemon"

// ServerDaemonAdapter adapts ServerDaemonService to the serverManagerCommand interface.
type ServerDaemonAdapter struct {
	svc *daemon.ServerDaemonService
}

// NewServerDaemonAdapter creates a ServerDaemonAdapter.
// Example: adapter := NewServerDaemonAdapter(serverDaemonService).
func NewServerDaemonAdapter(svc *daemon.ServerDaemonService) *ServerDaemonAdapter {
	return &ServerDaemonAdapter{svc: svc}
}

func (a *ServerDaemonAdapter) Start(projectPath string) error {
	return a.svc.Start(projectPath)
}

func (a *ServerDaemonAdapter) Stop(projectPath string) error {
	return a.svc.Stop(projectPath)
}

func (a *ServerDaemonAdapter) Status(projectPath string) error {
	return a.svc.Status(projectPath)
}
