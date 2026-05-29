// hooks_integration_test.go — §4.2 integration tests for ai hooks install and wiring.
// Uses sandbox() from harness_test.go; tests that use sandbox MUST NOT call t.Parallel().
package cmd_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	cmd "github.com/convergent-systems-co/aiConstitution/src/cmd/ai/cmd"
)

// execHooksCmd executes "ai hooks <args...>" in the current sandbox environment.
// Callers must call sandbox(t) before execHooksCmd.
func execHooksCmd(t *testing.T, args ...string) string {
	t.Helper()
	root := cmd.NewRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs(append([]string{"hooks"}, args...))
	if err := root.Execute(); err != nil {
		t.Fatalf("ai hooks %s: %v\noutput:\n%s", strings.Join(args, " "), err, buf.String())
	}
	return buf.String()
}

// countHookCommands counts the total number of hook command entries across all
// events in a settings map. Used to detect spurious duplication on re-run.
func countHookCommands(settings map[string]any) int {
	hooks, _ := settings["hooks"].(map[string]any)
	total := 0
	for _, eventVal := range hooks {
		groups, _ := eventVal.([]any)
		for _, g := range groups {
			m, _ := g.(map[string]any)
			hookList, _ := m["hooks"].([]any)
			total += len(hookList)
		}
	}
	return total
}

// --- §4.2 Extract ---

// TestHooksInstallAll_ExtractsInfrastructure verifies that "ai hooks install --all"
// extracts the embedded infrastructure files (_lib.py, patterns.json,
// command-wrappers.toml) into AI_ROOT/hooks/ with correct permissions.
func TestHooksInstallAll_ExtractsInfrastructure(t *testing.T) {
	s := sandbox(t)

	execHooksCmd(t, "install", "--all")

	hooksDir := filepath.Join(s.AIRoot, "hooks")

	// _lib.py must exist and be executable.
	libPy := filepath.Join(hooksDir, "_lib.py")
	libInfo, err := os.Stat(libPy)
	if err != nil {
		t.Fatalf("_lib.py not extracted to %s: %v", hooksDir, err)
	}
	if libInfo.Mode()&0o100 == 0 {
		t.Errorf("_lib.py is not executable (mode=%o)", libInfo.Mode())
	}

	// patterns.json must exist.
	if _, err := os.Stat(filepath.Join(hooksDir, "patterns.json")); err != nil {
		t.Fatalf("patterns.json not extracted to %s: %v", hooksDir, err)
	}

	// command-wrappers.toml must exist.
	if _, err := os.Stat(filepath.Join(hooksDir, "command-wrappers.toml")); err != nil {
		t.Fatalf("command-wrappers.toml not extracted to %s: %v", hooksDir, err)
	}
}

// --- §4.2 Wire (direct seam) ---

// TestHooksWire_CreatesSettingsJSON verifies that updateSettingsJSON produces
// valid JSON with canonical "ai hooks run" commands and no absolute paths.
func TestHooksWire_CreatesSettingsJSON(t *testing.T) {
	s := sandbox(t)

	settingsPath := filepath.Join(s.ClaudeDir, "settings.json")
	hooksDir := filepath.Join(s.AIRoot, "hooks")

	if err := cmd.UpdateSettingsJSONForTest(settingsPath, hooksDir); err != nil {
		t.Fatalf("UpdateSettingsJSON: %v", err)
	}

	data, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("settings.json not written: %v", err)
	}

	// Must be valid JSON.
	var settings map[string]any
	if err := json.Unmarshal(data, &settings); err != nil {
		t.Fatalf("settings.json contains invalid JSON: %v\n%s", err, data)
	}

	// Must have a non-empty "hooks" key.
	hooks, ok := settings["hooks"].(map[string]any)
	if !ok || len(hooks) == 0 {
		t.Fatal("settings.json missing or empty 'hooks' key")
	}

	// At least one canonical event must be present.
	foundEvent := false
	for _, wantEvent := range []string{"PreToolUse", "PostToolUse", "SessionStart"} {
		if _, exists := hooks[wantEvent]; exists {
			foundEvent = true
			break
		}
	}
	if !foundEvent {
		t.Error("settings.json has no recognized hook events (PreToolUse, PostToolUse, SessionStart)")
	}

	raw := string(data)

	// Every command must use "ai hooks run", never an absolute path.
	if strings.Contains(raw, "python3 /") || strings.Contains(raw, "python /") {
		t.Error("settings.json contains absolute Python path; wiring must use 'ai hooks run <slug>'")
	}
	if !strings.Contains(raw, "ai hooks run") {
		t.Error("settings.json contains no 'ai hooks run' command")
	}
}

// TestHooksInstallAll_WiresSettings verifies that "ai hooks install --all"
// also wires .claude/settings.json (not only extracts hooks).
func TestHooksInstallAll_WiresSettings(t *testing.T) {
	s := sandbox(t)

	execHooksCmd(t, "install", "--all")

	settingsPath := filepath.Join(s.ClaudeDir, "settings.json")
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("settings.json not created by 'hooks install --all': %v\nCLAUDE_CONFIG_DIR=%s",
			err, s.ClaudeDir)
	}
	var settings map[string]any
	if err := json.Unmarshal(data, &settings); err != nil {
		t.Fatalf("settings.json invalid JSON: %v", err)
	}
	if _, ok := settings["hooks"]; !ok {
		t.Error("settings.json missing 'hooks' key after 'hooks install --all'")
	}
}

