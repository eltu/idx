package search

import "testing"

func TestColoredHelpersAndAbsInt(t *testing.T) {
	if coloredFilePath("a.go", false) != "a.go" {
		t.Fatal("expected plain path when ANSI disabled")
	}
	if coloredFilePath("a.go", true) == "a.go" {
		t.Fatal("expected ANSI-decorated path when ANSI enabled")
	}

	if coloredLineNumber(7, false) != "7" {
		t.Fatal("expected plain line number when ANSI disabled")
	}
	if coloredLineNumber(7, true) == "7" {
		t.Fatal("expected ANSI-decorated line number when ANSI enabled")
	}

	if absInt(-3) != 3 {
		t.Fatal("expected absInt to convert negative number")
	}
	if absInt(4) != 4 {
		t.Fatal("expected absInt to keep positive number")
	}
}

func TestHighlightTermsInLineBranches(t *testing.T) {
	line := "go search guide"
	terms := []string{"go", "search"}

	if highlightTermsInLine(line, terms, false) != line {
		t.Fatal("expected unchanged line when ANSI disabled")
	}

	highlighted := highlightTermsInLine(line, terms, true)
	if highlighted == line {
		t.Fatal("expected highlighted output when ANSI enabled")
	}

	unchanged := highlightTermsInLine("alpha beta", []string{"zzz"}, true)
	if unchanged != "alpha beta" {
		t.Fatal("expected unchanged line when no term matches")
	}
}

func TestFileNameMatchBonusExactStem(t *testing.T) {
	// Exact match on the file name stem (without extension) returns 1.0.
	bonus := fileNameMatchBonus([]string{"main"}, "main.go")
	if bonus != 1.0 {
		t.Fatalf("expected 1.0 for exact stem match, got %v", bonus)
	}
}

func TestFileNameMatchBonusPartialToken(t *testing.T) {
	// "main" is an exact token of "main_test.go" – same bonus as exact stem.
	bonus := fileNameMatchBonus([]string{"main"}, "main_test.go")
	if bonus != 1.0 {
		t.Fatalf("expected 1.0 for exact token match in main_test.go, got %v", bonus)
	}
}

func TestFileNameMatchBonusCamelCase(t *testing.T) {
	// "index" is a CamelCase token of "InvertedIndex.go".
	bonus := fileNameMatchBonus([]string{"index"}, "InvertedIndex.go")
	if bonus < 0.5 {
		t.Fatalf("expected >= 0.5 for CamelCase token match, got %v", bonus)
	}
}

func TestFileNameMatchBonusSnakeCase(t *testing.T) {
	// "search" is a snake_case token of "search_scoring.go".
	bonus := fileNameMatchBonus([]string{"search"}, "search_scoring.go")
	if bonus < 0.5 {
		t.Fatalf("expected >= 0.5 for snake_case token match, got %v", bonus)
	}
}

func TestFileNameMatchBonusNoMatch(t *testing.T) {
	// No query term appears in the filename – bonus must be zero.
	bonus := fileNameMatchBonus([]string{"daemon"}, "search_scoring.go")
	if bonus != 0.0 {
		t.Fatalf("expected 0.0 for non-matching query term, got %v", bonus)
	}
}

func TestFileNameTokensSnakeCase(t *testing.T) {
	tokens := fileNameTokens("search_scoring.go")
	want := map[string]bool{"search": true, "scoring": true, "go": true}
	for _, tok := range tokens {
		delete(want, tok)
	}
	if len(want) != 0 {
		t.Fatalf("missing tokens from snake_case split: %v", want)
	}
}

func TestFileNameTokensCamelCase(t *testing.T) {
	tokens := fileNameTokens("InvertedIndex.go")
	want := map[string]bool{"inverted": true, "index": true, "go": true}
	for _, tok := range tokens {
		delete(want, tok)
	}
	if len(want) != 0 {
		t.Fatalf("missing tokens from CamelCase split: %v", want)
	}
}
