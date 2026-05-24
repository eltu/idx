package tui

import "strings"

var inspectAvailableCommands = []string{"index", "tlog"}

func autocompleteInspectCommand(query string) string {
	normalized := strings.TrimSpace(strings.TrimPrefix(query, ":"))
	if normalized == "" {
		return query
	}

	matches := inspectCommandSuggestions(normalized)
	if len(matches) == 1 {
		return matches[0]
	}

	if len(matches) <= 1 {
		return query
	}

	prefix := matches[0]
	for _, match := range matches[1:] {
		prefix = inspectCommonPrefix(prefix, match)
		if prefix == "" {
			break
		}
	}

	if len(prefix) > len(normalized) {
		return prefix
	}

	return query
}

func inspectCommandSuggestions(query string) []string {
	normalized := strings.TrimSpace(strings.TrimPrefix(query, ":"))
	if normalized == "" {
		return append([]string(nil), inspectAvailableCommands...)
	}

	suggestions := make([]string, 0, len(inspectAvailableCommands))
	for _, command := range inspectAvailableCommands {
		if strings.HasPrefix(command, normalized) {
			suggestions = append(suggestions, command)
		}
	}

	return suggestions
}

func inspectCommonPrefix(left string, right string) string {
	leftRunes := []rune(left)
	rightRunes := []rune(right)
	maxLen := len(leftRunes)
	if len(rightRunes) < maxLen {
		maxLen = len(rightRunes)
	}

	index := 0
	for index < maxLen && leftRunes[index] == rightRunes[index] {
		index++
	}

	return string(leftRunes[:index])
}
