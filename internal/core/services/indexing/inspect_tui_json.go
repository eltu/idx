package indexing

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"idx/internal/core/domain"
)

func updateInspectJSONMode(model inspectModel, key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key.String() {
	case "ctrl+c", "q":
		model.quitting = true
		return model, tea.Quit
	case "esc":
		model.mode = model.jsonReturnMode
		if model.mode == inspectViewModeLogs {
			model = adjustInspectLogsViewport(model)
		} else {
			model = adjustInspectDocumentsViewport(model)
		}
		return model, nil
	case "up", "k":
		if model.jsonStart > 0 {
			model.jsonStart--
		}
	case "down", "j":
		if model.jsonStart < maxInt(len(model.jsonLines)-inspectJSONHeight(model), 0) {
			model.jsonStart++
		}
	case "pgup":
		model.jsonStart = maxInt(model.jsonStart-inspectJSONHeight(model), 0)
	case "pgdown":
		model.jsonStart += inspectJSONHeight(model)
		model = adjustInspectJSONViewport(model)
		return model, nil
	}
	model = adjustInspectJSONViewport(model)
	return model, nil
}

func inspectJSONView(model inspectModel) string {
	var builder strings.Builder
	builder.WriteString(inspectTitleStyle.Render("idx inspect - Document JSON (read-only)"))
	builder.WriteString("\n")
	builder.WriteString(inspectHelpStyle.Render("Navigate JSON: up/down (or k/j, pgup/pgdown) | Back: esc | Commands: : | Quit: q, ctrl+c"))
	builder.WriteString("\n\n")
	builder.WriteString(inspectDocumentPathStyle.Render(inspectTruncateLine("Document: "+model.jsonTitle, model.width)))
	builder.WriteString("\n\n")

	start, end := inspectJSONRange(model)
	for i := start; i < end; i++ {
		builder.WriteString(inspectHighlightJSONLine(inspectTruncateLine(model.jsonLines[i], model.width)))
		builder.WriteString("\n")
	}
	builder.WriteString("\n")
	builder.WriteString(inspectStatusLineStyle.Render(fmt.Sprintf("Showing lines %d-%d of %d", start+1, end, len(model.jsonLines))))
	return builder.String()
}

func inspectHighlightJSONLine(line string) string {
	var builder strings.Builder
	for i := 0; i < len(line); {
		if line[i] == '"' {
			token, end := inspectReadJSONString(line, i)
			if inspectJSONStringIsKey(line, end) {
				builder.WriteString(inspectJSONKeyStyle.Render(token))
			} else {
				builder.WriteString(inspectJSONStringStyle.Render(token))
			}
			i = end
			continue
		}
		if inspectStartsJSONNumber(line, i) {
			token, end := inspectReadJSONNumber(line, i)
			builder.WriteString(inspectJSONNumberStyle.Render(token))
			i = end
			continue
		}
		if token, end, ok := inspectReadJSONKeyword(line, i); ok {
			builder.WriteString(inspectJSONKeywordStyle.Render(token))
			i = end
			continue
		}
		ch := line[i]
		if strings.ContainsRune("{}[]:,", rune(ch)) {
			builder.WriteString(inspectJSONPunctStyle.Render(string(ch)))
		} else {
			builder.WriteString(inspectJSONDefaultStyle.Render(string(ch)))
		}
		i++
	}
	return builder.String()
}

func inspectReadJSONString(line string, start int) (string, int) {
	for i := start + 1; i < len(line); i++ {
		if line[i] == '\\' {
			i++
			continue
		}
		if line[i] == '"' {
			return line[start : i+1], i + 1
		}
	}
	return line[start:], len(line)
}

func inspectJSONStringIsKey(line string, from int) bool {
	for i := from; i < len(line); i++ {
		if line[i] == ' ' || line[i] == '\t' {
			continue
		}
		return line[i] == ':'
	}
	return false
}

