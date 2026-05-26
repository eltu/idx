package skills

import (
	"fmt"
	"io"
)

// SkillsInstallService installs idx skills for a specific editor by copying
// bundled skill files to the editor's skills directory.
// Example: svc := NewSkillsInstallService(installer, os.Stdout).
type SkillsInstallService struct {
	installer Installer
	out       io.Writer
}

// NewSkillsInstallService constructs the skills installation use case.
// Example: svc := NewSkillsInstallService(installer, os.Stdout).
func NewSkillsInstallService(installer Installer, out io.Writer) *SkillsInstallService {
	return &SkillsInstallService{installer: installer, out: out}
}

// Install validates the editor, copies the bundled skill files to the editor's
// skills directory, and writes progress output.
func (s *SkillsInstallService) Install(editor string) error {
	if err := validateEditor(editor); err != nil {
		return err
	}
	writeHeader(s.out, editor)
	writeStep(s.out, 1, 2, fmt.Sprintf("Installing skills for %s...", displayName(editor)))
	if err := s.installer.Install(editor); err != nil {
		return fmt.Errorf("failed to install skills for %q: %w", editor, err)
	}
	writeSuccess(s.out, editor)
	return nil
}
