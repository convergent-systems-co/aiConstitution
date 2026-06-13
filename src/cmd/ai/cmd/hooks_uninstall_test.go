package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ─── scrubHookWiring tests ─────────────────────────────────────────────────

func TestScrubHookWiring_RemovesMatchingCommand(t *testing.T) {
	settings := map[string]any{
		"hooks": map[string]any{
			"Stop": []any{
				map[string]any{
					"hooks": []any{
						map[string]any{"type": "command", "command": "ai hooks run checkpoint-tick"},
						map[string]any{"type": "command", "command": "ai hooks run audit-logger"},
					},
				},
			},
		},
	}
	path := writeSettingsAny(t, settings)

	if err := scrubHookWiring(path, "ai hooks run checkpoint-tick"); err != nil {
		t.Fatalf("scrubHookWiring: %v", err)
	}

	raw := readSettingsAny(t, path)
	hooksMap := raw["hooks"].(map[string]any)
	stopEntries := hooksMap["Stop"].([]any)
	if len(stopEntries) != 1 {
		t.Fatalf("expected 1 group in Stop, got %d", len(stopEntries))
	}
	group := stopEntries[0].(map[string]any)
	hookList := group["hooks"].([]any)
	if len(hookList) != 1 {
		t.Fatalf("expected 1 hook remaining, got %d", len(hookList))
	}
	entry := hookList[0].(map[string]any)
	if entry["command"] != "ai hooks run audit-logger" {
		t.Errorf("expected audit-logger to remain, got %q", entry["command"])
	}
}

func TestScrubHookWiring_DropsEmptyGroup(t *testing.T) {
	settings := map[string]any{
		"hooks": map[string]any{
			"Stop": []any{
				map[string]any{
					"hooks": []any{
						map[string]any{"type": "command", "command": "ai hooks run checkpoint-tick"},
					},
				},
			},
		},
	}
	path := writeSettingsAny(t, settings)

	if err := scrubHookWiring(path, "ai hooks run checkpoint-tick"); err != nil {
		t.Fatalf("scrubHookWiring: %v", err)
	}

	raw := readSettingsAny(t, path)
	hooksMap := raw["hooks"].(map[string]any)
	// The Stop event should have been completely removed.
	if _, ok := hooksMap["Stop"]; ok {
		t.Error("expected Stop event to be removed when all its hooks were scrubbed")
	}
}

func TestScrubHookWiring_NoMatch_FileUnchanged(t *testing.T) {
	settings := map[string]any{
		"hooks": map[string]any{
			"Stop": []any{
				map[string]any{
					"hooks": []any{
						map[string]any{"type": "command", "command": "ai hooks run audit-logger"},
					},
				},
			},
		},
	}
	path := writeSettingsAny(t, settings)
	before, _ := os.ReadFile(path)

	if err := scrubHookWiring(path, "ai hooks run nonexistent"); err != nil {
		t.Fatalf("scrubHookWiring: %v", err)
	}

	after, _ := os.ReadFile(path)
	if string(before) != string(after) {
		t.Error("expected file to be unchanged when no match found")
	}
}

func TestScrubHookWiring_MissingFile_IsNoop(t *testing.T) {
	err := scrubHookWiring("/no/such/settings.json", "ai hooks run whatever")
	if err != nil {
		t.Errorf("expected nil for missing file, got %v", err)
	}
}

func TestScrubHookWiring_PreservesOtherKeys(t *testing.T) {
	settings := map[string]any{
		"model": "claude-opus-4-8",
		"hooks": map[string]any{
			"Stop": []any{
				map[string]any{
					"hooks": []any{
						map[string]any{"type": "command", "command": "ai hooks run checkpoint-tick"},
						map[string]any{"type": "command", "command": "ai hooks run audit-logger"},
					},
				},
			},
		},
	}
	path := writeSettingsAny(t, settings)

	if err := scrubHookWiring(path, "ai hooks run checkpoint-tick"); err != nil {
		t.Fatalf("scrubHookWiring: %v", err)
	}

	raw := readSettingsAny(t, path)
	if raw["model"] != "claude-opus-4-8" {
		t.Errorf("expected model key preserved, got %v", raw["model"])
	}
}

// ─── runHooksUninstall tests ───────────────────────────────────────────────

