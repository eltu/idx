package services_test

import (
	"errors"
	"path/filepath"
	"testing"

	"idx/internal/core/domain"
	"idx/internal/core/ports"
	"idx/internal/core/services"
)

type fakeProjectTree struct {
	currentDir string
	gitRoot    string
	readDirMap map[string][]domain.DirectoryEntry
	existing   map[string]bool
	writes     map[string]string
	gitRootErr error
}

func newFakeProjectTree(currentDir string, gitRoot string) *fakeProjectTree {
	return &fakeProjectTree{
		currentDir: currentDir,
		gitRoot:    gitRoot,
		readDirMap: map[string][]domain.DirectoryEntry{},
		existing:   map[string]bool{},
		writes:     map[string]string{},
	}
}

func (tree *fakeProjectTree) CurrentDir() (string, error) {
	return tree.currentDir, nil
}

func (tree *fakeProjectTree) FindGitRoot(startDir string) (string, error) {
	if tree.gitRootErr != nil {
		return "", tree.gitRootErr
	}

	return tree.gitRoot, nil
}

func (tree *fakeProjectTree) ReadDir(path string) ([]domain.DirectoryEntry, error) {
	entries, ok := tree.readDirMap[path]
	if !ok {
		return []domain.DirectoryEntry{}, nil
	}

	return entries, nil
}

func (tree *fakeProjectTree) Exists(path string) (bool, error) {
	return tree.existing[path], nil
}

func (tree *fakeProjectTree) WriteFile(path string, content []byte) error {
	tree.writes[path] = string(content)
	return nil
}

type fakeIgnoreMatcherFactory struct {
	ignoredPaths map[string]bool
}

func (factory fakeIgnoreMatcherFactory) New(projectRoot string) (ports.IgnoreMatcher, error) {
	return fakeIgnoreMatcher{ignoredPaths: factory.ignoredPaths}, nil
}

type fakeIgnoreMatcher struct {
	ignoredPaths map[string]bool
}

func (matcher fakeIgnoreMatcher) Matches(path string) (bool, error) {
	return matcher.ignoredPaths[path], nil
}

type fakeTextOutput struct{}

func (output fakeTextOutput) WriteLine(text string) error {
	return nil
}

func TestInitCommandServiceRunWritesIndexFilesForAllowedEntries(t *testing.T) {
	rootDir := filepath.Join(string(filepath.Separator), "repo")
	childDir := filepath.Join(rootDir, "child")
	emptyDir := filepath.Join(rootDir, "empty")
	vendorDir := filepath.Join(rootDir, "vendor")

	tree := newFakeProjectTree(rootDir, rootDir)
	tree.readDirMap[rootDir] = []domain.DirectoryEntry{
		{Name: ".git", Path: filepath.Join(rootDir, ".git"), IsDir: true},
		{Name: ".gitignore", Path: filepath.Join(rootDir, ".gitignore"), IsDir: false},
		{Name: "allowed.txt", Path: filepath.Join(rootDir, "allowed.txt"), IsDir: false},
		{Name: "child", Path: childDir, IsDir: true},
		{Name: "empty", Path: emptyDir, IsDir: true},
		{Name: "ignored.log", Path: filepath.Join(rootDir, "ignored.log"), IsDir: false},
		{Name: "vendor", Path: vendorDir, IsDir: true},
	}
	tree.readDirMap[childDir] = []domain.DirectoryEntry{{Name: "nested.txt", Path: filepath.Join(childDir, "nested.txt"), IsDir: false}}
	tree.readDirMap[emptyDir] = []domain.DirectoryEntry{}
	tree.readDirMap[vendorDir] = []domain.DirectoryEntry{{Name: "skip.txt", Path: filepath.Join(vendorDir, "skip.txt"), IsDir: false}}

	matcherFactory := fakeIgnoreMatcherFactory{ignoredPaths: map[string]bool{
		"ignored.log": true,
		"vendor/":     true,
	}}
	service := services.NewInitCommandService(tree, matcherFactory, fakeTextOutput{})

	err := service.Run()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	rootIndexPath := filepath.Join(rootDir, ".index")
	if tree.writes[rootIndexPath] != ".gitignore\nallowed.txt\nchild/\nempty/\n" {
		t.Fatalf("unexpected root index content %q", tree.writes[rootIndexPath])
	}

	childIndexPath := filepath.Join(childDir, ".index")
	if tree.writes[childIndexPath] != "nested.txt\n" {
		t.Fatalf("unexpected child index content %q", tree.writes[childIndexPath])
	}

	emptyIndexPath := filepath.Join(emptyDir, ".index")
	if tree.writes[emptyIndexPath] != "" {
		t.Fatalf("expected empty index content, got %q", tree.writes[emptyIndexPath])
	}

	vendorIndexPath := filepath.Join(vendorDir, ".index")
	if _, ok := tree.writes[vendorIndexPath]; ok {
		t.Fatalf("did not expect index for ignored directory %q", vendorIndexPath)
	}
}

func TestInitCommandServiceRunRejectsDirectoryOutsideGitProject(t *testing.T) {
	rootDir := filepath.Join(string(filepath.Separator), "repo")
	tree := newFakeProjectTree(rootDir, "")
	tree.gitRootErr = errors.New("directory \"/repo\" is not inside a git project: expected a path with a .git entry in the current directory or one of its parents")
	matcherFactory := fakeIgnoreMatcherFactory{ignoredPaths: map[string]bool{}}
	service := services.NewInitCommandService(tree, matcherFactory, fakeTextOutput{})

	err := service.Run()
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
}

func TestInitCommandServiceRunSkipsWhenIndexAlreadyExists(t *testing.T) {
	rootDir := filepath.Join(string(filepath.Separator), "repo")
	tree := newFakeProjectTree(rootDir, rootDir)
	tree.existing[filepath.Join(rootDir, ".index")] = true
	matcherFactory := fakeIgnoreMatcherFactory{ignoredPaths: map[string]bool{}}
	output := &capturingTextOutput{}
	service := services.NewInitCommandService(tree, matcherFactory, output)

	err := service.Run()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(output.lines) != 1 {
		t.Fatalf("expected 1 output line, got %d", len(output.lines))
	}

	if output.lines[0] != "ℹ️ Este projeto ja possui indice. Voce pode executar idx search." {
		t.Fatalf("unexpected output message %q", output.lines[0])
	}

	if len(tree.writes) != 0 {
		t.Fatalf("expected no index writes, got %d writes", len(tree.writes))
	}
}

type capturingTextOutput struct {
	lines []string
}

func (output *capturingTextOutput) WriteLine(text string) error {
	output.lines = append(output.lines, text)
	return nil
}
