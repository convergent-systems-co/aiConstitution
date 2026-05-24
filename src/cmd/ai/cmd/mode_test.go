package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// modeTestEnv sets AI_ROOT + AICONST_CONFIG_DIR to fresh temp dirs and
// drops a single persona (`debugger.md`) into governance/personas/agentic/.
// Returns the AI root for further mutation by the test.
func modeTestEnv(t *testing.T) string {
	t.Helper()
	aiRoot := t.TempDir()
	cfg := t.TempDir()
	t.Setenv("AI_ROOT", aiRoot)
	t.Setenv("AICONST_CONFIG_DIR", cfg)

	personasDir := filepath.Join(aiRoot, "governance", "personas", "agentic")
	if err := os.MkdirAll(personasDir, 0o750); err != nil {
		t.Fatalf("mkdir personas: %v", err)
	}
	if err := os.WriteFile(filepath.Join(personasDir, "debugger.md"), []byte("# debugger\n"), 0o600); err != nil {
		t.Fatalf("write persona: %v", err)
	}
	return aiRoot
}

func runMode(t *testing.T, args ...string) string {
	t.Helper()
	buf := &bytes.Buffer{}
	root := newModeCmd()
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs(args)
	if err := root.Execute(); err != nil {
		t.Fatalf("mode %v: %v", args, err)
	}
	return buf.String()
}

func TestModeListShowsShippedPersonas(t *testing.T) {
	modeTestEnv(t)
	out := runMode(t, "list")
	if !strings.Contains(out, "debugger") {
		t.Errorf("want list output to include debugger, got:\n%s", out)
	}
}

func TestModeCurrentDefaultIsNone(t *testing.T) {
	modeTestEnv(t)
	out := runMode(t, "current")
	if !strings.Contains(out, "(none)") {
		t.Errorf("want current to be (none), got:\n%s", out)
	}
}

func TestModeActivateThenCurrentReturnsName(t *testing.T) {
	modeTestEnv(t)
	out := runMode(t, "debugger")
	if !strings.Contains(out, "Mode set: debugger") {
		t.Errorf("want activate confirmation, got:\n%s", out)
	}

	out = runMode(t, "current")
	if !strings.Contains(out, "debugger") {
		t.Errorf("want current=debugger, got:\n%s", out)
	}
}

func TestModeClearResetsCurrent(t *testing.T) {
	modeTestEnv(t)
	_ = runMode(t, "debugger")
	_ = runMode(t, "clear")
	out := runMode(t, "current")
	if !strings.Contains(out, "(none)") {
		t.Errorf("want current=(none) after clear, got:\n%s", out)
	}
}

func TestModeActivateUnknownPersonaErrors(t *testing.T) {
	modeTestEnv(t)
	buf := &bytes.Buffer{}
	root := newModeCmd()
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"nonexistent"})
	if err := root.Execute(); err == nil {
		t.Errorf("want error activating unknown persona, got nil. output:\n%s", buf.String())
	}
}
