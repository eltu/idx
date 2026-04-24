package ports

const (
	SearchOutputText = "text"
	SearchOutputJSON = "json"
)

// SearchOptions controls optional output behaviour of the search command.
type SearchOptions struct {
	Format      string
	Context     int
	PrettyJSON  bool
	MatchesOnly bool
	FilesOnly   bool
	PathQuery   string
	PathQueries []string
	Limit       int
}
