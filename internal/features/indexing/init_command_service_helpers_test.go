package indexing_test

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"sync"

	"idx/internal/features/indexing"
	"idx/internal/shared/filesystem"
)

type fakeProjectTree struct {
	currentDir string
	currentErr error
	gitRoot    string
	readDirMap map[string][]filesystem.DirectoryEntry
	readDirErr map[string]error
	existing   map[string]bool
	existsErr  map[string]error
	removed    []string
	removeErrs map[string]error
	writes     map[string]string
	gitRootErr error
}

func newFakeProjectTree(currentDir, gitRoot string) *fakeProjectTree {
	return &fakeProjectTree{
		currentDir: currentDir,
		gitRoot:    gitRoot,
		readDirMap: map[string][]filesystem.DirectoryEntry{},
		readDirErr: map[string]error{},
		existing:   map[string]bool{},
		existsErr:  map[string]error{},
		removed:    []string{},
		removeErrs: map[string]error{},
		writes:     map[string]string{},
	}
}

func (tree *fakeProjectTree) CurrentDir() (string, error) {
	if tree.currentErr != nil {
		return "", tree.currentErr
	}

	return tree.currentDir, nil
}

func (tree *fakeProjectTree) FindGitRoot(startDir string) (string, error) {
	if tree.gitRootErr != nil {
		return "", tree.gitRootErr
	}

	return tree.gitRoot, nil
}

func (tree *fakeProjectTree) ReadDir(path string) ([]filesystem.DirectoryEntry, error) {
	if err, ok := tree.readDirErr[path]; ok {
		return nil, err
	}

	entries, ok := tree.readDirMap[path]
	if !ok {
		return []filesystem.DirectoryEntry{}, nil
	}

	return entries, nil
}

func (tree *fakeProjectTree) Exists(path string) (bool, error) {
	if err, ok := tree.existsErr[path]; ok {
		return false, err
	}

	return tree.existing[path], nil
}

func (tree *fakeProjectTree) RemoveAll(path string) error {
	tree.removed = append(tree.removed, path)
	if err, hasError := tree.removeErrs[path]; hasError {
		return err
	}

	return nil
}

func (tree *fakeProjectTree) WriteFile(path string, content []byte) error {
	tree.writes[path] = string(content)
	return nil
}

type fakeIgnoreMatcherFactory struct {
	ignoredPaths map[string]bool
	err          error
}

func (factory fakeIgnoreMatcherFactory) New(projectRoot string) (filesystem.IgnoreMatcher, error) {
	if factory.err != nil {
		return nil, factory.err
	}

	return fakeIgnoreMatcher{ignoredPaths: factory.ignoredPaths}, nil
}

type fakeIgnoreMatcher struct {
	ignoredPaths map[string]bool
}

func (matcher fakeIgnoreMatcher) Matches(path string) (bool, error) {
	return matcher.ignoredPaths[path], nil
}

type fakeFileReader struct {
	files map[string]string
}

func (reader fakeFileReader) ReadFile(path string) (string, error) {
	content, ok := reader.files[path]
	if !ok {
		return "", errors.New("file not found")
	}
	return content, nil
}

type fakeBM25Indexer struct{}

func (indexer *fakeBM25Indexer) BuildIndex(documents []indexing.IndexDocument) (*indexing.InvertedIndex, error) {
	index := indexing.NewInvertedIndex()
	for _, document := range documents {
		index.AddDocument(document.Name, document.Path, 10)
	}
	index.DocumentCount = len(documents)
	index.CalculateAverageDocLen()
	return index, nil
}

type fakeIndexRepository struct {
	mu           sync.RWMutex
	savedIndices map[string]*indexing.InvertedIndex
}

func (repo *fakeIndexRepository) SaveIndex(directoryPath string, index *indexing.InvertedIndex) error {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	repo.savedIndices[directoryPath] = index
	return nil
}

