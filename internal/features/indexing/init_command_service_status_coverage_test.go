package indexing

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTruncateStatusColumn_ShortValue_FitsWithinMaxWidth(t *testing.T) {
	t.Parallel()

	// Act
	got := truncateStatusColumn("short", 10)

	// Assert
	assert.Equal(t, "short", got)
}

func TestTruncateStatusColumn_LongValue_TruncatesWithEllipsis(t *testing.T) {
	t.Parallel()

	// Act
	got := truncateStatusColumn("very long value here", 10)

	// Assert
	assert.Len(t, got, 10)
	assert.Equal(t, "...", got[len(got)-3:])
}

func TestTruncateStatusColumn_MaxWidthLessThanFour_TruncatesHard(t *testing.T) {
	t.Parallel()

	// Act
	got := truncateStatusColumn("hello", 3)

	// Assert
	assert.Equal(t, "hel", got)
}

func TestTruncateStatusColumn_MaxWidthZero_TruncatesHard(t *testing.T) {
	t.Parallel()

	// Act
	got := truncateStatusColumn("hello", 0)

	// Assert
	assert.Equal(t, "", got)
}

func TestTruncateStatusColumn_ExactLength_NoTruncation(t *testing.T) {
	t.Parallel()

	// Act
	got := truncateStatusColumn("hello", 5)

	// Assert
	assert.Equal(t, "hello", got)
}
