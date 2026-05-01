package indexing

import (
	"sort"
	"strings"
	"time"
)

func sortInspectLogsNewestFirst(rows []inspectLogRow) {
	sort.SliceStable(rows, func(i int, j int) bool {
		left := rows[i]
		right := rows[j]

		leftTime, leftOK := parseInspectLogTime(left.indexedAt)
		rightTime, rightOK := parseInspectLogTime(right.indexedAt)
		if leftOK && rightOK {
			return leftTime.After(rightTime)
		}

		if leftOK {
			return true
		}

		if rightOK {
			return false
		}

		return left.indexedAt > right.indexedAt
	})
}

func parseInspectLogTime(value string) (time.Time, bool) {
	if strings.TrimSpace(value) == "" || value == "-" {
		return time.Time{}, false
	}

	layouts := []string{time.RFC3339Nano, time.RFC3339, "2006-01-02 15:04:05", "2006-01-02 15:04:05Z07:00"}
	for _, layout := range layouts {
		parsed, err := time.Parse(layout, value)
		if err == nil {
			return parsed, true
		}
	}

	return time.Time{}, false
}

func inspectStringField(fields map[string]any, keys ...string) string {
	for _, key := range keys {
		value, ok := fields[key]
		if !ok {
			continue
		}

		stringValue, isString := value.(string)
		if !isString {
			continue
		}

		trimmed := strings.TrimSpace(stringValue)
		if trimmed != "" {
			return trimmed
		}
	}

	return ""
}

func parseInspectSummaryFields(summary string) (string, string, string) {
	if strings.TrimSpace(summary) == "" {
		return "", "", ""
	}

	normalized := strings.NewReplacer(",", " ",
		";", " ",
		"|", " ",
	).Replace(summary)

	parsed := map[string]string{}
	for _, token := range strings.Fields(normalized) {
		if strings.Contains(token, "=") {
			parts := strings.SplitN(token, "=", 2)
			if len(parts) == 2 {
				parsed[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
			}
			continue
		}

		if strings.Contains(token, ":") {
			parts := strings.SplitN(token, ":", 2)
			if len(parts) == 2 {
				parsed[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
			}
		}
	}

	indexedAt := parsed["indexed_at"]
	pathValue := parsed["path"]
	hash := parsed["hash"]

	if indexedAt == "" {
		indexedAt = extractSummaryValue(summary, "indexed_at")
	}
	if pathValue == "" {
		pathValue = extractSummaryValue(summary, "path")
	}
	if hash == "" {
		hash = extractSummaryValue(summary, "hash")
	}

	return indexedAt, pathValue, hash
}

func extractSummaryValue(summary string, key string) string {
	patterns := []string{key + "=", key + ":"}
	for _, pattern := range patterns {
		start := strings.Index(summary, pattern)
		if start < 0 {
			continue
		}

		value := strings.TrimSpace(summary[start+len(pattern):])
		if value == "" {
			continue
		}

		for index, runeValue := range value {
			if runeValue == ',' || runeValue == ';' || runeValue == '|' || runeValue == ' ' || runeValue == '\t' {
				return strings.TrimSpace(value[:index])
			}
		}

		return strings.TrimSpace(value)
	}

	return ""
}
