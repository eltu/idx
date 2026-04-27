package indexing_test

import (
	"errors"
	"path/filepath"
	"testing"

	"idx/internal/core/domain"
	"idx/internal/core/ports"
	"idx/internal/core/services/indexing"
)

func TestInitCommandServiceSyncReturnsCurrentDirError(t *testing.T) {
	tree := newFakeProjectTree("", "")
	tree.currentErr = errors.New("cwd unavailable")

	service := indexing.NewInitCommandService(
		tree,
		fakeIgnoreMatcherFactory{ignoredPaths: map[string]bool{}},
		&capturingTextOutput{},
		fakeFileReader{files: map[string]string{}},
		&fakeBM25Indexer{},
		&fakeIndexRepository{savedIndices: map[string]*domain.InvertedIndex{}},
		newFakeChecksumRepository(),
	)

	err := service.Sync()
	if err == nil {
		t.Fatal("expected current directory error, got nil")
	}
}

func TestInitCommandServiceSyncReturnsErrorWhenDependenciesAreNil(t *testing.T) {
	service := indexing.NewInitCommandService(nil, nil, nil, nil, nil, nil, nil)

	err := service.Sync()
	if err == nil {
		t.Fatal("expected dependency validation error, got nil")
	}
}

func TestInitCommandServiceSyncReturnsMatcherFactoryError(t *testing.T) {
	rootDir := filepath.Join(string(filepath.Separator), "repo")
	tree := newFakeProjectTree(rootDir, rootDir)
	tree.existing[filepath.Join(rootDir, ".idx", "index.idx")] = true

	service := indexing.NewInitCommandService(
		tree,
		fakeIgnoreMatcherFactory{err: errors.New("bad gitignore")},
		&capturingTextOutput{},
		fakeFileReader{files: map[string]string{}},
		&fakeBM25Indexer{},
		&fakeIndexRepository{savedIndices: map[string]*domain.InvertedIndex{}},
		newFakeChecksumRepository(),
	)

	err := service.Sync()
	if err == nil {
		t.Fatal("expected matcher factory error, got nil")
	}
}

func TestInitCommandServiceSyncRebuildsAllIndexedDirectories(t *testing.T) {
	rootDir := filepath.Join(string(filepath.Separator), "repo")
	childDir := filepath.Join(rootDir, "child")
	otherDir := filepath.Join(rootDir, "other")
	tree := newFakeProjectTree(rootDir, rootDir)
	tree.existing[filepath.Join(rootDir, ".idx", "index.idx")] = true
	tree.existing[filepath.Join(childDir, ".idx", "index.idx")] = true
	tree.readDirMap[rootDir] = []domain.DirectoryEntry{
		{Name: ".idx", Path: filepath.Join(rootDir, ".idx"), IsDir: true},
		{Name: "root.txt", Path: filepath.Join(rootDir, "root.txt"), IsDir: false},
		{Name: "child", Path: childDir, IsDir: true},
		{Name: "other", Path: otherDir, IsDir: true},
	}
	tree.readDirMap[childDir] = []domain.DirectoryEntry{{Name: "nested.txt", Path: filepath.Join(childDir, "nested.txt"), IsDir: false}}
	tree.readDirMap[otherDir] = []domain.DirectoryEntry{{Name: "other.txt", Path: filepath.Join(otherDir, "other.txt"), IsDir: false}}
	output := &capturingTextOutput{}
	fileReader := fakeFileReader{files: map[string]string{
		filepath.Join(rootDir, "root.txt"):    "root content",
		filepath.Join(childDir, "nested.txt"): "nested content",
		filepath.Join(otherDir, "other.txt"):  "other content",
	}}
	indexRepo := &fakeIndexRepository{savedIndices: make(map[string]*domain.InvertedIndex)}
	service := indexing.NewInitCommandService(tree, fakeIgnoreMatcherFactory{ignoredPaths: map[string]bool{}}, output, fileReader, &fakeBM25Indexer{}, indexRepo, newFakeChecksumRepository())

	err := service.Sync()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if indexRepo.savedIndices[rootDir].DocumentCount != 1 {
		t.Fatalf("expected synced root index with 1 document, got %d", indexRepo.savedIndices[rootDir].DocumentCount)
	}

	if indexRepo.savedIndices[childDir].DocumentCount != 1 {
		t.Fatalf("expected synced child index with 1 document, got %d", indexRepo.savedIndices[childDir].DocumentCount)
	}

	if indexRepo.savedIndices[otherDir].DocumentCount != 1 {
		t.Fatalf("expected sync to create a new index for %q with 1 document, got %d", otherDir, indexRepo.savedIndices[otherDir].DocumentCount)
	}
}

