package indexing

import (
	"errors"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"idx/internal/core/domain"
	"idx/internal/core/ports"
)

func TestInitCommandServiceBuildAndSaveIndexErrorBranches(t *testing.T) {
	root := filepath.Join(string(filepath.Separator), "repo")
	entry := domain.DirectoryEntry{Name: "a.txt", Path: filepath.Join(root, "a.txt")}
	snapshot := ports.DirectoryChecksumSnapshot{Files: map[string]ports.FileChecksumState{}}

	service := newValidInternalService(root)
	service.fileReader = internalFileReader{err: errors.New("read failure")}
	if err := service.buildAndSaveIndex(root, []domain.DirectoryEntry{entry}, snapshot, map[string]struct{}{}); err == nil {
		t.Fatal("expected file read error")
	}

	service = newValidInternalService(root)
	service.fileReader = internalFileReader{files: map[string]string{entry.Path: "hello"}}
	service.indexer = internalIndexer{err: errors.New("indexer failure")}
	if err := service.buildAndSaveIndex(root, []domain.DirectoryEntry{entry}, snapshot, map[string]struct{}{}); err == nil {
		t.Fatal("expected indexer error")
	}

	service = newValidInternalService(root)
	service.fileReader = internalFileReader{files: map[string]string{entry.Path: "hello"}}
	service.indexRepo = internalIndexRepo{saveErr: errors.New("save failure")}
	if err := service.buildAndSaveIndex(root, []domain.DirectoryEntry{entry}, snapshot, map[string]struct{}{}); err == nil {
		t.Fatal("expected index save error")
	}
}

func TestInitCommandServiceShouldReindexDirectoryBranches(t *testing.T) {
	root := filepath.Join(string(filepath.Separator), "repo")
	service := newValidInternalService(root)
	tree := service.projectTree.(*internalProjectTree)

	tree.existsErr[indexFilePath(root)] = errors.New("exists failure")
	if _, err := service.shouldReindexDirectory(root, map[string]string{}); err == nil {
		t.Fatal("expected hasDirectoryIndex error")
	}

	service = newValidInternalService(root)
	service.checksumRepo = internalChecksumRepo{loadErr: errors.New("load failure")}
	if _, err := service.shouldReindexDirectory(root, map[string]string{}); err == nil {
		t.Fatal("expected checksum repository load error")
	}

	service = newValidInternalService(root)
	service.checksumRepo = internalChecksumRepo{loadData: map[string]string{}, exists: false}
	should, err := service.shouldReindexDirectory(root, map[string]string{})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !should {
		t.Fatal("expected reindex when checksums are not present")
	}
}

func TestInitCommandServiceHelpersCoverUncoveredBranches(t *testing.T) {
	root := filepath.Join(string(filepath.Separator), "repo")
	indexedAt := time.Unix(1700000000, 0).UTC()

	if got := buildChangedLogEntries(nil, ports.DirectoryChecksumSnapshot{Files: map[string]ports.FileChecksumState{}}, map[string]struct{}{}, indexedAt); len(got) != 0 {
		t.Fatalf("expected empty log entries, got %d", len(got))
	}

	entries := []domain.DirectoryEntry{{Name: "a.txt", Path: filepath.Join(root, "a.txt")}}
	snapshot := ports.DirectoryChecksumSnapshot{Files: map[string]ports.FileChecksumState{}}
	changed := map[string]struct{}{"a.txt": {}}
	if got := buildChangedLogEntries(entries, snapshot, changed, indexedAt); len(got) != 0 {
		t.Fatalf("expected skip when snapshot state missing, got %d", len(got))
	}

	_, err := filterEntries([]domain.DirectoryEntry{{Name: "a.txt", Path: filepath.Join(root, "a.txt")}}, root, internalMatcher{err: errors.New("matcher failure")})
	if err == nil {
		t.Fatal("expected filterEntries matcher error")
	}

	if sameChecksums(map[string]string{"a": "1"}, map[string]string{"a": "1", "b": "2"}) {
		t.Fatal("expected sameChecksums false when map sizes differ")
	}

	if sameSnapshotChecksums(map[string]ports.FileChecksumState{"a": {Checksum: "1"}}, map[string]ports.FileChecksumState{"a": {Checksum: "2"}}) {
		t.Fatal("expected sameSnapshotChecksums false when checksum differs")
	}

	matcher := &countingMatcher{}
	filtered, err := filterEntries([]domain.DirectoryEntry{{Name: "link", Path: filepath.Join(root, "link"), IsSymlink: true}, {Name: "file.txt", Path: filepath.Join(root, "file.txt")}}, root, matcher)
	if err != nil {
		t.Fatalf("expected filterEntries to skip symlink without errors, got %v", err)
	}
	if len(filtered) != 1 || filtered[0].Name != "file.txt" {
		t.Fatalf("expected only non-symlink file to remain, got %+v", filtered)
	}
	if matcher.calls != 1 {
		t.Fatalf("expected matcher to be called only for non-symlink entries, got %d calls", matcher.calls)
	}
}

