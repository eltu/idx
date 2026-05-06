package indexing

import "testing"

func TestTruncateStatusColumnFitsWithinMaxWidth(t *testing.T) {
	got := truncateStatusColumn("short", 10)
	if got != "short" {
		t.Fatalf("expected 'short', got %q", got)
	}
}

func TestTruncateStatusColumnTruncatesWithEllipsis(t *testing.T) {
	got := truncateStatusColumn("very long value here", 10)
	if len(got) != 10 {
		t.Fatalf("expected length 10, got %d (%q)", len(got), got)
	}
	if got[len(got)-3:] != "..." {
		t.Fatalf("expected trailing '...', got %q", got)
	}
}

func TestTruncateStatusColumnMaxWidthLessThanFourTruncatesHard(t *testing.T) {
	got := truncateStatusColumn("hello", 3)
	if got != "hel" {
		t.Fatalf("expected 'hel' for maxWidth=3, got %q", got)
	}
}

func TestTruncateStatusColumnMaxWidthZeroTruncatesHard(t *testing.T) {
	got := truncateStatusColumn("hello", 0)
	if got != "" {
		t.Fatalf("expected empty for maxWidth=0, got %q", got)
	}
}

func TestTruncateStatusColumnExactLengthNoTruncation(t *testing.T) {
	got := truncateStatusColumn("hello", 5)
	if got != "hello" {
		t.Fatalf("expected 'hello' for exact fit, got %q", got)
	}
}
