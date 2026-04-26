package search

import (
	"testing"

	"idx/internal/core/ports"
)

func TestNormalizedSearchOptions(t *testing.T) {
	options := normalizedSearchOptions(ports.SearchOptions{
		Format:      "",
		Context:     -1,
		PrettyJSON:  true,
		PathQuery:   "  internal/core  ",
		PathQueries: []string{"", "internal/core", "internal/core", " docs "},
		Limit:       -2,
	})

	if options.Format != ports.SearchOutputText {
		t.Fatalf("expected default text format, got %q", options.Format)
	}
	if options.Context != 0 {
		t.Fatalf("expected normalized context 0, got %d", options.Context)
	}
	if options.PrettyJSON {
		t.Fatal("expected pretty json disabled when format is text")
	}
	if options.Limit != 0 {
		t.Fatalf("expected normalized limit 0, got %d", options.Limit)
	}
	if len(options.PathQueries) != 2 {
		t.Fatalf("expected deduplicated path queries, got %v", options.PathQueries)
	}
}

func TestSearchResultOptionHelpers(t *testing.T) {
	results := []searchResult{
		{directoryPath: "/repo", fileName: "a.go", score: 1.0, matchedLines: []matchedLine{{lineNumber: 1, content: "alpha", isMatch: true}, {lineNumber: 2, content: "ctx", isMatch: false}}},
		{directoryPath: "/repo", fileName: "a.go", score: 0.5, matchedLines: []matchedLine{{lineNumber: 3, content: "alpha", isMatch: true}}},
		{directoryPath: "/repo", fileName: "b.go", score: 0.8, matchedLines: []matchedLine{{lineNumber: 1, content: "beta", isMatch: false}}},
	}

	filesOnly := filesOnlyResults(results)
	if len(filesOnly) != 2 {
		t.Fatalf("expected deduplicated files, got %v", filesOnly)
	}

	matchedOnly := matchesOnlyResults(results)
	if len(matchedOnly) != 2 {
		t.Fatalf("expected results with at least one matched line, got %v", matchedOnly)
	}
	if len(matchedOnly[0].matchedLines) == 0 || !matchedOnly[0].matchedLines[0].isMatch {
		t.Fatalf("expected only matched lines, got %v", matchedOnly[0].matchedLines)
	}

	limited := limitedResults(results, 1)
	if len(limited) != 1 {
		t.Fatalf("expected one limited result, got %d", len(limited))
	}
}

type outputSpy struct {
	lines []string
}

func (spy *outputSpy) WriteLine(text string) error {
	spy.lines = append(spy.lines, text)
	return nil
}

func TestWriteEmptySearchResultsFormats(t *testing.T) {
	output := &outputSpy{}
	service := SearchCommandService{output: output}

	if err := service.writeEmptySearchResults(ports.SearchOptions{Format: ports.SearchOutputText}); err != nil {
		t.Fatalf("expected text empty result write success, got %v", err)
	}
	if output.lines[0] != "Nenhum resultado encontrado." {
		t.Fatalf("unexpected text empty output %q", output.lines[0])
	}

	if err := service.writeEmptySearchResults(ports.SearchOptions{Format: ports.SearchOutputJSON}); err != nil {
		t.Fatalf("expected json empty result write success, got %v", err)
	}
	if output.lines[1] != "[]" {
		t.Fatalf("unexpected json empty output %q", output.lines[1])
	}
}
