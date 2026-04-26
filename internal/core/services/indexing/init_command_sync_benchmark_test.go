package indexing_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"idx/internal/adapters/repository"
	"idx/internal/core/domain"
	"idx/internal/core/ports"
	"idx/internal/core/services/indexing"
)

type benchmarkProjectTree struct {
	root          string
	stripMetadata bool
}

func (tree benchmarkProjectTree) CurrentDir() (string, error) {
	return tree.root, nil
}

func (tree benchmarkProjectTree) FindGitRoot(startDir string) (string, error) {
	return tree.root, nil
}

func (tree benchmarkProjectTree) ReadDir(path string) ([]domain.DirectoryEntry, error) {
	directoryEntries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}

	entries := make([]domain.DirectoryEntry, 0, len(directoryEntries))
	for _, entry := range directoryEntries {
		info, err := entry.Info()
		if err != nil {
			return nil, err
		}

		size := info.Size()
		modTime := info.ModTime().UnixNano()
		if tree.stripMetadata {
			size = 0
			modTime = 0
		}

		entries = append(entries, domain.DirectoryEntry{
			Name:            entry.Name(),
			Path:            filepath.Join(path, entry.Name()),
			IsDir:           entry.IsDir(),
			Size:            size,
			ModTimeUnixNano: modTime,
		})
	}

	return entries, nil
}

func (tree benchmarkProjectTree) Exists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}

	return false, err
}

func (tree benchmarkProjectTree) RemoveAll(path string) error {
	return os.RemoveAll(path)
}

func (tree benchmarkProjectTree) WriteFile(path string, content []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0750); err != nil {
		return err
	}

	return os.WriteFile(path, content, 0600)
}

type benchmarkCountingFileReader struct {
	base      ports.FileReader
	readCount int64
}

func (reader *benchmarkCountingFileReader) ReadFile(path string) (string, error) {
	reader.readCount++
	return reader.base.ReadFile(path)
}

func (reader *benchmarkCountingFileReader) Reset() {
	reader.readCount = 0
}

func (reader *benchmarkCountingFileReader) Count() int64 {
	return reader.readCount
}

func BenchmarkSyncMetadataIncremental(b *testing.B) {
	b.Run("baseline_full_rehash", func(b *testing.B) {
		benchmarkSyncScenario(b, true)
	})

	b.Run("optimized_metadata_reuse", func(b *testing.B) {
		benchmarkSyncScenario(b, false)
	})
}

func benchmarkSyncScenario(b *testing.B, stripMetadata bool) {
	rootDir := b.TempDir()
	if err := createBenchmarkCorpus(rootDir, 400, 1024); err != nil {
		b.Fatalf("failed to create benchmark corpus: %v", err)
	}

	projectTree := benchmarkProjectTree{root: rootDir, stripMetadata: stripMetadata}
	matcherFactory := fakeIgnoreMatcherFactory{ignoredPaths: map[string]bool{}}
	fileReader := &benchmarkCountingFileReader{base: repository.NewOSFileReader()}
	indexer := indexing.NewBM25IndexService()
	indexRepo := repository.NewBinaryIndexRepository(projectTree)
	checksumRepo := repository.NewDirectoryChecksumRepository()
	output := &capturingTextOutput{}
	service := indexing.NewInitCommandService(projectTree, matcherFactory, output, fileReader, indexer, indexRepo, checksumRepo)

	if err := service.Run(); err != nil {
		b.Fatalf("failed to initialize index before benchmark: %v", err)
	}

	fileReader.Reset()
	var totalReads int64
	b.ResetTimer()
	for run := 0; run < b.N; run++ {
		fileReader.Reset()
		if err := service.Sync(); err != nil {
			b.Fatalf("sync failed: %v", err)
		}
		totalReads += fileReader.Count()
	}
	b.StopTimer()

	b.ReportMetric(float64(totalReads)/float64(b.N), "file_reads/op")
}

func createBenchmarkCorpus(rootDir string, files int, bytesPerFile int) error {
	if err := os.MkdirAll(filepath.Join(rootDir, ".git"), 0750); err != nil {
		return err
	}

	content := make([]byte, bytesPerFile)
	for i := range content {
		content[i] = 'a' + byte(i%26)
	}

	for index := 0; index < files; index++ {
		name := fmt.Sprintf("file-%04d.txt", index)
		path := filepath.Join(rootDir, name)
		if err := os.WriteFile(path, content, 0600); err != nil {
			return err
		}
	}

	return nil
}
