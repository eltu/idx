package services_test

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"idx/internal/core/domain"
	"idx/internal/core/services"
)

func TestDestroyCommandServiceRunRemovesIdxDirectoriesRecursively(t *testing.T) {
	rootDir := filepath.Join(string(filepath.Separator), "repo")
	apiDir := filepath.Join(rootDir, "cmd", "api")
	coreDir := filepath.Join(rootDir, "internal", "core")

	tree := newFakeProjectTree(rootDir, rootDir)
	tree.readDirMap[rootDir] = []domain.DirectoryEntry{
		{Name: ".git", Path: filepath.Join(rootDir, ".git"), IsDir: true},
		{Name: ".idx", Path: filepath.Join(rootDir, ".idx"), IsDir: true},
		{Name: "cmd", Path: filepath.Join(rootDir, "cmd"), IsDir: true},
		{Name: "internal", Path: filepath.Join(rootDir, "internal"), IsDir: true},
	}
	tree.readDirMap[filepath.Join(rootDir, "cmd")] = []domain.DirectoryEntry{{Name: "api", Path: apiDir, IsDir: true}}
	tree.readDirMap[apiDir] = []domain.DirectoryEntry{{Name: ".idx", Path: filepath.Join(apiDir, ".idx"), IsDir: true}}
	tree.readDirMap[filepath.Join(rootDir, "internal")] = []domain.DirectoryEntry{{Name: "core", Path: coreDir, IsDir: true}}
	tree.readDirMap[coreDir] = []domain.DirectoryEntry{{Name: ".idx", Path: filepath.Join(coreDir, ".idx"), IsDir: true}}

	output := &capturingTextOutput{}
	service := services.NewDestroyCommandService(tree, output)

	err := service.Run()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(tree.removed) != 3 {
		t.Fatalf("expected 3 removed directories, got %d", len(tree.removed))
	}

	if tree.removed[0] != filepath.Join(rootDir, ".idx") {
		t.Fatalf("unexpected first removed path %q", tree.removed[0])
	}

	if len(output.lines) != 1 {
		t.Fatalf("expected 1 output line, got %d", len(output.lines))
	}

	if output.lines[0] != "🧹 Index metadata removed from project." {
		t.Fatalf("unexpected output message %q", output.lines[0])
	}
}

func TestDestroyCommandServiceRunRequiresProjectRoot(t *testing.T) {
	rootDir := filepath.Join(string(filepath.Separator), "repo")
	currentDir := filepath.Join(rootDir, "internal")

	tree := newFakeProjectTree(currentDir, rootDir)
	output := &capturingTextOutput{}
	service := services.NewDestroyCommandService(tree, output)

	err := service.Run()
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	expectedMessage := "destroy must run from project root: got current directory \"/repo/internal\", expected root directory \"/repo\""
	if err.Error() != expectedMessage {
		t.Fatalf("unexpected error message %q", err.Error())
	}

	if len(tree.removed) != 0 {
		t.Fatalf("expected no directories removed, got %d", len(tree.removed))
	}
}

func TestDestroyCommandServiceRunContinuesAfterRemoveFailureAndReturnsError(t *testing.T) {
	rootDir := filepath.Join(string(filepath.Separator), "repo")
	apiDir := filepath.Join(rootDir, "cmd", "api")
	coreDir := filepath.Join(rootDir, "internal", "core")
	failingIdx := filepath.Join(apiDir, ".idx")

	tree := newFakeProjectTree(rootDir, rootDir)
	tree.readDirMap[rootDir] = []domain.DirectoryEntry{
		{Name: ".git", Path: filepath.Join(rootDir, ".git"), IsDir: true},
		{Name: ".idx", Path: filepath.Join(rootDir, ".idx"), IsDir: true},
		{Name: "cmd", Path: filepath.Join(rootDir, "cmd"), IsDir: true},
		{Name: "internal", Path: filepath.Join(rootDir, "internal"), IsDir: true},
	}
	tree.readDirMap[filepath.Join(rootDir, "cmd")] = []domain.DirectoryEntry{{Name: "api", Path: apiDir, IsDir: true}}
	tree.readDirMap[apiDir] = []domain.DirectoryEntry{{Name: ".idx", Path: failingIdx, IsDir: true}}
	tree.readDirMap[filepath.Join(rootDir, "internal")] = []domain.DirectoryEntry{{Name: "core", Path: coreDir, IsDir: true}}
	tree.readDirMap[coreDir] = []domain.DirectoryEntry{{Name: ".idx", Path: filepath.Join(coreDir, ".idx"), IsDir: true}}
	tree.removeErrs[failingIdx] = errors.New("permission denied")

	output := &capturingTextOutput{}
	service := services.NewDestroyCommandService(tree, output)

	err := service.Run()
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if len(tree.removed) != 3 {
		t.Fatalf("expected 3 removal attempts, got %d", len(tree.removed))
	}

	if !strings.Contains(err.Error(), failingIdx) {
		t.Fatalf("expected error to contain failing path %q, got %q", failingIdx, err.Error())
	}

	if len(output.lines) != 0 {
		t.Fatalf("expected no success output on partial failure, got %v", output.lines)
	}
}
