package filesystem

import (
	"context"
	"fmt"
	"os/exec"

	"idx/internal/core/ports"
)

type GitIgnoreMatcherFactory struct{}

type gitIgnoreMatcher struct {
	projectRoot string
	gitBin      string
}

// NewGitIgnoreMatcherFactory builds the adapter that evaluates .gitignore rules.
// Example: matcherFactory := NewGitIgnoreMatcherFactory().
func NewGitIgnoreMatcherFactory() GitIgnoreMatcherFactory {
	return GitIgnoreMatcherFactory{}
}

func (factory GitIgnoreMatcherFactory) New(projectRoot string) (ports.IgnoreMatcher, error) {
	gitBin, err := exec.LookPath("git")
	if err != nil {
		return nil, fmt.Errorf("git binary not found in PATH: %w", err)
	}

	matcher := gitIgnoreMatcher{projectRoot: projectRoot, gitBin: gitBin}
	if err := matcher.verifyGitBinary(); err != nil {
		return nil, err
	}

	return matcher, nil
}

func (matcher gitIgnoreMatcher) Matches(path string) (bool, error) {
	err := matcher.runCheckIgnore(path)
	if err == nil {
		return true, nil
	}

	if exitCode(err) == 1 {
		return false, nil
	}

	return false, fmt.Errorf("failed to evaluate ignore rules for path %q: got error %v, expected git check-ignore to exit with status 0 or 1", path, err)
}

func (matcher gitIgnoreMatcher) runCheckIgnore(path string) error {
	command := exec.CommandContext(context.Background(), matcher.gitBin, "-C", matcher.projectRoot, "check-ignore", "--no-index", "-q", path)
	return command.Run()
}

func (matcher gitIgnoreMatcher) verifyGitBinary() error {
	command := exec.CommandContext(context.Background(), matcher.gitBin, "-C", matcher.projectRoot, "rev-parse", "--git-dir")
	if err := command.Run(); err != nil {
		return fmt.Errorf("failed to validate git project %q: got error %v, expected a directory with a readable git repository", matcher.projectRoot, err)
	}

	return nil
}

func exitCode(err error) int {
	exitError, ok := err.(*exec.ExitError)
	if !ok {
		return -1
	}

	return exitError.ExitCode()
}
