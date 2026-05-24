package indexing

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const idxIgnoreLine = ".idx/\n"

func (service InitCommandService) ensureIdxRuleInGitIgnore(projectRoot string) error {
	gitIgnorePath := filepath.Join(projectRoot, ".gitignore")
	content, err := service.fileReader.ReadFile(gitIgnorePath)
	if err != nil {
		if !isMissingFileError(err) {
			return fmt.Errorf("failed to read project .gitignore %q: got error %v, expected readable file", gitIgnorePath, err)
		}

		return service.projectTree.WriteFile(gitIgnorePath, []byte(idxIgnoreLine))
	}

	if hasIdxDirectoryIgnoreRule(content) {
		return nil
	}

	updated := appendIdxDirectoryIgnoreRule(content)
	return service.projectTree.WriteFile(gitIgnorePath, []byte(updated))
}

func hasIdxDirectoryIgnoreRule(content string) bool {
	lines := strings.Split(content, "\n")
	for _, rawLine := range lines {
		line := strings.TrimSpace(rawLine)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "!") {
			continue
		}

		if normalizeIgnorePattern(line) == ".idx" {
			return true
		}
	}

	return false
}

func normalizeIgnorePattern(pattern string) string {
	normalized := strings.TrimSpace(pattern)
	normalized = strings.TrimPrefix(normalized, "/")
	normalized = strings.TrimSuffix(normalized, "/")
	normalized = strings.TrimSuffix(normalized, "/**")
	normalized = strings.TrimPrefix(normalized, "**/")
	return normalized
}

func appendIdxDirectoryIgnoreRule(content string) string {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return idxIgnoreLine
	}

	if strings.HasSuffix(content, "\n") {
		return content + idxIgnoreLine
	}

	return content + "\n" + idxIgnoreLine
}

func isMissingFileError(err error) bool {
	if os.IsNotExist(err) {
		return true
	}

	message := strings.ToLower(err.Error())
	return strings.Contains(message, "file not found") || strings.Contains(message, "no such file or directory")
}
