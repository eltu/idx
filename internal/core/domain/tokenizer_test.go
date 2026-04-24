package domain

import "testing"

func TestTokenizeTextUsesWhitespaceOnly(t *testing.T) {
	text := "fmt.Println(\"Hello\") if x == y { return a+b }"
	tokens := TokenizeText(text)
	expected := []string{"fmt.println(\"hello\")", "if", "x", "==", "y", "{", "return", "a+b", "}"}

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
