package search_test

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"idx/internal/adapters/handlers/cli"
	"idx/internal/adapters/repository"
	"idx/internal/core/domain"
	"idx/internal/core/services/indexing"
	search "idx/internal/core/services/search"
)

type benchmarkCorpusSpec struct {
	name        string
	directories int
	filesPerDir int
}

type benchmarkProjectTree struct {
	currentDir string
	rootDir    string
}

func BenchmarkSearchVsGrep(b *testing.B) {
	repoRoot := benchmarkRepoRoot(b)
	binaryPath := buildBenchmarkBinary(b, repoRoot)
	specs := []benchmarkCorpusSpec{
		{name: "files-500", directories: 20, filesPerDir: 25},
		{name: "files-2000", directories: 40, filesPerDir: 50},
	}

	for _, spec := range specs {
		repositoryPath := createBenchmarkRepository(b, spec)
		service := buildBenchmarkSearchService(b, repositoryPath)

		b.Run(spec.name+"/service", func(b *testing.B) {
			b.ReportAllocs()
			for iteration := 0; iteration < b.N; iteration++ {
				if err := service.Run("needle"); err != nil {
					b.Fatalf("expected search to succeed, got %v", err)
				}
			}
		})

		b.Run(spec.name+"/cli", func(b *testing.B) {
			for iteration := 0; iteration < b.N; iteration++ {
				command := exec.CommandContext(context.Background(), binaryPath, "search", "needle") //nolint:gosec
				command.Dir = repositoryPath
				command.Stdout = io.Discard
				command.Stderr = io.Discard
				if err := command.Run(); err != nil {
					b.Fatalf("expected CLI search to succeed, got %v", err)
				}
			}
		})

		b.Run(spec.name+"/grep", func(b *testing.B) {
			for iteration := 0; iteration < b.N; iteration++ {
				command := exec.CommandContext(context.Background(), "grep", "-Rnw", "--exclude-dir=.git", "--exclude-dir=.idx", "needle", ".")
				command.Dir = repositoryPath
				command.Stdout = io.Discard
				command.Stderr = io.Discard
				if err := command.Run(); err != nil {
					b.Fatalf("expected grep to succeed, got %v", err)
				}
			}
		})
	}
}

func benchmarkRepoRoot(b testing.TB) string {
	b.Helper()
	_, filePath, _, ok := runtime.Caller(0)
	if !ok {
		b.Fatal("expected caller information for benchmark file")
	}

	return filepath.Clean(filepath.Join(filepath.Dir(filePath), "..", "..", ".."))
}

func buildBenchmarkBinary(b testing.TB, repoRoot string) string {
	b.Helper()
	binaryPath := filepath.Join(b.TempDir(), "idx-bench")
	command := exec.CommandContext(context.Background(), "go", "build", "-o", binaryPath, "./cmd/idx") //nolint:gosec
	command.Dir = repoRoot
	command.Stdout = io.Discard
	command.Stderr = io.Discard
	if err := command.Run(); err != nil {
		b.Fatalf("expected benchmark binary build to succeed, got %v", err)
	}

	return binaryPath
}

func createBenchmarkRepository(b testing.TB, spec benchmarkCorpusSpec) string {
	b.Helper()
	repositoryPath := filepath.Join(b.TempDir(), spec.name)
	if err := os.MkdirAll(repositoryPath, 0750); err != nil {
		b.Fatalf("expected temp repository creation to succeed, got %v", err)
	}

	gitInit := exec.CommandContext(context.Background(), "git", "init", "-q") //nolint:gosec
	gitInit.Dir = repositoryPath
	gitInit.Stdout = io.Discard
	gitInit.Stderr = io.Discard
	if err := gitInit.Run(); err != nil {
		b.Fatalf("expected git init to succeed, got %v", err)
	}

	for directory := 1; directory <= spec.directories; directory++ {
		directoryPath := filepath.Join(repositoryPath, fmt.Sprintf("dir%d", directory))
		if err := os.MkdirAll(directoryPath, 0750); err != nil {
			b.Fatalf("expected corpus directory creation to succeed, got %v", err)
		}

		for file := 1; file <= spec.filesPerDir; file++ {
			content := benchmarkFileContent(spec.filesPerDir, directory, file)
			filePath := filepath.Join(directoryPath, fmt.Sprintf("file%d.txt", file))
			if err := os.WriteFile(filePath, []byte(content), 0600); err != nil {
				b.Fatalf("expected corpus file write to succeed, got %v", err)
			}
		}
	}

	return repositoryPath
}

func benchmarkFileContent(filesPerDir int, directory int, file int) string {
	lines := []string{
		"alpha beta gamma delta epsilon",
		"common tokens for benchmark corpus",
		fmt.Sprintf("directory %d file %d repeated content words words words", directory, file),
	}

	if ((directory*filesPerDir)+file)%17 == 0 {
		lines = append(lines, "needle token appears here with tree and search terms")
	}

	return strings.Join(lines, "\n") + "\n"
}

func buildBenchmarkSearchService(b testing.TB, repositoryPath string) search.SearchCommandService {
	b.Helper()
	projectTree := benchmarkProjectTree{currentDir: repositoryPath, rootDir: repositoryPath}
	output := cli.NewLineWriter(io.Discard)
	matcherFactory := repository.NewGitIgnoreMatcherFactory()
	fileReader := repository.NewOSFileReader()
	indexer := indexing.NewBM25IndexService()
	indexRepo := repository.NewBinaryIndexRepository()
	checksumRepo := repository.NewDirectoryChecksumRepository()
	initService := indexing.NewInitCommandService(projectTree, matcherFactory, output, fileReader, indexer, indexRepo, checksumRepo, nil)
	if err := initService.Run(); err != nil {
		b.Fatalf("expected benchmark indexing to succeed, got %v", err)
	}

	return search.NewSearchCommandService(projectTree, output, fileReader, indexRepo)
}

func (tree benchmarkProjectTree) CurrentDir() (string, error) {
	return tree.currentDir, nil
}

func (tree benchmarkProjectTree) FindGitRoot(startDir string) (string, error) {
	if strings.HasPrefix(startDir, tree.rootDir) {
		return tree.rootDir, nil
	}

	return "", fmt.Errorf("benchmark project tree expected path under %q, got %q", tree.rootDir, startDir)
}

func (tree benchmarkProjectTree) ReadDir(path string) ([]domain.DirectoryEntry, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}

	result := make([]domain.DirectoryEntry, 0, len(entries))
	for _, entry := range entries {
		result = append(result, domain.DirectoryEntry{
			Name:  entry.Name(),
			Path:  filepath.Join(path, entry.Name()),
			IsDir: entry.IsDir(),
		})
	}

	return result, nil
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
	return os.WriteFile(path, content, 0600)
}
