package ports

// ReadLogRepository records per-file access history for the read command.
// Entries are consolidated (one per path) and pruned after 30 days of inactivity.
// The log is used as a boost signal by the search ranking pipeline.
type ReadLogRepository interface {
	// RecordRead upserts an access entry for relativePath under projectRoot.
	// On each call the timestamp is refreshed and the read count is incremented.
	RecordRead(projectRoot, relativePath string) error
}
