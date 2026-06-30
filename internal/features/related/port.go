package related

// RelatedRunner returns files most likely relevant to the given file.
// Example: err := runner.Run("internal/features/search/service.go", opts).
type RelatedRunner interface {
	Run(filePath string, opts Options) error
}
