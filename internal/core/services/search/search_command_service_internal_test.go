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
		From:        -3,
		Size:        -2,
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
	if options.From != 0 {
		t.Fatalf("expected normalized from 0, got %d", options.From)
	}
	if options.Size != 0 {
		t.Fatalf("expected normalized size 0, got %d", options.Size)
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

	paginated := paginatedResults(results, 1, 1)
	if len(paginated) != 1 {
		t.Fatalf("expected one paginated result, got %d", len(paginated))
	}
	if paginated[0].fileName != "a.go" || paginated[0].score != 0.5 {
		t.Fatalf("expected second ranked result after from=1, got %+v", paginated[0])
	}

	pageWithoutSize := paginatedResults(results, 2, 0)
	if len(pageWithoutSize) != 1 {
		t.Fatalf("expected one result from from=2 and size=0, got %d", len(pageWithoutSize))
	}

	outOfRange := paginatedResults(results, 10, 1)
	if len(outOfRange) != 0 {
		t.Fatalf("expected empty result for out-of-range from, got %d", len(outOfRange))
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
	if output.lines[0] != "No results found." {
		t.Fatalf("unexpected text empty output %q", output.lines[0])
	}

	if err := service.writeEmptySearchResults(ports.SearchOptions{Format: ports.SearchOutputJSON}); err != nil {
		t.Fatalf("expected json empty result write success, got %v", err)
	}
	if output.lines[1] != "[]" {
		t.Fatalf("unexpected json empty output %q", output.lines[1])
	}
}
