package domain

type DirectoryEntry struct {
	Name            string
	Path            string
	IsDir           bool
	Size            int64
	ModTimeUnixNano int64
}
