package search

const (
	OutputText = "text"
	OutputJSON = "json"
)

const (
	// OperatorAND requires all query terms to be present in a document (default).
	OperatorAND = "AND"
	// OperatorOR requires at least one query term to be present in a document.
	OperatorOR = "OR"
)

// Options controls optional output behavior of the search command.
type Options struct {
	Format                 string
	Context                int
	PrettyJSON             bool
	Explain                bool
	AgentCompact           bool
	FilesOnly              bool
	CountOnly              bool
	Timing                 bool
	PathQuery              string
	PathQueries            []string
	ExtensionQuery         string
	ExtensionQueries       []string
	From                   int
	Size                   int
	Operator               string
	RelaxationEnabled      bool
	RelaxationMinExclusive int
	PopularityWeight       float64
}
