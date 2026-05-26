package search

import (
	"encoding/json"
)

type jsonSearchResponse struct {
	Count   int                `json:"count"`
	Results []jsonSearchResult `json:"results"`
}

type jsonSearchResult struct {
	File    string                `json:"file"`
	Name    string                `json:"name"`
	Path    string                `json:"path"`
	Score   *float64              `json:"score,omitempty"`
	Stale   bool                  `json:"stale,omitempty"`
	Matches []jsonSearchMatchLine `json:"matches"`
}

type jsonSearchMatchLine struct {
	Line    int    `json:"line"`
	Content string `json:"content"`
	Match   bool   `json:"match"`
}

func (service SearchCommandService) writeResultsJSON(results []searchResult, projectRoot string, options Options, totalMatches int) error {
	if options.FilesOnly {
		filePaths, err := service.jsonFilePaths(results, projectRoot)
		if err != nil {
			return err
		}

		encoded, err := encodeSearchJSON(filePaths, options.PrettyJSON)
		if err != nil {
			return err
		}

		return service.output.WriteLine(string(encoded))
	}

	response, err := service.jsonSearchResponse(results, projectRoot, totalMatches, options)
	if err != nil {
		return err
	}

	encoded, err := encodeSearchJSON(response, options.PrettyJSON)
	if err != nil {
		return err
	}

	return service.output.WriteLine(string(encoded))
}

func (service SearchCommandService) jsonFilePaths(results []searchResult, projectRoot string) ([]string, error) {
	filePaths := make([]string, 0, len(results))
	for _, result := range results {
		projectRelativePath, err := relativeResultPath(projectRoot, result.directoryPath, result.fileName)
		if err != nil {
			return nil, err
		}

		filePaths = append(filePaths, projectRelativePath)
	}

	return filePaths, nil
}

func (service SearchCommandService) jsonSearchResponse(results []searchResult, projectRoot string, totalMatches int, options Options) (jsonSearchResponse, error) {
	payload := make([]jsonSearchResult, 0, len(results))
	for _, result := range results {
		jsonResult, err := service.jsonSearchResult(result, projectRoot, options)
		if err != nil {
			return jsonSearchResponse{}, err
		}

		payload = append(payload, jsonResult)
	}

	return jsonSearchResponse{Count: totalMatches, Results: payload}, nil
}

func (service SearchCommandService) jsonSearchResult(result searchResult, projectRoot string, options Options) (jsonSearchResult, error) {
	projectRelativePath, err := relativeResultPath(projectRoot, result.directoryPath, result.fileName)
	if err != nil {
		return jsonSearchResult{}, err
	}

	var score *float64
	if options.Explain {
		score = &result.score
	}

	return jsonSearchResult{
		File:    projectRelativePath,
		Name:    result.fileName,
		Path:    projectRelativePath,
		Score:   score,
		Stale:   result.stale,
		Matches: jsonMatchLines(result.matchedLines),
	}, nil
}

func jsonMatchLines(matchedLines []matchedLine) []jsonSearchMatchLine {
	matches := make([]jsonSearchMatchLine, 0, len(matchedLines))
	for _, line := range matchedLines {
		matches = append(matches, jsonSearchMatchLine{Line: line.lineNumber, Content: line.content, Match: line.isMatch})
	}

	return matches
}

func encodeSearchJSON(value any, pretty bool) ([]byte, error) {
	if pretty {
		return json.MarshalIndent(value, "", "  ")
	}

	return json.Marshal(value)
}
