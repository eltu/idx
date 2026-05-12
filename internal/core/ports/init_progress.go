package ports

import "context"

// InitProgress reports directory indexing progress during idx init.
// Example: progress.StartCounting(); progress.SetTotal(15); progress.IncrementDir("/path/to/dir"); progress.Finish()
type InitProgress interface {
	StartCounting()
	SetTotal(total int)
	IncrementDir(dirPath string)
	Finish()
	Context() context.Context
}
