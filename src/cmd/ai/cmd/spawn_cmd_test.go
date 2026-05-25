package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// spawnTestEnv sets up a temp AI_ROOT with a personas/ subdir and a temp
// ConfigDir for mode.json/spawn.json. Returns (personasDir, configDir).
func spawnTestEnv(t *testing.T) (personasDir, configDir string) {
	t.Helper()
	aiRoot := t.TempDir()
	personasDir = filepath.Join(aiRoot, "personas")
	if err := os.MkdirAll(personasDir, 0o750); err != nil {
		t.Fatalf("mkdir personas: %v", err)
	}
	t.Setenv("AI_ROOT", aiRoot)

	configDir = t.TempDir()
	t.Setenv("AICONST_CONFIG_DIR", configDir)
	return personasDir, configDir
}

// writePersonaForSpawn writes a minimal persona YAML file.
func writePersonaForSpawn(t *testing.T, dir, name string) {
	t.Helper()
	content := "name: " + name + "\ntype: agentic\ndescription: \"Spawn test persona\"\n"
	if err := os.WriteFile(filepath.Join(dir, name+".yaml"), []byte(content), 0o644); err != nil {
		t.Fatalf("write persona: %v", err)
	}
}

// execSpawn runs ai spawn with args and captures stdout.
func execSpawn(t *testing.T, args ...string) (stdout string, err error) {
	t.Helper()
	var buf bytes.Buffer
	root := NewRootCmd()
	root.SetOut(&buf)
	root.SetErr(&bytes.Buffer{})
	root.SetArgs(append([]string{"spawn"}, args...))
	err = root.Execute()
	return buf.String(), err
}

// ---------- #220 spawn ----------

func TestSpawn_WritesSpawnJSON(t *testing.T) {
	personasDir, configDir := spawnTestEnv(t)
	writePersonaForSpawn(t, personasDir, "coder")

	_, err := execSpawn(t, "coder")
	if err != nil {
		t.Fatalf("spawn returned error: %v", err)
	}

	spawnFile := filepath.Join(configDir, "state", "spawn.json")
	data, readErr := os.ReadFile(spawnFile)
	if readErr != nil {
		t.Fatalf("expected spawn.json at %s, got: %v", spawnFile, readErr)
	}

	var got map[string]interface{}
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("spawn.json is not valid JSON: %v\ncontent: %s", err, string(data))
	}

	if got["persona"] != "coder" {
		t.Errorf("expected persona=coder, got persona=%v", got["persona"])
	}
	if _, ok := got["spawnedAt"]; !ok {
		t.Errorf("expected spawnedAt field in spawn.json, got keys: %v", got)
	}
	if _, ok := got["parentMode"]; !ok {
		t.Errorf("expected parentMode field in spawn.json, got keys: %v", got)
	}
}

func TestSpawn_PrintsPersonaContentAsMarkdown(t *testing.T) {
	personasDir, _ := spawnTestEnv(t)
	writePersonaForSpawn(t, personasDir, "coder")

	out, err := execSpawn(t, "coder")
	if err != nil {
		t.Fatalf("spawn returned error: %v", err)
	}
	// Should include persona file content in activation block
	if !strings.Contains(out, "coder") {
		t.Errorf("expected persona name in output, got:\n%s", out)
	}
	// Should include some markdown structure
	if !strings.Contains(out, "#") && !strings.Contains(out, "```") {
		t.Errorf("expected markdown in output, got:\n%s", out)
	}
}

func TestSpawn_WithParentMode_RecordsMode(t *testing.T) {
	personasDir, configDir := spawnTestEnv(t)
	writePersonaForSpawn(t, personasDir, "reviewer")

	// Write a mode.json to simulate active mode
	modeContent := `{"mode":"pm","activatedAt":"2026-05-24T00:00:00Z","discipline":"plan-first"}`
	if err := os.WriteFile(filepath.Join(configDir, "mode.json"), []byte(modeContent), 0o644); err != nil {
		t.Fatalf("write mode.json: %v", err)
	}

	_, err := execSpawn(t, "reviewer")
	if err != nil {
		t.Fatalf("spawn returned error: %v", err)
	}

	spawnFile := filepath.Join(configDir, "state", "spawn.json")
	data, readErr := os.ReadFile(spawnFile)
	if readErr != nil {
		t.Fatalf("expected spawn.json at %s: %v", spawnFile, readErr)
	}

	var got map[string]interface{}
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("spawn.json invalid JSON: %v", err)
	}
	if got["parentMode"] != "pm" {
		t.Errorf("expected parentMode=pm, got parentMode=%v", got["parentMode"])
	}
}

func TestSpawn_PersonaNotFound_ReturnsError(t *testing.T) {
	_, _ = spawnTestEnv(t)

	_, err := execSpawn(t, "ghost")
	if err == nil {
		t.Fatal("expected error for missing persona in spawn, got nil")
	}
}
