package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// modeListTestEnv sets up both AICONST_CONFIG_DIR and AI_ROOT in temp dirs,
// creates the expected subdirs, and returns (configDir, aiRoot).
func modeListTestEnv(t *testing.T) (configDir, aiRoot string) {
	t.Helper()
	cfgTmp := t.TempDir()
	aiTmp := t.TempDir()
	t.Setenv("AICONST_CONFIG_DIR", cfgTmp)
	t.Setenv("AI_ROOT", aiTmp)
	return cfgTmp, aiTmp
}

// writePersonaYAMLForMode writes a minimal persona YAML under aiRoot/personas/<name>.yaml.
func writePersonaYAMLForMode(t *testing.T, aiRoot, name, kind string) {
	t.Helper()
	dir := filepath.Join(aiRoot, "personas")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatalf("mkdir personas: %v", err)
	}
	content := "name: " + name + "\ntype: " + kind + "\n"
	if err := os.WriteFile(filepath.Join(dir, name+".yaml"), []byte(content), 0o644); err != nil {
		t.Fatalf("write persona yaml: %v", err)
	}
}

// writeProfileYAMLForMode writes a minimal profile YAML under configDir/profiles/<name>.yaml.
func writeProfileYAMLForMode(t *testing.T, configDir, name string) {
	t.Helper()
	dir := filepath.Join(configDir, "profiles")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatalf("mkdir profiles: %v", err)
	}
	content := "name: " + name + "\ndescription: \"test profile\"\n"
	if err := os.WriteFile(filepath.Join(dir, name+".yaml"), []byte(content), 0o644); err != nil {
		t.Fatalf("write profile yaml: %v", err)
	}
}

// writeModeJSON writes a mode.json to configDir/mode.json.
func writeModeJSON(t *testing.T, configDir, mode string) {
	t.Helper()
	data, err := json.Marshal(map[string]string{
		"mode":        mode,
		"activatedAt": "2026-01-01T00:00:00Z",
		"discipline":  "plan-first",
	})
	if err != nil {
		t.Fatalf("marshal mode.json: %v", err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "mode.json"), data, 0o644); err != nil {
		t.Fatalf("write mode.json: %v", err)
	}
}

// ---------- mode current ----------

func TestModeCurrent_NoFile(t *testing.T) {
	modeListTestEnv(t) // no mode.json written

	out, err := execMode(t, "mode", "current")
	if err != nil {
		t.Fatalf("mode current returned error: %v", err)
	}
	if !strings.Contains(out, "(none)") {
		t.Errorf("expected '(none)' when mode.json absent, got:\n%s", out)
	}
}

func TestModeCurrent_WithMode(t *testing.T) {
	configDir, _ := modeListTestEnv(t)
	writeModeJSON(t, configDir, "pm")

	out, err := execMode(t, "mode", "current")
	if err != nil {
		t.Fatalf("mode current returned error: %v", err)
	}
	if !strings.Contains(out, "pm") {
		t.Errorf("expected 'pm' in output, got:\n%s", out)
	}
}

// ---------- mode list ----------

func TestModeList_EmptyDirs(t *testing.T) {
	modeListTestEnv(t) // dirs exist but are empty

	out, err := execMode(t, "mode", "list")
	if err != nil {
		t.Fatalf("mode list returned error: %v", err)
	}
	if !strings.Contains(out, "(no modes available)") {
		t.Errorf("expected '(no modes available)', got:\n%s", out)
	}
}

func TestModeList_WithPersonas(t *testing.T) {
	configDir, aiRoot := modeListTestEnv(t)
	writePersonaYAMLForMode(t, aiRoot, "coder", "agentic")
	writeProfileYAMLForMode(t, configDir, "frontend")

	out, err := execMode(t, "mode", "list")
	if err != nil {
		t.Fatalf("mode list returned error: %v", err)
	}
	if !strings.Contains(out, "coder") {
		t.Errorf("expected 'coder' in output, got:\n%s", out)
	}
	if !strings.Contains(out, "persona") {
		t.Errorf("expected 'persona' type in output, got:\n%s", out)
	}
	if !strings.Contains(out, "frontend") {
		t.Errorf("expected 'frontend' in output, got:\n%s", out)
	}
	if !strings.Contains(out, "profile") {
		t.Errorf("expected 'profile' type in output, got:\n%s", out)
	}
}

// ---------- mode show ----------

func TestModeShow_Found(t *testing.T) {
	configDir, aiRoot := modeListTestEnv(t)
	writePersonaYAMLForMode(t, aiRoot, "mymode", "agentic")
	_ = configDir

	out, err := execMode(t, "mode", "show", "mymode")
	if err != nil {
		t.Fatalf("mode show returned error: %v", err)
	}
	if !strings.Contains(out, "name: mymode") {
		t.Errorf("expected persona content in output, got:\n%s", out)
	}
}

func TestModeShow_NotFound(t *testing.T) {
	modeListTestEnv(t)

	_, err := execMode(t, "mode", "show", "ghost")
	if err == nil {
		t.Fatal("expected error for missing mode, got nil")
	}
}
