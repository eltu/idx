package indexing

import (
	"context"
	"time"
)

// Indexer builds an InvertedIndex from a list of documents.
type Indexer interface {
	BuildIndex(documents []IndexDocument) (*InvertedIndex, error)
}

// IndexRepository defines saving and loading index to storage.
type IndexRepository interface {
	SaveIndex(directoryPath string, index *InvertedIndex) error
	LoadIndex(directoryPath string) (*InvertedIndex, error)
}

// FileChecksumState holds per-file checksum metadata.
type FileChecksumState struct {
	Checksum        string
	Size            int64
	ModTimeUnixNano int64
}

// DirectoryChecksumSnapshot holds metadata-aware checksum data for a directory.
type DirectoryChecksumSnapshot struct {
	Files map[string]FileChecksumState
}

// DirectoryChecksumRepository defines checksum metadata storage per directory.
type DirectoryChecksumRepository interface {
	Load(directoryPath string) (map[string]string, bool, error)
	Save(directoryPath string, checksums map[string]string) error
}

// DirectoryChecksumSnapshotRepository provides metadata-aware checksum snapshots.
// Implementations can use this to avoid rehashing unchanged files during sync.
type DirectoryChecksumSnapshotRepository interface {
	LoadSnapshot(directoryPath string) (DirectoryChecksumSnapshot, bool, error)
	SaveSnapshot(directoryPath string, snapshot DirectoryChecksumSnapshot) error
}

// PathRunner defines the contract for initializing a project.
type PathRunner interface {
	// RunFromPath runs the index initialization from a specific directory.
	RunFromPath(projectPath string) error
}

// Progress reports directory indexing progress during idx init.
// Example: progress.StartCounting(); progress.SetTotal(15); progress.IncrementDir("/path/to/dir"); progress.Finish().
type Progress interface {
	StartCounting()
	SetTotal(total int)
	IncrementDir(dirPath string)
	Finish()
	Context() context.Context
}

// InspectUIRunner renders the inspect user interface for a loaded index.
// Example: err := runner.Run(index).
type InspectUIRunner interface {
	Run(index *InvertedIndex) error
}

// ProjectMonitorChecker reports whether a project is already being watched by the daemon.
// Implemented by features/daemon; declared here to break the import cycle
// (daemon imports indexing.PathRunner; indexing uses this to guard manual watch).
type ProjectMonitorChecker interface {
	IsProjectMonitored(projectRoot string) (bool, error)
	// ProjectStatus returns daemon status details for the status panel.
	// Returns nil without error when the project is not monitored.
	ProjectStatus(projectRoot string) (*DaemonProjectStatus, error)
}

// DaemonProjectStatus holds the daemon status for a single project (used by the status panel).
type DaemonProjectStatus struct {
	Enabled   bool
	PID       int
	StartedAt time.Time
}