func TestInitCommandServiceDirectoryChecksumsAndSameChecksumsBranches(t *testing.T) {
	root := filepath.Join(string(filepath.Separator), "repo")
	service := newValidInternalService(root)

	firstPath := filepath.Join(root, "a.txt")
	secondPath := filepath.Join(root, "b.txt")
	service.fileReader = internalFileReader{files: map[string]string{firstPath: "a", secondPath: "b"}}

	checksums, err := service.directoryChecksums([]domain.DirectoryEntry{{Name: "a.txt", Path: firstPath}, {Name: "b.txt", Path: secondPath}})
	if err != nil {
		t.Fatalf("expected checksums without error, got %v", err)
	}
	if len(checksums) != 2 {
		t.Fatalf("expected 2 checksums, got %d", len(checksums))
	}

	service.fileReader = internalFileReader{err: errors.New("read failure")}
	if _, err := service.directoryChecksums([]domain.DirectoryEntry{{Name: "a.txt", Path: firstPath}}); err == nil {
		t.Fatal("expected directoryChecksums read error")
	}

	if !sameChecksums(map[string]string{"a": "1"}, map[string]string{"a": "1"}) {
		t.Fatal("expected sameChecksums true for equal maps")
	}
	if sameChecksums(map[string]string{"a": "1"}, map[string]string{"b": "1"}) {
		t.Fatal("expected sameChecksums false when key is missing")
	}
	if sameChecksums(map[string]string{"a": "1"}, map[string]string{"a": "2"}) {
		t.Fatal("expected sameChecksums false when value differs")
	}
}

func TestInitCommandServiceLoadAndSaveChecksumSnapshotBranches(t *testing.T) {
	root := filepath.Join(string(filepath.Separator), "repo")
	service := newValidInternalService(root)

	service.checksumRepo = internalChecksumRepo{loadData: map[string]string{"a.txt": "hash"}, exists: true}
	snapshot, exists, err := service.loadChecksumSnapshot(root)
	if err != nil {
		t.Fatalf("expected snapshot load without error, got %v", err)
	}
	if !exists || snapshot.Files["a.txt"].Checksum != "hash" {
		t.Fatalf("expected converted snapshot state, got exists=%v snapshot=%v", exists, snapshot.Files)
	}

	service.checksumRepo = internalChecksumRepo{saveErr: errors.New("save failure")}
	err = service.saveChecksumSnapshot(root, ports.DirectoryChecksumSnapshot{Files: map[string]ports.FileChecksumState{"a.txt": {Checksum: "hash"}}})
	if err == nil {
		t.Fatal("expected fallback save checksum error")
	}

	service.checksumRepo = internalSnapshotChecksumRepo{loadSnapshotResult: ports.DirectoryChecksumSnapshot{Files: map[string]ports.FileChecksumState{"a.txt": {Checksum: "hash"}}}, loadSnapshotExists: true}
	snapshot, exists, err = service.loadChecksumSnapshot(root)
	if err != nil {
		t.Fatalf("expected snapshot repository load without error, got %v", err)
	}
	if !exists || snapshot.Files["a.txt"].Checksum != "hash" {
		t.Fatalf("expected snapshot repository data, got exists=%v snapshot=%v", exists, snapshot.Files)
	}

	service.checksumRepo = internalSnapshotChecksumRepo{saveSnapshotErr: errors.New("snapshot save failure")}
	err = service.saveChecksumSnapshot(root, ports.DirectoryChecksumSnapshot{Files: map[string]ports.FileChecksumState{}})
	if err == nil {
		t.Fatal("expected snapshot repository save error")
	}
}

