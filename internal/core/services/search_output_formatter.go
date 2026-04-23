package services

import (
	"fmt"
	"sort"
	"strings"
)

const (
	ansiReset      = "\033[0m"
	ansiLightBlue  = "\033[96m"
	ansiBoldYellow = "\033[1;33m"
	ansiLightGreen = "\033[92m"
)

// coloredFilePath wraps path in light-blue ANSI codes for terminal display when enabled.
func coloredFilePath(path string, useANSI bool) string {
	if !useANSI {
		return path
	}

	return ansiLightBlue + path + ansiReset
}

// coloredLineNumber wraps a line number in light-green ANSI codes when enabled.
func coloredLineNumber(n int, useANSI bool) string {
	if !useANSI {
		return fmt.Sprintf("%d", n)
	}

	return fmt.Sprintf("%s%d%s", ansiLightGreen, n, ansiReset)
}

// highlightTermsInLine wraps each whole-word match
// Matching is case-insensitive; original casing is preserved in the output.
// Example: highlightTermsInLine("go search guide", []string{"go"}) → "\033[1;33mgo\033[0m search guide"
func highlightTermsInLine(line string, terms []string, useANSI bool) string {
	if !useANSI {
		return line
	}

	lower := strings.ToLower(line)

	type span struct{ start, end int }
	var spans []span

	for _, term := range terms {
		start := 0
		for start < len(lower) {
			idx := strings.Index(lower[start:], term)
			if idx < 0 {
				break
			}
			abs := start + idx
			before := abs == 0 || !isWordChar(lower[abs-1])
			after := abs+len(term) >= len(lower) || !isWordChar(lower[abs+len(term)])
			if before && after {
				spans = append(spans, span{abs, abs + len(term)})
			}
			start = abs + 1
		}
	}

	if len(spans) == 0 {
		return line
	}

	sort.Slice(spans, func(i, j int) bool { return spans[i].start < spans[j].start })

	merged := []span{spans[0]}
	for _, s := range spans[1:] {
		last := &merged[len(merged)-1]
		if s.start <= last.end {
			if s.end > last.end {
				last.end = s.end
			}
		} else {
			merged = append(merged, s)
		}
	}

	var sb strings.Builder
	pos := 0
	for _, s := range merged {
		sb.WriteString(line[pos:s.start])
		sb.WriteString(ansiBoldYellow)
		sb.WriteString(line[s.start:s.end])
		sb.WriteString(ansiReset)
		pos = s.end
	}
	sb.WriteString(line[pos:])
	return sb.String()
}
