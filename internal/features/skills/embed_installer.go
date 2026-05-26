package skills

import (
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

const (
	assetSkillDir    = "assets/idx-search"
	skillName        = "idx-search"
	claudePermission = "Bash(idx *)"
	editorClaude     = "claude"
)

//go:embed assets
var skillsFS embed.FS

// EmbedSkillsInstaller copies bundled skill files from the embedded FS to the
// editor's skills directory. No network or git binary required.
// Example: installer := NewEmbedSkillsInstaller().
type EmbedSkillsInstaller struct {
	homeDir   func() (string, error)
	mkdirAll  func(string, os.FileMode) error
	writeFile func(string, []byte, os.FileMode) error
	readFile  func(string) ([]byte, error)
}

// NewEmbedSkillsInstaller creates an installer that uses OS defaults.
// Example: installer := NewEmbedSkillsInstaller().
func NewEmbedSkillsInstaller() *EmbedSkillsInstaller {
	return NewEmbedSkillsInstallerWithDeps(os.UserHomeDir, os.MkdirAll, os.WriteFile, os.ReadFile)
}

// NewEmbedSkillsInstallerWithDeps creates an installer with injected OS dependencies for testing.
func NewEmbedSkillsInstallerWithDeps(
	homeDir func() (string, error),
	mkdirAll func(string, os.FileMode) error,
	writeFile func(string, []byte, os.FileMode) error,
	readFile func(string) ([]byte, error),
) *EmbedSkillsInstaller {
	return &EmbedSkillsInstaller{
		homeDir:   homeDir,
		mkdirAll:  mkdirAll,
		writeFile: writeFile,
		readFile:  readFile,
	}
}

// Install copies bundled skill files to the editor's skills directory and,
// for Claude, ensures the idx permission entry is present in settings.json.
func (i *EmbedSkillsInstaller) Install(editor string) error {
	homeDir, err := i.homeDir()
	if err != nil {
		return fmt.Errorf("failed to resolve home directory: %w", err)
	}
	if err := i.copySkillFiles(editorSkillsDir(editor, homeDir)); err != nil {
		return err
	}
	if editor == editorClaude {
		return i.configureClaude(homeDir)
	}
	return nil
}

func editorSkillsDir(editor, homeDir string) string {
	return filepath.Join(homeDir, "."+editor, "skills", skillName)
}

func (i *EmbedSkillsInstaller) copySkillFiles(targetDir string) error {
	return fs.WalkDir(skillsFS, assetSkillDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		return i.copyEntry(path, d, targetDir)
	})
}

func (i *EmbedSkillsInstaller) copyEntry(srcPath string, d fs.DirEntry, targetDir string) error {
	relPath, _ := filepath.Rel(assetSkillDir, srcPath)
	destPath := filepath.Join(targetDir, relPath)
	if d.IsDir() {
		return i.mkdirAll(destPath, 0750)
	}
	data, err := skillsFS.ReadFile(srcPath)
	if err != nil {
		return fmt.Errorf("failed to read embedded file %q: %w", srcPath, err)
	}
	return i.writeFile(destPath, data, 0600)
}

func (i *EmbedSkillsInstaller) configureClaude(homeDir string) error {
	settingsPath := filepath.Join(homeDir, ".claude", "settings.json")
	settings, err := i.loadClaudeSettings(settingsPath)
	if err != nil {
		return err
	}
	if hasClaudePermission(settings) {
		return nil
	}
	addClaudePermission(settings)
	return i.saveClaudeSettings(settingsPath, settings)
}

func (i *EmbedSkillsInstaller) loadClaudeSettings(path string) (map[string]interface{}, error) {
	data, err := i.readFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return defaultClaudeSettings(), nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read %q: %w", path, err)
	}
	var settings map[string]interface{}
	if err := json.Unmarshal(data, &settings); err != nil {
		return nil, fmt.Errorf("failed to parse %q: %w", path, err)
	}
	return settings, nil
}

func defaultClaudeSettings() map[string]interface{} {
	return map[string]interface{}{
		"permissions": map[string]interface{}{"allow": []interface{}{}},
	}
}

func hasClaudePermission(settings map[string]interface{}) bool {
	for _, p := range claudeAllowList(settings) {
		if p == claudePermission {
			return true
		}
	}
	return false
}

func addClaudePermission(settings map[string]interface{}) {
	perms := claudePermissionsMap(settings)
	perms["allow"] = append(claudeAllowList(settings), claudePermission)
	settings["permissions"] = perms
}

func claudePermissionsMap(settings map[string]interface{}) map[string]interface{} {
	if p, ok := settings["permissions"].(map[string]interface{}); ok {
		return p
	}
	return map[string]interface{}{}
}

func claudeAllowList(settings map[string]interface{}) []interface{} {
	if allow, ok := claudePermissionsMap(settings)["allow"].([]interface{}); ok {
		return allow
	}
	return []interface{}{}
}

func (i *EmbedSkillsInstaller) saveClaudeSettings(path string, settings map[string]interface{}) error {
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal settings: %w", err)
	}
	if err := i.mkdirAll(filepath.Dir(path), 0750); err != nil {
		return fmt.Errorf("failed to create directory for %q: %w", path, err)
	}
	if err := i.writeFile(path, append(data, '\n'), 0600); err != nil {
		return fmt.Errorf("failed to write %q: %w", path, err)
	}
	return nil
}
