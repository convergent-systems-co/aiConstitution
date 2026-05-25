package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// modeTestEnv sets up a temp ConfigDir and injects it via AICONST_CONFIG_DIR.
// Returns the config dir path.
func modeTestEnv(t *testing.T) (configDir string) {
	t.Helper()
	tmp := t.TempDir()
	t.Setenv("AICONST_CONFIG_DIR", tmp)
	return tmp
}

// execMode runs the mode (or pm-mode) subcommand with args and captures stdout.
func execMode(t *testing.T, args ...string) (stdout string, err error) {
	t.Helper()
	var buf bytes.Buffer
	root := NewRootCmd()
	root.SetOut(&buf)
	root.SetErr(&bytes.Buffer{})
	root.SetArgs(args)
	err = root.Execute()
	return buf.String(), err
}

// ---------- #219 pm-mode ----------

func TestPmMode_WritesCorrectModeJSON(t *testing.T) {
	configDir := modeTestEnv(t)

	_, err := execMode(t, "pm-mode")
	if err != nil {
		t.Fatalf("pm-mode returned error: %v", err)
	}

	modeFile := filepath.Join(configDir, "mode.json")
	data, readErr := os.ReadFile(modeFile)
	if readErr != nil {
		t.Fatalf("expected mode.json at %s, got: %v", modeFile, readErr)
	}

	var got map[string]interface{}
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("mode.json is not valid JSON: %v\ncontent: %s", err, string(data))
	}

	if got["mode"] != "pm" {
		t.Errorf("expected mode=pm, got mode=%v", got["mode"])
	}
	if got["discipline"] != "plan-first" {
		t.Errorf("expected discipline=plan-first, got discipline=%v", got["discipline"])
	}
	if _, ok := got["activatedAt"]; !ok {
		t.Errorf("expected activatedAt field in mode.json, got keys: %v", got)
	}
}

func TestPmMode_PrintsActivationMessage(t *testing.T) {
	_ = modeTestEnv(t)

	out, err := execMode(t, "pm-mode")
	if err != nil {
		t.Fatalf("pm-mode returned error: %v", err)
	}
	if !strings.Contains(out, "PM mode activated") {
		t.Errorf("expected activation message, got:\n%s", out)
	}
	if !strings.Contains(out, "plan-first") {
		t.Errorf("expected 'plan-first' in output, got:\n%s", out)
	}
}
