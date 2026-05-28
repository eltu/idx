package ipc

import (
	"strings"
	"testing"
)

func TestSocketPathReturnsSockSuffix(t *testing.T) {
	path := SocketPath("/home/user/myproject")
	if !strings.HasSuffix(path, ".sock") {
		t.Errorf("expected .sock suffix, got %q", path)
	}
}

func TestSocketPathContainsDotIdxDir(t *testing.T) {
	path := SocketPath("/home/user/myproject")
	if !strings.Contains(path, ".idx") {
		t.Errorf("expected .idx dir in socket path, got %q", path)
	}
}

func TestSocketPathIsInsideProject(t *testing.T) {
	path := SocketPath("/home/user/myproject")
	if !strings.HasPrefix(path, "/home/user/myproject") {
		t.Errorf("expected socket path inside project root, got %q", path)
	}
}

func TestSocketPathDistinguishesDifferentProjects(t *testing.T) {
	a := SocketPath("/home/user/work/myapp")
	b := SocketPath("/home/user/personal/myapp")
	if a == b {
		t.Errorf("expected different socket paths for different projects, got %q", a)
	}
}

func TestSocketPathIsDeterministic(t *testing.T) {
	projectPath := "/some/project"
	first := SocketPath(projectPath)
	second := SocketPath(projectPath)
	if first != second {
		t.Errorf("expected deterministic socket path, got %q then %q", first, second)
	}
}
