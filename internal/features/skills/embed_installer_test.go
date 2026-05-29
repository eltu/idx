package skills_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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

func TestInstall_CopiesSkillFilesToEditorDir(t *testing.T) {
	t.Parallel()

	for _, editor := range []string{"claude", "cursor", "copilot"} {
		editor := editor
		t.Run(editor, func(t *testing.T) {
			t.Parallel()

			// Arrange
			installer, homeDir := newTestEmbedInstaller(t)

			// Act
			require.NoError(t, installer.Install(editor))

			// Assert
			targetDir := filepath.Join(homeDir, "."+editor, "skills", "idx-search")
			_, err := os.Stat(filepath.Join(targetDir, "SKILL.md"))
			require.NoError(t, err, "expected SKILL.md at %q", targetDir)
			_, err = os.Stat(filepath.Join(targetDir, "references", "idx-commands.md"))
			require.NoError(t, err, "expected references/idx-commands.md at %q", targetDir)
		})
	}
}

func TestInstall_CreatesClaudeSettingsWithPermission(t *testing.T) {
	t.Parallel()

	// Arrange
	installer, homeDir := newTestEmbedInstaller(t)

	// Act
	require.NoError(t, installer.Install("claude"))

	// Assert
	data, err := os.ReadFile(filepath.Join(homeDir, ".claude", "settings.json"))
	require.NoError(t, err)
	assert.Contains(t, string(data), "Bash(idx *)")
}

func TestInstall_DoesNotDuplicateClaudePermission(t *testing.T) {
	t.Parallel()

	// Arrange
	installer, homeDir := newTestEmbedInstaller(t)
	require.NoError(t, installer.Install("claude"))

	// Act — install again
	require.NoError(t, installer.Install("claude"))

	// Assert
	data, _ := os.ReadFile(filepath.Join(homeDir, ".claude", "settings.json"))
	assert.Equal(t, 1, strings.Count(string(data), "Bash(idx *)"), "expected exactly 1 permission entry")
}

func TestInstall_PreservesExistingClaudeSettingsFields(t *testing.T) {
	t.Parallel()

	// Arrange
	installer, homeDir := newTestEmbedInstaller(t)
	settingsDir := filepath.Join(homeDir, ".claude")
	require.NoError(t, os.MkdirAll(settingsDir, 0750))
	existing := `{"mcpServers":{"server1":{}},"permissions":{"allow":[]}}`
	require.NoError(t, os.WriteFile(filepath.Join(settingsDir, "settings.json"), []byte(existing), 0600))

	// Act
	require.NoError(t, installer.Install("claude"))

	// Assert
	data, _ := os.ReadFile(filepath.Join(settingsDir, "settings.json"))
	assert.Contains(t, string(data), "mcpServers")
	assert.Contains(t, string(data), "Bash(idx *)")
}

func TestInstall_DoesNotWriteClaudeSettingsForOtherEditors(t *testing.T) {
	t.Parallel()

	for _, editor := range []string{"cursor", "copilot"} {
		editor := editor
		t.Run(editor, func(t *testing.T) {
			t.Parallel()

			// Arrange
			installer, homeDir := newTestEmbedInstaller(t)

			// Act
			require.NoError(t, installer.Install(editor))

			// Assert
			settingsPath := filepath.Join(homeDir, ".claude", "settings.json")
			_, err := os.Stat(settingsPath)
			assert.True(t, os.IsNotExist(err), "expected no settings.json for editor %q", editor)
		})
	}
}

func TestInstall_ReturnsErrorWhenHomeDirFails(t *testing.T) {
	t.Parallel()

	// Arrange
	homeDirErr := errors.New("home dir unavailable")
	installer := skills.NewEmbedSkillsInstallerWithDeps(
		func() (string, error) { return "", homeDirErr },
		os.MkdirAll, os.WriteFile, os.ReadFile,
	)

	// Act
	err := installer.Install("claude")

	// Assert
	require.ErrorIs(t, err, homeDirErr)
}

