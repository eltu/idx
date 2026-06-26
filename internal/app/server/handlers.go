package server

import (
	"context"
	"encoding/json"
	"strings"

	featindexing "idx/internal/features/indexing"
	featlifecycle "idx/internal/features/lifecycle"
	featread "idx/internal/features/read"
	featrelated "idx/internal/features/related"
	featsearch "idx/internal/features/search"
	sharedconfig "idx/internal/shared/config"
	sharedfs "idx/internal/shared/filesystem"
	idxipc "idx/internal/shared/ipc"
	"idx/internal/shared/readlog"
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
	svc := featread.NewReadCommandService(s.deps.ProjectTree, streamer, capture).
		WithReadLog(s.deps.ReadLogRepo)
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
	out := capture.joined()
	if err != nil && out == "" {
		out = err.Error()
	}
	return idxipc.CommandResponse{Success: err == nil, Output: out}, nil
}

func (s *indexServer) handleStatus(_ context.Context, _ json.RawMessage) (any, error) {
	capture := &captureWriter{}
	svc := featindexing.NewInitCommandService(initDepsWithCapture(s.deps, capture))
	err := svc.Status()
	return idxipc.CommandResponse{Success: err == nil, Output: capture.joined()}, nil
}

func (s *indexServer) handleInspect(_ context.Context, params json.RawMessage) (any, error) {
	var req idxipc.InspectRequest
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, err
	}

	svc := featindexing.NewInitCommandService(initDepsWithCapture(s.deps, &captureWriter{}))
	return svc.LoadInspectIndex(req.IndexPath)
}

func (s *indexServer) handleDestroy(_ context.Context, _ json.RawMessage) (any, error) {
	capture := &captureWriter{}
	svc := featlifecycle.NewDestroyCommandService(s.deps.ProjectTree, capture)
	err := svc.Run()
	return idxipc.CommandResponse{Success: err == nil, Output: capture.joined()}, nil
}

func (s *indexServer) handleRelated(_ context.Context, params json.RawMessage) (any, error) {
	var req idxipc.RelatedRequest
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, err
	}

	capture := &captureWriter{}
	svc := featrelated.NewRelatedCommandService(
		s.deps.ProjectTree, s.deps.IndexRepo, s.deps.ReadLogRepo, capture,
	)
	opts := featrelated.Options{
		Format:  featrelated.OutputJSON,
		Size:    req.Size,
		Skip:    req.Skip,
		Since:   req.Since,
		Ext:     req.Ext,
		Compact: req.Compact,
	}
	if err := svc.Run(req.FilePath, opts); err != nil {
		return nil, err
	}

	return parseRelatedJSON(capture.firstLine())
}

func parseRelatedJSON(line string) (idxipc.RelatedResponse, error) {
	if line == "" {
		return idxipc.RelatedResponse{Results: []idxipc.RelatedResult{}}, nil
	}
	var results []struct {
		Path   string  `json:"path"`
		Score  float64 `json:"score"`
		Reason string  `json:"reason"`
	}
	if err := json.Unmarshal([]byte(line), &results); err != nil {
		return idxipc.RelatedResponse{}, err
	}
	out := make([]idxipc.RelatedResult, 0, len(results))
	for _, r := range results {
		out = append(out, idxipc.RelatedResult{Path: r.Path, Score: r.Score, Reason: r.Reason})
	}
	return idxipc.RelatedResponse{Count: len(out), Results: out}, nil
}

func (s *indexServer) handleConfig(_ context.Context, _ json.RawMessage) (any, error) {
	var sb strings.Builder
	if err := sharedconfig.FormatOutput(&sb, s.deps.Config, s.deps.ConfigFilePath, s.deps.ConfigOverrides); err != nil {
		return nil, err
	}
	return idxipc.CommandResponse{Success: true, Output: sb.String()}, nil
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
		// Always JSON: the server serializes results into SearchResponse for the client.
		// FilesOnly is intentionally omitted: the service emits a bare JSON array when
		// FilesOnly=true, which parseSearchJSON cannot handle — the client filters instead.
		Format:                 featsearch.OutputJSON,
		Context:                req.Context,
		Explain:                req.Explain,
		PathQueries:            req.PathQueries,
		ExtensionQueries:       req.ExtensionQueries,
		From:                   req.From,
		Size:                   req.Size,
		Operator:               operatorOrDefault(req.Operator),
		PopularityWeight:       req.PopularityWeight,
		RelaxationEnabled:      req.RelaxationEnabled,
		RelaxationMinExclusive: req.RelaxationMin,
		Since:                  req.Since,
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
	ProjectTree     sharedfs.ProjectTree
	MatcherFactory  sharedfs.IgnoreMatcherBuilder
	FileReader      sharedfs.FileReader
	Indexer         featindexing.Indexer
	IndexRepo       featindexing.IndexRepository
	ChecksumRepo    featindexing.DirectoryChecksumRepository
	DaemonRepo      featindexing.ProjectMonitorChecker
	ReadLogRepo     readlog.LogRepository
	SearchTuning    featsearch.SearchServiceOptions
	SocketPath      string
	Config          sharedconfig.IdxConfig
	ConfigFilePath  string
	ConfigOverrides []string
}
