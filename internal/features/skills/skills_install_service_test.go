package skills_test

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"go.uber.org/mock/gomock"

	"idx/internal/features/skills"
	"idx/internal/features/skills/mocks"
)

func newTestService(t *testing.T) (*mocks.MockSkillsInstaller, *bytes.Buffer, *skills.SkillsInstallService) {
	t.Helper()
	ctrl := gomock.NewController(t)
	installer := mocks.NewMockSkillsInstaller(ctrl)
	out := &bytes.Buffer{}
	svc := skills.NewSkillsInstallService(installer, out)
	return installer, out, svc
}

func TestInstallSucceedsForValidEditors(t *testing.T) {
	for _, editor := range skills.SupportedEditors {
		t.Run(editor, func(t *testing.T) {
			installer, _, svc := newTestService(t)
			installer.EXPECT().Install(editor).Return(nil)

			if err := svc.Install(editor); err != nil {
				t.Fatalf("expected success for editor %q, got %v", editor, err)
			}
		})
	}
}

func TestInstallRejectsUnknownEditor(t *testing.T) {
	installer, _, svc := newTestService(t)
	installer.EXPECT().Install(gomock.Any()).Times(0)

	err := svc.Install("vim")
	if err == nil {
		t.Fatal("expected error for unknown editor, got nil")
	}
	if !strings.Contains(err.Error(), "vim") {
		t.Fatalf("expected error to mention %q, got %q", "vim", err.Error())
	}
}

func TestInstallRejectsEmptyEditor(t *testing.T) {
	installer, _, svc := newTestService(t)
	installer.EXPECT().Install(gomock.Any()).Times(0)

	if err := svc.Install(""); err == nil {
		t.Fatal("expected error for empty editor, got nil")
	}
}

func TestInstallPropagatesInstallerError(t *testing.T) {
	installer, _, svc := newTestService(t)
	installErr := errors.New("disk full")
	installer.EXPECT().Install("claude").Return(installErr)

	err := svc.Install("claude")
	if !errors.Is(err, installErr) {
		t.Fatalf("expected wrapped installer error, got %v", err)
	}
}

func TestInstallOutputContainsEditorName(t *testing.T) {
	installer, out, svc := newTestService(t)
	installer.EXPECT().Install("copilot").Return(nil)

	if err := svc.Install("copilot"); err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if !strings.Contains(out.String(), "copilot") {
		t.Fatalf("expected output to mention editor, got %q", out.String())
	}
}