func TestInitCommandServiceSyncSkipsReindexWhenChecksumsUnchanged(t *testing.T) {
	rootDir := filepath.Join(string(filepath.Separator), "repo")
	filePath := filepath.Join(rootDir, "root.txt")
	tree := newFakeProjectTree(rootDir, rootDir)
	tree.existing[filepath.Join(rootDir, ".idx", "index.idx")] = true
	tree.readDirMap[rootDir] = []domain.DirectoryEntry{{Name: ".idx", Path: filepath.Join(rootDir, ".idx"), IsDir: true}, {Name: "root.txt", Path: filePath, IsDir: false}}

	indexRepo := &fakeIndexRepository{savedIndices: make(map[string]*domain.InvertedIndex)}
	checksumRepo := newFakeChecksumRepository()
	checksumRepo.exists[rootDir] = true
	checksumRepo.checksums[rootDir] = map[string]string{"root.txt": checksumFromContent("root content")}
	service := indexing.NewInitCommandService(tree, fakeIgnoreMatcherFactory{ignoredPaths: map[string]bool{}}, &capturingTextOutput{}, fakeFileReader{files: map[string]string{filePath: "root content"}}, &fakeBM25Indexer{}, indexRepo, checksumRepo)

	err := service.Sync()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(indexRepo.savedIndices) != 0 {
		t.Fatalf("expected no reindex saves for unchanged checksum, got %d", len(indexRepo.savedIndices))
	}
}

func TestInitCommandServiceSyncReindexesWhenChecksumsChanged(t *testing.T) {
	rootDir := filepath.Join(string(filepath.Separator), "repo")
	filePath := filepath.Join(rootDir, "root.txt")
	tree := newFakeProjectTree(rootDir, rootDir)
	tree.existing[filepath.Join(rootDir, ".idx", "index.idx")] = true
	tree.readDirMap[rootDir] = []domain.DirectoryEntry{{Name: ".idx", Path: filepath.Join(rootDir, ".idx"), IsDir: true}, {Name: "root.txt", Path: filePath, IsDir: false}}

	indexRepo := &fakeIndexRepository{savedIndices: make(map[string]*domain.InvertedIndex)}
	checksumRepo := newFakeChecksumRepository()
	checksumRepo.exists[rootDir] = true
	checksumRepo.checksums[rootDir] = map[string]string{"root.txt": checksumFromContent("old root content")}
	service := indexing.NewInitCommandService(tree, fakeIgnoreMatcherFactory{ignoredPaths: map[string]bool{}}, &capturingTextOutput{}, fakeFileReader{files: map[string]string{filePath: "new root content"}}, &fakeBM25Indexer{}, indexRepo, checksumRepo)

	err := service.Sync()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if checksumRepo.saveCount[rootDir] != 1 {
		t.Fatalf("expected checksum update count 1, got %d", checksumRepo.saveCount[rootDir])
	}
}

func TestInitCommandServiceSyncRemovesIndexWhenDirectoryHasNoIndexableFiles(t *testing.T) {
	rootDir := filepath.Join(string(filepath.Separator), "repo")
	tree := newFakeProjectTree(rootDir, rootDir)
	tree.existing[filepath.Join(rootDir, ".idx", "index.idx")] = true
	tree.readDirMap[rootDir] = []domain.DirectoryEntry{{Name: ".idx", Path: filepath.Join(rootDir, ".idx"), IsDir: true}}

	service := indexing.NewInitCommandService(tree, fakeIgnoreMatcherFactory{ignoredPaths: map[string]bool{}}, &capturingTextOutput{}, fakeFileReader{files: map[string]string{}}, &fakeBM25Indexer{}, &fakeIndexRepository{savedIndices: make(map[string]*domain.InvertedIndex)}, newFakeChecksumRepository())

	err := service.Sync()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	removedIndexPath := filepath.Join(rootDir, ".idx")
	if len(tree.removed) != 1 || tree.removed[0] != removedIndexPath {
		t.Fatalf("expected removed index path %q, got %v", removedIndexPath, tree.removed)
	}
}

