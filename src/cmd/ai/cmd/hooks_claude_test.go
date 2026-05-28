package cmd_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/convergent-systems-co/aiConstitution/src/cmd/ai/cmd"
)

// writeStubHooks creates a tiny ~/.ai/hooks/ tree with a few of the
// canonical hook filenames so the install test has something to wire.
func writeStubHooks(t *testing.T, aiRoot string, names ...string) string {
	t.Helper()
	hooksDir := filepath.Join(aiRoot, "hooks")
	if err := os.MkdirAll(hooksDir, 0o750); err != nil {
		t.Fatal(err)
	}
	for _, n := range names {
		if err := os.WriteFile(filepath.Join(hooksDir, n), []byte("# stub\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return hooksDir
}

func TestHooksInstallClaudeCreatesSettings(t *testing.T) {
	aiRoot := t.TempDir()
	repoRoot := t.TempDir()
	t.Setenv("AI_ROOT", aiRoot)

	writeStubHooks(t, aiRoot, "audit.py", "branch-guard.py")

	root := cmd.NewRootCmd()
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"hooks", "install", "--claude", "--claude-root", repoRoot})
	if err := root.Execute(); err != nil {
		t.Fatalf("install --claude error: %v\noutput:%s", err, buf)
	}

	settingsPath := filepath.Join(repoRoot, ".claude", "settings.json")
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("read settings.json: %v", err)
	}
	var s struct {
		Hooks map[string][]map[string]string `json:"hooks"`
	}
	if err := json.Unmarshal(data, &s); err != nil {
		t.Fatalf("decode settings.json: %v\nraw: %s", err, data)
	}
	if len(s.Hooks["PreToolUse"]) == 0 {
		t.Errorf("PreToolUse hooks empty:\n%s", data)
	}
	// branch-guard.py should produce ONE PreToolUse entry; audit.py should produce
	// one too (audit fires on every event). So total PreToolUse >= 2.
	if len(s.Hooks["PreToolUse"]) < 2 {
		t.Errorf("PreToolUse < 2 entries:\n%s", data)
	}
	// audit.py wires into Stop as well.
	if len(s.Hooks["Stop"]) == 0 {
		t.Errorf("Stop hooks empty:\n%s", data)
	}
}

func TestHooksInstallClaudeIdempotent(t *testing.T) {
	aiRoot := t.TempDir()
	repoRoot := t.TempDir()
	t.Setenv("AI_ROOT", aiRoot)
	writeStubHooks(t, aiRoot, "audit.py", "branch-guard.py")

	// First run.
	root1 := cmd.NewRootCmd()
	root1.SetOut(&bytes.Buffer{})
	root1.SetErr(&bytes.Buffer{})
	root1.SetArgs([]string{"hooks", "install", "--claude", "--claude-root", repoRoot})
	if err := root1.Execute(); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(filepath.Join(repoRoot, ".claude", "settings.json"))
	if err != nil {
		t.Fatal(err)
	}

	// Second run.
	root2 := cmd.NewRootCmd()
	root2.SetOut(&bytes.Buffer{})
	root2.SetErr(&bytes.Buffer{})
	root2.SetArgs([]string{"hooks", "install", "--claude", "--claude-root", repoRoot})
	if err := root2.Execute(); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(filepath.Join(repoRoot, ".claude", "settings.json"))
	if err != nil {
		t.Fatal(err)
	}

	// Re-running must NOT duplicate entries — count of PreToolUse stays the same.
	var b, a struct {
		Hooks map[string][]map[string]string `json:"hooks"`
	}
	if err := json.Unmarshal(before, &b); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(after, &a); err != nil {
		t.Fatal(err)
	}
	for ev := range b.Hooks {
		if len(a.Hooks[ev]) != len(b.Hooks[ev]) {
			t.Errorf("event %s grew on idempotent re-run: before=%d after=%d", ev, len(b.Hooks[ev]), len(a.Hooks[ev]))
		}
	}
}

