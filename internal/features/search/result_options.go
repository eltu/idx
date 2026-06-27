package search

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

func cacheKeyFor(query string, options Options) string {
	keyParts := []string{
		fmt.Sprintf("q:%s", query),
		fmt.Sprintf("fmt:%s", options.Format),
		fmt.Sprintf("ctx:%d", options.Context),
		fmt.Sprintf("fo:%v", options.FilesOnly),
		fmt.Sprintf("pq:%s", strings.Join(options.PathQueries, ":")),
		fmt.Sprintf("eq:%s", strings.Join(options.ExtensionQueries, ":")),
		fmt.Sprintf("op:%s", options.Operator),
		fmt.Sprintf("rel-en:%v", options.RelaxationEnabled),
		fmt.Sprintf("rel-min:%d", options.RelaxationMinExclusive),
	}
	keyStr := strings.Join(keyParts, "|")
	hash := sha256.Sum256([]byte(keyStr))
	return fmt.Sprintf("%x", hash)
}

func normalizedSearchOptions(options Options) Options {
	normalized := options
	if normalized.Format == "" {
		normalized.Format = OutputText
	}

	if normalized.Context < 0 {
		normalized.Context = 0
	}

	if normalized.Format != OutputJSON {
		normalized.PrettyJSON = false
	}

	if normalized.From < 0 {
		normalized.From = 0
	}

	if normalized.Size < 0 {
		normalized.Size = 0
	}

	normalized.PathQuery = strings.TrimSpace(normalized.PathQuery)
	normalized.PathQueries = normalizedFilterQueries(normalized.PathQueries, normalized.PathQuery)
	normalized.ExtensionQuery = normalizeExtensionValue(normalized.ExtensionQuery)
	normalized.ExtensionQueries = normalizedExtensionQueries(normalized.ExtensionQueries, normalized.ExtensionQuery)

	if normalized.Operator == "" {
		normalized.Operator = OperatorAND
	}

	if normalized.RelaxationMinExclusive < 0 {
		normalized.RelaxationMinExclusive = 0
	}

	return normalized
}

func normalizedExtensionQueries(queries []string, fallback string) []string {
	normalized := normalizedFilterQueries(queries, fallback)
	extensions := make([]string, 0, len(normalized))
	seen := make(map[string]struct{})
	for _, query := range normalized {
		extension := normalizeExtensionValue(query)
		if extension == "" {
			continue
		}

		if _, exists := seen[extension]; exists {
			continue
		}

		seen[extension] = struct{}{}
		extensions = append(extensions, extension)
	}

	return extensions
}

func normalizeExtensionValue(value string) string {
	trimmed := strings.TrimSpace(strings.ToLower(value))
	if trimmed == "" {
		return ""
	}

	if strings.HasPrefix(trimmed, ".") {
		return strings.TrimPrefix(trimmed, ".")
	}

	return trimmed
}

func normalizedFilterQueries(queries []string, fallback string) []string {
	if len(queries) == 0 && fallback != "" {
		queries = append(queries, fallback)
	}

	normalized := make([]string, 0, len(queries))
	seen := make(map[string]struct{})
	for _, query := range queries {
		trimmed := strings.TrimSpace(query)
		if trimmed == "" {
			continue
		}

		if _, exists := seen[trimmed]; exists {
			continue
		}

		seen[trimmed] = struct{}{}
		normalized = append(normalized, trimmed)
	}

	return normalized
}

func applySearchResultOptions(results []searchResult, options Options) []searchResult {
	filtered := results
	if options.FilesOnly {
		filtered = filesOnlyResults(filtered)
	}

	return paginatedResults(filtered, options.From, options.Size)
}

func paginatedResults(results []searchResult, from, size int) []searchResult {
	if from < 0 {
		from = 0
	}

	if from >= len(results) {
		return []searchResult{}
	}

	sliced := results[from:]
	return limitedResults(sliced, size)
}

func filesOnlyResult(r searchResult) searchResult {
	return searchResult{
		directoryPath: r.directoryPath,
		fileName:      r.fileName,
		matchedLines:  []matchedLine{},
		score:         r.score,
	}
}

func filesOnlyResults(results []searchResult) []searchResult {
	uniqueFiles := make(map[string]searchResult)
	for _, result := range results {
		fullPath := filepath.Join(result.directoryPath, result.fileName)
		if existing, exists := uniqueFiles[fullPath]; exists {
			if result.score > existing.score {
				uniqueFiles[fullPath] = filesOnlyResult(result)
			}
			continue
		}
		uniqueFiles[fullPath] = filesOnlyResult(result)
	}

	filtered := make([]searchResult, 0, len(uniqueFiles))
	for _, result := range uniqueFiles {
		filtered = append(filtered, result)
	}

	sort.Slice(filtered, func(i, j int) bool {
		if filtered[i].score != filtered[j].score {
			return filtered[i].score > filtered[j].score
		}
		iPath := filepath.Join(filtered[i].directoryPath, filtered[i].fileName)
		jPath := filepath.Join(filtered[j].directoryPath, filtered[j].fileName)
		return iPath < jPath
	})

	return filtered
}

func limitedResults(results []searchResult, limit int) []searchResult {
	if limit <= 0 || len(results) <= limit {
		return results
	}

	return results[:limit]
}

