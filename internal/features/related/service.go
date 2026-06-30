package related

import (
	"fmt"
	"path/filepath"

	"idx/internal/features/indexing"
	"idx/internal/shared/coread"
	sharedfs "idx/internal/shared/filesystem"
	"idx/internal/shared/gitutil"
	"idx/internal/shared/output"
)

// RelatedCommandService finds files related to a target file using git co-change
// history, persistent co-read affinity, and BM25 term co-occurrence.
type RelatedCommandService struct {
	projectTree sharedfs.ProjectTree
	indexRepo   indexing.IndexRepository
	coReadRepo  coread.MatrixRepository
	output      output.Writer
}

// RelatedDeps holds the required collaborators for RelatedCommandService.
type RelatedDeps struct {
	ProjectTree sharedfs.ProjectTree
	IndexRepo   indexing.IndexRepository
	CoReadRepo  coread.MatrixRepository
	Output      output.Writer
}

// NewRelatedCommandService creates the related command use case.
// Example: svc := NewRelatedCommandService(RelatedDeps{ProjectTree: tree, IndexRepo: repo, CoReadRepo: cr, Output: out}).
func NewRelatedCommandService(deps RelatedDeps) RelatedCommandService {
	return RelatedCommandService{
		projectTree: deps.ProjectTree,
		indexRepo:   deps.IndexRepo,
		coReadRepo:  deps.CoReadRepo,
		output:      deps.Output,
	}
}

// Run finds and writes files related to filePath.
// Example: err := svc.Run("internal/features/search/service.go", opts).
func (svc RelatedCommandService) Run(filePath string, opts Options) error {
	projectRoot, absPath, err := svc.resolveTarget(filePath)
	if err != nil {
		return err
	}

	dirs, err := indexing.IndexedDirectories(svc.projectTree, projectRoot)
	if err != nil {
		return fmt.Errorf("failed to list indexed directories: %w", err)
	}

	changedFiles, err := resolveChangedFiles(opts.Since, projectRoot)
	if err != nil {
		return err
	}

	candidates, err := svc.computeRelated(absPath, projectRoot, dirs)
	if err != nil {
		return err
	}

	limit := opts.Size
	if limit <= 0 {
		limit = defaultResultSize
	}

	results := buildResults(candidates, projectRoot, limit+opts.Skip)
	results = applyFilters(results, changedFiles, opts.Ext, projectRoot)
	results = applySkip(results, opts.Skip)
	return writeRelatedResults(results, opts, svc.output)
}

func resolveChangedFiles(since, projectRoot string) (map[string]bool, error) {
	if since == "" {
		return nil, nil
	}
	return gitutil.ChangedFilesSince(projectRoot, since)
}

func (svc RelatedCommandService) resolveTarget(filePath string) (string, string, error) {
	cwd, err := svc.projectTree.CurrentDir()
	if err != nil {
		return "", "", fmt.Errorf("failed to resolve current directory: %w", err)
	}

	absPath := filepath.Clean(filePath)
	if !filepath.IsAbs(absPath) {
		absPath = filepath.Clean(filepath.Join(cwd, filePath))
	}

	projectRoot, err := svc.projectTree.FindGitRoot(cwd)
	if err != nil {
		return "", "", fmt.Errorf("failed to find git root: %w", err)
	}

	return projectRoot, absPath, nil
}

// computeRelated runs term scoring, co-read matrix, and git co-change signals.
func (svc RelatedCommandService) computeRelated(
	targetPath, projectRoot string,
	dirs []string,
) (map[string]*candidateScore, error) {
	relPath, err := filepath.Rel(projectRoot, targetPath)
	if err != nil {
		return nil, fmt.Errorf("failed to compute relative path for %q: %w", targetPath, err)
	}
	relPath = filepath.ToSlash(relPath)

	targetTerms, err := svc.collectTargetTerms(targetPath, dirs)
	if err != nil {
		return nil, err
	}

	candidates, err := svc.scoreAllCandidates(targetPath, dirs, targetTerms)
	if err != nil {
		return nil, err
	}

	if err := applyCoReadMatrix(candidates, svc.coReadRepo, targetPath, projectRoot, relPath); err != nil {
		return nil, err
	}
	applyGitCoChange(candidates, projectRoot, relPath)

	return candidates, nil
}
