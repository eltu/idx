package skills_test

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"

	"go.uber.org/mock/gomock"

	"idx/internal/core/services/skills"
	"idx/internal/core/services/skills/mocks"
)

const testTempDir = "/tmp/idx-skills-test"

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
			installer.EXPECT().CloneRepo(gomock.Any()).Return(testTempDir, nil)
			installer.EXPECT().RunInstallScript(testTempDir, editor, gomock.Any()).Return(nil)
			installer.EXPECT().Cleanup(testTempDir).Return(nil)

			if err := svc.Install(editor, false); err != nil {
				t.Fatalf("expected success for editor %q, got %v", editor, err)
			}
		})
	}
}

func TestInstallRejectsUnknownEditor(t *testing.T) {
	installer, _, svc := newTestService(t)
	installer.EXPECT().CloneRepo(gomock.Any()).Times(0)

	err := svc.Install("vim", false)
	if err == nil {
		t.Fatal("expected error for unknown editor, got nil")
	}
	if !strings.Contains(err.Error(), "vim") {
		t.Fatalf("expected error to mention %q, got %q", "vim", err.Error())
	}
}

func TestInstallRejectsEmptyEditor(t *testing.T) {
	installer, _, svc := newTestService(t)
	installer.EXPECT().CloneRepo(gomock.Any()).Times(0)

	err := svc.Install("", false)
	if err == nil {
		t.Fatal("expected error for empty editor, got nil")
	}
}

func TestInstallPropagatesCloneError(t *testing.T) {
	installer, _, svc := newTestService(t)
	cloneErr := errors.New("network unavailable")
	installer.EXPECT().CloneRepo(gomock.Any()).Return("", cloneErr)
	installer.EXPECT().Cleanup(gomock.Any()).Times(0)

	err := svc.Install("claude", false)
	if err == nil {
		t.Fatal("expected error propagation from CloneRepo, got nil")
	}
	if !errors.Is(err, cloneErr) {
		t.Fatalf("expected wrapped clone error, got %v", err)
	}
}

func TestInstallPropagatesScriptError(t *testing.T) {
	installer, _, svc := newTestService(t)
	scriptErr := errors.New("script exited with status 1")
	installer.EXPECT().CloneRepo(gomock.Any()).Return(testTempDir, nil)
	installer.EXPECT().RunInstallScript(testTempDir, "claude", gomock.Any()).Return(scriptErr)
	installer.EXPECT().Cleanup(testTempDir).Return(nil)

	err := svc.Install("claude", false)
	if err == nil {
		t.Fatal("expected error propagation from RunInstallScript, got nil")
	}
	if !errors.Is(err, scriptErr) {
		t.Fatalf("expected wrapped script error, got %v", err)
	}
}

func TestInstallCleansUpEvenOnScriptFailure(t *testing.T) {
	installer, _, svc := newTestService(t)
	installer.EXPECT().CloneRepo(gomock.Any()).Return(testTempDir, nil)
	installer.EXPECT().RunInstallScript(testTempDir, "cursor", gomock.Any()).Return(errors.New("oops"))
	installer.EXPECT().Cleanup(testTempDir).Return(nil).Times(1)

	_ = svc.Install("cursor", false)
}

func TestInstallVerbosePassesWriterToSubprocesses(t *testing.T) {
	installer, out, svc := newTestService(t)
	installer.EXPECT().CloneRepo(out).Return(testTempDir, nil)
	installer.EXPECT().RunInstallScript(testTempDir, "claude", out).Return(nil)
	installer.EXPECT().Cleanup(testTempDir).Return(nil)

	if err := svc.Install("claude", true); err != nil {
		t.Fatalf("expected success, got %v", err)
	}
}

func TestInstallSilentPassesDiscardToSubprocesses(t *testing.T) {
	installer, _, svc := newTestService(t)
	installer.EXPECT().CloneRepo(io.Discard).Return(testTempDir, nil)
	installer.EXPECT().RunInstallScript(testTempDir, "claude", io.Discard).Return(nil)
	installer.EXPECT().Cleanup(testTempDir).Return(nil)

	if err := svc.Install("claude", false); err != nil {
		t.Fatalf("expected success, got %v", err)
	}
}

func TestInstallOutputContainsEditorName(t *testing.T) {
	installer, out, svc := newTestService(t)
	installer.EXPECT().CloneRepo(gomock.Any()).Return(testTempDir, nil)
	installer.EXPECT().RunInstallScript(testTempDir, "copilot", gomock.Any()).Return(nil)
	installer.EXPECT().Cleanup(testTempDir).Return(nil)

	if err := svc.Install("copilot", false); err != nil {
		t.Fatalf("expected success, got %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "copilot") {
		t.Fatalf("expected output to mention editor, got %q", output)
	}
}
