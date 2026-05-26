package skills_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"idx/internal/features/skills"
)

func newTestEmbedInstaller(t *testing.T) (*skills.EmbedSkillsInstaller, string) {
	t.Helper()
	tmpDir := t.TempDir()
	installer := skills.NewEmbedSkillsInstallerWithDeps(
		func() (string, error) { return tmpDir, nil },
		os.MkdirAll,
		os.WriteFile,
		os.ReadFile,
	)
	return installer, tmpDir
}

func TestInstallCopiesSkillFilesToEditorDir(t *testing.T) {
	for _, editor := range []string{"claude", "cursor", "copilot"} {
		t.Run(editor, func(t *testing.T) {
			installer, homeDir := newTestEmbedInstaller(t)
			if err := installer.Install(editor); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			targetDir := filepath.Join(homeDir, "."+editor, "skills", "idx-search")
			if _, err := os.Stat(filepath.Join(targetDir, "SKILL.md")); err != nil {
				t.Fatalf("expected SKILL.md at %q: %v", targetDir, err)
			}
			if _, err := os.Stat(filepath.Join(targetDir, "references", "idx-commands.md")); err != nil {
				t.Fatalf("expected references/idx-commands.md at %q: %v", targetDir, err)
			}
		})
	}
}

func TestInstallCreatesClaudeSettingsWithPermission(t *testing.T) {
	installer, homeDir := newTestEmbedInstaller(t)
	if err := installer.Install("claude"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(homeDir, ".claude", "settings.json"))
	if err != nil {
		t.Fatalf("expected settings.json to be created: %v", err)
	}
	if !strings.Contains(string(data), "Bash(idx *)") {
		t.Fatalf("expected permission in settings.json, got %q", string(data))
	}
}

func TestInstallDoesNotDuplicateClaudePermission(t *testing.T) {
	installer, homeDir := newTestEmbedInstaller(t)
	_ = installer.Install("claude")
	if err := installer.Install("claude"); err != nil {
		t.Fatalf("second install failed: %v", err)
	}
	data, _ := os.ReadFile(filepath.Join(homeDir, ".claude", "settings.json"))
	if count := strings.Count(string(data), "Bash(idx *)"); count != 1 {
		t.Fatalf("expected 1 permission entry, found %d in %q", count, string(data))
	}
}

