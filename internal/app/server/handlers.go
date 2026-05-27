package server

import (
	"context"
	"encoding/json"

	featindexing "idx/internal/features/indexing"
	featread "idx/internal/features/read"
	featsearch "idx/internal/features/search"
	sharedfs "idx/internal/shared/filesystem"
	idxipc "idx/internal/shared/ipc"
)

func (s *indexServer) handleSearch(_ context.Context, params json.RawMessage) (any, error) {
	var req idxipc.SearchRequest
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, err
	}

	capture := &captureWriter{}
	svc := featsearch.NewSearchCommandService(s.deps.ProjectTree, capture, s.deps.FileReader, s.deps.IndexRepo).
		WithTuning(s.deps.SearchTuning).
		WithReadLog(s.deps.ReadLogRepo)

	opts := searchOptionsFromRequest(req)
	if err := svc.RunWithOptions(req.Query, opts); err != nil {
		return nil, err
	}

	return parseSearchJSON(capture.firstLine())
}

func (s *indexServer) handleRead(_ context.Context, params json.RawMessage) (any, error) {
	var req idxipc.ReadRequest
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, err
	}

	capture := &captureWriter{}
	streamer := featread.NewOSFileStreamer()
	svc := featread.NewReadCommandService(s.deps.ProjectTree, streamer, capture)
	if err := svc.RunWithOptions(req.FilePath, req.FromLine, req.ToLine); err != nil {
		return idxipc.ReadResponse{Lines: []string{}}, nil
	}

	return idxipc.ReadResponse{Lines: capture.lines}, nil
}

func (s *indexServer) handleInit(_ context.Context, _ json.RawMessage) (any, error) {
	capture := &captureWriter{}
	svc := featindexing.NewInitCommandService(initDepsWithCapture(s.deps, capture))
	err := svc.Run()
	return idxipc.CommandResponse{Success: err == nil, Output: capture.joined()}, nil
}

func (s *indexServer) handleSync(_ context.Context, _ json.RawMessage) (any, error) {
	capture := &captureWriter{}
	svc := featindexing.NewInitCommandService(initDepsWithCapture(s.deps, capture))
	err := svc.Sync()
	return idxipc.CommandResponse{Success: err == nil, Output: capture.joined()}, nil
}

func (s *indexServer) handleStatus(_ context.Context, _ json.RawMessage) (any, error) {
	capture := &captureWriter{}
	svc := featindexing.NewInitCommandService(initDepsWithCapture(s.deps, capture))
	err := svc.Status()
	return idxipc.CommandResponse{Success: err == nil, Output: capture.joined()}, nil
}

func initDepsWithCapture(deps ServerDeps, capture *captureWriter) featindexing.InitCommandServiceDeps {
	return featindexing.InitCommandServiceDeps{
		ProjectTree:    deps.ProjectTree,
		MatcherFactory: deps.MatcherFactory,
		Output:         capture,
		FileReader:     deps.FileReader,
		Indexer:        deps.Indexer,
		IndexRepo:      deps.IndexRepo,
		ChecksumRepo:   deps.ChecksumRepo,
		DaemonRepo:     deps.DaemonRepo,
	}
}

func searchOptionsFromRequest(req idxipc.SearchRequest) featsearch.Options {
	return featsearch.Options{
		// Always use JSON so the server can parse the output into a structured response.
		Format:           featsearch.OutputJSON,
		Context:          req.Context,
		AgentCompact:     req.AgentCompact,
		FilesOnly:        req.FilesOnly,
		Explain:          req.Explain,
		PathQueries:      req.PathQueries,
		ExtensionQueries: req.ExtensionQueries,
		From:             req.From,
		Size:             req.Size,
		Operator:         operatorOrDefault(req.Operator),
		PopularityWeight: req.PopularityWeight,
	}
}

func operatorOrDefault(op string) string {
	if op == "" {
		return featsearch.OperatorAND
	}
	return op
}

// parseSearchJSON deserializes the JSON line emitted by SearchCommandService.
func parseSearchJSON(line string) (idxipc.SearchResponse, error) {
	if line == "" {
		return idxipc.SearchResponse{Results: []idxipc.SearchResult{}}, nil
	}

	var raw struct {
		Count   int `json:"count"`
		Results []struct {
			File    string   `json:"file"`
			Name    string   `json:"name"`
			Path    string   `json:"path"`
			Score   *float64 `json:"score,omitempty"`
			Stale   bool     `json:"stale,omitempty"`
			Matches []struct {
				Line    int    `json:"line"`
				Content string `json:"content"`
				Match   bool   `json:"match"`
			} `json:"matches"`
		} `json:"results"`
	}

	if err := json.Unmarshal([]byte(line), &raw); err != nil {
		return idxipc.SearchResponse{}, err
	}

	results := make([]idxipc.SearchResult, 0, len(raw.Results))
	for _, r := range raw.Results {
		matches := make([]idxipc.MatchedLine, 0, len(r.Matches))
		for _, m := range r.Matches {
			matches = append(matches, idxipc.MatchedLine{Line: m.Line, Content: m.Content, Match: m.Match})
		}
		results = append(results, idxipc.SearchResult{
			File:    r.File,
			Name:    r.Name,
			Path:    r.Path,
			Score:   r.Score,
			Stale:   r.Stale,
			Matches: matches,
		})
	}

	return idxipc.SearchResponse{Count: raw.Count, Results: results}, nil
}

// ServerDeps groups all collaborators for the index server.
type ServerDeps struct {
	ProjectTree    sharedfs.ProjectTree
	MatcherFactory sharedfs.IgnoreMatcherBuilder
	FileReader     sharedfs.FileReader
	Indexer        featindexing.Indexer
	IndexRepo      featindexing.IndexRepository
	ChecksumRepo   featindexing.DirectoryChecksumRepository
	DaemonRepo     featindexing.ProjectMonitorChecker
	ReadLogRepo    featread.LogRepository
	SearchTuning   featsearch.SearchServiceOptions
	SocketPath     string
}
