package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// runSkills executes `idx skills <args>` and returns captured output and error.
func runSkills(t *testing.T, args ...string) (string, error) {
	t.Helper()
	var buf bytes.Buffer
	err := run(append([]string{"idx", "skills"}, args...), &buf)
	return buf.String(), err
}

// tempHomeDir creates an isolated home directory for skills installation tests.
// Skills use os.UserHomeDir() which reads $HOME, so setting HOME redirects all writes.
func tempHomeDir(t *testing.T) string {
	t.Helper()
	home, err := os.MkdirTemp("/tmp", "idx-skills-home")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(home) })
	t.Setenv("HOME", home)
	return home
}

// --- idx skills install (validation, no server needed) ---

func TestCLI_SkillsInstall_MissingEditor_ReturnsError(t *testing.T) {
	// Arrange — no editor argument
	projectRoot := newTestProject(t)
	t.Chdir(projectRoot)

	// Act
	_, err := runSkills(t, "install")

	// Assert — writeSkillsMissingEditorError prints to stdout; RunE returns fmt.Errorf("")
	require.Error(t, err)
}

func TestCLI_SkillsInstall_UnknownEditor_ReturnsError(t *testing.T) {
	t.Parallel()

	// Act
	_, err := runSkills(t, "install", "vscode")

	// Assert
	require.Error(t, err)
	assert.ErrorContains(t, err, "unsupported editor")
}

// --- idx skills (no subcommand — shows custom help) ---

func TestCLI_Skills_NoSubcommand_ReturnsNoError(t *testing.T) {
	t.Parallel()

	// Act — idx skills with no args triggers the custom help function
	_, err := runSkills(t)

	// Assert — custom help writes to cmd.OutOrStdout() (os.Stdout), not captured buffer.
	// We verify the command exits cleanly.
	require.NoError(t, err)
}

// --- idx skills install <editor> (full install, redirected HOME) ---

func TestCLI_SkillsInstall_Claude_CreatesSkillFiles(t *testing.T) {
	// Arrange
	home := tempHomeDir(t)
	projectRoot := newTestProject(t)
	t.Chdir(projectRoot)

	// Act
	out, err := runSkills(t, "install", "claude")

	// Assert
	require.NoError(t, err)
	assert.Contains(t, out, "claude")

	skillsDir := filepath.Join(home, ".claude", "skills", "idx")
	entries, statErr := os.ReadDir(skillsDir)
	require.NoError(t, statErr, "expected .claude/skills/idx/ to be created")
	assert.NotEmpty(t, entries, "expected skill files to be written")
}

func TestCLI_SkillsInstall_Claude_UpdatesSettingsJson(t *testing.T) {
	// Arrange
	home := tempHomeDir(t)
	projectRoot := newTestProject(t)
	t.Chdir(projectRoot)

	// Act
	_, err := runSkills(t, "install", "claude")
	require.NoError(t, err)

	// Assert — settings.json must contain the Bash(idx *) permission
	settingsPath := filepath.Join(home, ".claude", "settings.json")
	data, readErr := os.ReadFile(settingsPath)
	require.NoError(t, readErr, "expected settings.json to be created for claude")

	var settings map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &settings), "settings.json must be valid JSON")

	raw, ok := settings["permissions"].(map[string]interface{})
	require.True(t, ok, "expected 'permissions' object in settings.json")
	allow, ok := raw["allow"].([]interface{})
	require.True(t, ok, "expected 'allow' list in permissions")
	found := false
	for _, entry := range allow {
		if s, ok := entry.(string); ok && s == "Bash(idx *)" {
			found = true
			break
		}
	}
	assert.True(t, found, "expected 'Bash(idx *)' in settings.json allow list")
}

func TestCLI_SkillsInstall_Claude_WhenSettingsJsonExists_PreservesExistingPermissions(t *testing.T) {
	// Arrange — pre-populate settings.json with an existing permission
	home := tempHomeDir(t)
	projectRoot := newTestProject(t)
	t.Chdir(projectRoot)

	claudeDir := filepath.Join(home, ".claude")
	require.NoError(t, os.MkdirAll(claudeDir, 0o750))
	existing := `{"permissions":{"allow":["Bash(git *)"]}}` + "\n"
	require.NoError(t, os.WriteFile(filepath.Join(claudeDir, "settings.json"), []byte(existing), 0o600))

	// Act
	_, err := runSkills(t, "install", "claude")
	require.NoError(t, err)

	// Assert — both permissions present
	data, readErr := os.ReadFile(filepath.Join(claudeDir, "settings.json"))
	require.NoError(t, readErr)
	assert.Contains(t, string(data), "Bash(git *)")
	assert.Contains(t, string(data), "Bash(idx *)")
}

func TestCLI_SkillsInstall_Copilot_CreatesSkillFiles(t *testing.T) {
	// Arrange
	home := tempHomeDir(t)
	projectRoot := newTestProject(t)
	t.Chdir(projectRoot)

	// Act
	out, err := runSkills(t, "install", "copilot")

	// Assert
	require.NoError(t, err)
	assert.Contains(t, out, "copilot")

	skillsDir := filepath.Join(home, ".copilot", "skills", "idx")
	entries, statErr := os.ReadDir(skillsDir)
	require.NoError(t, statErr, "expected .copilot/skills/idx/ to be created")
	assert.NotEmpty(t, entries)
}

func TestCLI_SkillsInstall_Cursor_CreatesSkillFiles(t *testing.T) {
	// Arrange
	home := tempHomeDir(t)
	projectRoot := newTestProject(t)
	t.Chdir(projectRoot)

	// Act
	out, err := runSkills(t, "install", "cursor")

	// Assert
	require.NoError(t, err)
	assert.Contains(t, out, "cursor")

	skillsDir := filepath.Join(home, ".cursor", "skills", "idx")
	entries, statErr := os.ReadDir(skillsDir)
	require.NoError(t, statErr, "expected .cursor/skills/idx/ to be created")
	assert.NotEmpty(t, entries)
}

func TestCLI_SkillsInstall_IsIdempotent_SecondInstallSucceeds(t *testing.T) {
	// Arrange
	home := tempHomeDir(t)
	projectRoot := newTestProject(t)
	t.Chdir(projectRoot)

	// Act — install twice
	_, err := runSkills(t, "install", "claude")
	require.NoError(t, err, "first install must succeed")

	_, err = runSkills(t, "install", "claude")
	require.NoError(t, err, "second install must be idempotent")

	// Assert — single permission entry (no duplication)
	data, _ := os.ReadFile(filepath.Join(home, ".claude", "settings.json"))
	count := 0
	for _, b := range splitJSON(string(data), "Bash(idx *)") {
		if b {
			count++
		}
	}
	assert.Equal(t, 1, count, "expected exactly one Bash(idx *) entry after two installs")
}

// splitJSON counts how many times needle appears as a string value in jsonStr.
// Used to verify no duplicate entries in the allow list.
func splitJSON(jsonStr, needle string) []bool {
	var results []bool
	for i := 0; i < len(jsonStr)-len(needle)+1; i++ {
		if jsonStr[i:i+len(needle)] == needle {
			results = append(results, true)
		}
	}
	return results
}
