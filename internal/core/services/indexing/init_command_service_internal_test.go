package indexing

import (
	"errors"
	"path/filepath"
	"testing"

	"idx/internal/core/domain"
	"idx/internal/core/ports"
)

type internalProjectTreeStub struct {
	exists map[string]bool
}

func (tree internalProjectTreeStub) CurrentDir() (string, error) {
	return "", nil
}

func (tree internalProjectTreeStub) FindGitRoot(string) (string, error) {
	return "", nil
}

func (tree internalProjectTreeStub) ReadDir(string) ([]domain.DirectoryEntry, error) {
	return nil, nil
}

func (tree internalProjectTreeStub) Exists(path string) (bool, error) {
	return tree.exists[path], nil
}

func (tree internalProjectTreeStub) RemoveAll(string) error {
	return nil
}

func (tree internalProjectTreeStub) WriteFile(string, []byte) error {
	return nil
}

type legacyChecksumRepoStub struct {
	checksums map[string]map[string]string
	exists    map[string]bool
	saved     map[string]map[string]string
	loadErr   error
	saveErr   error
}

func (repo *legacyChecksumRepoStub) Load(directoryPath string) (map[string]string, bool, error) {
	if repo.loadErr != nil {
		return nil, false, repo.loadErr
	}

	if !repo.exists[directoryPath] {
		return map[string]string{}, false, nil
	}

	current := repo.checksums[directoryPath]
	cloned := make(map[string]string, len(current))
	for fileName, checksum := range current {
		cloned[fileName] = checksum
	}

	return cloned, true, nil
}

func (repo *legacyChecksumRepoStub) Save(directoryPath string, checksums map[string]string) error {
	if repo.saveErr != nil {
		return repo.saveErr
	}

	cloned := make(map[string]string, len(checksums))
	for fileName, checksum := range checksums {
		cloned[fileName] = checksum
	}

	repo.saved[directoryPath] = cloned
	return nil
}

type readerStub struct {
	files map[string]string
	err   error
}

func (reader readerStub) ReadFile(path string) (string, error) {
	if reader.err != nil {
		return "", reader.err
	}

	content, ok := reader.files[path]
	if !ok {
		return "", errors.New("file not found")
	}

	return content, nil
}

func TestLoadChecksumSnapshotFallbackUsesLegacyRepository(t *testing.T) {
	repo := &legacyChecksumRepoStub{
		checksums: map[string]map[string]string{"/repo": {"a.go": "abc"}},
		exists:    map[string]bool{"/repo": true},
		saved:     map[string]map[string]string{},
	}

	service := InitCommandService{checksumRepo: repo}
	snapshot, exists, err := service.loadChecksumSnapshot("/repo")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !exists {
		t.Fatal("expected snapshot to exist")
	}
	if snapshot.Files["a.go"].Checksum != "abc" {
		t.Fatalf("expected checksum abc, got %q", snapshot.Files["a.go"].Checksum)
	}
}

func TestSaveChecksumSnapshotFallbackUsesLegacyRepository(t *testing.T) {
	repo := &legacyChecksumRepoStub{
		checksums: map[string]map[string]string{},
		exists:    map[string]bool{},
		saved:     map[string]map[string]string{},
	}

	service := InitCommandService{checksumRepo: repo}
	err := service.saveChecksumSnapshot("/repo", ports.DirectoryChecksumSnapshot{Files: map[string]ports.FileChecksumState{"a.go": {Checksum: "abc", Size: 10}}})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if repo.saved["/repo"]["a.go"] != "abc" {
		t.Fatalf("expected saved checksum abc, got %q", repo.saved["/repo"]["a.go"])
	}
}

func TestShouldReindexDirectoryDecisions(t *testing.T) {
	root := "/repo"
	indexPath := filepath.Join(root, ".idx", "index.idx")

	t.Run("reindex when index does not exist", func(t *testing.T) {
		repo := &legacyChecksumRepoStub{checksums: map[string]map[string]string{}, exists: map[string]bool{}, saved: map[string]map[string]string{}}
		service := InitCommandService{projectTree: internalProjectTreeStub{exists: map[string]bool{}}, checksumRepo: repo}

		reindex, err := service.shouldReindexDirectory(root, map[string]string{"a.go": "1"})
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if !reindex {
			t.Fatal("expected reindex when index is missing")
		}
	})

	t.Run("reindex when checksum file does not exist", func(t *testing.T) {
		repo := &legacyChecksumRepoStub{checksums: map[string]map[string]string{}, exists: map[string]bool{}, saved: map[string]map[string]string{}}
		service := InitCommandService{projectTree: internalProjectTreeStub{exists: map[string]bool{indexPath: true}}, checksumRepo: repo}

		reindex, err := service.shouldReindexDirectory(root, map[string]string{"a.go": "1"})
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if !reindex {
			t.Fatal("expected reindex when checksums are missing")
		}
	})

	t.Run("skip reindex when checksums are unchanged", func(t *testing.T) {
		repo := &legacyChecksumRepoStub{
			checksums: map[string]map[string]string{root: {"a.go": "1"}},
			exists:    map[string]bool{root: true},
			saved:     map[string]map[string]string{},
		}
		service := InitCommandService{projectTree: internalProjectTreeStub{exists: map[string]bool{indexPath: true}}, checksumRepo: repo}

		reindex, err := service.shouldReindexDirectory(root, map[string]string{"a.go": "1"})
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if reindex {
			t.Fatal("expected no reindex when checksums are equal")
		}
	})

	t.Run("reindex when checksums differ", func(t *testing.T) {
		repo := &legacyChecksumRepoStub{
			checksums: map[string]map[string]string{root: {"a.go": "1"}},
			exists:    map[string]bool{root: true},
			saved:     map[string]map[string]string{},
		}
		service := InitCommandService{projectTree: internalProjectTreeStub{exists: map[string]bool{indexPath: true}}, checksumRepo: repo}

		reindex, err := service.shouldReindexDirectory(root, map[string]string{"a.go": "2"})
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if !reindex {
			t.Fatal("expected reindex when checksums differ")
		}
	})
}

func TestDirectoryChecksumsAndComparisonHelpers(t *testing.T) {
	service := InitCommandService{fileReader: readerStub{files: map[string]string{"/repo/a.go": "alpha", "/repo/b.go": "beta"}}}

	checksums, err := service.directoryChecksums([]domain.DirectoryEntry{{Name: "a.go", Path: "/repo/a.go"}, {Name: "b.go", Path: "/repo/b.go"}})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(checksums) != 2 {
		t.Fatalf("expected two checksums, got %d", len(checksums))
	}
	if checksums["a.go"] == checksums["b.go"] {
		t.Fatal("expected different files to produce different checksums")
	}

	if !sameChecksums(map[string]string{"a": "1"}, map[string]string{"a": "1"}) {
		t.Fatal("expected sameChecksums to return true for equal maps")
	}
	if sameChecksums(map[string]string{"a": "1"}, map[string]string{"a": "2"}) {
		t.Fatal("expected sameChecksums to return false for different values")
	}
	if sameChecksums(map[string]string{"a": "1"}, map[string]string{"b": "1"}) {
		t.Fatal("expected sameChecksums to return false for different keys")
	}

	_, err = InitCommandService{fileReader: readerStub{err: errors.New("read failed")}}.directoryChecksums([]domain.DirectoryEntry{{Name: "a.go", Path: "/repo/a.go"}})
	if err == nil {
		t.Fatal("expected read error from directoryChecksums")
	}
}