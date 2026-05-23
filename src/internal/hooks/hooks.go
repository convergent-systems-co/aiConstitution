// Package hooks manages the hook registration topology — wiring
// ~/.ai/hooks/*.py files into Claude Code, Copilot CLI, Cursor, and
// the command-wrapper preHook/postHook chains.
//
// Schema source of truth: ~/.ai/hooks/command-wrappers.toml plus the
// individual tool registration files (~/.claude/settings.json for
// Claude Code, etc.).
package hooks

// Hook is the in-memory description of one installed hook.
type Hook struct {
	Name        string   // e.g. "audit", "secret-block"
	Path        string   // absolute path to the script
	Language    string   // "python" | "sh" | "go" | "node"
	Events      []string // "PreToolUse" | "PostToolUse" | "pre-commit" | "command-pre" | ...
	WiredInto   []string // "claude" | "copilot" | "cursor" | "git-wrapper" | ...
	SelfCheckOK bool     // last --self-check exit status (true == zero)
}

// List returns the inventory of installed hooks plus their wiring
// status. TBD for v0.8.
func List() ([]Hook, error) {
	return nil, nil
}

// Install adds a hook to the appropriate registration surfaces (idempotent).
// TBD for v0.8.
func Install(_ string) error {
	return nil
}

// Evaluate runs each installed hook's --self-check and returns the
// aggregated findings. TBD for v0.8.
func Evaluate() ([]Finding, error) {
	return nil, nil
}

// Finding is one item emitted by Evaluate.
type Finding struct {
	Hook     string
	Severity string // "info" | "warn" | "error"
	Message  string
}
