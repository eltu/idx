package ipc

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSocketPath_HasDotSockSuffix(t *testing.T) {
	t.Parallel()

	// Act
	path := SocketPath("/home/user/myproject")

	// Assert
	assert.True(t, strings.HasSuffix(path, ".sock"), "expected .sock suffix, got %q", path)
}

func TestSocketPath_ContainsDotIdxDirectory(t *testing.T) {
	t.Parallel()

	// Act
	path := SocketPath("/home/user/myproject")

	// Assert
	assert.Contains(t, path, ".idx")
}

func TestSocketPath_IsInsideProjectRoot(t *testing.T) {
	t.Parallel()

	// Act
	path := SocketPath("/home/user/myproject")

	// Assert
	assert.True(t, strings.HasPrefix(path, "/home/user/myproject"), "expected path inside project root, got %q", path)
}

func TestSocketPath_IsDifferentForDifferentProjects(t *testing.T) {
	t.Parallel()

	// Act
	a := SocketPath("/home/user/work/myapp")
	b := SocketPath("/home/user/personal/myapp")

	// Assert
	assert.NotEqual(t, a, b)
}

func TestSocketPath_IsDeterministic(t *testing.T) {
	t.Parallel()

	// Arrange
	projectPath := "/some/project"

	// Act
	first := SocketPath(projectPath)
	second := SocketPath(projectPath)

	// Assert
	assert.Equal(t, first, second)
}
