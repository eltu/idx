package search

import (
	"testing"
	"time"

	"idx/internal/core/ports"
)

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

func TestPopularityBonus(t *testing.T) {
	now := time.Date(2026, 5, 19, 12, 0, 0, 0, time.UTC)

	t.Run("zero read count returns zero", func(t *testing.T) {
		entry := ports.ReadLogEntry{ReadCount: 0, LastReadAt: now}
		if got := popularityBonus(entry, now, 0.3); got != 0 {
			t.Fatalf("expected 0 bonus for unread file, got %f", got)
		}
	})

	t.Run("zero weight returns zero", func(t *testing.T) {
		entry := ports.ReadLogEntry{ReadCount: 10, LastReadAt: now}
		if got := popularityBonus(entry, now, 0); got != 0 {
			t.Fatalf("expected 0 bonus when weight is zero, got %f", got)
		}
	})

	t.Run("10 reads today approaches full weight", func(t *testing.T) {
		entry := ports.ReadLogEntry{ReadCount: 10, LastReadAt: now}
		got := popularityBonus(entry, now, 0.3)
		// log1p(10)/log1p(10) * decay(0) * 0.3 = 1.0 * 1.0 * 0.3 = 0.3
		if got < 0.29 || got > 0.31 {
			t.Fatalf("expected ~0.30 bonus, got %f", got)
		}
	})

	t.Run("14-day-old read has half decay", func(t *testing.T) {
		past := now.Add(-14 * 24 * time.Hour)
		entry := ports.ReadLogEntry{ReadCount: 10, LastReadAt: past}
		got := popularityBonus(entry, now, 0.3)
		// decay(14 days) = 0.5; raw = 1.0 * 0.5 * 0.3 = 0.15
		if got < 0.14 || got > 0.16 {
			t.Fatalf("expected ~0.15 bonus at 14-day half-life, got %f", got)
		}
	})

	t.Run("bonus is capped at weight even for very high read counts", func(t *testing.T) {
		entry := ports.ReadLogEntry{ReadCount: 10000, LastReadAt: now}
		got := popularityBonus(entry, now, 0.3)
		if got > 0.3+1e-9 {
			t.Fatalf("expected bonus capped at weight 0.3, got %f", got)
		}
	})

	t.Run("future timestamp treated as now (no negative decay)", func(t *testing.T) {
		future := now.Add(24 * time.Hour)
		entry := ports.ReadLogEntry{ReadCount: 5, LastReadAt: future}
		got := popularityBonus(entry, now, 0.3)
		if got < 0 {
			t.Fatalf("expected non-negative bonus for future timestamp, got %f", got)
		}
	})
}
