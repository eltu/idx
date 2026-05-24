package filesystem

// DirectoryEntry is metadata for a single file-system entry.
type DirectoryEntry struct {
	Name            string
	Path            string
	IsDir           bool
	IsSymlink       bool
	Size            int64
	ModTimeUnixNano int64
}

// ProjectTree is the file-system navigation port.
type ProjectTree interface {
	CurrentDir() (string, error)
	FindGitRoot(startDir string) (string, error)
	ReadDir(path string) ([]DirectoryEntry, error)
	Exists(path string) (bool, error)
	RemoveAll(path string) error
	WriteFile(path string, content []byte) error
}

// FileReader reads entire file content as a string.
type FileReader interface {
	ReadFile(path string) (string, error)
}

// IgnoreMatcher reports whether a path should be excluded from indexing.
type IgnoreMatcher interface {
	Matches(path string) (bool, error)
}

// IgnoreMatcherBuilder builds an IgnoreMatcher for a given project root.
type IgnoreMatcherBuilder interface {
	New(projectRoot string) (IgnoreMatcher, error)
}