func TestInitCommandServiceSyncRemovesIndexForNowIgnoredDirectory(t *testing.T) {
	rootDir := filepath.Join(string(filepath.Separator), "repo")
	ignoredDir := filepath.Join(rootDir, "vendor")
	tree := newFakeProjectTree(rootDir, rootDir)
	tree.existing[filepath.Join(rootDir, ".idx", "index.idx")] = true
	tree.existing[filepath.Join(ignoredDir, ".idx", "index.idx")] = true
	tree.readDirMap[rootDir] = []domain.DirectoryEntry{{Name: ".idx", Path: filepath.Join(rootDir, ".idx"), IsDir: true}, {Name: "root.txt", Path: filepath.Join(rootDir, "root.txt"), IsDir: false}, {Name: "vendor", Path: ignoredDir, IsDir: true}}
	tree.readDirMap[ignoredDir] = []domain.DirectoryEntry{{Name: "dep.txt", Path: filepath.Join(ignoredDir, "dep.txt"), IsDir: false}}

	indexRepo := &fakeIndexRepository{savedIndices: make(map[string]*domain.InvertedIndex)}
	service := indexing.NewInitCommandService(tree, fakeIgnoreMatcherFactory{ignoredPaths: map[string]bool{"vendor/": true}}, &capturingTextOutput{}, fakeFileReader{files: map[string]string{filepath.Join(rootDir, "root.txt"): "root content"}}, &fakeBM25Indexer{}, indexRepo, newFakeChecksumRepository())

	err := service.Sync()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	removedIndexPath := filepath.Join(ignoredDir, ".idx")
	if len(tree.removed) != 1 || tree.removed[0] != removedIndexPath {
		t.Fatalf("expected removed index path %q, got %v", removedIndexPath, tree.removed)
	}

	if _, exists := indexRepo.savedIndices[ignoredDir]; exists {
		t.Fatalf("did not expect ignored directory %q to be reindexed", ignoredDir)
	}
}

func TestInitCommandServiceSyncCreatesIndexWhenDirectoryLeavesGitignore(t *testing.T) {
	rootDir := filepath.Join(string(filepath.Separator), "repo")
	newlyAllowedDir := filepath.Join(rootDir, "src")
	tree := newFakeProjectTree(rootDir, rootDir)
	tree.existing[filepath.Join(rootDir, ".idx", "index.idx")] = true
	tree.readDirMap[rootDir] = []domain.DirectoryEntry{{Name: ".idx", Path: filepath.Join(rootDir, ".idx"), IsDir: true}, {Name: "root.txt", Path: filepath.Join(rootDir, "root.txt"), IsDir: false}, {Name: "src", Path: newlyAllowedDir, IsDir: true}}
	tree.readDirMap[newlyAllowedDir] = []domain.DirectoryEntry{{Name: "app.txt", Path: filepath.Join(newlyAllowedDir, "app.txt"), IsDir: false}}

	indexRepo := &fakeIndexRepository{savedIndices: make(map[string]*domain.InvertedIndex)}
	service := indexing.NewInitCommandService(tree, fakeIgnoreMatcherFactory{ignoredPaths: map[string]bool{}}, &capturingTextOutput{}, fakeFileReader{files: map[string]string{filepath.Join(rootDir, "root.txt"): "root content", filepath.Join(newlyAllowedDir, "app.txt"): "source content"}}, &fakeBM25Indexer{}, indexRepo, newFakeChecksumRepository())

	err := service.Sync()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if indexRepo.savedIndices[newlyAllowedDir].DocumentCount != 1 {
		t.Fatalf("expected newly allowed directory %q to be indexed with 1 document, got %d", newlyAllowedDir, indexRepo.savedIndices[newlyAllowedDir].DocumentCount)
	}
}

func TestInitCommandServiceSyncReindexesDirectoryWhenFileLeavesGitignore(t *testing.T) {
	rootDir := filepath.Join(string(filepath.Separator), "repo")
	tree := newFakeProjectTree(rootDir, rootDir)
	tree.existing[filepath.Join(rootDir, ".idx", "index.idx")] = true
	tree.readDirMap[rootDir] = []domain.DirectoryEntry{{Name: ".idx", Path: filepath.Join(rootDir, ".idx"), IsDir: true}, {Name: "allowed.txt", Path: filepath.Join(rootDir, "allowed.txt"), IsDir: false}, {Name: "debug.log", Path: filepath.Join(rootDir, "debug.log"), IsDir: false}}

	indexRepo := &fakeIndexRepository{savedIndices: make(map[string]*domain.InvertedIndex)}
	service := indexing.NewInitCommandService(tree, fakeIgnoreMatcherFactory{ignoredPaths: map[string]bool{}}, &capturingTextOutput{}, fakeFileReader{files: map[string]string{filepath.Join(rootDir, "allowed.txt"): "allowed content", filepath.Join(rootDir, "debug.log"): "log content"}}, &fakeBM25Indexer{}, indexRepo, newFakeChecksumRepository())

	err := service.Sync()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if indexRepo.savedIndices[rootDir].DocumentCount != 2 {
		t.Fatalf("expected root index to include file that left .gitignore with 2 documents, got %d", indexRepo.savedIndices[rootDir].DocumentCount)
	}
}

