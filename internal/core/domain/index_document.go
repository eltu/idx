package domain

// IndexDocument describes a file to be indexed.
type IndexDocument struct {
	Name    string
	Path    string
	Content string
}
