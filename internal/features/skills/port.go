package skills

// Installer copies bundled skill files into the editor's skills directory.
// Example: installer.Install("claude").
type Installer interface {
	Install(editor string) error
}
