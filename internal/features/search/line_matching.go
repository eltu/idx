package search

import "strings"

func matchingLinesInContent(content string, terms []string, contextSize int) []matchedLine {
	lines := strings.Split(content, "\n")
	matchedIndexes := matchedLineIndexes(lines, terms)

	if len(matchedIndexes) == 0 {
		return []matchedLine{}
	}

	if contextSize <= 0 {
		return matchedLinesWithoutContext(lines, matchedIndexes)
	}

	includedIndexes := includedContextIndexes(matchedIndexes, contextSize, len(lines))
	result := make([]matchedLine, 0, len(includedIndexes))
	for index, line := range lines {
		if _, exists := includedIndexes[index]; !exists {
			continue
		}

		_, isMatch := matchedIndexes[index]
		result = append(result, matchedLine{lineNumber: index + 1, content: line, isMatch: isMatch})
	}

	return result
}

func matchedLineIndexes(lines, terms []string) map[int]struct{} {
	matchedIndexes := make(map[int]struct{})
	for index, line := range lines {
		if lineContainsAnyTerm(line, terms) {
			matchedIndexes[index] = struct{}{}
		}
	}

	return matchedIndexes
}

func matchedLinesWithoutContext(lines []string, matchedIndexes map[int]struct{}) []matchedLine {
	matches := make([]matchedLine, 0, len(matchedIndexes))
	for index, line := range lines {
		if _, exists := matchedIndexes[index]; !exists {
			continue
		}

		matches = append(matches, matchedLine{lineNumber: index + 1, content: line, isMatch: true})
	}

	return matches
}

func includedContextIndexes(matchedIndexes map[int]struct{}, contextSize, lineCount int) map[int]struct{} {
	includedIndexes := make(map[int]struct{})
	for index := range matchedIndexes {
		start := index - contextSize
		if start < 0 {
			start = 0
		}

		end := index + contextSize
		if end >= lineCount {
			end = lineCount - 1
		}

		for contextIndex := start; contextIndex <= end; contextIndex++ {
			includedIndexes[contextIndex] = struct{}{}
		}
	}

	return includedIndexes
}

// maxTermsOnLine returns the maximum number of distinct query terms that
// co-occur on any single matched line. Only direct match lines (isMatch=true)
// are considered. This value is used as a tiebreaker in ranking: a file where
// all terms appear on the same line (e.g. "err := root.Execute()") is more
// relevant than one where terms are scattered across different lines.
func maxTermsOnLine(lines []matchedLine, terms []string) int {
	maxTerms := 0
	for _, line := range lines {
		termCount := matchedTermCountOnLine(line, terms)
		if termCount > maxTerms {
			maxTerms = termCount
		}
	}
	return maxTerms
}

func matchedTermCountOnLine(line matchedLine, terms []string) int {
	if !line.isMatch {
		return 0
	}

	lowerLine := strings.ToLower(line.content)
	count := 0
	for _, term := range terms {
		if lineContainsTerm(lowerLine, term) {
			count++
		}
	}

	return count
}

// lineContainsAnyTerm returns true when the line contains at least one term as a whole word token.
// Whole-word is defined by the same character class as the tokenizer: [a-zA-Z0-9_].
func lineContainsAnyTerm(line string, terms []string) bool {
	lower := strings.ToLower(line)
	for _, term := range terms {
		if lineContainsTerm(lower, term) {
			return true
		}
	}
	return false
}

func lineContainsTerm(lowerLine, term string) bool {
	start := 0
	for {
		idx := strings.Index(lowerLine[start:], term)
		if idx < 0 {
			return false
		}
		abs := start + idx
		before := abs == 0 || !isWordChar(lowerLine[abs-1])
		after := abs+len(term) == len(lowerLine) || !isWordChar(lowerLine[abs+len(term)])
		if before && after {
			return true
		}
		start = abs + 1
	}
}

func isWordChar(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '_'
}
