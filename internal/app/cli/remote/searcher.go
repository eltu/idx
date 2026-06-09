package remote

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"charm.land/lipgloss/v2"

	featsearch "idx/internal/features/search"
	idxipc "idx/internal/shared/ipc"
	sharedoutput "idx/internal/shared/output"
)

var searchTimingStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#64748B"))

// RemoteSearcher implements cli.Searcher by calling idx.search over the socket.
type RemoteSearcher struct {
	client *SocketClient
	output sharedoutput.Writer
}

// NewRemoteSearcher creates a Searcher backed by the idx JSON-RPC server.
// Example: s := NewRemoteSearcher(client, writer).
func NewRemoteSearcher(client *SocketClient, output sharedoutput.Writer) *RemoteSearcher {
	return &RemoteSearcher{client: client, output: output}
}

// RunWithOptions sends a search request and writes formatted results to output.
func (s *RemoteSearcher) RunWithOptions(query string, opts featsearch.Options) error {
	req := searchRequestFromOptions(query, opts)
	var resp idxipc.SearchResponse

	start := time.Now()
	if err := s.client.Call(idxipc.MethodSearch, req, &resp); err != nil {
		return err
	}
	elapsed := time.Since(start)

	if opts.CountOnly {
		if err := s.output.WriteLine(fmt.Sprintf("%d", resp.Count)); err != nil {
			return err
		}
	} else if err := writeSearchResults(resp, query, opts, s.output); err != nil {
		return err
	}

	if opts.Timing {
		return s.output.WriteLine(searchTimingStyle.Render(fmt.Sprintf("  ⏱  %dms", elapsed.Milliseconds())))
	}
	return nil
}

func searchRequestFromOptions(query string, opts featsearch.Options) idxipc.SearchRequest {
	return idxipc.SearchRequest{
		Query:             query,
		Size:              opts.Size,
		Operator:          opts.Operator,
		Format:            opts.Format,
		Context:           opts.Context,
		ExtensionQueries:  opts.ExtensionQueries,
		PathQueries:       opts.PathQueries,
		PopularityWeight:  opts.PopularityWeight,
		FilesOnly:         opts.FilesOnly,
		AgentCompact:      opts.AgentCompact,
		Explain:           opts.Explain,
		From:              opts.From,
		RelaxationEnabled: opts.RelaxationEnabled,
		RelaxationMin:     opts.RelaxationMinExclusive,
		CountOnly:         opts.CountOnly,
		Timing:            opts.Timing,
	}
}

func writeSearchResults(resp idxipc.SearchResponse, query string, opts featsearch.Options, out sharedoutput.Writer) error {
	if opts.Format == featsearch.OutputJSON {
		return writeSearchResultsJSON(resp, opts, out)
	}
	return writeSearchResultsText(resp, query, opts, out)
}

func writeSearchResultsJSON(resp idxipc.SearchResponse, opts featsearch.Options, out sharedoutput.Writer) error {
	var (
		payload any
		err     error
		encoded []byte
	)

	if opts.FilesOnly {
		paths := make([]string, 0, len(resp.Results))
		for _, r := range resp.Results {
			paths = append(paths, r.Path)
		}
		payload = paths
	} else {
		payload = resp
	}

	if opts.PrettyJSON {
		encoded, err = json.MarshalIndent(payload, "", "  ")
	} else {
		encoded, err = json.Marshal(payload)
	}
	if err != nil {
		return fmt.Errorf("failed to encode search response as JSON: %w", err)
	}
	return out.WriteLine(string(encoded))
}

const msgNoResultsFound = "No results found."
const msgStaleResult = "└── ⚠ file not found — index is outdated, run idx sync"
const msgResultsHeader = "📁 Found %d file(s) matching your search"
const msgResultsHeaderPaginated = "📁 Found %d file(s) matching your search (showing %d with pagination)"

func writeSearchResultsText(resp idxipc.SearchResponse, query string, opts featsearch.Options, out sharedoutput.Writer) error {
	if len(resp.Results) == 0 {
		return out.WriteLine(msgNoResultsFound)
	}
	if !opts.AgentCompact {
		if err := writeResultsHeader(resp.Count, len(resp.Results), out); err != nil {
			return err
		}
	}
	terms := queryTerms(query)
	for _, r := range resp.Results {
		if err := writeRichTextResult(r, terms, opts, out); err != nil {
			return err
		}
	}
	return nil
}

func writeResultsHeader(totalCount, displayedCount int, out sharedoutput.Writer) error {
	var msg string
	if displayedCount == totalCount {
		msg = fmt.Sprintf(msgResultsHeader, totalCount)
	} else {
		msg = fmt.Sprintf(msgResultsHeaderPaginated, totalCount, displayedCount)
	}
	return out.WriteLine(msg)
}

func writeRichTextResult(r idxipc.SearchResult, terms []string, opts featsearch.Options, out sharedoutput.Writer) error {
	if opts.FilesOnly {
		return out.WriteLine(r.Path)
	}

	useANSI := !opts.AgentCompact
	header := featsearch.ColoredFilePath(r.Path, useANSI)
	if opts.Explain && r.Score != nil {
		header = fmt.Sprintf("%s (score: %.4f)", header, *r.Score)
	}
	if err := out.WriteLine(header); err != nil {
		return err
	}

	if r.Stale {
		return out.WriteLine(msgStaleResult)
	}

	matches := r.Matches
	if opts.MatchesOnly {
		matches = onlyMatchedLines(matches)
	}

	for i, m := range matches {
		var line string
		if opts.AgentCompact {
			line = featsearch.FormattedMatchedLineCompact(m.Line, strings.TrimRight(m.Content, "\r\n"))
		} else {
			line = featsearch.FormattedMatchedLine(i, len(matches), m.Line, m.Content, m.Match, terms, useANSI)
		}
		if err := out.WriteLine(line); err != nil {
			return err
		}
	}

	if !opts.AgentCompact {
		return out.WriteLine("")
	}
	return nil
}

func onlyMatchedLines(matches []idxipc.MatchedLine) []idxipc.MatchedLine {
	filtered := make([]idxipc.MatchedLine, 0, len(matches))
	for _, m := range matches {
		if m.Match {
			filtered = append(filtered, m)
		}
	}
	return filtered
}

func queryTerms(query string) []string {
	return strings.Fields(strings.ToLower(query))
}
