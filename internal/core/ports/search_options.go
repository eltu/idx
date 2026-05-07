package ports

const (
	SearchOutputText = "text"
	SearchOutputJSON = "json"
)

const (
	// SearchOperatorAND requires all query terms to be present in a document (default).
	SearchOperatorAND = "AND"
	// SearchOperatorOR requires at least one query term to be present in a document.
	SearchOperatorOR = "OR"
)

// SearchOptions controls optional output behaviour of the search command.
type SearchOptions struct {
	Format                 string
	Context                int
	PrettyJSON             bool
	Explain                bool
	MatchesOnly            bool
	FilesOnly              bool
	PathQuery              string
	PathQueries            []string
	ExtensionQuery         string
	ExtensionQueries       []string
	From                   int
	Size                   int
	Operator               string
	RelaxationEnabled      bool
	RelaxationMinExclusive int
}
