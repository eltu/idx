package services

import (
	"encoding/json"

	"idx/internal/core/ports"
)

type jsonSearchResult struct {
	File    string                `json:"file"`
	Score   float64               `json:"score"`
	Matches []jsonSearchMatchLine `json:"matches"`
}

type jsonSearchMatchLine struct {
	Line    int    `json:"line"`
	Content string `json:"content"`
	Match   bool   `json:"match"`
}

func (service SearchCommandService) writeResultsJSON(results []searchResult, projectRoot string, options ports.SearchOptions) error {
	if options.FilesOnly {
		// For --files-only, return just an array of file paths.
		filePaths := make([]string, 0, len(results))
		for _, result := range results {
			projectRelativePath, err := relativeResultPath(projectRoot, result.directoryPath, result.fileName)
			if err != nil {
				return err
			}

			filePaths = append(filePaths, projectRelativePath)
		}

		var (
			encoded []byte
			err     error
		)
		if options.PrettyJSON {
			encoded, err = json.MarshalIndent(filePaths, "", "  ")
		} else {
			encoded, err = json.Marshal(filePaths)
		}
		if err != nil {
			return err
		}

		return service.output.WriteLine(string(encoded))
	}

	// Standard JSON output with matches.
	payload := make([]jsonSearchResult, 0, len(results))
	for _, result := range results {
		projectRelativePath, err := relativeResultPath(projectRoot, result.directoryPath, result.fileName)
		if err != nil {
			return err
		}

		matches := make([]jsonSearchMatchLine, 0, len(result.matchedLines))
		for _, line := range result.matchedLines {
			matches = append(matches, jsonSearchMatchLine{Line: line.lineNumber, Content: line.content, Match: line.isMatch})
		}

		payload = append(payload, jsonSearchResult{File: projectRelativePath, Score: result.score, Matches: matches})
	}

	var (
		encoded []byte
		err     error
	)
	if options.PrettyJSON {
		encoded, err = json.MarshalIndent(payload, "", "  ")
	} else {
		encoded, err = json.Marshal(payload)
	}
	if err != nil {
		return err
	}

	return service.output.WriteLine(string(encoded))
}
