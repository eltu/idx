package services_test

import (
	"errors"
	"path/filepath"
	"testing"

	"idx/internal/core/domain"
	"idx/internal/core/services"
)

type fakeSearchIndexRepository struct {
	index   *domain.InvertedIndex
	loadErr error
	loaded  []string
}

func (repo *fakeSearchIndexRepository) LoadIndex(directoryPath string) (*domain.InvertedIndex, error) {
	repo.loaded = append(repo.loaded, directoryPath)
	if repo.loadErr != nil {
		return nil, repo.loadErr
	}

	return repo.index, nil
}

func TestSearchCommandServiceRunRanksResultsByBM25Score(t *testing.T) {
	rootDir := filepath.Join(string(filepath.Separator), "repo")
	tree := newFakeProjectTree(rootDir, rootDir)
	output := &capturingTextOutput{}
	repo := &fakeSearchIndexRepository{index: searchableIndexWithPartialMatch()}
	service := services.NewSearchCommandService(tree, output, repo)

	err := service.Run("go search")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(repo.loaded) != 1 || repo.loaded[0] != rootDir {
		t.Fatalf("expected load for %q, got %v", rootDir, repo.loaded)
	}

	if len(output.lines) != 2 {
		t.Fatalf("expected 2 output lines, got %d", len(output.lines))
	}

	if output.lines[0] != "./guide.md (score: 0.8996)" {
		t.Fatalf("expected best result first, got %q", output.lines[0])
	}

	if output.lines[1] != "./readme.md (score: 0.5921)" {
		t.Fatalf("expected second result, got %q", output.lines[1])
	}
}

func TestSearchCommandServiceRunRequiresAllTermsInDocument(t *testing.T) {
	rootDir := filepath.Join(string(filepath.Separator), "repo")
	tree := newFakeProjectTree(rootDir, rootDir)
	output := &capturingTextOutput{}
	repo := &fakeSearchIndexRepository{index: searchableIndexWithPartialMatch()}
	service := services.NewSearchCommandService(tree, output, repo)

	err := service.Run("module idx")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(output.lines) != 1 {
		t.Fatalf("expected 1 output line, got %d", len(output.lines))
	}

	if output.lines[0] != "./go.mod (score: 1.9906)" {
		t.Fatalf("expected only full match result, got %q", output.lines[0])
	}
}

func TestSearchCommandServiceRunWritesNoResultsMessage(t *testing.T) {
	rootDir := filepath.Join(string(filepath.Separator), "repo")
	tree := newFakeProjectTree(rootDir, rootDir)
	output := &capturingTextOutput{}
	repo := &fakeSearchIndexRepository{index: searchableIndex()}
	service := services.NewSearchCommandService(tree, output, repo)

	err := service.Run("python")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(output.lines) != 1 {
		t.Fatalf("expected 1 output line, got %d", len(output.lines))
	}

	if output.lines[0] != "Nenhum resultado encontrado." {
		t.Fatalf("unexpected output message %q", output.lines[0])
	}
}

func TestSearchCommandServiceRunReturnsLoadError(t *testing.T) {
	rootDir := filepath.Join(string(filepath.Separator), "repo")
	tree := newFakeProjectTree(rootDir, rootDir)
	repo := &fakeSearchIndexRepository{loadErr: errors.New("boom")}
	service := services.NewSearchCommandService(tree, &capturingTextOutput{}, repo)

	err := service.Run("go")
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
}

func TestSearchCommandServiceRunBoostsDocumentsWithNearbyTerms(t *testing.T) {
	rootDir := filepath.Join(string(filepath.Separator), "repo")
	tree := newFakeProjectTree(rootDir, rootDir)
	output := &capturingTextOutput{}
	repo := &fakeSearchIndexRepository{index: searchableIndexWithProximity()}
	service := services.NewSearchCommandService(tree, output, repo)

	err := service.Run("module idx")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(output.lines) != 2 {
		t.Fatalf("expected 2 output lines, got %d", len(output.lines))
	}

	if output.lines[0] != "./near.txt (score: 3.5000)" {
		t.Fatalf("expected nearby terms first, got %q", output.lines[0])
	}

	if output.lines[1] != "./far.txt (score: 2.0300)" {
		t.Fatalf("expected far terms second, got %q", output.lines[1])
	}
}

