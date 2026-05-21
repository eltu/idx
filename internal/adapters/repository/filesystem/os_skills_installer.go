package filesystem

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
)

const skillsRepoURL = "https://github.com/eltu/idx-skills"

// OSSkillsInstaller implements ports.SkillsInstaller using local git and shell execution.
type OSSkillsInstaller struct{}

// NewOSSkillsInstaller creates an installer that uses the system git binary and shell.
// Example: installer := filesystem.NewOSSkillsInstaller()
func NewOSSkillsInstaller() *OSSkillsInstaller {
	return &OSSkillsInstaller{}
}

// CloneRepo clones skillsRepoURL into a fresh temp directory, streaming git output to out.
func (i *OSSkillsInstaller) CloneRepo(out io.Writer) (string, error) {
	gitBin, err := exec.LookPath("git")
	if err != nil {
		return "", fmt.Errorf("git binary not found in PATH: %w", err)
	}

	tempDir, err := os.MkdirTemp("", "idx-skills-*")
	if err != nil {
		return "", fmt.Errorf("failed to create temp directory: %w", err)
	}

	cmd := exec.Command(gitBin, "clone", skillsRepoURL, tempDir)
	cmd.Stdout = out
	cmd.Stderr = out
	if err := cmd.Run(); err != nil {
		os.RemoveAll(tempDir) //nolint:errcheck
		return "", fmt.Errorf("git clone failed: %w", err)
	}

	return tempDir, nil
}

// RunInstallScript executes install-skills.sh <editor> inside dir,
// streaming stdout and stderr to out for real-time display.
func (i *OSSkillsInstaller) RunInstallScript(dir, editor string, out io.Writer) error {
	cmd := exec.Command(filepath.Join(dir, "install-skills.sh"), editor)
	cmd.Dir = dir
	cmd.Stdout = out
	cmd.Stderr = out
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("install-skills.sh exited with error: %w", err)
	}
	return nil
}

// Cleanup removes tempDir and all its contents.
func (i *OSSkillsInstaller) Cleanup(tempDir string) error {
	if err := os.RemoveAll(tempDir); err != nil {
		return fmt.Errorf("failed to remove temp directory %q: %w", tempDir, err)
	}
	return nil
}