func inspectStartsJSONNumber(line string, start int) bool {
	ch := line[start]
	if ch >= '0' && ch <= '9' {
		return true
	}
	return ch == '-' && start+1 < len(line) && line[start+1] >= '0' && line[start+1] <= '9'
}

func inspectReadJSONNumber(line string, start int) (string, int) {
	i := start
	if line[i] == '-' {
		i++
	}
	i = inspectConsumeJSONDigits(line, i)
	if i < len(line) && line[i] == '.' {
		i++
		i = inspectConsumeJSONDigits(line, i)
	}
	i = inspectConsumeJSONExponent(line, i)
	return line[start:i], i
}

func inspectConsumeJSONDigits(line string, start int) int {
	index := start
	for index < len(line) && line[index] >= '0' && line[index] <= '9' {
		index++
	}
	return index
}

func inspectConsumeJSONExponent(line string, start int) int {
	index := start
	if index >= len(line) || (line[index] != 'e' && line[index] != 'E') {
		return index
	}
	index++
	if index < len(line) && (line[index] == '+' || line[index] == '-') {
		index++
	}
	return inspectConsumeJSONDigits(line, index)
}

func inspectReadJSONKeyword(line string, start int) (string, int, bool) {
	literals := []string{"true", "false", "null"}
	for _, literal := range literals {
		if strings.HasPrefix(line[start:], literal) {
			end := start + len(literal)
			if end < len(line) {
				next := line[end]
				if (next >= 'a' && next <= 'z') || (next >= 'A' && next <= 'Z') || (next >= '0' && next <= '9') || next == '_' {
					continue
				}
			}
			return literal, end, true
		}
	}
	return "", start, false
}

func inspectJSONHeight(model inspectModel) int { return maxInt(model.height-7, 1) }

func inspectJSONRange(model inspectModel) (int, int) {
	if len(model.jsonLines) == 0 {
		return 0, 0
	}
	start := maxInt(model.jsonStart, 0)
	end := minInt(start+inspectJSONHeight(model), len(model.jsonLines))
	if start >= end {
		start = len(model.jsonLines) - 1
		end = len(model.jsonLines)
	}
	return start, end
}

func adjustInspectJSONViewport(model inspectModel) inspectModel {
	if len(model.jsonLines) == 0 {
		model.jsonStart = 0
		return model
	}
	model.jsonStart = clampInt(model.jsonStart, 0, maxInt(len(model.jsonLines)-inspectJSONHeight(model), 0))
	return model
}

func inspectDocumentJSON(index *domain.InvertedIndex, row inspectDocumentRow) (string, error) {
	if index == nil {
		return "", fmt.Errorf("index is nil")
	}
	docStats, ok := index.Documents[row.key]
	if !ok || docStats == nil {
		return "", fmt.Errorf("document %q not found in index", row.key)
	}
	type inspectTermEntry struct {
		Term      string `json:"term"`
		TF        int    `json:"tf"`
		Positions []int  `json:"positions"`
	}
	type inspectDocumentPayload struct {
		Name        string             `json:"name"`
		Path        string             `json:"path"`
		Length      int                `json:"length"`
		UniqueTerms int                `json:"uniqueTerms"`
		Terms       []inspectTermEntry `json:"terms"`
	}
	terms := make([]inspectTermEntry, 0)
	for term, termStats := range index.Terms {
		if termStats == nil {
			continue
		}
		docTerm, exists := termStats.Docs[row.key]
		if !exists || docTerm == nil {
			continue
		}
		terms = append(terms, inspectTermEntry{Term: term, TF: docTerm.TF, Positions: append([]int(nil), docTerm.Positions...)})
	}
	sort.Slice(terms, func(i int, j int) bool { return terms[i].Term < terms[j].Term })
	payload := inspectDocumentPayload{Name: docStats.Name, Path: docStats.Path, Length: docStats.Length, UniqueTerms: len(terms), Terms: terms}
	bytes, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to encode inspect document payload: %w", err)
	}
	return string(bytes), nil
}
