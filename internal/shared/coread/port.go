package coread

// MatrixRepository persists pairwise co-read counts across sessions.
// On every idx read, RecordCoRead is called with the file just accessed;
// the implementation increments the count for each pair within the session
// window. LoadCoReads returns accumulated counts so the related command can
// rank candidates by how often they are read in the same session as the target.
//
// Example: repo := NewCoReadMatrixRepository().
type MatrixRepository interface {
	// RecordCoRead updates co-occurrence counts between relPath and all files
	// accessed within the current session window (typically 30 min).
	// relPath must be relative to projectRoot, using forward slashes.
	RecordCoRead(projectRoot, relPath string) error

	// LoadCoReads returns co-occurrence counts for relPath.
	// Keys are relative paths (e.g. "internal/features/search/service.go").
	// Returns an empty map (not an error) when no data exists for relPath.
	LoadCoReads(projectRoot, relPath string) (map[string]uint32, error)
}
