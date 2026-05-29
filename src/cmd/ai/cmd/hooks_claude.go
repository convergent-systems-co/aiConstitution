package cmd

import (
	"strings"
)

// purgeMalformedHookEntries scrubs settings.hooks of every shape that is not
// a canonical Claude Code hook group. Idempotent.
//
// A canonical entry under hooks[<event>] is:
//
//	{ "matcher": "<optional>", "hooks": [ {"type": "command", "command": "..."} ] }
//
// This function drops every entry that does not match that shape, including:
//
//   - Flat entries at the matcher level — e.g. {"type": "<eventName>", "command": "..."} —
//     written by the removed addClaudeEntry path.
//   - {"hooks": null} and {"hooks": []} stubs left behind when the canonical
//     writer round-trips flat entries through []hookGroup.
//   - Pre-v1.3 absolute-path commands such as
//     "python3 /home/user/.ai/hooks/audit.py" or a bare path under /.ai/hooks/.
//   - Nested entries whose type is not "command", or whose command is empty.
//
// Surviving entries are rewritten to the minimal canonical form (only matcher
// and hooks; no extraneous fields), so a subsequent JSON round-trip into
// []hookGroup is lossless.
func purgeMalformedHookEntries(settings map[string]any) {
	hooks, ok := settings["hooks"].(map[string]any)
	if !ok {
		return
	}
	for event, val := range hooks {
		entries, ok := val.([]any)
		if !ok {
			continue
		}
		kept := make([]any, 0, len(entries))
		for _, entry := range entries {
			m, ok := entry.(map[string]any)
			if !ok {
				continue
			}
			cleaned, ok := cleanHookGroup(m)
			if !ok {
				continue
			}
			kept = append(kept, cleaned)
		}
		hooks[event] = kept
	}
}

// cleanHookGroup validates a single matcher-level entry. Returns the scrubbed
// canonical form and ok=true when at least one valid nested command remains;
// ok=false means the entry should be dropped entirely.
func cleanHookGroup(m map[string]any) (map[string]any, bool) {
	raw, ok := m["hooks"].([]any)
	if !ok || len(raw) == 0 {
		return nil, false
	}
	cleanedHooks := make([]any, 0, len(raw))
	for _, h := range raw {
		hm, ok := h.(map[string]any)
		if !ok {
			continue
		}
		t, _ := hm["type"].(string)
		cmd, _ := hm["command"].(string)
		cmd = strings.TrimSpace(cmd)
		if t != "command" || cmd == "" {
			continue
		}
		if isAbsoluteHookCmd(cmd) {
			continue
		}
		if isRetiredHookCmd(cmd) {
			continue
		}
		cleanedHooks = append(cleanedHooks, map[string]any{
			"type":    "command",
			"command": cmd,
		})
	}
	if len(cleanedHooks) == 0 {
		return nil, false
	}
	out := map[string]any{"hooks": cleanedHooks}
	if matcher, ok := m["matcher"].(string); ok && matcher != "" {
		out["matcher"] = matcher
	}
	return out, true
}

// retiredHookCommands is the set of `ai hooks run <slug>` invocations that
// reference hooks no longer present in the ai-atoms.com catalog under that
// name, or that were mis-wired into Claude events when they belong elsewhere.
// Purge drops any entry that matches; the canonical wiring then re-adds the
// correct replacement on the next install.
//
//   - "ai hooks run audit"          → catalog renamed to "audit-logger".
//   - "ai hooks run audit-command"  → wrapper postHook (invoked by ~/.ai/bin/{git,gh});
//     never a Claude Code event hook. Earlier canonicalWiring wrongly placed it in PreToolUse.
var retiredHookCommands = map[string]bool{
	"ai hooks run audit":         true,
	"ai hooks run audit-command": true,
}

// isRetiredHookCmd reports whether cmd matches a known retired wiring that
// should be scrubbed on every install.
func isRetiredHookCmd(cmd string) bool {
	return retiredHookCommands[cmd]
}

// isAbsoluteHookCmd reports whether cmd looks like a pre-v1.3 absolute-path
// hook invocation. Both patterns are matched:
//
//	python3 /some/path/.ai/hooks/foo.py   (with python3 prefix)
//	/some/path/.ai/hooks/foo.py           (bare path, no interpreter)
func isAbsoluteHookCmd(cmd string) bool {
	return strings.Contains(cmd, "/.ai/hooks/") || strings.Contains(cmd, "\\.ai\\hooks\\") ||
		(strings.HasPrefix(cmd, "python3 ") && strings.Contains(cmd, "/hooks/"))
}