func TestRunHooksUninstall_RemovesFileOnly_NoWire(t *testing.T) {
	tmpDir := t.TempDir()
	hooksDir := filepath.Join(tmpDir, "hooks")
	if err := os.MkdirAll(hooksDir, 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	hookFile := filepath.Join(hooksDir, "checkpoint-tick.py")
	if err := os.WriteFile(hookFile, []byte("#!/usr/bin/env python3\npass\n"), 0o755); err != nil {
		t.Fatalf("write hook: %v", err)
	}
	claudeDir := filepath.Join(tmpDir, "claude")
	if err := os.MkdirAll(claudeDir, 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// Write settings.json with wiring — should NOT be touched without --wire.
	settings := map[string]any{
		"hooks": map[string]any{
			"Stop": []any{
				map[string]any{
					"hooks": []any{
						map[string]any{"type": "command", "command": "ai hooks run checkpoint-tick"},
					},
				},
			},
		},
	}
	settingsPath := filepath.Join(claudeDir, "settings.json")
	settingsData, _ := json.MarshalIndent(settings, "", "  ")
	if err := os.WriteFile(settingsPath, settingsData, 0o644); err != nil {
		t.Fatalf("write settings: %v", err)
	}
	before, _ := os.ReadFile(settingsPath)

	t.Setenv("AI_ROOT", tmpDir)
	t.Setenv("CLAUDE_CONFIG_DIR", claudeDir)

	var out bytes.Buffer
	if err := runHooksUninstall("checkpoint-tick", false, &out); err != nil {
		t.Fatalf("runHooksUninstall: %v", err)
	}

	// File must be gone.
	if _, err := os.Stat(hookFile); !os.IsNotExist(err) {
		t.Error("expected hook file to be removed")
	}

	// settings.json must be untouched.
	after, _ := os.ReadFile(settingsPath)
	if string(before) != string(after) {
		t.Error("expected settings.json to be unchanged without --wire")
	}

	// Output should contain the --wire hint.
	if !strings.Contains(out.String(), "--wire") {
		t.Errorf("expected --wire hint in output, got: %s", out.String())
	}
}

func TestRunHooksUninstall_WireFlag_ScrubsWiring(t *testing.T) {
	tmpDir := t.TempDir()
	hooksDir := filepath.Join(tmpDir, "hooks")
	if err := os.MkdirAll(hooksDir, 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	hookFile := filepath.Join(hooksDir, "checkpoint-tick.py")
	if err := os.WriteFile(hookFile, []byte("#!/usr/bin/env python3\npass\n"), 0o755); err != nil {
		t.Fatalf("write hook: %v", err)
	}
	claudeDir := filepath.Join(tmpDir, "claude")
	if err := os.MkdirAll(claudeDir, 0o750); err != nil {
		t.Fatalf("mkdir claude dir: %v", err)
	}
	settings := map[string]any{
		"hooks": map[string]any{
			"Stop": []any{
				map[string]any{
					"hooks": []any{
						map[string]any{"type": "command", "command": "ai hooks run checkpoint-tick"},
						map[string]any{"type": "command", "command": "ai hooks run audit-logger"},
					},
				},
			},
		},
	}
	settingsPath := filepath.Join(claudeDir, "settings.json")
	settingsData, _ := json.MarshalIndent(settings, "", "  ")
	if err := os.WriteFile(settingsPath, settingsData, 0o644); err != nil {
		t.Fatalf("write settings: %v", err)
	}

	t.Setenv("AI_ROOT", tmpDir)
	t.Setenv("CLAUDE_CONFIG_DIR", claudeDir)

	var out bytes.Buffer
	if err := runHooksUninstall("checkpoint-tick", true, &out); err != nil {
		t.Fatalf("runHooksUninstall: %v", err)
	}

	// Hook file must be gone.
	if _, err := os.Stat(hookFile); !os.IsNotExist(err) {
		t.Error("expected hook file to be removed")
	}

	// Wiring for checkpoint-tick must be scrubbed; audit-logger must remain.
	raw := readSettingsAny(t, settingsPath)
	hooksMap := raw["hooks"].(map[string]any)
	stopEntries := hooksMap["Stop"].([]any)
	group := stopEntries[0].(map[string]any)
	hookList := group["hooks"].([]any)
	if len(hookList) != 1 {
		t.Fatalf("expected 1 hook remaining, got %d", len(hookList))
	}
	if hookList[0].(map[string]any)["command"] != "ai hooks run audit-logger" {
		t.Errorf("expected audit-logger to remain")
	}

	got := out.String()
	if !strings.Contains(got, "Removed") {
		t.Errorf("expected 'Removed' in output, got: %s", got)
	}
	if !strings.Contains(got, "Scrubbed") {
		t.Errorf("expected 'Scrubbed' in output, got: %s", got)
	}
}

func TestRunHooksUninstall_AcceptsExtension(t *testing.T) {
	tmpDir := t.TempDir()
	hooksDir := filepath.Join(tmpDir, "hooks")
	if err := os.MkdirAll(hooksDir, 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	hookFile := filepath.Join(hooksDir, "my-hook.py")
	if err := os.WriteFile(hookFile, []byte("#!/usr/bin/env python3\n"), 0o755); err != nil {
		t.Fatalf("write hook: %v", err)
	}

	claudeDir := filepath.Join(tmpDir, "claude")
	if err := os.MkdirAll(claudeDir, 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	t.Setenv("AI_ROOT", tmpDir)
	t.Setenv("CLAUDE_CONFIG_DIR", claudeDir)

	var out bytes.Buffer
	// Pass with ".py" extension — should still work.
	if err := runHooksUninstall("my-hook.py", false, &out); err != nil {
		t.Fatalf("runHooksUninstall: %v", err)
	}
	if _, err := os.Stat(hookFile); !os.IsNotExist(err) {
		t.Error("expected hook file to be removed when extension is passed")
	}
}

func TestRunHooksUninstall_NotInstalled_IsNoop(t *testing.T) {
	tmpDir := t.TempDir()
	hooksDir := filepath.Join(tmpDir, "hooks")
	if err := os.MkdirAll(hooksDir, 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	claudeDir := filepath.Join(tmpDir, "claude")
	if err := os.MkdirAll(claudeDir, 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	t.Setenv("AI_ROOT", tmpDir)
	t.Setenv("CLAUDE_CONFIG_DIR", claudeDir)

	var out bytes.Buffer
	if err := runHooksUninstall("nonexistent", false, &out); err != nil {
		t.Fatalf("expected nil for nonexistent hook, got: %v", err)
	}
	if !strings.Contains(out.String(), "Not found") {
		t.Errorf("expected 'Not found' message, got: %s", out.String())
	}
}

// ─── checkDeprecatedHooks tests ────────────────────────────────────────────

func TestCheckDeprecatedHooks_WhenPresentWarns(t *testing.T) {
	tmpDir := t.TempDir()
	hooksDir := filepath.Join(tmpDir, "hooks")
	if err := os.MkdirAll(hooksDir, 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// Plant the deprecated hook.
	if err := os.WriteFile(filepath.Join(hooksDir, "checkpoint-tick.py"), []byte(""), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	var buf bytes.Buffer
	checkDeprecatedHooks(&buf, tmpDir)
	got := buf.String()
	if !strings.Contains(got, "[⚠]") || !strings.Contains(got, "checkpoint-tick") {
		t.Errorf("expected deprecation warning for checkpoint-tick, got: %q", got)
	}
	if !strings.Contains(got, "ai hooks uninstall --wire checkpoint-tick") {
		t.Errorf("expected uninstall --wire hint, got: %q", got)
	}
}

func TestCheckDeprecatedHooks_WhenAbsentNoWarning(t *testing.T) {
	tmpDir := t.TempDir()
	hooksDir := filepath.Join(tmpDir, "hooks")
	if err := os.MkdirAll(hooksDir, 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// No checkpoint-tick.py present.

	var buf bytes.Buffer
	checkDeprecatedHooks(&buf, tmpDir)
	got := buf.String()
	if strings.Contains(got, "checkpoint-tick") {
		t.Errorf("expected no warning when hook is absent, got: %q", got)
	}
}

// ─── helpers ──────────────────────────────────────────────────────────────

func writeSettingsAny(t *testing.T, v any) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		t.Fatalf("marshal settings: %v", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write settings: %v", err)
	}
	return path
}

func readSettingsAny(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read settings: %v", err)
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("parse settings: %v", err)
	}
	return raw
}
