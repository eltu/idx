package search

import (
	"crypto/md5"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"idx/internal/core/ports"
)

func cacheKeyFor(query string, options ports.SearchOptions) string {
	keyParts := []string{
		fmt.Sprintf("q:%s", query),
		fmt.Sprintf("fmt:%s", options.Format),
		fmt.Sprintf("ctx:%d", options.Context),
		fmt.Sprintf("mo:%v", options.MatchesOnly),
		fmt.Sprintf("fo:%v", options.FilesOnly),
		fmt.Sprintf("pq:%s", strings.Join(options.PathQueries, ":")),
	}
	keyStr := strings.Join(keyParts, "|")
	hash := md5.Sum([]byte(keyStr))
	return fmt.Sprintf("%x", hash)
}

func normalizedSearchOptions(options ports.SearchOptions) ports.SearchOptions {
	normalized := options
	if normalized.Format == "" {
		normalized.Format = ports.SearchOutputText
	}

	if normalized.Context < 0 {
		normalized.Context = 0
	}

	if normalized.Format != ports.SearchOutputJSON {
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

	if normalized.Operator == "" {
		normalized.Operator = ports.SearchOperatorAND
	}

	return normalized
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

func applySearchResultOptions(results []searchResult, options ports.SearchOptions, hasContentTerms bool) []searchResult {
	filtered := results
	if options.FilesOnly {
		filtered = filesOnlyResults(filtered)
	} else if options.MatchesOnly && hasContentTerms {
		filtered = matchesOnlyResults(filtered)
	}

	return paginatedResults(filtered, options.From, options.Size)
}

func paginatedResults(results []searchResult, from int, size int) []searchResult {
	if from < 0 {
		from = 0
	}

	if from >= len(results) {
		return []searchResult{}
	}

	sliced := results[from:]
	return limitedResults(sliced, size)
}

func filesOnlyResults(results []searchResult) []searchResult {
	uniqueFiles := make(map[string]searchResult)
	for _, result := range results {
		fullPath := result.directoryPath + "/" + result.fileName
		if existing, exists := uniqueFiles[fullPath]; exists {
			if result.score > existing.score {
				uniqueFiles[fullPath] = searchResult{directoryPath: result.directoryPath, fileName: result.fileName, matchedLines: []matchedLine{}, score: result.score}
			}
			continue
		}

		uniqueFiles[fullPath] = searchResult{directoryPath: result.directoryPath, fileName: result.fileName, matchedLines: []matchedLine{}, score: result.score}
	}

	filtered := make([]searchResult, 0, len(uniqueFiles))
	for _, result := range uniqueFiles {
		filtered = append(filtered, result)
	}

	sort.Slice(filtered, func(i int, j int) bool {
		if filtered[i].score != filtered[j].score {
			return filtered[i].score > filtered[j].score
		}
		iPath := filtered[i].directoryPath + "/" + filtered[i].fileName
		jPath := filtered[j].directoryPath + "/" + filtered[j].fileName
		return iPath < jPath
	})

	return filtered
}

func matchesOnlyResults(results []searchResult) []searchResult {
	filtered := make([]searchResult, 0, len(results))
	for _, result := range results {
		matchedLines := onlyMatchedLines(result.matchedLines)
		if len(matchedLines) == 0 {
			continue
		}

		filtered = append(filtered, searchResult{directoryPath: result.directoryPath, fileName: result.fileName, matchedLines: matchedLines, score: result.score})
	}

	return filtered
}

func onlyMatchedLines(lines []matchedLine) []matchedLine {
	matched := make([]matchedLine, 0, len(lines))
	for _, line := range lines {
		if !line.isMatch {
			continue
		}

		matched = append(matched, line)
	}

	return matched
}

func limitedResults(results []searchResult, limit int) []searchResult {
	if limit <= 0 || len(results) <= limit {
		return results
	}

	return results[:limit]
}

func (service SearchCommandService) writeEmptySearchResults(options ports.SearchOptions) error {
	if err := service.validateDependencies(); err != nil {
		return err
	}

	if options.Format == ports.SearchOutputJSON {
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

func (service SearchCommandService) writeSearchResults(results []searchResult, projectRoot string, terms []string, options ports.SearchOptions, totalMatches int) error {
	if err := service.validateDependencies(); err != nil {
		return err
	}

	if options.Format == ports.SearchOutputJSON {
		return service.writeResultsJSON(results, projectRoot, options, totalMatches)
	}

	if err := service.writeResultsHeader(len(results), totalMatches, options); err != nil {
		return err
	}

	return service.writeResults(results, projectRoot, terms, true, options)
}

func (service SearchCommandService) writeResultsHeader(displayedCount int, totalCount int, options ports.SearchOptions) error {
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

func (service SearchCommandService) writeResults(results []searchResult, projectRoot string, terms []string, useANSI bool, options ports.SearchOptions) error {
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

func (service SearchCommandService) writeDetailedResults(results []searchResult, projectRoot string, terms []string, useANSI bool, options ports.SearchOptions) error {
	for _, result := range results {
		if err := service.writeResultBlock(result, projectRoot, terms, useANSI, options); err != nil {
			return err
		}
	}

	return nil
}

func (service SearchCommandService) writeResultBlock(result searchResult, projectRoot string, terms []string, useANSI bool, options ports.SearchOptions) error {
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

	if err := service.writeMatchedLines(result.matchedLines, terms, useANSI); err != nil {
		return err
	}

	return service.output.WriteLine("")
}

func (service SearchCommandService) writeMatchedLines(lines []matchedLine, terms []string, useANSI bool) error {
	for index, line := range lines {
		entry := formattedMatchedLine(index, len(lines), line, terms, useANSI)
		if err := service.output.WriteLine(entry); err != nil {
			return err
		}
	}

	return nil
}

func formattedMatchedLine(index int, total int, line matchedLine, terms []string, useANSI bool) string {
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
