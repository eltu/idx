package skills

import "io"

// Installer provides operations needed to install skills.
type Installer interface {
	CloneRepo(out io.Writer) (string, error)
	RunInstallScript(dir, editor string, out io.Writer) error
	Cleanup(tempDir string) error
}
