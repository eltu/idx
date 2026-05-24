package indexing

import "testing"

func TestTokenizeTextUsesStandardDelimiters(t *testing.T) {
	text := "fmt.Println(\"Hello\") if x == y { return a+b }"
	tokens := TokenizeText(text)
	expected := []string{"fmt.println", "hello", "if", "x", "y", "return", "a", "b"}

	if len(tokens) != len(expected) {
		t.Fatalf("expected %d tokens, got %d", len(expected), len(tokens))
	}

	for index, token := range tokens {
		if token.Token != expected[index] {
			t.Fatalf("expected token %q at index %d, got %q", expected[index], index, token.Token)
		}
	}
}

func TestTokenizeTextHandlesPeriodsByContext(t *testing.T) {
	text := "version 1.26.1. domain example.com. trailing."
	tokens := TokenizeText(text)
	expected := []string{"version", "1.26.1", "domain", "example.com", "trailing"}

	if len(tokens) != len(expected) {
		t.Fatalf("expected %d tokens, got %d", len(expected), len(tokens))
	}

	for index, token := range tokens {
		if token.Token != expected[index] {
			t.Fatalf("expected token %q at index %d, got %q", expected[index], index, token.Token)
		}
	}
}

func TestTokenizeTextSplitsHyphenWordsUnlessNumeric(t *testing.T) {
	text := "foo-bar F-150 alpha-beta x-1 2024-10"
	tokens := TokenizeText(text)
	expected := []string{"foo", "bar", "f-150", "alpha", "beta", "x-1", "2024-10"}

	if len(tokens) != len(expected) {
		t.Fatalf("expected %d tokens, got %d", len(expected), len(tokens))
	}

	for index, token := range tokens {
		if token.Token != expected[index] {
			t.Fatalf("expected token %q at index %d, got %q", expected[index], index, token.Token)
		}
	}
}

func TestTokenizeTextDoesNotRemoveStopwords(t *testing.T) {
	text := "the and if to"
	tokens := TokenizeText(text)
	expected := []string{"the", "and", "if", "to"}

	if len(tokens) != len(expected) {
		t.Fatalf("expected %d tokens, got %d", len(expected), len(tokens))
	}

	for index, token := range tokens {
		if token.Token != expected[index] {
			t.Fatalf("expected token %q at index %d, got %q", expected[index], index, token.Token)
		}
	}
}

func TestTokenizeTextTracksTokenPositions(t *testing.T) {
	text := "go\tfmt.Println\nx"
	tokens := TokenizeText(text)
	expectedTokens := []string{"go", "fmt.println", "x"}
	expectedPositions := []int{0, 3, 15}

	if len(tokens) != len(expectedTokens) {
		t.Fatalf("expected %d tokens, got %d", len(expectedTokens), len(tokens))
	}

	for index, token := range tokens {
		if token.Token != expectedTokens[index] {
			t.Fatalf("expected token %q at index %d, got %q", expectedTokens[index], index, token.Token)
		}

		if token.Position != expectedPositions[index] {
			t.Fatalf("expected position %d at index %d, got %d", expectedPositions[index], index, token.Position)
		}
	}
}

func TestTokenizeFileNameSnakeCase(t *testing.T) {
	tokens := tokenStrings(TokenizeFileName("main_test.go"))
	assertTokensEqual(t, tokens, []string{"main", "test", "go"})
}

func TestTokenizeFileNameCamelCase(t *testing.T) {
	tokens := tokenStrings(TokenizeFileName("InvertedIndex.go"))
	assertTokensEqual(t, tokens, []string{"inverted", "index", "go"})
}

func TestTokenizeFileNameMixed(t *testing.T) {
	tokens := tokenStrings(TokenizeFileName("bm25_score.go"))
	assertTokensEqual(t, tokens, []string{"bm25", "score", "go"})
}

func TestTokenizeFileNameStem(t *testing.T) {
	// No extension — still splits on underscore.
	tokens := tokenStrings(TokenizeFileName("search_service"))
	assertTokensEqual(t, tokens, []string{"search", "service"})
}

func TestSplitCamelCaseWordsEmptyReturnsNil(t *testing.T) {
	result := splitCamelCaseWords([]rune{})
	if result != nil {
		t.Fatalf("expected nil for empty rune slice, got %v", result)
	}
}

func TestIsNumericHyphenNonAlphanumericAdjacentReturnsFalse(t *testing.T) {
	// "-" followed by " " (space) — non-alphanumeric right side → false
	if isNumericHyphen("a- b", 1) {
		t.Fatal("expected false when right char is non-alphanumeric")
	}
}

func TestIsNumericHyphenAtBoundaryReturnsFalse(t *testing.T) {
	if isNumericHyphen("-ab", 0) {
		t.Fatal("expected false for hyphen at index 0")
	}
	if isNumericHyphen("ab-", 2) {
		t.Fatal("expected false for hyphen at last index")
	}
}

func tokenStrings(twp []TokenWithPosition) []string {
	out := make([]string, len(twp))
	for i, t := range twp {
		out[i] = t.Token
	}

	return out
}

func assertTokensEqual(t *testing.T, got []string, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("expected tokens %v, got %v", want, got)
	}

	for i, tok := range got {
		if tok != want[i] {
			t.Fatalf("expected token[%d]=%q, got %q (full: %v)", i, want[i], tok, got)
		}
	}
}
