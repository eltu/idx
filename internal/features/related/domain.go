package related

const (
	OutputText = "text"
	OutputJSON = "json"

	ReasonCoRead      = "co-read"
	ReasonTermOverlap = "term-overlap"
	ReasonBoth        = "both"

	defaultResultSize = 10
	coReadWeight      = 0.7
	termOverlapWeight = 0.3
	coReadWindowHours = 2.0
	bm25K1            = 1.5
	bm25B             = 0.75
)

// Options controls the related command output.
type Options struct {
	Format string
	Size   int
}

// Result is a single related file with its relevance score and signal origin.
type Result struct {
	Path   string  `json:"path"`
	Score  float64 `json:"score"`
	Reason string  `json:"reason"` // "co-read", "term-overlap", or "both"
}
