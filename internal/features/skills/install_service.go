package skills

import (
	"fmt"
	"io"

	
)

// subprocessWriter returns the writer to use for subprocess output.
// When verbose is false, all subprocess stdout/stderr goes to io.Discard.
func subprocessWriter(verbose bool, out io.Writer) io.Writer {
	if verbose {
		return out
	}
	return io.Discard
}

// SkillsInstallService installs idx skills for a specific editor by cloning
// the skills repository and running the install script.
// Example: svc := NewSkillsInstallService(installer, os.Stdout)
type SkillsInstallService struct {
	installer Installer
	out       io.Writer
}

// NewSkillsInstallService constructs the skills installation use case.
// Example: svc := NewSkillsInstallService(installer, os.Stdout)
func NewSkillsInstallService(installer Installer, out io.Writer) *SkillsInstallService {
	return &SkillsInstallService{installer: installer, out: out}
}

// Install validates the editor, clones the skills repository, runs the install script,
// and cleans up the temp directory. When verbose is false, subprocess output is suppressed.
// Returns a descriptive error on any failure.
func (s *SkillsInstallService) Install(editor string, verbose bool) error {
	if err := validateEditor(editor); err != nil {
		return err
	}

	subOut := subprocessWriter(verbose, s.out)

	writeHeader(s.out, editor)

	writeStep(s.out, 1, 3, "Cloning github.com/eltu/idx-skills...")
	tempDir, err := s.installer.CloneRepo(subOut)
	if err != nil {
		return fmt.Errorf("failed to clone idx-skills: %w", err)
	}
	defer s.installer.Cleanup(tempDir) //nolint:errcheck

	writeStep(s.out, 2, 3, fmt.Sprintf("Installing skills for %s...", editor))
	if err := s.installer.RunInstallScript(tempDir, editor, subOut); err != nil {
		return fmt.Errorf("install script failed for %q: %w", editor, err)
	}

	writeStep(s.out, 3, 3, "Cleaning up...")
	writeSuccess(s.out, editor)
	return nil
}
