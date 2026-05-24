package cmd_test

// hooks_install_test.go — tests for `ai hooks install --all` (stories #84 + #96)
//
// RED criteria (current hooks.go will fail):
// - AC2: settings.json is NOT currently updated after extracting hooks
// - AC3: event-to-hook mapping is not written to settings.json
// - AC4: idempotency (safe to run twice) — not tested/implemented
// - AC5: existing unrelated keys in settings.json are preserved

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/convergent-systems-co/aiConstitution/src/cmd/ai/cmd"
)

// expectedHookMapping is the authoritative event→hook mapping per stories #84+#96.
// PreToolUse has two entries: one for all tools (audit, worktree-guard, secret-block)
// and one Bash-only matcher entry (branch-guard).
var expectedHookMapping = map[string][]string{
	"SessionStart":     {"audit.py"},
	"UserPromptSubmit": {"audit.py"},
	"PostToolUse":      {"audit.py"},
	"Stop":             {"audit.py", "checkpoint-tick.py"},
	"SessionEnd":       {"audit.py"},
	"SubagentStop":     {"audit.py"},
	"PreCompact":       {"audit.py"},
	// PreToolUse has two hook groups; tested separately below
}

// hookConfig mirrors the Claude Code settings.json hook entry structure.
type hookConfig struct {
	Type    string `json:"type"`
	Command string `json:"command"`
}

type hookGroup struct {
	Matcher string       `json:"matcher,omitempty"`
	Hooks   []hookConfig `json:"hooks"`
}

type settingsHooks struct {
	Hooks map[string][]hookGroup `json:"hooks"`
}

func runInstallAll(t *testing.T, aiRoot string) {
	t.Helper()
	root := cmd.NewRootCmd()
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"hooks", "install", "--all"})
	t.Setenv("AI_ROOT", aiRoot)
	if err := root.Execute(); err != nil {
		t.Fatalf("hooks install --all failed: %v\noutput: %s", err, buf)
	}
}

func readSettings(t *testing.T, aiRoot string) map[string]any {
	t.Helper()
	// Claude Code settings.json lives at ~/.claude/settings.json in production,
	// but the install command must use CLAUDE_SETTINGS_PATH env var or derive
	// from the user's home. For tests, we check that the command respects
	// CLAUDE_CONFIG_DIR or writes to a known test path.
	//
	// The install command is expected to write to:
	//   ${CLAUDE_CONFIG_DIR}/settings.json  (if CLAUDE_CONFIG_DIR is set)
	//   ${HOME}/.claude/settings.json       (fallback)
	//
	// Tests set HOME to a temp dir via t.TempDir to avoid clobbering real settings.
	homePath := os.Getenv("HOME")
	settingsPath := filepath.Join(homePath, ".claude", "settings.json")
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("settings.json not found at %s: %v", settingsPath, err)
	}
	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("settings.json is not valid JSON: %v\ncontent: %s", err, data)
	}
	return result
}

// TestHooksInstallCreatesSettingsJSON verifies AC2: after install --all,
// ~/.claude/settings.json contains a "hooks" section.
func TestHooksInstallCreatesSettingsJSON(t *testing.T) {
	homeDir := t.TempDir()
	aiRoot := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("AI_ROOT", aiRoot)

	runInstallAll(t, aiRoot)

	settings := readSettings(t, aiRoot)
	_, hasHooks := settings["hooks"]
	if !hasHooks {
		t.Error("settings.json does not contain a 'hooks' key after install --all")
	}
}

// TestHooksInstallSessionStartWiring verifies AC3: SessionStart → audit.py.
func TestHooksInstallSessionStartWiring(t *testing.T) {
	homeDir := t.TempDir()
	aiRoot := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("AI_ROOT", aiRoot)

	runInstallAll(t, aiRoot)

	settings := readSettings(t, aiRoot)
	hooks, _ := settings["hooks"].(map[string]any)
	if hooks == nil {
		t.Fatal("hooks key missing or not an object")
	}
	sessionStart, ok := hooks["SessionStart"]
	if !ok {
		t.Fatal("SessionStart key missing from hooks")
	}
	// Must contain at least one hook group with audit.py
	raw, _ := json.Marshal(sessionStart)
	if !strings.Contains(string(raw), "audit.py") {
		t.Errorf("SessionStart hooks do not contain audit.py\ncontent: %s", raw)
	}
}

// TestHooksInstallPreToolUseWiring verifies AC3: PreToolUse has branch-guard, secret-block, worktree-guard.
func TestHooksInstallPreToolUseWiring(t *testing.T) {
	homeDir := t.TempDir()
	aiRoot := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("AI_ROOT", aiRoot)

	runInstallAll(t, aiRoot)

	settings := readSettings(t, aiRoot)
	hooks, _ := settings["hooks"].(map[string]any)
	if hooks == nil {
		t.Fatal("hooks key missing")
	}
	preToolUse, ok := hooks["PreToolUse"]
	if !ok {
		t.Fatal("PreToolUse key missing from hooks")
	}
	raw, _ := json.Marshal(preToolUse)
	rawStr := string(raw)

	requiredHooks := []string{"audit.py", "secret-block.py", "worktree-guard.py", "branch-guard.py"}
	for _, required := range requiredHooks {
		if !strings.Contains(rawStr, required) {
			t.Errorf("PreToolUse hooks missing %s\ncontent: %s", required, rawStr)
		}
	}
}