func TestInitCommandServiceShouldReindexDirectoryAndMetadataBranches(t *testing.T) {
	root := filepath.Join(string(filepath.Separator), "repo")
	service := newValidInternalService(root)
	tree := service.projectTree.(*internalProjectTree)

	tree.existsMap[indexFilePath(root)] = false
	should, err := service.shouldReindexDirectory(root, map[string]string{})
	if err != nil {
		t.Fatalf("expected no error when no index, got %v", err)
	}
	if !should {
		t.Fatal("expected shouldReindexDirectory true when index is missing")
	}

	tree.existsMap[indexFilePath(root)] = true
	service.checksumRepo = internalChecksumRepo{loadData: map[string]string{"a": "1"}, exists: true}
	should, err = service.shouldReindexDirectory(root, map[string]string{"a": "1"})
	if err != nil {
		t.Fatalf("expected no error for equal checksums, got %v", err)
	}
	if should {
		t.Fatal("expected shouldReindexDirectory false when checksums are equal")
	}

	should, err = service.shouldReindexDirectory(root, map[string]string{"a": "2"})
	if err != nil {
		t.Fatalf("expected no error for different checksums, got %v", err)
	}
	if !should {
		t.Fatal("expected shouldReindexDirectory true when checksums differ")
	}

	if metadataUnchanged(domain.DirectoryEntry{Size: 10, ModTimeUnixNano: 20}, ports.FileChecksumState{Size: 10, ModTimeUnixNano: 20}) == false {
		t.Fatal("expected metadataUnchanged true for identical metadata")
	}
}

func TestInitCommandServiceRemoveDirectoryIndexErrorBranch(t *testing.T) {
	root := filepath.Join(string(filepath.Separator), "repo")
	service := newValidInternalService(root)
	service.projectTree.(*internalProjectTree).removeErr = errors.New("remove failure")

	err := service.removeDirectoryIndex(root)
	if err == nil {
		t.Fatal("expected removeDirectoryIndex error")
	}
}

func TestCloneInspectDocTermStatsNilReturnsEmpty(t *testing.T) {
	got := cloneInspectDocTermStats(nil)
	if got == nil {
		t.Fatal("expected non-nil result for nil input")
	}
	if got.TF != 0 || len(got.Positions) != 0 {
		t.Fatalf("expected empty DocTermStats, got TF=%d positions=%v", got.TF, got.Positions)
	}
}

func TestCloneInspectDocTermStatsCopiesPositions(t *testing.T) {
	original := &domain.DocTermStats{TF: 3, Positions: []int{1, 4, 7}}
	cloned := cloneInspectDocTermStats(original)

	if cloned.TF != 3 {
		t.Fatalf("expected TF=3, got %d", cloned.TF)
	}
	if len(cloned.Positions) != 3 || cloned.Positions[0] != 1 || cloned.Positions[2] != 7 {
		t.Fatalf("expected positions [1 4 7], got %v", cloned.Positions)
	}

	original.Positions[0] = 99
	if cloned.Positions[0] == 99 {
		t.Fatal("expected deep copy of positions slice, but clone was mutated")
	}
}