func (repo *fakeIndexRepository) LoadIndex(directoryPath string) (*indexing.InvertedIndex, error) {
	repo.mu.RLock()
	defer repo.mu.RUnlock()
	index, ok := repo.savedIndices[directoryPath]
	if !ok {
		return nil, errors.New("index not found")
	}

	return index, nil
}

type fakeChecksumRepository struct {
	mu        sync.RWMutex
	checksums map[string]map[string]string
	snapshots map[string]indexing.DirectoryChecksumSnapshot
	exists    map[string]bool
	saveCount map[string]int
}

func newFakeChecksumRepository() *fakeChecksumRepository {
	return &fakeChecksumRepository{
		checksums: map[string]map[string]string{},
		snapshots: map[string]indexing.DirectoryChecksumSnapshot{},
		exists:    map[string]bool{},
		saveCount: map[string]int{},
	}
}

func (repo *fakeChecksumRepository) Load(directoryPath string) (map[string]string, bool, error) {
	repo.mu.RLock()
	defer repo.mu.RUnlock()
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

func (repo *fakeChecksumRepository) Save(directoryPath string, checksums map[string]string) error {
	cloned := make(map[string]string, len(checksums))
	files := make(map[string]indexing.FileChecksumState, len(checksums))
	for fileName, checksum := range checksums {
		cloned[fileName] = checksum
		files[fileName] = indexing.FileChecksumState{Checksum: checksum}
	}

	repo.mu.Lock()
	defer repo.mu.Unlock()
	repo.exists[directoryPath] = true
	repo.checksums[directoryPath] = cloned
	repo.snapshots[directoryPath] = indexing.DirectoryChecksumSnapshot{Files: files}
	repo.saveCount[directoryPath]++
	return nil
}

func (repo *fakeChecksumRepository) LoadSnapshot(directoryPath string) (indexing.DirectoryChecksumSnapshot, bool, error) {
	repo.mu.RLock()
	defer repo.mu.RUnlock()
	if !repo.exists[directoryPath] {
		return indexing.DirectoryChecksumSnapshot{Files: map[string]indexing.FileChecksumState{}}, false, nil
	}

	snapshot, ok := repo.snapshots[directoryPath]
	if !ok {
		files := make(map[string]indexing.FileChecksumState, len(repo.checksums[directoryPath]))
		for fileName, checksum := range repo.checksums[directoryPath] {
			files[fileName] = indexing.FileChecksumState{Checksum: checksum}
		}

		return indexing.DirectoryChecksumSnapshot{Files: files}, true, nil
	}

	cloned := make(map[string]indexing.FileChecksumState, len(snapshot.Files))
	for fileName, state := range snapshot.Files {
		cloned[fileName] = state
	}

	return indexing.DirectoryChecksumSnapshot{Files: cloned}, true, nil
}

func (repo *fakeChecksumRepository) SaveSnapshot(directoryPath string, snapshot indexing.DirectoryChecksumSnapshot) error {
	checksums := make(map[string]string, len(snapshot.Files))
	files := make(map[string]indexing.FileChecksumState, len(snapshot.Files))
	for fileName, state := range snapshot.Files {
		checksums[fileName] = state.Checksum
		files[fileName] = state
	}

	repo.mu.Lock()
	defer repo.mu.Unlock()
	repo.exists[directoryPath] = true
	repo.checksums[directoryPath] = checksums
	repo.snapshots[directoryPath] = indexing.DirectoryChecksumSnapshot{Files: files}
	repo.saveCount[directoryPath]++

	return nil
}

type countingFileReader struct {
	files     map[string]string
	readCount int
}

func (reader *countingFileReader) ReadFile(path string) (string, error) {
	reader.readCount++
	content, ok := reader.files[path]
	if !ok {
		return "", errors.New("file not found")
	}

	return content, nil
}

func checksumFromContent(content string) string {
	sum := sha256.Sum256([]byte(content))
	return hex.EncodeToString(sum[:])
}

type capturingTextOutput struct {
	lines []string
}

func (output *capturingTextOutput) WriteLine(text string) error {
	output.lines = append(output.lines, text)
	return nil
}
