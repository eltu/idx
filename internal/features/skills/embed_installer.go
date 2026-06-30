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
	assetSkillDir         = "assets/idx"
	assetClaudeProjectDir = "claude-project"
	assetHookSrc          = "assets/idx/claude-project/block-shell-tools.sh"
	assetContextHookSrc   = "assets/idx/claude-project/context-hook.sh"
	assetReadHookSrc      = "assets/idx/claude-project/block-read-tool.sh"
	assetGrepHookSrc      = "assets/idx/claude-project/block-grep-tool.sh"
	skillName             = "idx"
	claudePermission      = "Bash(idx *)"
	projectHookName       = "idx-block.sh"
	contextHookName       = "idx-context-hook.sh"
	readHookName          = "idx-read-block.sh"
	grepHookName          = "idx-grep-block.sh"
	projectHookCommand    = "~/.claude/" + projectHookName
	contextHookCommand    = "~/.claude/" + contextHookName
	readHookCommand       = "~/.claude/" + readHookName
	grepHookCommand       = "~/.claude/" + grepHookName
	hookEventPreToolCall  = "PreToolUse"
	hookEventUserPrompt   = "UserPromptSubmit"
	matcherBash           = "Bash"
	matcherRead           = "Read"
	matcherGrep           = "Grep"
	editorClaude          = "claude"
	claudeDir             = ".claude"
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
	return i.configureClaudeProject(projectRoot)
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
// and installs all enforcement hook scripts to ~/.claude/.
func (i *EmbedSkillsInstaller) configureClaude(homeDir string) error {
	settingsPath := filepath.Join(homeDir, claudeDir, "settings.json")
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
	scripts := []struct{ src, name string }{
		{assetHookSrc, projectHookName},
		{assetContextHookSrc, contextHookName},
		{assetReadHookSrc, readHookName},
		{assetGrepHookSrc, grepHookName},
	}
	for _, s := range scripts {
		if err := i.installEmbeddedScript(s.src, filepath.Join(homeDir, claudeDir, s.name)); err != nil {
			return err
		}
	}
	return nil
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

// configureClaudeProject registers PreToolUse hooks (Bash, Read, Grep) and the
// UserPromptSubmit hook in the project's .claude/settings.json.
// Hook commands use literal "~" so the file is portable and can be versioned.
func (i *EmbedSkillsInstaller) configureClaudeProject(projectRoot string) error {
	projectSettingsPath := filepath.Join(projectRoot, claudeDir, "settings.json")
	preToolHooks := []struct{ cmd, matcher string }{
		{projectHookCommand, matcherBash},
		{readHookCommand, matcherRead},
		{grepHookCommand, matcherGrep},
	}
	for _, h := range preToolHooks {
		if err := i.upsertProjectHook(projectSettingsPath, hookEventPreToolCall, h.cmd, h.matcher); err != nil {
			return err
		}
	}
	return i.upsertProjectHook(projectSettingsPath, hookEventUserPrompt, contextHookCommand, "")
}

// upsertProjectHook registers a hook command under the given event type in the project
// .claude/settings.json. matcher sets the "matcher" field when non-empty (PreToolUse hooks).
// It is idempotent: running it again does not add a duplicate entry.
func (i *EmbedSkillsInstaller) upsertProjectHook(path, eventType, hookCmd, matcher string) error {
	settings, err := i.loadClaudeSettings(path)
	if err != nil {
		return err
	}
	if hookEntryExists(settings, eventType, hookCmd) {
		return nil
	}
	addHookEntry(settings, eventType, hookCmd, matcher)
	return i.saveClaudeSettings(path, settings)
}

func hookEntryExists(settings map[string]any, eventType, hookCmd string) bool {
	for _, entry := range hookEventEntries(settings, eventType) {
		if hooksSliceContains(entry, hookCmd) {
			return true
		}
	}
	return false
}

func addHookEntry(settings map[string]any, eventType, hookCmd, matcher string) {
	hooks := hooksSectionMap(settings)
	entries := hookEventEntries(settings, eventType)
	newEntry := map[string]any{
		"hooks": []any{
			map[string]any{"type": "command", "command": hookCmd},
		},
	}
	if matcher != "" {
		newEntry["matcher"] = matcher
	}
	hooks[eventType] = append(entries, newEntry)
	settings["hooks"] = hooks
}

func hooksSectionMap(settings map[string]any) map[string]any {
	if h, ok := settings["hooks"].(map[string]any); ok {
		return h
	}
	return map[string]any{}
}

func hookEventEntries(settings map[string]any, eventType string) []any {
	h := hooksSectionMap(settings)
	if entries, ok := h[eventType].([]any); ok {
		return entries
	}
	return []any{}
}

func hooksSliceContains(entry any, hookCmd string) bool {
	m, ok := entry.(map[string]any)
	if !ok {
		return false
	}
	hooks, ok := m["hooks"].([]any)
	if !ok {
		return false
	}
	for _, h := range hooks {
		hm, ok := h.(map[string]any)
		if !ok {
			continue
		}
		if cmd, ok := hm["command"].(string); ok && cmd == hookCmd {
			return true
		}
	}
	return false
}

func (i *EmbedSkillsInstaller) loadClaudeSettings(path string) (map[string]any, error) {
	data, err := i.readFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return defaultClaudeSettings(), nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read %q: %w", path, err)
	}
	var settings map[string]any
	if err := json.Unmarshal(data, &settings); err != nil {
		return nil, fmt.Errorf("failed to parse %q: %w", path, err)
	}
	return settings, nil
}

func defaultClaudeSettings() map[string]any {
	return map[string]any{
		"permissions": map[string]any{"allow": []any{}},
	}
}

func hasClaudePermission(settings map[string]any) bool {
	for _, p := range claudeAllowList(settings) {
		if p == claudePermission {
			return true
		}
	}
	return false
}

func addClaudePermission(settings map[string]any) {
	perms := claudePermissionsMap(settings)
	perms["allow"] = append(claudeAllowList(settings), claudePermission)
	settings["permissions"] = perms
}

func claudePermissionsMap(settings map[string]any) map[string]any {
	if p, ok := settings["permissions"].(map[string]any); ok {
		return p
	}
	return map[string]any{}
}

func claudeAllowList(settings map[string]any) []any {
	if allow, ok := claudePermissionsMap(settings)["allow"].([]any); ok {
		return allow
	}
	return []any{}
}

func (i *EmbedSkillsInstaller) saveClaudeSettings(path string, settings map[string]any) error {
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