func TestRunInspectTUIForDirectoryCallsInspectUI(t *testing.T) {
	root := filepath.Join(string(filepath.Separator), "repo")
	called := false
	service := newValidInternalService(root)
	service.inspectUI = internalInspectUIRunnerFunc(func(_ *domain.InvertedIndex) error {
		called = true
		return nil
	})

	if err := service.runInspectTUIForDirectory(root); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !called {
		t.Fatal("expected inspectUI.Run to be called")
	}
}

func TestRunInspectTUIForDirectoryPropagatesIndexLoadError(t *testing.T) {
	root := filepath.Join(string(filepath.Separator), "repo")
	service := newValidInternalService(root)
	service.indexRepo = internalIndexRepo{loadErr: errors.New("load failure")}

	if err := service.runInspectTUIForDirectory(root); err == nil {
		t.Fatal("expected index load error to be propagated")
	}
}

type internalInspectUIRunnerFunc func(*domain.InvertedIndex) error

func (fn internalInspectUIRunnerFunc) Run(index *domain.InvertedIndex) error { return fn(index) }

func TestMergeInspectTermsCopiesTermsWithNamespacedDocIDs(t *testing.T) {
	source := domain.NewInvertedIndex()
	source.AddDocument("service.go", "internal/service.go", 10)
	source.AddTerm("alpha", "service.go", 2, []int{1, 5})

	target := domain.NewInvertedIndex()
	mergeInspectTerms(target, "/repo/internal", source)

	termStats, ok := target.Terms["alpha"]
	if !ok {
		t.Fatal("expected term 'alpha' to be merged into target")
	}

	expectedDocID := "/repo/internal::service.go"
	if _, ok := termStats.Docs[expectedDocID]; !ok {
		t.Fatalf("expected namespaced docID %q in merged terms, got keys: %v", expectedDocID, termStats.Docs)
	}
}

func TestMergeInspectTermsSkipsNilTermStats(t *testing.T) {
	source := domain.NewInvertedIndex()
	source.Terms["alpha"] = nil

	target := domain.NewInvertedIndex()
	mergeInspectTerms(target, "/repo", source)

	if _, ok := target.Terms["alpha"]; ok {
		t.Fatal("expected nil term stats to be skipped during merge")
	}
}

func TestRunInspectTUIForDirectoriesCallsInspectUI(t *testing.T) {
	root := filepath.Join(string(filepath.Separator), "repo")
	called := false
	service := newValidInternalService(root)
	service.indexRepo = internalIndexRepo{index: domain.NewInvertedIndex()}
	service.inspectUI = internalInspectUIRunnerFunc(func(_ *domain.InvertedIndex) error {
		called = true
		return nil
	})

	if err := service.runInspectTUIForDirectories([]string{root}); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !called {
		t.Fatal("expected inspectUI.Run to be called for directories")
	}
}

func TestRunInspectTUIForDirectoriesPropagatesLoadError(t *testing.T) {
	root := filepath.Join(string(filepath.Separator), "repo")
	service := newValidInternalService(root)
	service.indexRepo = internalIndexRepo{loadErr: errors.New("load failure")}

	if err := service.runInspectTUIForDirectories([]string{root}); err == nil {
		t.Fatal("expected index load error to be propagated from runInspectTUIForDirectories")
	}
}

func TestWatchReturnsErrorForNonPositiveDebounce(t *testing.T) {
	root := filepath.Join(string(filepath.Separator), "repo")
	service := newValidInternalService(root)

	err := service.Watch(false, 0)
	if err == nil {
		t.Fatal("expected error for zero debounce")
	}
}

