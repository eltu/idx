package skills

// Installer copies bundled skill files into the editor's skills directory and
// optionally configures the project at projectRoot for per-project enforcement.
// projectRoot == "" skips project configuration.
// Example: installer.Install("claude", "/home/user/myproject").
type Installer interface {
	Install(editor, projectRoot string) error
}
