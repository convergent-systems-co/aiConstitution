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