func TestInitCommandServiceSyncRequiresProjectRootIndex(t *testing.T) {
	rootDir := filepath.Join(string(filepath.Separator), "repo")
	tree := newFakeProjectTree(rootDir, rootDir)
	service := indexing.NewInitCommandService(tree, fakeIgnoreMatcherFactory{ignoredPaths: map[string]bool{}}, &capturingTextOutput{}, fakeFileReader{files: make(map[string]string)}, &fakeBM25Indexer{}, &fakeIndexRepository{savedIndices: make(map[string]*domain.InvertedIndex)}, newFakeChecksumRepository())

	err := service.Sync()
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
}

func TestInitCommandServiceSyncMustRunFromProjectRoot(t *testing.T) {
	rootDir := filepath.Join(string(filepath.Separator), "repo")
	childDir := filepath.Join(rootDir, "child")
	tree := newFakeProjectTree(childDir, rootDir)
	tree.existing[filepath.Join(rootDir, ".idx", "index.idx")] = true
	service := indexing.NewInitCommandService(tree, fakeIgnoreMatcherFactory{ignoredPaths: map[string]bool{}}, &capturingTextOutput{}, fakeFileReader{files: make(map[string]string)}, &fakeBM25Indexer{}, &fakeIndexRepository{savedIndices: make(map[string]*domain.InvertedIndex)}, newFakeChecksumRepository())

	err := service.Sync()
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
}

func TestInitCommandServiceSyncSkipsRehashWhenMetadataUnchanged(t *testing.T) {
	rootDir := filepath.Join(string(filepath.Separator), "repo")
	filePath := filepath.Join(rootDir, "root.txt")
	tree := newFakeProjectTree(rootDir, rootDir)
	tree.existing[filepath.Join(rootDir, ".idx", "index.idx")] = true
	tree.readDirMap[rootDir] = []domain.DirectoryEntry{{Name: ".idx", Path: filepath.Join(rootDir, ".idx"), IsDir: true}, {Name: "root.txt", Path: filePath, IsDir: false, Size: int64(len("root content")), ModTimeUnixNano: 42}}

	reader := &countingFileReader{files: map[string]string{filePath: "root content"}}
	indexRepo := &fakeIndexRepository{savedIndices: make(map[string]*domain.InvertedIndex)}
	checksumRepo := newFakeChecksumRepository()
	checksumRepo.exists[rootDir] = true
	checksumRepo.snapshots[rootDir] = ports.DirectoryChecksumSnapshot{Files: map[string]ports.FileChecksumState{"root.txt": {Checksum: checksumFromContent("root content"), Size: int64(len("root content")), ModTimeUnixNano: 42}}}

	service := indexing.NewInitCommandService(tree, fakeIgnoreMatcherFactory{ignoredPaths: map[string]bool{}}, &capturingTextOutput{}, reader, &fakeBM25Indexer{}, indexRepo, checksumRepo)

	err := service.Sync()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if reader.readCount != 0 {
		t.Fatalf("expected 0 file reads when metadata is unchanged, got %d", reader.readCount)
	}
}

func TestInitCommandServiceSyncRehashesWhenMetadataChanges(t *testing.T) {
	rootDir := filepath.Join(string(filepath.Separator), "repo")
	filePath := filepath.Join(rootDir, "root.txt")
	tree := newFakeProjectTree(rootDir, rootDir)
	tree.existing[filepath.Join(rootDir, ".idx", "index.idx")] = true
	tree.readDirMap[rootDir] = []domain.DirectoryEntry{{Name: ".idx", Path: filepath.Join(rootDir, ".idx"), IsDir: true}, {Name: "root.txt", Path: filePath, IsDir: false, Size: int64(len("new content")), ModTimeUnixNano: 200}}

	reader := &countingFileReader{files: map[string]string{filePath: "new content"}}
	indexRepo := &fakeIndexRepository{savedIndices: make(map[string]*domain.InvertedIndex)}
	checksumRepo := newFakeChecksumRepository()
	checksumRepo.exists[rootDir] = true
	checksumRepo.snapshots[rootDir] = ports.DirectoryChecksumSnapshot{Files: map[string]ports.FileChecksumState{"root.txt": {Checksum: checksumFromContent("old content"), Size: int64(len("old content")), ModTimeUnixNano: 100}}}

	service := indexing.NewInitCommandService(tree, fakeIgnoreMatcherFactory{ignoredPaths: map[string]bool{}}, &capturingTextOutput{}, reader, &fakeBM25Indexer{}, indexRepo, checksumRepo)

	err := service.Sync()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if reader.readCount == 0 {
		t.Fatal("expected at least one file read when metadata changes")
	}
}