func TestInstallPreservesExistingClaudeSettingsFields(t *testing.T) {
	installer, homeDir := newTestEmbedInstaller(t)
	settingsDir := filepath.Join(homeDir, ".claude")
	_ = os.MkdirAll(settingsDir, 0750)
	existing := `{"mcpServers":{"server1":{}},"permissions":{"allow":[]}}`
	_ = os.WriteFile(filepath.Join(settingsDir, "settings.json"), []byte(existing), 0600)

	if err := installer.Install("claude"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	data, _ := os.ReadFile(filepath.Join(settingsDir, "settings.json"))
	if !strings.Contains(string(data), "mcpServers") {
		t.Fatalf("expected existing fields to be preserved, got %q", string(data))
	}
	if !strings.Contains(string(data), "Bash(idx *)") {
		t.Fatalf("expected permission to be added, got %q", string(data))
	}
}

func TestInstallDoesNotWriteClaudeSettingsForOtherEditors(t *testing.T) {
	for _, editor := range []string{"cursor", "copilot"} {
		t.Run(editor, func(t *testing.T) {
			installer, homeDir := newTestEmbedInstaller(t)
			if err := installer.Install(editor); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			settingsPath := filepath.Join(homeDir, ".claude", "settings.json")
			if _, err := os.Stat(settingsPath); !os.IsNotExist(err) {
				t.Fatalf("expected no settings.json for editor %q, got err %v", editor, err)
			}
		})
	}
}

func TestInstallReturnsErrorWhenHomeDirFails(t *testing.T) {
	homeDirErr := errors.New("home dir unavailable")
	installer := skills.NewEmbedSkillsInstallerWithDeps(
		func() (string, error) { return "", homeDirErr },
		os.MkdirAll, os.WriteFile, os.ReadFile,
	)
	if err := installer.Install("claude"); !errors.Is(err, homeDirErr) {
		t.Fatalf("expected homeDir error, got %v", err)
	}
}

func TestInstallReturnsErrorWhenMkdirAllFails(t *testing.T) {
	tmpDir := t.TempDir()
	mkdirErr := errors.New("permission denied")
	installer := skills.NewEmbedSkillsInstallerWithDeps(
		func() (string, error) { return tmpDir, nil },
		func(string, os.FileMode) error { return mkdirErr },
		os.WriteFile, os.ReadFile,
	)
	if err := installer.Install("cursor"); !errors.Is(err, mkdirErr) {
		t.Fatalf("expected mkdirAll error, got %v", err)
	}
}

func TestInstallReturnsErrorWhenWriteFileFails(t *testing.T) {
	tmpDir := t.TempDir()
	writeErr := errors.New("disk full")
	installer := skills.NewEmbedSkillsInstallerWithDeps(
		func() (string, error) { return tmpDir, nil },
		os.MkdirAll,
		func(string, []byte, os.FileMode) error { return writeErr },
		os.ReadFile,
	)
	if err := installer.Install("cursor"); !errors.Is(err, writeErr) {
		t.Fatalf("expected writeFile error, got %v", err)
	}
}

func TestNewEmbedSkillsInstallerIsNotNil(t *testing.T) {
	if skills.NewEmbedSkillsInstaller() == nil {
		t.Fatal("expected non-nil installer from NewEmbedSkillsInstaller")
	}
}

func TestInstallReturnsErrorForMalformedClaudeSettingsJSON(t *testing.T) {
	installer, homeDir := newTestEmbedInstaller(t)
	settingsDir := filepath.Join(homeDir, ".claude")
	_ = os.MkdirAll(settingsDir, 0750)
	_ = os.WriteFile(filepath.Join(settingsDir, "settings.json"), []byte("not valid json"), 0600)

	if err := installer.Install("claude"); err == nil {
		t.Fatal("expected error for malformed settings.json, got nil")
	}
}

func TestInstallReturnsErrorWhenReadFileFailsWithNonNotExistError(t *testing.T) {
	tmpDir := t.TempDir()
	readErr := errors.New("permission denied")
	installer := skills.NewEmbedSkillsInstallerWithDeps(
		func() (string, error) { return tmpDir, nil },
		os.MkdirAll,
		os.WriteFile,
		func(path string) ([]byte, error) {
			if strings.HasSuffix(path, "settings.json") {
				return nil, readErr
			}
			return os.ReadFile(path)
		},
	)
	if err := installer.Install("claude"); !errors.Is(err, readErr) {
		t.Fatalf("expected readFile error, got %v", err)
	}
}

func TestInstallHandlesNonMapPermissionsInClaudeSettings(t *testing.T) {
	installer, homeDir := newTestEmbedInstaller(t)
	settingsDir := filepath.Join(homeDir, ".claude")
	_ = os.MkdirAll(settingsDir, 0750)
	_ = os.WriteFile(filepath.Join(settingsDir, "settings.json"), []byte(`{"permissions":"not-a-map"}`), 0600)

	if err := installer.Install("claude"); err != nil {
		t.Fatalf("unexpected error for non-map permissions: %v", err)
	}
	data, _ := os.ReadFile(filepath.Join(settingsDir, "settings.json"))
	if !strings.Contains(string(data), "Bash(idx *)") {
		t.Fatalf("expected permission to be added despite non-map permissions, got %q", string(data))
	}
}

func TestInstallHandlesNonArrayAllowInClaudeSettings(t *testing.T) {
	installer, homeDir := newTestEmbedInstaller(t)
	settingsDir := filepath.Join(homeDir, ".claude")
	_ = os.MkdirAll(settingsDir, 0750)
	_ = os.WriteFile(filepath.Join(settingsDir, "settings.json"), []byte(`{"permissions":{"allow":"not-an-array"}}`), 0600)

	if err := installer.Install("claude"); err != nil {
		t.Fatalf("unexpected error for non-array allow: %v", err)
	}
	data, _ := os.ReadFile(filepath.Join(settingsDir, "settings.json"))
	if !strings.Contains(string(data), "Bash(idx *)") {
		t.Fatalf("expected permission to be added despite non-array allow, got %q", string(data))
	}
}

func TestInstallReturnsErrorWhenClaudeSettingsDirCreationFails(t *testing.T) {
	tmpDir := t.TempDir()
	mkdirErr := errors.New("cannot create .claude directory")
	installer := skills.NewEmbedSkillsInstallerWithDeps(
		func() (string, error) { return tmpDir, nil },
		func(path string, mode os.FileMode) error {
			// Fail only when saveClaudeSettings tries to create the .claude dir itself.
			if filepath.Base(path) == ".claude" {
				return mkdirErr
			}
			return os.MkdirAll(path, mode)
		},
		os.WriteFile, os.ReadFile,
	)
	if err := installer.Install("claude"); !errors.Is(err, mkdirErr) {
		t.Fatalf("expected mkdirAll error from saveClaudeSettings, got %v", err)
	}
}

func TestInstallReturnsErrorWhenClaudeSettingsWriteFails(t *testing.T) {
	tmpDir := t.TempDir()
	writeErr := errors.New("settings write failed")
	installer := skills.NewEmbedSkillsInstallerWithDeps(
		func() (string, error) { return tmpDir, nil },
		os.MkdirAll,
		func(path string, data []byte, mode os.FileMode) error {
			if strings.HasSuffix(path, "settings.json") {
				return writeErr
			}
			return os.WriteFile(path, data, mode)
		},
		os.ReadFile,
	)
	if err := installer.Install("claude"); !errors.Is(err, writeErr) {
		t.Fatalf("expected writeFile error from saveClaudeSettings, got %v", err)
	}
}
