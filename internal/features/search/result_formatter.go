package search

import (
	"fmt"
	"sort"
	"strings"
)

const (
	ansiReset      = "\033[0m"
	ansiFilePath   = "\033[38;2;99;102;241m"   // #6366F1 Primary — file paths
	ansiMatchBold  = "\033[1;38;2;251;191;36m" // #FBBF24 Accent bold — matched terms
	ansiLineNumber = "\033[38;2;100;116;139m"  // #64748B Muted — line numbers
)

// coloredFilePath wraps path in light-blue ANSI codes for terminal display when enabled.
func coloredFilePath(path string, useANSI bool) string {
	if !useANSI {
		return path
	}

	return ansiFilePath + path + ansiReset
}

// coloredLineNumber wraps a line number in light-green ANSI codes when enabled.
func coloredLineNumber(n int, useANSI bool) string {
	if !useANSI {
		return fmt.Sprintf("%d", n)
	}

	return fmt.Sprintf("%s%d%s", ansiLineNumber, n, ansiReset)
}

// highlightTermsInLine wraps each whole-word match
// Matching is case-insensitive; original casing is preserved in the output.
// Example: highlightTermsInLine("go search guide", []string{"go"}) → "\033[1;33mgo\033[0m search guide".
func highlightTermsInLine(line string, terms []string, useANSI bool) string {
	if !useANSI {
		return line
	}

	spans := findHighlightSpans(line, terms)
	if len(spans) == 0 {
		return line
	}

	merged := mergeHighlightSpans(spans)
	return renderHighlightedLine(line, merged)
}

type highlightSpan struct {
	start int
	end   int
}

func findHighlightSpans(line string, terms []string) []highlightSpan {
	lower := strings.ToLower(line)
	spans := make([]highlightSpan, 0)
	for _, term := range terms {
		start := 0
		for start < len(lower) {
			idx := strings.Index(lower[start:], term)
			if idx < 0 {
				break
			}

			abs := start + idx
			if !isWholeWordMatch(lower, abs, len(term)) {
				start = abs + 1
				continue
			}

			spans = append(spans, highlightSpan{start: abs, end: abs + len(term)})
			start = abs + 1
		}
	}

	return spans
}

func isWholeWordMatch(line string, start, length int) bool {
	before := start == 0 || !isWordChar(line[start-1])
	after := start+length >= len(line) || !isWordChar(line[start+length])
	return before && after
}

func mergeHighlightSpans(spans []highlightSpan) []highlightSpan {
	sort.Slice(spans, func(i, j int) bool {
		return spans[i].start < spans[j].start
	})

	merged := []highlightSpan{spans[0]}
	for _, current := range spans[1:] {
		last := &merged[len(merged)-1]
		if current.start > last.end {
			merged = append(merged, current)
			continue
		}

		if current.end > last.end {
			last.end = current.end
		}
	}

	return merged
}

func renderHighlightedLine(line string, spans []highlightSpan) string {
	var builder strings.Builder
	position := 0
	for _, span := range spans {
		builder.WriteString(line[position:span.start])
		builder.WriteString(ansiMatchBold)
		builder.WriteString(line[span.start:span.end])
		builder.WriteString(ansiReset)
		position = span.end
	}

	builder.WriteString(line[position:])
	return builder.String()
}

// ColoredFilePath wraps path in the idx file-path color for terminal display when useANSI is true.
// Example: ColoredFilePath("internal/main.go", true).
func ColoredFilePath(path string, useANSI bool) string { return coloredFilePath(path, useANSI) }

// ColoredLineNumber wraps n in the idx line-number color for terminal display when useANSI is true.
// Example: ColoredLineNumber(42, true).
func ColoredLineNumber(n int, useANSI bool) string { return coloredLineNumber(n, useANSI) }

// HighlightTermsInLine wraps whole-word matches of terms in the idx accent color when useANSI is true.
// Example: HighlightTermsInLine("func main()", []string{"main"}, true).
func HighlightTermsInLine(line string, terms []string, useANSI bool) string {
	return highlightTermsInLine(line, terms, useANSI)
}
