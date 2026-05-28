package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// claudeEventMap maps each canonical Python hook filename (under
// ~/.ai/hooks/) to the Claude Code event(s) it should be wired into
// in .claude/settings.json. The mapping is the source of truth for
// `ai hooks install --claude` — adding a new hook means adding a
// row here.
//
// Per Common.md §5.5 the audit hook is wired into every event; the
// guards fire on PreToolUse; the post-action and stop hooks fire on
// PostToolUse and Stop respectively.
var claudeEventMap = map[string][]string{
	"audit.py": {
		"SessionStart", "UserPromptSubmit", "PreToolUse", "PostToolUse",
		"Stop", "SessionEnd", "SubagentStop", "PreCompact",
	},
	"audit-command.py":              {"PostToolUse"},
	"audit-logger.py":               {"PreToolUse"},
	"branch-guard.py":               {"PreToolUse"},
	"worktree-guard.py":              {"PreToolUse"},
	"secret-block.py":                {"PreToolUse"},
	"no-verify-strip.py":             {"PreToolUse"},
	"op-redact.py":                   {"PostToolUse"},
	"destructive-gh-guard.py":        {"PreToolUse"},
	"destructive-kubectl-guard.py":   {"PreToolUse"},
	"destructive-terraform-guard.py": {"PreToolUse"},
	"checkpoint-tick.py":             {"Stop"},
	// secret-precommit.py is a git pre-commit hook, not a Claude Code event hook.
}

// claudeSettings is the minimal in-memory shape of .claude/settings.json
// that we touch. Other fields in the file are preserved by round-tripping
// through map[string]any.
//
// On disk:
//
//	{
//	  "hooks": {
//	    "PreToolUse": [
//	      {"type": "PreToolUse", "command": "python3 /.../audit.py"},
//	      ...
//	    ],
//	    ...
//	  }
//	}
type claudeHookEntry struct {
	Type    string `json:"type"`
	Command string `json:"command"`
}

// purgeOldHookEntries removes settings.hooks entries that use the old absolute-path
// format (pre-v1.3 wiring). These are replaced by the portable "ai hooks run <slug>"
// format. Must be called before addClaudeEntry to avoid duplicates.
func purgeOldHookEntries(settings map[string]any) {
	hooks, ok := settings["hooks"].(map[string]any)
	if !ok {
		return
	}
	for event, val := range hooks {
		entries, ok := val.([]any)
		if !ok {
			continue
		}
		kept := entries[:0]
		for _, entry := range entries {
			m, ok := entry.(map[string]any)
			if !ok {
				kept = append(kept, entry)
				continue
			}
			if isOldHookEntry(m) {
				continue // drop it
			}
			kept = append(kept, entry)
		}
		hooks[event] = kept
	}
}

// isOldHookEntry returns true for entries written by the pre-v1.3 wiring code:
//   - flat: {"command": "python3 /abs/path/to/.ai/hooks/foo.py"}
//   - flat: {"command": "/abs/path/to/.ai/hooks/foo.py"}  (no python3 prefix)
//   - group: {"hooks": [{"command": "/abs/path/to/.ai/hooks/foo.py"}]}
func isOldHookEntry(m map[string]any) bool {
	// Flat format: top-level "command" field.
	if hookCmd, ok := m["command"].(string); ok && isAbsoluteHookCmd(hookCmd) {
		return true
	}
	// Group format: "hooks" array with command entries.
	if hookList, ok := m["hooks"].([]any); ok {
		for _, h := range hookList {
			hm, ok := h.(map[string]any)
			if !ok {
				continue
			}
			if hookCmd, ok := hm["command"].(string); ok && isAbsoluteHookCmd(hookCmd) {
				return true
			}
		}
	}
	return false
}

