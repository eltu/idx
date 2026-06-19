package skills

import (
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

const (
	assetSkillDir         = "assets/idx-search"
	assetClaudeProjectDir = "claude-project"
	assetHookSrc          = "assets/idx-search/claude-project/block-shell-tools.sh"
	assetContextHookSrc   = "assets/idx-search/claude-project/context-hook.sh"
	skillName             = "idx-search"
	claudePermission      = "Bash(idx *)"
	projectHookName       = "idx-search-block.sh"
	contextHookName       = "idx-search-context-hook.sh"
	projectHookCommand    = "~/.claude/" + projectHookName
	contextHookCommand    = "~/.claude/" + contextHookName
	hookEventPreToolCall  = "PreToolCall"
	hookEventUserPrompt   = "UserPromptSubmit"
	editorClaude          = "claude"
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

// Install copies bundled skill files to the editor's skills directory and, for Claude,
// patches ~/.claude/settings.json and (when projectRoot != "") configures the project.
func (i *EmbedSkillsInstaller) Install(editor, projectRoot string) error {
	homeDir, err := i.homeDir()
	if err != nil {
		return fmt.Errorf("failed to resolve home directory: %w", err)
	}
	if err := i.copySkillFiles(editorSkillsDir(editor, homeDir)); err != nil {
		return err
	}
	if editor != editorClaude {
		return nil
	}
	if err := i.configureClaude(homeDir); err != nil {
		return err
	}
	if projectRoot == "" {
		return nil
	}
	return i.configureClaudeProject(homeDir, projectRoot)
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
	// claude-project/ contains hook scripts installed separately; skip from skill copy.
	if relPath == assetClaudeProjectDir || strings.HasPrefix(relPath, assetClaudeProjectDir+string(filepath.Separator)) {
		return nil
	}
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

// configureClaude patches ~/.claude/settings.json to allow Bash(idx *) permissions
// and installs both enforcement hook scripts to ~/.claude/.
func (i *EmbedSkillsInstaller) configureClaude(homeDir string) error {
	settingsPath := filepath.Join(homeDir, ".claude", "settings.json")
	settings, err := i.loadClaudeSettings(settingsPath)
	if err != nil {
		return err
	}
	if !hasClaudePermission(settings) {
		addClaudePermission(settings)
		if err := i.saveClaudeSettings(settingsPath, settings); err != nil {
			return err
		}
	}
	if err := i.installHookScript(homeDir); err != nil {
		return err
	}
	return i.installContextHookScript(homeDir)
}

// installHookScript copies the embedded PreToolCall hook to ~/.claude/idx-search-hook.sh.
func (i *EmbedSkillsInstaller) installHookScript(homeDir string) error {
	return i.installEmbeddedScript(assetHookSrc, filepath.Join(homeDir, ".claude", projectHookName))
}

// installContextHookScript copies the embedded UserPromptSubmit hook to ~/.claude/idx-search-context-hook.sh.
func (i *EmbedSkillsInstaller) installContextHookScript(homeDir string) error {
	return i.installEmbeddedScript(assetContextHookSrc, filepath.Join(homeDir, ".claude", contextHookName))
}

func (i *EmbedSkillsInstaller) installEmbeddedScript(assetSrc, destPath string) error {
	data, err := skillsFS.ReadFile(assetSrc)
	if err != nil {
		return fmt.Errorf("failed to read embedded script %q: %w", assetSrc, err)
	}
	if err := i.mkdirAll(filepath.Dir(destPath), 0750); err != nil {
		return fmt.Errorf("failed to create directory for %q: %w", destPath, err)
	}
	return i.writeFile(destPath, data, 0750)
}

// configureClaudeProject registers the PreToolCall and UserPromptSubmit hooks
// in the project's .claude/settings.json. No project files are modified beyond that.
func (i *EmbedSkillsInstaller) configureClaudeProject(homeDir, projectRoot string) error {
	projectSettingsPath := filepath.Join(projectRoot, ".claude", "settings.json")
	preToolCmd := strings.ReplaceAll(projectHookCommand, "~", homeDir)
	contextCmd := strings.ReplaceAll(contextHookCommand, "~", homeDir)
	if err := i.upsertProjectHook(projectSettingsPath, hookEventPreToolCall, preToolCmd, true); err != nil {
		return err
	}
	return i.upsertProjectHook(projectSettingsPath, hookEventUserPrompt, contextCmd, false)
}

// upsertProjectHook registers a hook command under the given event type in the project
// .claude/settings.json. withMatcher adds a "matcher":"Bash" field (used for PreToolCall).
// It is idempotent: running it again does not add a duplicate entry.
func (i *EmbedSkillsInstaller) upsertProjectHook(path, eventType, hookCmd string, withMatcher bool) error {
	settings, err := i.loadClaudeSettings(path)
	if err != nil {
		return err
	}
	if hookEntryExists(settings, eventType, hookCmd) {
		return nil
	}
	addHookEntry(settings, eventType, hookCmd, withMatcher)
	return i.saveClaudeSettings(path, settings)
}

func hookEntryExists(settings map[string]interface{}, eventType, hookCmd string) bool {
	for _, entry := range hookEventEntries(settings, eventType) {
		if hooksSliceContains(entry, hookCmd) {
			return true
		}
	}
	return false
}

func addHookEntry(settings map[string]interface{}, eventType, hookCmd string, withMatcher bool) {
	hooks := hooksSectionMap(settings)
	entries := hookEventEntries(settings, eventType)
	newEntry := map[string]interface{}{
		"hooks": []interface{}{
			map[string]interface{}{"type": "command", "command": hookCmd},
		},
	}
	if withMatcher {
		newEntry["matcher"] = "Bash"
	}
	hooks[eventType] = append(entries, newEntry)
	settings["hooks"] = hooks
}

func hooksSectionMap(settings map[string]interface{}) map[string]interface{} {
	if h, ok := settings["hooks"].(map[string]interface{}); ok {
		return h
	}
	return map[string]interface{}{}
}

func hookEventEntries(settings map[string]interface{}, eventType string) []interface{} {
	h := hooksSectionMap(settings)
	if entries, ok := h[eventType].([]interface{}); ok {
		return entries
	}
	return []interface{}{}
}

func hooksSliceContains(entry interface{}, hookCmd string) bool {
	m, ok := entry.(map[string]interface{})
	if !ok {
		return false
	}
	hooks, ok := m["hooks"].([]interface{})
	if !ok {
		return false
	}
	for _, h := range hooks {
		hm, ok := h.(map[string]interface{})
		if !ok {
			continue
		}
		if cmd, ok := hm["command"].(string); ok && cmd == hookCmd {
			return true
		}
	}
	return false
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
