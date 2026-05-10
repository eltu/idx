package cli

import "idx/internal/core/services/daemon"

// DaemonServiceAdapter wraps DaemonService to satisfy the daemonableCommand interface
// expected by CommandRunner.
type DaemonServiceAdapter struct {
	daemon *daemon.DaemonService
}

// NewDaemonServiceAdapter constructs the adapter for use in CommandRunner.
// Example: adapter := NewDaemonServiceAdapter(daemonService).
func NewDaemonServiceAdapter(d *daemon.DaemonService) *DaemonServiceAdapter {
	return &DaemonServiceAdapter{daemon: d}
}

func (a *DaemonServiceAdapter) Enable(projectPath string) error {
	return a.daemon.Enable(projectPath)
}

func (a *DaemonServiceAdapter) Disable(projectPath string) error {
	return a.daemon.Disable(projectPath)
}

func (a *DaemonServiceAdapter) Status() error {
	return a.daemon.Status()
}
