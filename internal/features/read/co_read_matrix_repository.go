package read

import (
	"encoding/gob"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	coReadMatrixFileName = "co_read_matrix.idx"
	coReadMatrixCacheTTL = 5 * time.Minute
	coReadSessionWindow  = 30 * time.Minute
)

// CoReadMatrixRepository persists pairwise co-read counts to .idx/co_read_matrix.idx.
// Files read within the session window are considered co-read; their pair counts
// accumulate across sessions and survive process restarts.
//
// Example: repo := NewCoReadMatrixRepository().
type CoReadMatrixRepository struct {
	mu sync.Mutex

	// sessionReads tracks files accessed during the current session window.
	sessionReads map[string]time.Time

	// matrix is the in-memory working copy for writes (write path).
	matrix    coReadMatrix
	matrixExp time.Time

	// readCache is used independently by LoadCoReads (read path).
	readCache    coReadMatrix
	readCacheExp time.Time
}

type coReadMatrix = map[string]map[string]uint32

// NewCoReadMatrixRepository creates a co-read matrix repository backed by the local filesystem.
// Example: repo := NewCoReadMatrixRepository().
func NewCoReadMatrixRepository() *CoReadMatrixRepository {
	return &CoReadMatrixRepository{
		sessionReads: make(map[string]time.Time),
	}
}

// RecordCoRead updates co-occurrence counts between relPath and all files in the
// current session window, then persists the matrix to disk.
func (r *CoReadMatrixRepository) RecordCoRead(projectRoot, relPath string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()

	if r.matrix == nil || now.After(r.matrixExp) {
		m, err := loadCoReadMatrix(coReadMatrixPath(projectRoot))
		if err != nil {
			return err
		}
		r.matrix = m
		r.matrixExp = now.Add(coReadMatrixCacheTTL)
	}

	r.purgeSession(now)
	r.updateCounts(relPath)
	r.sessionReads[relPath] = now

	return saveCoReadMatrix(coReadMatrixPath(projectRoot), r.matrix)
}

// purgeSession removes session entries older than coReadSessionWindow.
func (r *CoReadMatrixRepository) purgeSession(now time.Time) {
	cutoff := now.Add(-coReadSessionWindow)
	for path, t := range r.sessionReads {
		if t.Before(cutoff) {
			delete(r.sessionReads, path)
		}
	}
}

// updateCounts increments co-occurrence counts between relPath and each session entry.
func (r *CoReadMatrixRepository) updateCounts(relPath string) {
	for sessionPath := range r.sessionReads {
		if sessionPath == relPath {
			continue
		}
		if r.matrix[relPath] == nil {
			r.matrix[relPath] = make(map[string]uint32)
		}
		if r.matrix[sessionPath] == nil {
			r.matrix[sessionPath] = make(map[string]uint32)
		}
		r.matrix[relPath][sessionPath]++
		r.matrix[sessionPath][relPath]++
	}
}

// LoadCoReads returns co-occurrence counts for relPath from the cached matrix.
// Returns an empty map (not an error) when no data exists for relPath.
func (r *CoReadMatrixRepository) LoadCoReads(projectRoot, relPath string) (map[string]uint32, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	if r.readCache == nil || now.After(r.readCacheExp) {
		m, err := loadCoReadMatrix(coReadMatrixPath(projectRoot))
		if err != nil {
			return nil, err
		}
		r.readCache = m
		r.readCacheExp = now.Add(coReadMatrixCacheTTL)
	}

	counts := r.readCache[relPath]
	if len(counts) == 0 {
		return map[string]uint32{}, nil
	}
	// Clone to prevent callers from mutating the cache.
	out := make(map[string]uint32, len(counts))
	maps.Copy(out, counts)
	return out, nil
}

func coReadMatrixPath(projectRoot string) string {
	return filepath.Join(projectRoot, ".idx", coReadMatrixFileName)
}

func loadCoReadMatrix(path string) (coReadMatrix, error) {
	f, err := os.Open(path) //nolint:gosec
	if os.IsNotExist(err) {
		return make(coReadMatrix), nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to open co-read matrix %q: %w", path, err)
	}
	defer func() { _ = f.Close() }()

	var m coReadMatrix
	if err := gob.NewDecoder(f).Decode(&m); err != nil {
		// Treat corrupt file as empty rather than surfacing a hard error.
		return make(coReadMatrix), nil
	}
	return m, nil
}

func saveCoReadMatrix(path string, m coReadMatrix) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return fmt.Errorf("failed to create .idx directory %q: %w", filepath.Dir(path), err)
	}

	tmp := path + ".tmp"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600) //nolint:gosec
	if err != nil {
		return fmt.Errorf("failed to create co-read matrix temp file %q: %w", tmp, err)
	}
	if encErr := gob.NewEncoder(f).Encode(m); encErr != nil {
		_ = f.Close()
		return fmt.Errorf("failed to encode co-read matrix: %w", encErr)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("failed to close co-read matrix temp file: %w", err)
	}
	return os.Rename(tmp, path)
}
