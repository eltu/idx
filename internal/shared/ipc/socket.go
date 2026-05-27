package ipc

import (
	"os"
	"path/filepath"
	"strings"
)

// SocketPath returns the Unix socket path for an idx server bound to projectRoot.
// Pattern: ~/.idx/<sanitized-project-name>.sock
// Example: SocketPath("/home/user/myproject") → "/home/user/.idx/myproject.sock".
func SocketPath(projectRoot string) string {
	home, _ := os.UserHomeDir()
	name := sanitizeSocketSegment(filepath.Base(projectRoot))
	return filepath.Join(home, ".idx", name+".sock")
}

const unknownProject = "unknown-project"

// sanitizeSocketSegment converts a directory name into a safe socket filename segment.
func sanitizeSocketSegment(name string) string {
	if name == "" || name == "." || name == string(filepath.Separator) {
		return unknownProject
	}

	b := strings.Builder{}
	b.Grow(len(name))
	for i := range len(name) {
		ch := name[i]
		if isSocketSafeChar(ch) {
			b.WriteByte(ch)
			continue
		}
		b.WriteByte('_')
	}

	clean := strings.Trim(b.String(), "._-")
	if clean == "" {
		return unknownProject
	}

	return clean
}

func isSocketSafeChar(ch byte) bool {
	return (ch >= 'a' && ch <= 'z') ||
		(ch >= 'A' && ch <= 'Z') ||
		(ch >= '0' && ch <= '9') ||
		ch == '-' || ch == '_' || ch == '.'
}
