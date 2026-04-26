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
