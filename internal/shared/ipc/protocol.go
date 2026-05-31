package ipc

// JSON-RPC method names for the idx IPC server.
const (
	MethodSearch  = "idx.search"
	MethodInit    = "idx.init"
	MethodSync    = "idx.sync"
	MethodStatus  = "idx.status"
	MethodRead    = "idx.read"
	MethodInspect = "idx.inspect"
	MethodDestroy = "idx.destroy"
)

// SearchRequest carries all search parameters over RPC.
type SearchRequest struct {
	Query             string   `json:"query"`
	Size              int      `json:"size,omitempty"`
	Operator          string   `json:"operator,omitempty"`
	Format            string   `json:"format,omitempty"`
	Context           int      `json:"context,omitempty"`
	ExtensionQueries  []string `json:"ext,omitempty"`
	PathQueries       []string `json:"path,omitempty"`
	PopularityWeight  float64  `json:"popularity_weight,omitempty"`
	FilesOnly         bool     `json:"files_only,omitempty"`
	AgentCompact      bool     `json:"agent_compact,omitempty"`
	Explain           bool     `json:"explain,omitempty"`
	From              int      `json:"from,omitempty"`
	RelaxationEnabled bool     `json:"relaxation_enabled,omitempty"`
	RelaxationMin     int      `json:"relaxation_min,omitempty"`
}

// SearchResponse carries structured search results.
type SearchResponse struct {
	Count   int            `json:"count"`
	Results []SearchResult `json:"results"`
}

// SearchResult describes a single matched file.
type SearchResult struct {
	File    string        `json:"file"`
	Name    string        `json:"name"`
	Path    string        `json:"path"`
	Score   *float64      `json:"score,omitempty"`
	Stale   bool          `json:"stale,omitempty"`
	Matches []MatchedLine `json:"matches"`
}

// MatchedLine is one line returned with context or match highlight.
type MatchedLine struct {
	Line    int    `json:"line"`
	Content string `json:"content"`
	Match   bool   `json:"match"`
}

// ReadRequest specifies a file and optional line range.
type ReadRequest struct {
	FilePath string `json:"file_path"`
	FromLine int    `json:"from_line,omitempty"`
	ToLine   int    `json:"to_line,omitempty"`
}

// ReadResponse carries the requested file lines.
type ReadResponse struct {
	Lines []string `json:"lines"`
}

// CommandResponse is used by init, sync, status, and destroy — commands whose output
// is human-readable text produced by the service layer.
type CommandResponse struct {
	Success bool   `json:"success"`
	Output  string `json:"output"`
}

// InspectRequest specifies an optional directory path for idx.inspect.
// An empty IndexPath causes the server to merge all indexed project directories.
type InspectRequest struct {
	IndexPath string `json:"index_path,omitempty"`
}
