package domain

type DirectoryEntry struct {
	Name            string
	Path            string
	IsDir           bool
	IsSymlink       bool
	Size            int64
	ModTimeUnixNano int64
}