func TestInstall_ReturnsErrorWhenMkdirAllFails(t *testing.T) {
	t.Parallel()

	// Arrange
	tmpDir := t.TempDir()
	mkdirErr := errors.New("permission denied")
	installer := skills.NewEmbedSkillsInstallerWithDeps(
		func() (string, error) { return tmpDir, nil },
		func(string, os.FileMode) error { return mkdirErr },
		os.WriteFile, os.ReadFile,
	)

	// Act
	err := installer.Install("cursor")

	// Assert
	require.ErrorIs(t, err, mkdirErr)
}

func TestInstall_ReturnsErrorWhenWriteFileFails(t *testing.T) {
	t.Parallel()

	// Arrange
	tmpDir := t.TempDir()
	writeErr := errors.New("disk full")
	installer := skills.NewEmbedSkillsInstallerWithDeps(
		func() (string, error) { return tmpDir, nil },
		os.MkdirAll,
		func(string, []byte, os.FileMode) error { return writeErr },
		os.ReadFile,
	)

	// Act
	err := installer.Install("cursor")

	// Assert
	require.ErrorIs(t, err, writeErr)
}

func TestNewEmbedSkillsInstaller_ReturnsNonNil(t *testing.T) {
	t.Parallel()

	// Act & Assert
	assert.NotNil(t, skills.NewEmbedSkillsInstaller())
}

func TestInstall_ReturnsErrorForMalformedClaudeSettingsJSON(t *testing.T) {
	t.Parallel()

	// Arrange
	installer, homeDir := newTestEmbedInstaller(t)
	settingsDir := filepath.Join(homeDir, ".claude")
	require.NoError(t, os.MkdirAll(settingsDir, 0750))
	require.NoError(t, os.WriteFile(filepath.Join(settingsDir, "settings.json"), []byte("not valid json"), 0600))

	// Act
	err := installer.Install("claude")

	// Assert
	require.Error(t, err)
}

func TestInstall_ReturnsErrorWhenReadFileFailsWithNonNotExistError(t *testing.T) {
	t.Parallel()

	// Arrange
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

	// Act
	err := installer.Install("claude")

	// Assert
	require.ErrorIs(t, err, readErr)
}

func TestInstall_HandlesNonMapPermissionsInClaudeSettings(t *testing.T) {
	t.Parallel()

	// Arrange
	installer, homeDir := newTestEmbedInstaller(t)
	settingsDir := filepath.Join(homeDir, ".claude")
	require.NoError(t, os.MkdirAll(settingsDir, 0750))
	require.NoError(t, os.WriteFile(filepath.Join(settingsDir, "settings.json"), []byte(`{"permissions":"not-a-map"}`), 0600))

	// Act
	require.NoError(t, installer.Install("claude"))

	// Assert
	data, _ := os.ReadFile(filepath.Join(settingsDir, "settings.json"))
	assert.Contains(t, string(data), "Bash(idx *)")
}

func TestInstall_HandlesNonArrayAllowInClaudeSettings(t *testing.T) {
	t.Parallel()

	// Arrange
	installer, homeDir := newTestEmbedInstaller(t)
	settingsDir := filepath.Join(homeDir, ".claude")
	require.NoError(t, os.MkdirAll(settingsDir, 0750))
	require.NoError(t, os.WriteFile(filepath.Join(settingsDir, "settings.json"), []byte(`{"permissions":{"allow":"not-an-array"}}`), 0600))

	// Act
	require.NoError(t, installer.Install("claude"))

	// Assert
	data, _ := os.ReadFile(filepath.Join(settingsDir, "settings.json"))
	assert.Contains(t, string(data), "Bash(idx *)")
}

func TestInstall_ReturnsErrorWhenClaudeSettingsDirCreationFails(t *testing.T) {
	t.Parallel()

	// Arrange
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

	// Act
	err := installer.Install("claude")

	// Assert
	require.ErrorIs(t, err, mkdirErr)
}

func TestInstall_ReturnsErrorWhenClaudeSettingsWriteFails(t *testing.T) {
	t.Parallel()

	// Arrange
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

	// Act
	err := installer.Install("claude")

	// Assert
	require.ErrorIs(t, err, writeErr)
}
