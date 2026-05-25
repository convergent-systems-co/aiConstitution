package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// personaTestEnv sets up a temp AI_ROOT with a personas/ subdirectory.
// Returns the personas dir path. Uses t.Setenv for automatic cleanup.
func personaTestEnv(t *testing.T) (personasDir string) {
	t.Helper()
	tmp := t.TempDir()
	personasDir = filepath.Join(tmp, "personas")
	if err := os.MkdirAll(personasDir, 0o750); err != nil {
		t.Fatalf("mkdir personas: %v", err)
	}
	t.Setenv("AI_ROOT", tmp)
	return personasDir
}

// writePersonaYAML writes a persona YAML file to dir/<name>.yaml.
func writePersonaYAML(t *testing.T, dir, name, personaType, description string) {
	t.Helper()
	content := "name: " + name + "\ntype: " + personaType + "\ndescription: \"" + description + "\"\n"
	if err := os.WriteFile(filepath.Join(dir, name+".yaml"), []byte(content), 0o644); err != nil {
		t.Fatalf("write persona yaml: %v", err)
	}
}

// execPersona runs the persona subcommand with args and captures stdout.
func execPersona(t *testing.T, args ...string) (stdout string, err error) {
	t.Helper()
	var buf bytes.Buffer
	root := NewRootCmd()
	root.SetOut(&buf)
	root.SetErr(&bytes.Buffer{})
	root.SetArgs(append([]string{"persona"}, args...))
	err = root.Execute()
	return buf.String(), err
}

// ---------- #217 persona list ----------

func TestPersonaList_ShowsNameTypeDescription(t *testing.T) {
	dir := personaTestEnv(t)
	writePersonaYAML(t, dir, "coder", "agentic", "A coding assistant")
	writePersonaYAML(t, dir, "critic", "reviewer", "A code reviewer")

	out, err := execPersona(t, "list")
	if err != nil {
		t.Fatalf("persona list returned error: %v", err)
	}
	if !strings.Contains(out, "coder") {
		t.Errorf("expected 'coder' in output, got:\n%s", out)
	}
	if !strings.Contains(out, "agentic") {
		t.Errorf("expected 'agentic' in output, got:\n%s", out)
	}
	if !strings.Contains(out, "A coding assistant") {
		t.Errorf("expected description in output, got:\n%s", out)
	}
	if !strings.Contains(out, "critic") {
		t.Errorf("expected 'critic' in output, got:\n%s", out)
	}
	if !strings.Contains(out, "reviewer") {
		t.Errorf("expected 'reviewer' in output, got:\n%s", out)
	}
}

func TestPersonaList_TypeFilter_Agentic(t *testing.T) {
	dir := personaTestEnv(t)
	writePersonaYAML(t, dir, "coder", "agentic", "A coding assistant")
	writePersonaYAML(t, dir, "critic", "reviewer", "A code reviewer")

	out, err := execPersona(t, "list", "--type", "agentic")
	if err != nil {
		t.Fatalf("persona list --type agentic returned error: %v", err)
	}
	if !strings.Contains(out, "coder") {
		t.Errorf("expected 'coder' in filtered output, got:\n%s", out)
	}
	if strings.Contains(out, "critic") {
		t.Errorf("expected 'critic' to be filtered out, got:\n%s", out)
	}
}

func TestPersonaList_TypeFilter_Reviewer(t *testing.T) {
	dir := personaTestEnv(t)
	writePersonaYAML(t, dir, "coder", "agentic", "A coding assistant")
	writePersonaYAML(t, dir, "critic", "reviewer", "A code reviewer")

	out, err := execPersona(t, "list", "--type", "reviewer")
	if err != nil {
		t.Fatalf("persona list --type reviewer returned error: %v", err)
	}
	if strings.Contains(out, "coder") {
		t.Errorf("expected 'coder' to be filtered out, got:\n%s", out)
	}
	if !strings.Contains(out, "critic") {
		t.Errorf("expected 'critic' in filtered output, got:\n%s", out)
	}
}

func TestPersonaList_Empty_PrintsNotice(t *testing.T) {
	_ = personaTestEnv(t) // creates empty personas dir

	out, err := execPersona(t, "list")
	if err != nil {
		t.Fatalf("persona list returned error: %v", err)
	}
	if !strings.Contains(out, "(no personas installed)") {
		t.Errorf("expected '(no personas installed)', got:\n%s", out)
	}
}

func TestPersonaList_DirNotExist_PrintsNotice(t *testing.T) {
	tmp := t.TempDir()
	// AI_ROOT without a personas/ subdir
	t.Setenv("AI_ROOT", tmp)

	out, err := execPersona(t, "list")
	if err != nil {
		t.Fatalf("persona list returned error: %v", err)
	}
	if !strings.Contains(out, "(no personas installed)") {
		t.Errorf("expected '(no personas installed)' when dir missing, got:\n%s", out)
	}
}

// ---------- #218 persona show ----------

func TestPersonaShow_PrintsContent(t *testing.T) {
	dir := personaTestEnv(t)
	writePersonaYAML(t, dir, "mycoder", "agentic", "A great coder")

	out, err := execPersona(t, "show", "mycoder")
	if err != nil {
		t.Fatalf("persona show returned error: %v", err)
	}
	if !strings.Contains(out, "name: mycoder") {
		t.Errorf("expected file content in output, got:\n%s", out)
	}
	if !strings.Contains(out, "A great coder") {
		t.Errorf("expected description in output, got:\n%s", out)
	}
}

func TestPersonaShow_NotFound_ReturnsError(t *testing.T) {
	_ = personaTestEnv(t)

	_, err := execPersona(t, "show", "ghost")
	if err == nil {
		t.Fatal("expected error for missing persona, got nil")
	}
}