func TestSearchCommandServiceRunWritesPathsRelativeToProjectRoot(t *testing.T) {
	rootDir := filepath.Join(string(filepath.Separator), "repo")
	childDir := filepath.Join(rootDir, "internal", "core")
	tree := newFakeProjectTree(childDir, rootDir)
	output := &capturingTextOutput{}
	repo := &fakeSearchIndexRepository{index: searchableIndexForRelativePath()}
	service := services.NewSearchCommandService(tree, output, repo)

	err := service.Run("module idx")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(output.lines) != 1 {
		t.Fatalf("expected 1 output line, got %d", len(output.lines))
	}

	if output.lines[0] != "internal/core/go.mod (score: 3.5000)" {
		t.Fatalf("expected project-relative path output, got %q", output.lines[0])
	}
}

func searchableIndex() *domain.InvertedIndex {
	index := domain.NewInvertedIndex()
	index.Documents["guide.md"] = &domain.DocStats{Length: 3}
	index.Documents["readme.md"] = &domain.DocStats{Length: 7}
	index.DocumentCount = len(index.Documents)
	index.AverageDocLength = 5
	index.Terms["go"] = &domain.TermStats{
		IDF: 0.4,
		Docs: map[string]*domain.DocTermStats{
			"guide.md":  {TF: 2},
			"readme.md": {TF: 1},
		},
	}
	index.Terms["search"] = &domain.TermStats{
		IDF: 0.2,
		Docs: map[string]*domain.DocTermStats{
			"guide.md":  {TF: 1},
			"readme.md": {TF: 2},
		},
	}

	return index
}

func searchableIndexWithPartialMatch() *domain.InvertedIndex {
	index := searchableIndex()
	index.Documents["go.mod"] = &domain.DocStats{Length: 4}
	index.Documents["AGENTS.md"] = &domain.DocStats{Length: 6}
	index.Documents[".gitignore"] = &domain.DocStats{Length: 5}
	index.DocumentCount = len(index.Documents)
	index.AverageDocLength = 5
	index.Terms["module"] = &domain.TermStats{
		IDF: 0.7,
		Docs: map[string]*domain.DocTermStats{
			"go.mod":     {TF: 1},
			".gitignore": {TF: 1},
		},
	}
	index.Terms["idx"] = &domain.TermStats{
		IDF: 0.8,
		Docs: map[string]*domain.DocTermStats{
			"go.mod":    {TF: 2},
			"AGENTS.md": {TF: 1},
		},
	}

	return index
}

func searchableIndexWithProximity() *domain.InvertedIndex {
	index := domain.NewInvertedIndex()
	index.Documents["near.txt"] = &domain.DocStats{Length: 5}
	index.Documents["far.txt"] = &domain.DocStats{Length: 5}
	index.DocumentCount = len(index.Documents)
	index.AverageDocLength = 5
	index.Terms["module"] = &domain.TermStats{
		IDF: 1.0,
		Docs: map[string]*domain.DocTermStats{
			"near.txt": {TF: 1, Positions: []int{1}},
			"far.txt":  {TF: 1, Positions: []int{1}},
		},
	}
	index.Terms["idx"] = &domain.TermStats{
		IDF: 1.0,
		Docs: map[string]*domain.DocTermStats{
			"near.txt": {TF: 1, Positions: []int{2}},
			"far.txt":  {TF: 1, Positions: []int{100}},
		},
	}

	return index
}

func searchableIndexForRelativePath() *domain.InvertedIndex {
	index := domain.NewInvertedIndex()
	index.Documents["go.mod"] = &domain.DocStats{Length: 5}
	index.DocumentCount = len(index.Documents)
	index.AverageDocLength = 5
	index.Terms["module"] = &domain.TermStats{
		IDF: 1.0,
		Docs: map[string]*domain.DocTermStats{
			"go.mod": {TF: 1, Positions: []int{1}},
		},
	}
	index.Terms["idx"] = &domain.TermStats{
		IDF: 1.0,
		Docs: map[string]*domain.DocTermStats{
			"go.mod": {TF: 1, Positions: []int{2}},
		},
	}

	return index
}
