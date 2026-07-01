package gitutil

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// resolveGitBinary returns the absolute path to the git executable, resolved
// once via PATH lookup so callers never hand a bare "git" to exec.CommandContext
// (avoids SonarQube go:S4036 -- PATH must only be searched through a controlled API).
func resolveGitBinary() (string, error) {
	gitPath, err := exec.LookPath("git")
	if err != nil {
		return "", fmt.Errorf("git executable not found in PATH: %w", err)
	}
	return gitPath, nil
}

// ChangedFilesSince returns the set of relative paths changed between ref and HEAD.
// Example: files, err := ChangedFilesSince("/repo", "HEAD~1").
func ChangedFilesSince(projectRoot, ref string) (map[string]bool, error) {
	gitPath, err := resolveGitBinary()
	if err != nil {
		return nil, err
	}
	cmd := exec.CommandContext( // #nosec G204 -- intentional git invocation; ref comes from validated CLI flag
		context.Background(), gitPath, "-C", projectRoot, "diff", "--name-only", ref+"...HEAD",
	)
	out, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return nil, fmt.Errorf("invalid git ref %q: %s", ref, strings.TrimSpace(string(exitErr.Stderr)))
		}
		return nil, fmt.Errorf("git diff failed for ref %q: %w", ref, err)
	}
	files := make(map[string]bool)
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line != "" {
			files[line] = true
		}
	}
	return files, nil
}

// CoChangedFiles returns a map of relative paths → commit count alongside relPath,
// plus the total number of commits that touched relPath.
// Uses two git calls: one to fetch commit SHAs, one to list files per commit.
// Example: coChanges, total, err := CoChangedFiles("/repo", "internal/features/search/service.go").
func CoChangedFiles(projectRoot, relPath string) (map[string]int, int, error) {
	shas, err := commitSHAs(projectRoot, relPath)
	if err != nil || len(shas) == 0 {
		return map[string]int{}, 0, err
	}
	raw, err := commitFiles(projectRoot, shas)
	if err != nil {
		return map[string]int{}, 0, err
	}
	return parseCoChangeFiles(raw, relPath, len(shas))
}

// commitSHAs returns SHA-1 hashes of commits that touched relPath.
func commitSHAs(projectRoot, relPath string) ([]string, error) {
	gitPath, err := resolveGitBinary()
	if err != nil {
		return nil, err
	}
	cmd := exec.CommandContext( // #nosec G204 -- intentional git invocation; relPath is a sanitized relative path
		context.Background(), gitPath, "-C", projectRoot, "log", "--format=%H", "--", relPath,
	)
	out, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return nil, fmt.Errorf("git log failed for %q: %s", relPath, strings.TrimSpace(string(exitErr.Stderr)))
		}
		return nil, fmt.Errorf("git log failed for %q: %w", relPath, err)
	}
	return strings.Fields(string(out)), nil
}

// commitFiles returns all file names touched by the given commits.
// git diff-tree outputs each SHA as a header line, then the file list.
func commitFiles(projectRoot string, shas []string) (string, error) {
	gitPath, err := resolveGitBinary()
	if err != nil {
		return "", err
	}
	// --root includes root commits (no parent) in the diff output.
	args := append([]string{"-C", projectRoot, "diff-tree", "--root", "-r", "--name-only"}, shas...) //nolint:gocritic
	cmd := exec.CommandContext(context.Background(), gitPath, args...)                               // #nosec G204
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git diff-tree failed: %w", err)
	}
	return string(out), nil
}

// parseCoChangeFiles counts how often each file co-appears with relPath across
// totalCommits commits. SHA header lines (40 hex chars) and relPath itself are skipped.
func parseCoChangeFiles(raw, relPath string, totalCommits int) (map[string]int, int, error) {
	coChanges := make(map[string]int)
	for _, line := range strings.Split(strings.TrimSpace(raw), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || isGitSHA(line) || line == relPath {
			continue
		}
		coChanges[line]++
	}
	return coChanges, totalCommits, nil
}

// isGitSHA returns true if s is a 40-character lowercase hexadecimal string (SHA-1).
func isGitSHA(s string) bool {
	if len(s) != 40 {
		return false
	}
	for _, c := range s {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}
