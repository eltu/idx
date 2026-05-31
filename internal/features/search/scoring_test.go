package search

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"idx/internal/shared/readlog"
)

const goSearchGuideLine = "go search guide"

func TestColoredFilePath_WithoutANSI_ReturnsPlainPath(t *testing.T) {
	t.Parallel()

	// Act & Assert
	assert.Equal(t, "a.go", coloredFilePath("a.go", false))
}

func TestColoredFilePath_WithANSI_ReturnsDecoratedPath(t *testing.T) {
	t.Parallel()

	// Act & Assert
	assert.NotEqual(t, "a.go", coloredFilePath("a.go", true))
}

func TestColoredLineNumber_WithoutANSI_ReturnsPlainNumber(t *testing.T) {
	t.Parallel()

	// Act & Assert
	assert.Equal(t, "7", coloredLineNumber(7, false))
}

func TestColoredLineNumber_WithANSI_ReturnsDecoratedNumber(t *testing.T) {
	t.Parallel()

	// Act & Assert
	assert.NotEqual(t, "7", coloredLineNumber(7, true))
}

func TestAbsInt_NegativeInput_ReturnsPositive(t *testing.T) {
	t.Parallel()

	// Act & Assert
	assert.Equal(t, 3, absInt(-3))
}

func TestAbsInt_PositiveInput_ReturnsUnchanged(t *testing.T) {
	t.Parallel()

	// Act & Assert
	assert.Equal(t, 4, absInt(4))
}

func TestHighlightTermsInLine_WithoutANSI_ReturnsUnchangedLine(t *testing.T) {
	t.Parallel()

	// Arrange
	line := goSearchGuideLine

	// Act & Assert
	assert.Equal(t, line, highlightTermsInLine(line, []string{"go", "search"}, false))
}

func TestHighlightTermsInLine_WithANSI_ReturnsHighlightedLine(t *testing.T) {
	t.Parallel()

	// Arrange
	line := goSearchGuideLine

	// Act
	highlighted := highlightTermsInLine(line, []string{"go", "search"}, true)

	// Assert
	assert.NotEqual(t, line, highlighted)
}

func TestHighlightTermsInLine_NoTermMatch_ReturnsUnchanged(t *testing.T) {
	t.Parallel()

	// Act & Assert
	assert.Equal(t, "alpha beta", highlightTermsInLine("alpha beta", []string{"zzz"}, true))
}

func TestFileNameMatchBonus_FileNames(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		terms     []string
		fileName  string
		wantMin   float64
		wantExact float64
	}{
		{
			name:      "ExactStem_ReturnsFull",
			terms:     []string{"main"},
			fileName:  "main.go",
			wantExact: 1.0,
		},
		{
			name:      "ExactTokenInSnakeCase_ReturnsFull",
			terms:     []string{"main"},
			fileName:  "main_test.go",
			wantExact: 1.0,
		},
		{
			name:     "CamelCase_ReturnsAtLeastHalf",
			terms:    []string{"index"},
			fileName: "InvertedIndex.go",
			wantMin:  0.5,
		},
		{
			name:     "SnakeCase_ReturnsAtLeastHalf",
			terms:    []string{"search"},
			fileName: "search_scoring.go",
			wantMin:  0.5,
		},
		{
			name:      "NoMatch_ReturnsZero",
			terms:     []string{"daemon"},
			fileName:  "search_scoring.go",
			wantExact: 0.0,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// Act
			bonus := fileNameMatchBonus(tc.terms, tc.fileName)

			// Assert
			if tc.wantExact != 0 || tc.wantMin == 0 {
				assert.Equal(t, tc.wantExact, bonus)
			} else {
				assert.GreaterOrEqual(t, bonus, tc.wantMin)
			}
		})
	}
}

func TestFileNameTokens_SnakeCase_ExtractsAllTokens(t *testing.T) {
	t.Parallel()

	// Act
	tokens := fileNameTokens("search_scoring.go")

	// Assert
	want := map[string]bool{"search": true, "scoring": true, "go": true}
	for _, tok := range tokens {
		delete(want, tok)
	}
	assert.Empty(t, want, "missing tokens from snake_case split")
}

func TestFileNameTokens_CamelCase_ExtractsAllTokens(t *testing.T) {
	t.Parallel()

	// Act
	tokens := fileNameTokens("InvertedIndex.go")

	// Assert
	want := map[string]bool{"inverted": true, "index": true, "go": true}
	for _, tok := range tokens {
		delete(want, tok)
	}
	assert.Empty(t, want, "missing tokens from CamelCase split")
}

func TestPopularityBonus(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 19, 12, 0, 0, 0, time.UTC)

	t.Run("ZeroReadCount_ReturnsZero", func(t *testing.T) {
		t.Parallel()

		entry := readlog.LogEntry{ReadCount: 0, LastReadAt: now}
		assert.Equal(t, 0.0, popularityBonus(entry, now, 0.3))
	})

	t.Run("ZeroWeight_ReturnsZero", func(t *testing.T) {
		t.Parallel()

		entry := readlog.LogEntry{ReadCount: 10, LastReadAt: now}
		assert.Equal(t, 0.0, popularityBonus(entry, now, 0))
	})

	t.Run("TenReadsToday_ApproachesFullWeight", func(t *testing.T) {
		t.Parallel()

		entry := readlog.LogEntry{ReadCount: 10, LastReadAt: now}
		got := popularityBonus(entry, now, 0.3)
		assert.InDelta(t, 0.30, got, 0.01)
	})

	t.Run("FourteenDayOldRead_HalfDecay", func(t *testing.T) {
		t.Parallel()

		past := now.Add(-14 * 24 * time.Hour)
		entry := readlog.LogEntry{ReadCount: 10, LastReadAt: past}
		got := popularityBonus(entry, now, 0.3)
		assert.InDelta(t, 0.15, got, 0.01)
	})

	t.Run("VeryHighReadCount_CappedAtWeight", func(t *testing.T) {
		t.Parallel()

		entry := readlog.LogEntry{ReadCount: 10000, LastReadAt: now}
		got := popularityBonus(entry, now, 0.3)
		assert.LessOrEqual(t, got, 0.3+1e-9)
	})

	t.Run("FutureTimestamp_NonNegativeBonus", func(t *testing.T) {
		t.Parallel()

		future := now.Add(24 * time.Hour)
		entry := readlog.LogEntry{ReadCount: 5, LastReadAt: future}
		got := popularityBonus(entry, now, 0.3)
		assert.GreaterOrEqual(t, got, 0.0)
	})
}

func TestWriteMatchedLines_DelegatesToWithOptions(t *testing.T) {
	t.Parallel()

	// Arrange
	out := &capturingOutput{}
	svc := SearchCommandService{output: out}
	lines := []matchedLine{{lineNumber: 1, content: "hello world"}}

	// Act
	err := svc.writeMatchedLines(lines, []string{"hello"}, false)

	// Assert
	require.NoError(t, err)
	assert.NotEmpty(t, out.lines)
}

type capturingOutput struct{ lines []string }

func (o *capturingOutput) WriteLine(text string) error {
	o.lines = append(o.lines, text)
	return nil
}
