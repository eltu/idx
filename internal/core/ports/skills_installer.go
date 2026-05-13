package ports

import "io"

// SkillsInstaller encapsulates cloning the idx-skills repository and running the install script.
// Three separate methods map to the three UI steps and allow granular mocking in tests.
type SkillsInstaller interface {
	// CloneRepo clones the idx-skills repository into a new temp directory,
	// writing git output to out (use io.Discard to suppress).
	// Returns the temp directory path for subsequent steps.
	// Example: dir, err := installer.CloneRepo(os.Stdout)
	CloneRepo(out io.Writer) (tempDir string, err error)

	// RunInstallScript executes ./install-skills.sh <editor> inside dir,
	// streaming stdout and stderr to out in real-time.
	// Example: err := installer.RunInstallScript(dir, "claude", os.Stdout)
	RunInstallScript(dir, editor string, out io.Writer) error

	// Cleanup removes tempDir and all its contents.
	// Example: err := installer.Cleanup(dir)
	Cleanup(tempDir string) error
}
