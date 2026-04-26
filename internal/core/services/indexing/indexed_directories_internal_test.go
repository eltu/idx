package indexing

import (
	"errors"
	"path/filepath"
	"testing"

	"idx/internal/core/domain"
)

type indexedTreeStub struct {
	entries map[string][]domain.DirectoryEntry
	exists  map[string]bool
	errDir  map[string]error
	errStat map[string]error
}

func (tree indexedTreeStub) CurrentDir() (string, error)        { return "", nil }
func (tree indexedTreeStub) FindGitRoot(string) (string, error) { return "", nil }
func (tree indexedTreeStub) RemoveAll(string) error             { return nil }
func (tree indexedTreeStub) WriteFile(string, []byte) error     { return nil }

func (tree indexedTreeStub) ReadDir(path string) ([]domain.DirectoryEntry, error) {
	if err := tree.errDir[path]; err != nil {
		return nil, err
	}
	return tree.entries[path], nil
}

func (tree indexedTreeStub) Exists(path string) (bool, error) {
	if err := tree.errStat[path]; err != nil {
		return false, err
	}
	return tree.exists[path], nil
}

type allowAllMatcher struct{}

func (allowAllMatcher) Matches(string) (bool, error) { return false, nil }

func TestIndexedDirectoriesAndEligibleDirectories(t *testing.T) {
	root := "/repo"
	child := filepath.Join(root, "child")
	tree := indexedTreeStub{
		entries: map[string][]domain.DirectoryEntry{
			root: []domain.DirectoryEntry{
				{Name: ".git", Path: filepath.Join(root, ".git"), IsDir: true},
				{Name: ".idx", Path: filepath.Join(root, ".idx"), IsDir: true},
				{Name: "child", Path: child, IsDir: true},
			},
			child: []domain.DirectoryEntry{{Name: "file.txt", Path: filepath.Join(child, "file.txt"), IsDir: false}},
		},
		exists: map[string]bool{
			filepath.Join(root, ".idx", "index.idx"):  true,
			filepath.Join(child, ".idx", "index.idx"): true,
		},
		errDir:  map[string]error{},
		errStat: map[string]error{},
	}

	indexed, err := IndexedDirectories(tree, root)
	if err != nil {
		t.Fatalf("expected indexed directories without error, got %v", err)
	}
	if len(indexed) != 2 {
		t.Fatalf("expected two indexed directories, got %v", indexed)
	}

	eligible, err := eligibleDirectories(tree, root, allowAllMatcher{})
	if err != nil {
		t.Fatalf("expected eligible directories without error, got %v", err)
	}
	if len(eligible) != 2 {
		t.Fatalf("expected two eligible directories, got %v", eligible)
	}
}

func TestIndexedDirectoriesErrors(t *testing.T) {
	root := "/repo"
	treeWithStatError := indexedTreeStub{
		entries: map[string][]domain.DirectoryEntry{root: []domain.DirectoryEntry{}},
		exists:  map[string]bool{},
		errDir:  map[string]error{},
		errStat: map[string]error{filepath.Join(root, ".idx", "index.idx"): errors.New("stat failed")},
	}

	if _, err := IndexedDirectories(treeWithStatError, root); err == nil {
		t.Fatal("expected error when index stat fails")
	}

	treeWithReadError := indexedTreeStub{
		entries: map[string][]domain.DirectoryEntry{},
		exists:  map[string]bool{},
		errDir:  map[string]error{root: errors.New("read failed")},
		errStat: map[string]error{},
	}

	if _, err := IndexedDirectories(treeWithReadError, root); err == nil {
		t.Fatal("expected error when directory read fails")
	}
}
