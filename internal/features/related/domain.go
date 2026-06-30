package related

const (
	OutputText = "text"
	OutputJSON = "json"

	ReasonGit         = "git"
	ReasonCoRead      = "co-read"
	ReasonTermOverlap = "term-overlap"
	ReasonBoth        = "both"

	defaultResultSize = 10

	// Weights for the three ranking signals (must sum to 1.0).
	gitCoChangeWeight  = 0.5
	coReadMatrixWeight = 0.3
	termOverlapWeight  = 0.2

	bm25K1 = 1.5
	bm25B  = 0.75
)

// Options controls the related command output.
type Options struct {
	Format  string
	Size    int
	Skip    int
	Since   string
	Ext     []string
	Compact bool
}

// Result is a single related file with its relevance score and signal origin.
type Result struct {
	Path   string  `json:"path"`
	Score  float64 `json:"score"`
	Reason string  `json:"reason"` // "git", "co-read", "term-overlap", or "both"
}
