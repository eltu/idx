package readlog

import "time"

// LogEntry holds the access history for a single file.
// Path is relative to the project root (e.g. "internal/features/search/service.go").
type LogEntry struct {
	Path       string
	ReadCount  int
	LastReadAt time.Time
}

// LogRepository records per-file access history for the read command.
// Entries are consolidated (one per path) and pruned after 30 days of inactivity.
// The log is used as a boost signal by the search ranking pipeline.
type LogRepository interface {
	// RecordRead upserts an access entry for relativePath under projectRoot.
	// On each call the timestamp is refreshed and the read count is incremented.
	RecordRead(projectRoot, relativePath string) error

	// LoadAll returns all current read log entries for projectRoot.
	// Returns an empty slice (not an error) when the log file does not exist yet.
	LoadAll(projectRoot string) ([]LogEntry, error)
}