func TestWatchReturnsErrorWhenDaemonAlreadyMonitoring(t *testing.T) {
	root := filepath.Join(string(filepath.Separator), "repo")
	service := newValidInternalService(root)
	service.daemonRepo = internalDaemonRepo{state: &domain.DaemonState{Projects: []domain.MonitoredProject{{Path: root, Enabled: true, PID: 1}}}}

	err := service.Watch(false, time.Second)
	if err == nil {
		t.Fatal("expected error when daemon is already monitoring")
	}
}

func TestWatchSkipsDaemonBlockWhenStartedByDaemon(t *testing.T) {
	root := filepath.Join(string(filepath.Separator), "repo")
	service := newValidInternalService(root)
	service.daemonRepo = internalDaemonRepo{state: &domain.DaemonState{Projects: []domain.MonitoredProject{{Path: root, Enabled: true, PID: 1}}}}
	t.Setenv(daemonChildEnvVar, "1")

	err := service.Watch(false, time.Second)
	if err != nil && strings.Contains(err.Error(), "cannot run watch: daemon is already monitoring this project") {
		t.Fatalf("expected daemon child watch to bypass daemon self-check, got %v", err)
	}
}

func TestWatchIgnoresOtherMonitoredProjects(t *testing.T) {
	root := filepath.Join(string(filepath.Separator), "repo")
	service := newValidInternalService(root)
	service.daemonRepo = internalDaemonRepo{state: &domain.DaemonState{Projects: []domain.MonitoredProject{{Path: filepath.Join(root, "other"), Enabled: true, PID: 1}}}}

	err := service.Watch(false, time.Second)
	if err != nil && strings.Contains(err.Error(), "cannot run watch: daemon is already monitoring this project") {
		t.Fatalf("expected unrelated monitored project to not block watch, got %v", err)
	}
}

func TestWatchReturnsErrorWhenDaemonCheckSkippedWithNilRepo(t *testing.T) {
	root := filepath.Join(string(filepath.Separator), "repo")
	service := newValidInternalService(root)
	service.daemonRepo = nil

	err := service.Watch(false, time.Second)
	_ = err
}

func TestIsMissingFileErrorReturnsTrueForNotExistError(t *testing.T) {
	err := os.ErrNotExist
	if !isMissingFileError(err) {
		t.Fatal("expected isMissingFileError to return true for os.ErrNotExist")
	}
}

func TestIsMissingFileErrorReturnsTrueForFileNotFoundMessage(t *testing.T) {
	err := errors.New("file not found")
	if !isMissingFileError(err) {
		t.Fatal("expected isMissingFileError to return true for 'file not found' message")
	}
}

func TestIsMissingFileErrorReturnsTrueForNoSuchFileMessage(t *testing.T) {
	err := errors.New("no such file or directory")
	if !isMissingFileError(err) {
		t.Fatal("expected isMissingFileError to return true for 'no such file or directory' message")
	}
}

func TestIsMissingFileErrorReturnsFalseForOtherErrors(t *testing.T) {
	err := errors.New("some other error")
	if isMissingFileError(err) {
		t.Fatal("expected isMissingFileError to return false for other errors")
	}
}

func TestInitCommandServiceInspectAndWriteInspectIndexErrorBranchesFromMovedFile(t *testing.T) {
	root := filepath.Join(string(filepath.Separator), "repo")
	service := newValidInternalService(root)
	tree := service.projectTree.(*internalProjectTree)

	tree.existsErr[indexFilePath(root)] = errors.New("exists failure")
	if err := service.Inspect("."); err == nil {
		t.Fatal("expected inspect exists error")
	}

	service.indexRepo = internalIndexRepo{loadErr: errors.New("load failure")}
	if err := service.writeInspectIndex(root); err == nil {
		t.Fatal("expected inspect load error")
	}

	invalid := domain.NewInvertedIndex()
	invalid.AverageDocLength = math.NaN()
	service.indexRepo = internalIndexRepo{index: invalid}
	if err := service.writeInspectIndex(root); err == nil {
		t.Fatal("expected JSON marshal error for invalid NaN payload")
	}
}