// --- §4.2 Merge safety ---

// TestHooksWire_MergeSafety verifies that updateSettingsJSON does not remove
// a user's pre-existing hook entry in an unrelated event group.
func TestHooksWire_MergeSafety(t *testing.T) {
	s := sandbox(t)

	settingsPath := filepath.Join(s.ClaudeDir, "settings.json")
	hooksDir := filepath.Join(s.AIRoot, "hooks")

	// Pre-seed with a user hook in PostToolUse using a custom matcher.
	existing := map[string]any{
		"hooks": map[string]any{
			"PostToolUse": []any{
				map[string]any{
					"matcher": "my-custom-tool",
					"hooks": []any{
						map[string]any{
							"type":    "command",
							"command": "my-custom-hook",
						},
					},
				},
			},
		},
	}
	raw, _ := json.Marshal(existing)
	if err := os.WriteFile(settingsPath, raw, 0o644); err != nil {
		t.Fatal(err)
	}

	if err := cmd.UpdateSettingsJSONForTest(settingsPath, hooksDir); err != nil {
		t.Fatalf("UpdateSettingsJSON: %v", err)
	}

	data, _ := os.ReadFile(settingsPath)
	if !strings.Contains(string(data), "my-custom-hook") {
		t.Error("user hook 'my-custom-hook' was removed by updateSettingsJSON; must survive merge")
	}
	if !strings.Contains(string(data), "my-custom-tool") {
		t.Error("user matcher 'my-custom-tool' was removed; must survive merge")
	}
}

// --- §4.2 Idempotency ---

// TestHooksWire_Idempotent verifies that running updateSettingsJSON twice
// produces the same number of hook commands — no duplicates added on re-run.
func TestHooksWire_Idempotent(t *testing.T) {
	s := sandbox(t)

	settingsPath := filepath.Join(s.ClaudeDir, "settings.json")
	hooksDir := filepath.Join(s.AIRoot, "hooks")

	if err := cmd.UpdateSettingsJSONForTest(settingsPath, hooksDir); err != nil {
		t.Fatalf("first UpdateSettingsJSON: %v", err)
	}
	data1, _ := os.ReadFile(settingsPath)
	var s1 map[string]any
	json.Unmarshal(data1, &s1) //nolint:errcheck
	count1 := countHookCommands(s1)

	if err := cmd.UpdateSettingsJSONForTest(settingsPath, hooksDir); err != nil {
		t.Fatalf("second UpdateSettingsJSON: %v", err)
	}
	data2, _ := os.ReadFile(settingsPath)
	var s2 map[string]any
	json.Unmarshal(data2, &s2) //nolint:errcheck
	count2 := countHookCommands(s2)

	if count1 == 0 {
		t.Error("no hook commands found after first run — wiring produced empty result")
	}
	if count1 != count2 {
		t.Errorf("second run added duplicate entries: first=%d commands, second=%d commands",
			count1, count2)
	}
}

// --- §4.2 Purge ---

// TestPurge_RemovesMalformedKeepsGood verifies that purgeMalformedHookEntries
// removes hook groups with null hooks, absolute-path commands, and retired
// command names, while preserving well-formed "ai hooks run" entries.
func TestPurge_RemovesMalformedKeepsGood(t *testing.T) {
	t.Parallel()

	settings := map[string]any{
		"hooks": map[string]any{
			"PreToolUse": []any{
				// Good: canonical "ai hooks run" command.
				map[string]any{
					"hooks": []any{
						map[string]any{
							"type":    "command",
							"command": "ai hooks run audit-logger",
						},
					},
				},
				// Bad: absolute path containing /.ai/hooks/.
				map[string]any{
					"hooks": []any{
						map[string]any{
							"type":    "command",
							"command": "python3 /home/user/.ai/hooks/audit.py",
						},
					},
				},
				// Bad: hooks is nil (not []any).
				map[string]any{
					"hooks": nil,
				},
				// Bad: retired command name.
				map[string]any{
					"hooks": []any{
						map[string]any{
							"type":    "command",
							"command": "ai hooks run audit-command",
						},
					},
				},
			},
		},
	}

	cmd.PurgeMalformedHookEntriesForTest(settings)

	hooks := settings["hooks"].(map[string]any)
	preToolUse, ok := hooks["PreToolUse"].([]any)
	if !ok {
		t.Fatal("PreToolUse key missing or wrong type after purge")
	}
	if len(preToolUse) != 1 {
		t.Fatalf("expected 1 surviving entry, got %d: %v", len(preToolUse), preToolUse)
	}

	surviving := preToolUse[0].(map[string]any)
	hookList := surviving["hooks"].([]any)
	entry := hookList[0].(map[string]any)
	if entry["command"] != "ai hooks run audit-logger" {
		t.Errorf("wrong entry survived purge: command=%q", entry["command"])
	}
}

// TestPurge_LeavesSettingsUntouchedWhenNoHooksKey verifies that
// purgeMalformedHookEntries is a no-op on a settings map with no "hooks" key.
func TestPurge_LeavesSettingsUntouchedWhenNoHooksKey(t *testing.T) {
	t.Parallel()
	settings := map[string]any{"theme": "dark"}
	cmd.PurgeMalformedHookEntriesForTest(settings)
	if v, ok := settings["theme"]; !ok || v != "dark" {
		t.Error("purge mutated a settings map with no 'hooks' key")
	}
}