func TestHooksInstallClaudePreservesExistingKeys(t *testing.T) {
	aiRoot := t.TempDir()
	repoRoot := t.TempDir()
	t.Setenv("AI_ROOT", aiRoot)
	writeStubHooks(t, aiRoot, "branch-guard.py")

	// Pre-existing settings.json with a key we don't manage.
	settingsDir := filepath.Join(repoRoot, ".claude")
	if err := os.MkdirAll(settingsDir, 0o750); err != nil {
		t.Fatal(err)
	}
	pre := []byte(`{"theme": "dark", "hooks": {}}` + "\n")
	if err := os.WriteFile(filepath.Join(settingsDir, "settings.json"), pre, 0o600); err != nil {
		t.Fatal(err)
	}

	root := cmd.NewRootCmd()
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	root.SetArgs([]string{"hooks", "install", "--claude", "--claude-root", repoRoot})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(filepath.Join(settingsDir, "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	var s map[string]any
	if err := json.Unmarshal(data, &s); err != nil {
		t.Fatalf("decode: %v\n%s", err, data)
	}
	if s["theme"] != "dark" {
		t.Errorf("theme was clobbered: %v", s["theme"])
	}
}

// ---------------------------------------------------------------------------
// Tests for purgeOldHookEntries (via PurgeOldHookEntriesForTest export)
// ---------------------------------------------------------------------------

func TestPurgeOldHookEntries_RemovesFlatAbsolutePath(t *testing.T) {
	settings := map[string]any{
		"hooks": map[string]any{
			"PreToolUse": []any{
				map[string]any{"type": "PreToolUse", "command": "python3 /home/user/.ai/hooks/audit.py"},
				map[string]any{"type": "PreToolUse", "command": "ai hooks run audit"},
			},
		},
	}
	cmd.PurgeOldHookEntriesForTest(settings)

	hooks, ok := settings["hooks"].(map[string]any)
	if !ok {
		t.Fatal("hooks key missing after purge")
	}
	entries, ok := hooks["PreToolUse"].([]any)
	if !ok {
		t.Fatal("PreToolUse key missing after purge")
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry after purge, got %d: %v", len(entries), entries)
	}
	m, ok := entries[0].(map[string]any)
	if !ok {
		t.Fatal("entry is not a map")
	}
	if m["command"] != "ai hooks run audit" {
		t.Errorf("wrong entry preserved: %v", m["command"])
	}
}

func TestPurgeOldHookEntries_RemovesGroupFormat(t *testing.T) {
	settings := map[string]any{
		"hooks": map[string]any{
			"PreToolUse": []any{
				// Group format: "hooks" array inside the entry
				map[string]any{
					"hooks": []any{
						map[string]any{"command": "/home/user/.ai/hooks/branch-guard.py"},
					},
				},
				map[string]any{"type": "PreToolUse", "command": "ai hooks run branch-guard"},
			},
		},
	}
	cmd.PurgeOldHookEntriesForTest(settings)

	hooks := settings["hooks"].(map[string]any)
	entries := hooks["PreToolUse"].([]any)
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry after purge, got %d: %v", len(entries), entries)
	}
	m := entries[0].(map[string]any)
	if m["command"] != "ai hooks run branch-guard" {
		t.Errorf("wrong entry preserved: %v", m["command"])
	}
}

func TestPurgeOldHookEntries_PreservesPortableEntries(t *testing.T) {
	settings := map[string]any{
		"hooks": map[string]any{
			"PreToolUse": []any{
				map[string]any{"type": "PreToolUse", "command": "ai hooks run audit"},
				map[string]any{"type": "PreToolUse", "command": "ai hooks run branch-guard"},
			},
		},
	}
	cmd.PurgeOldHookEntriesForTest(settings)

	hooks := settings["hooks"].(map[string]any)
	entries := hooks["PreToolUse"].([]any)
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries preserved, got %d: %v", len(entries), entries)
	}
}

func TestPurgeOldHookEntries_EmptySettings(t *testing.T) {
	// No "hooks" key — must not panic.
	settings := map[string]any{}
	cmd.PurgeOldHookEntriesForTest(settings)

	// Settings unchanged (no hooks key added).
	if _, ok := settings["hooks"]; ok {
		t.Error("purge should not add a hooks key when none existed")
	}
}

func TestInstallClaudeHooks_PurgesAndRewires(t *testing.T) {
	aiRoot := t.TempDir()
	repoRoot := t.TempDir()
	t.Setenv("AI_ROOT", aiRoot)

	hooksDir := writeStubHooks(t, aiRoot, "audit.py", "branch-guard.py")

	// Seed settings.json with old absolute-path entries so we simulate a
	// pre-v1.3 installation.
	settingsDir := filepath.Join(repoRoot, ".claude")
	if err := os.MkdirAll(settingsDir, 0o750); err != nil {
		t.Fatal(err)
	}
	oldSettings := map[string]any{
		"hooks": map[string]any{
			"PreToolUse": []any{
				map[string]any{"type": "PreToolUse", "command": "python3 /home/user/.ai/hooks/audit.py"},
				map[string]any{"type": "PreToolUse", "command": "python3 /home/user/.ai/hooks/branch-guard.py"},
			},
			"Stop": []any{
				map[string]any{"type": "Stop", "command": "python3 /home/user/.ai/hooks/audit.py"},
			},
		},
	}
	data, err := json.Marshal(oldSettings)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(settingsDir, "settings.json"), data, 0o600); err != nil {
		t.Fatal(err)
	}

	// Call installClaudeHooks directly via the exported wrapper.
	added, err := cmd.InstallClaudeHooksForTest(repoRoot, hooksDir)
	if err != nil {
		t.Fatalf("installClaudeHooks error: %v", err)
	}
	if added == 0 {
		t.Error("expected at least one entry added (old entries should have been purged first)")
	}

	// Read result and verify.
	result, err := os.ReadFile(filepath.Join(settingsDir, "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	var s struct {
		Hooks map[string][]map[string]string `json:"hooks"`
	}
	if err := json.Unmarshal(result, &s); err != nil {
		t.Fatalf("decode: %v\nraw: %s", err, result)
	}

	// Old absolute-path entries must be gone; new portable entries must be present.
	for ev, entries := range s.Hooks {
		seen := map[string]int{}
		for _, e := range entries {
			cmd := e["command"]
			seen[cmd]++
			// No old absolute-path format should survive.
			if len(cmd) > 0 && (contains(cmd, "/.ai/hooks/") || (hasPrefix(cmd, "python3 ") && contains(cmd, "/hooks/"))) {
				t.Errorf("event %s: old entry not purged: %q", ev, cmd)
			}
			// No duplicates.
			if seen[cmd] > 1 {
				t.Errorf("event %s: duplicate entry %q", ev, cmd)
			}
		}
	}

	// Portable entries must exist.
	foundPortable := false
	for _, entries := range s.Hooks {
		for _, e := range entries {
			if hasPrefix(e["command"], "ai hooks run ") {
				foundPortable = true
			}
		}
	}
	if !foundPortable {
		t.Errorf("no portable 'ai hooks run' entries found after re-wiring:\n%s", result)
	}
}

// contains and hasPrefix are thin string helpers used only in this test file
// to avoid importing strings in the _test package (it's already imported by
// the package under test, not the external test package).
func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 || stringIndex(s, sub) >= 0)
}

func hasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

func stringIndex(s, sub string) int {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
