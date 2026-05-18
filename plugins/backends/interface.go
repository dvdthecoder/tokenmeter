// Package backends defines the BackendAdapter interface.
// Each adapter knows how to detect, install into, and uninstall from
// a specific AI coding tool (Claude Code, Codex CLI, VS Code, etc.).
package backends

// BackendAdapter configures an AI coding tool to route through the proxy.
type BackendAdapter interface {
	// Name returns the backend identifier, e.g. "claudecode", "codex", "vscode".
	Name() string

	// Detect returns true if the tool appears to be installed on this machine.
	Detect() bool

	// Install configures the tool to route through the proxy at the given address.
	Install(proxyAddr string) error

	// Uninstall removes proxy configuration from the tool.
	Uninstall() error

	// Verify confirms traffic is flowing through the proxy after install.
	Verify(proxyAddr string) error
}

var registry = map[string]BackendAdapter{}

// Register adds a backend adapter to the global registry. Call from init().
func Register(b BackendAdapter) {
	registry[b.Name()] = b
}

// All returns all registered adapters.
func All() map[string]BackendAdapter {
	return registry
}

// Get returns a registered adapter by name.
func Get(name string) (BackendAdapter, bool) {
	b, ok := registry[name]
	return b, ok
}

// Detected returns all adapters whose tools are installed on this machine.
func Detected() []BackendAdapter {
	var found []BackendAdapter
	for _, b := range registry {
		if b.Detect() {
			found = append(found, b)
		}
	}
	return found
}