// isAbsoluteHookCmd reports whether cmd looks like a pre-v1.3 absolute-path
// hook invocation. Both patterns are matched:
//
//	python3 /some/path/.ai/hooks/foo.py   (with python3 prefix)
//	/some/path/.ai/hooks/foo.py           (bare path, no interpreter)
func isAbsoluteHookCmd(cmd string) bool {
	return strings.Contains(cmd, "/.ai/hooks/") ||
		(strings.HasPrefix(cmd, "python3 ") && strings.Contains(cmd, "/hooks/"))
}

// installClaudeHooks reads .claude/settings.json under repoRoot (creating
// {} if absent), wires every hook in hooksDir that has a known Claude
// event mapping, and writes the file back atomically. Idempotent.
//
// Returns the count of new entries added (zero on a no-op re-run).
func installClaudeHooks(repoRoot, hooksDir string) (int, error) {
	if repoRoot == "" {
		repoRoot = "."
	}
	settingsDir := filepath.Join(repoRoot, ".claude")
	settingsPath := filepath.Join(settingsDir, "settings.json")

	settings, err := loadClaudeSettings(settingsPath)
	if err != nil {
		return 0, err
	}

	// Purge entries written by the pre-v1.3 wiring that used absolute paths.
	// This prevents duplicate hook invocations when the user re-wires after
	// upgrading to the portable "ai hooks run <slug>" format.
	purgeOldHookEntries(settings)

	// Walk hooksDir.
	entries, err := os.ReadDir(hooksDir)
	if err != nil {
		return 0, fmt.Errorf("hooks: read %s: %w", hooksDir, err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		names = append(names, e.Name())
	}
	sort.Strings(names)

	added := 0
	for _, name := range names {
		events, ok := claudeEventMap[name]
		if !ok {
			continue
		}
		// Use "ai hooks run <slug>" instead of a hardcoded absolute path so that
		// settings.json entries work on Windows, Linux, and macOS without change.
		// The ai binary discovers the correct Python and hook path at runtime.
		slug := strings.TrimSuffix(name, filepath.Ext(name))
		cmd := "ai hooks run " + slug
		for _, ev := range events {
			if addClaudeEntry(settings, ev, cmd) {
				added++
			}
		}
	}

	if err := saveClaudeSettings(settingsPath, settings); err != nil {
		return added, err
	}
	return added, nil
}

// loadClaudeSettings reads settings.json into a generic map, returning
// an empty map if the file does not exist.
func loadClaudeSettings(path string) (map[string]any, error) {
	data, err := os.ReadFile(path) //nolint:gosec // G304: caller-derived path under repoRoot
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]any{}, nil
		}
		return nil, err
	}
	out := map[string]any{}
	if len(data) == 0 {
		return out, nil
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, fmt.Errorf("hooks: parse %s: %w", path, err)
	}
	return out, nil
}

// saveClaudeSettings writes the settings map back to disk atomically
// (temp file in the same dir + rename). Creates the .claude/ directory
// if absent.
func saveClaudeSettings(path string, settings map[string]any) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return err
	}
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')

	tmp, err := os.CreateTemp(dir, "settings-*.json.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}

// addClaudeEntry ensures settings.hooks[event] contains an entry with
// the given command. Returns true if a new entry was appended.
func addClaudeEntry(settings map[string]any, event, command string) bool {
	hooks, _ := settings["hooks"].(map[string]any)
	if hooks == nil {
		hooks = map[string]any{}
		settings["hooks"] = hooks
	}
	raw, ok := hooks[event].([]any)
	if !ok {
		raw = nil
	}

	// Idempotency: compare on (type, command) tuple.
	for _, e := range raw {
		m, ok := e.(map[string]any)
		if !ok {
			continue
		}
		if commandOf(m) == command {
			return false
		}
	}
	entry := claudeHookEntry{Type: event, Command: command}
	// Round-trip via JSON so the map stays []any of map[string]any
	// (consistent with what Unmarshal will produce on the next load).
	raw = append(raw, map[string]any{"type": entry.Type, "command": entry.Command})
	hooks[event] = raw
	return true
}

func commandOf(m map[string]any) string {
	v, _ := m["command"].(string)
	return strings.TrimSpace(v)
}