// TestHooksInstallBranchGuardHasBashMatcher verifies AC3: branch-guard.py is
// registered with matcher="Bash" under PreToolUse.
func TestHooksInstallBranchGuardHasBashMatcher(t *testing.T) {
	homeDir := t.TempDir()
	aiRoot := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("AI_ROOT", aiRoot)

	runInstallAll(t, aiRoot)

	settings := readSettings(t, aiRoot)
	raw, _ := json.Marshal(settings["hooks"])
	rawStr := string(raw)
	// The branch-guard entry must be paired with a "Bash" matcher
	if !strings.Contains(rawStr, `"matcher":"Bash"`) && !strings.Contains(rawStr, `"matcher": "Bash"`) {
		t.Errorf("branch-guard.py must be registered with matcher=Bash\nfull hooks: %s", rawStr)
	}
}

// TestHooksInstallStopWiring verifies AC3: Stop → audit.py + checkpoint-tick.py.
func TestHooksInstallStopWiring(t *testing.T) {
	homeDir := t.TempDir()
	aiRoot := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("AI_ROOT", aiRoot)

	runInstallAll(t, aiRoot)

	settings := readSettings(t, aiRoot)
	hooks, _ := settings["hooks"].(map[string]any)
	stop := hooks["Stop"]
	raw, _ := json.Marshal(stop)
	rawStr := string(raw)
	if !strings.Contains(rawStr, "audit.py") {
		t.Errorf("Stop hooks missing audit.py: %s", rawStr)
	}
	if !strings.Contains(rawStr, "checkpoint-tick.py") {
		t.Errorf("Stop hooks missing checkpoint-tick.py: %s", rawStr)
	}
}

// TestHooksInstallIdempotent verifies AC4: running install twice does not corrupt settings.
func TestHooksInstallIdempotent(t *testing.T) {
	homeDir := t.TempDir()
	aiRoot := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("AI_ROOT", aiRoot)

	runInstallAll(t, aiRoot)
	firstSettings := readSettings(t, aiRoot)
	firstRaw, _ := json.Marshal(firstSettings)

	runInstallAll(t, aiRoot)
	secondSettings := readSettings(t, aiRoot)
	secondRaw, _ := json.Marshal(secondSettings)

	if string(firstRaw) != string(secondRaw) {
		t.Errorf("settings.json changed between two installs (not idempotent)\nfirst: %s\nsecond: %s", firstRaw, secondRaw)
	}
}

// TestHooksInstallPreservesExistingKeys verifies AC5: unrelated keys in settings.json
// are not removed by the install command.
func TestHooksInstallPreservesExistingKeys(t *testing.T) {
	homeDir := t.TempDir()
	aiRoot := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("AI_ROOT", aiRoot)

	// Pre-seed settings.json with an unrelated key
	claudeDir := filepath.Join(homeDir, ".claude")
	if err := os.MkdirAll(claudeDir, 0o750); err != nil {
		t.Fatalf("mkdir %s: %v", claudeDir, err)
	}
	existing := map[string]any{
		"model":               "claude-sonnet",
		"someOtherKey":        "preserve-me",
		"todoFeatureEnabled":  false,
	}
	existingData, _ := json.MarshalIndent(existing, "", "  ")
	if err := os.WriteFile(filepath.Join(claudeDir, "settings.json"), existingData, 0o644); err != nil {
		t.Fatalf("write settings.json: %v", err)
	}

	runInstallAll(t, aiRoot)

	settings := readSettings(t, aiRoot)
	if v, ok := settings["someOtherKey"]; !ok || fmt.Sprintf("%v", v) != "preserve-me" {
		t.Errorf("install --all removed or changed 'someOtherKey': got %v", settings["someOtherKey"])
	}
	if v, ok := settings["model"]; !ok || fmt.Sprintf("%v", v) != "claude-sonnet" {
		t.Errorf("install --all removed or changed 'model': got %v", settings["model"])
	}
}

// TestHooksInstallAllEventsPresent verifies all 8 required event types are wired.
func TestHooksInstallAllEventsPresent(t *testing.T) {
	homeDir := t.TempDir()
	aiRoot := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("AI_ROOT", aiRoot)

	runInstallAll(t, aiRoot)

	settings := readSettings(t, aiRoot)
	hooks, _ := settings["hooks"].(map[string]any)
	requiredEvents := []string{
		"SessionStart", "UserPromptSubmit", "PreToolUse", "PostToolUse",
		"Stop", "SessionEnd", "SubagentStop", "PreCompact",
	}
	for _, event := range requiredEvents {
		if _, ok := hooks[event]; !ok {
			t.Errorf("hooks section missing required event: %s", event)
		}
	}
}
