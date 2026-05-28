package ipc

import (
	"path/filepath"
)

const serverSocketName = "server.sock"

// SocketPath returns the Unix socket path for an idx server bound to projectRoot.
// Pattern: <projectRoot>/.idx/server.sock
// Example: SocketPath("/home/user/myproject") → "/home/user/myproject/.idx/server.sock".
func SocketPath(projectRoot string) string {
	return filepath.Join(projectRoot, ".idx", serverSocketName)
}