func (service SearchCommandService) writeEmptySearchResults(options Options) error {
	if err := service.validateDependencies(); err != nil {
		return err
	}

	if options.Format == OutputJSON {
		emptyResponse := map[string]any{"count": 0, "results": []any{}}
		encoded, err := encodeOutputJSON(emptyResponse, options.PrettyJSON)
		if err != nil {
			return err
		}
		return service.output.WriteLine(string(encoded))
	}

	return service.output.WriteLine("No results found.")
}

func encodeOutputJSON(payload any, pretty bool) ([]byte, error) {
	if pretty {
		return json.MarshalIndent(payload, "", "  ")
	}

	return json.Marshal(payload)
}

func (service SearchCommandService) writeSearchResults(results []searchResult, projectRoot string, terms []string, options Options, totalMatches int) error {
	if err := service.validateDependencies(); err != nil {
		return err
	}

	if options.Format == OutputJSON {
		return service.writeResultsJSON(results, projectRoot, options, totalMatches)
	}

	if !options.AgentCompact {
		if err := service.writeResultsHeader(len(results), totalMatches, options); err != nil {
			return err
		}
	}

	return service.writeResults(results, projectRoot, terms, true, options)
}

func (service SearchCommandService) writeResultsHeader(displayedCount, totalCount int, _ Options) error {
	if err := service.validateDependencies(); err != nil {
		return err
	}

	var msg string
	if displayedCount == totalCount {
		msg = fmt.Sprintf("📁 Found %d file(s) matching your search", totalCount)
	} else {
		msg = fmt.Sprintf("📁 Found %d file(s) matching your search (showing %d with pagination)", totalCount, displayedCount)
	}
	return service.output.WriteLine(msg)
}

func (service SearchCommandService) writeResults(results []searchResult, projectRoot string, terms []string, useANSI bool, options Options) error {
	if err := service.validateDependencies(); err != nil {
		return err
	}

	if options.FilesOnly {
		return service.writeFilesOnlyResults(results, projectRoot)
	}

	return service.writeDetailedResults(results, projectRoot, terms, useANSI, options)
}

func (service SearchCommandService) writeFilesOnlyResults(results []searchResult, projectRoot string) error {
	for _, result := range results {
		projectRelativePath, err := relativeResultPath(projectRoot, result.directoryPath, result.fileName)
		if err != nil {
			return err
		}

		if err := service.output.WriteLine(projectRelativePath); err != nil {
			return err
		}
	}

	return nil
}

func (service SearchCommandService) writeDetailedResults(results []searchResult, projectRoot string, terms []string, useANSI bool, options Options) error {
	if options.AgentCompact {
		useANSI = false
	}

	for _, result := range results {
		if err := service.writeResultBlock(result, projectRoot, terms, useANSI, options); err != nil {
			return err
		}
	}

	return nil
}

func (service SearchCommandService) writeResultBlock(result searchResult, projectRoot string, terms []string, useANSI bool, options Options) error {
	projectRelativePath, err := relativeResultPath(projectRoot, result.directoryPath, result.fileName)
	if err != nil {
		return err
	}

	header := coloredFilePath(projectRelativePath, useANSI)
	if options.Explain {
		header = fmt.Sprintf("%s (score: %.4f)", header, result.score)
	}
	if err := service.output.WriteLine(header); err != nil {
		return err
	}

	if result.stale {
		return service.output.WriteLine("└── ⚠ file not found — index is outdated, run idx sync")
	}

	if err := service.writeMatchedLinesWithOptions(result.matchedLines, terms, useANSI, options); err != nil {
		return err
	}

	if options.AgentCompact {
		return nil
	}

	return service.output.WriteLine("")
}

func (service SearchCommandService) writeMatchedLines(lines []matchedLine, terms []string, useANSI bool) error {
	return service.writeMatchedLinesWithOptions(lines, terms, useANSI, Options{})
}

func (service SearchCommandService) writeMatchedLinesWithOptions(lines []matchedLine, terms []string, useANSI bool, options Options) error {
	for index, line := range lines {
		entry := formattedMatchedLine(index, len(lines), line, terms, useANSI)
		if options.AgentCompact {
			entry = formattedMatchedLineCompact(line)
		}
		if err := service.output.WriteLine(entry); err != nil {
			return err
		}
	}

	return nil
}

func formattedMatchedLineCompact(line matchedLine) string {
	lineContent := strings.TrimSpace(line.content)
	return fmt.Sprintf("%s:%s", coloredLineNumber(line.lineNumber, false), lineContent)
}

func formattedMatchedLine(index, total int, line matchedLine, terms []string, useANSI bool) string {
	prefix := "├──"
	if index == total-1 {
		prefix = "└──"
	}

	lineContent := line.content
	if line.isMatch {
		lineContent = highlightTermsInLine(line.content, terms, useANSI)
	}

	return fmt.Sprintf("%s %s: %s", prefix, coloredLineNumber(line.lineNumber, useANSI), lineContent)
}

// FormattedMatchedLine formats one search result line with a ├──/└── tree prefix.
// Example: FormattedMatchedLine(0, 2, 42, "func main()", true, []string{"main"}, true).
func FormattedMatchedLine(index, total, lineNumber int, content string, isMatch bool, terms []string, useANSI bool) string {
	return formattedMatchedLine(index, total, matchedLine{lineNumber: lineNumber, content: content, isMatch: isMatch}, terms, useANSI)
}

// FormattedMatchedLineCompact formats a search result line as "lineNum:content" without a tree prefix.
// Example: FormattedMatchedLineCompact(42, "func main()").
func FormattedMatchedLineCompact(lineNumber int, content string) string {
	return formattedMatchedLineCompact(matchedLine{lineNumber: lineNumber, content: content})
}
