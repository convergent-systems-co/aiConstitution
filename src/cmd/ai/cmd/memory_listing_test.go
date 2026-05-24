package cmd_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/convergent-systems-co/aiConstitution/src/cmd/ai/cmd"
)

// runRootCmd runs the full root command with args and returns combined output.
// Mirrors the style already used by memory_test.go for consistency.
func runRootCmd(t *testing.T, args ...string) (string, error) {
	t.Helper()
	root := cmd.NewRootCmd()
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs(args)
	err := root.Execute()
	return buf.String(), err
}

// setupMemoryFixture wires AI_ROOT to a fresh tmp dir with a populated
// memory/ subtree: one feedback file and a MEMORY.md pointer.
func setupMemoryFixture(t *testing.T, name, body string) string {
	t.Helper()
	aiRoot := t.TempDir()
	t.Setenv("AI_ROOT", aiRoot)
	memDir := filepath.Join(aiRoot, "memory")
	if err := os.MkdirAll(memDir, 0o750); err != nil {
		t.Fatalf("mkdir memory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(memDir, name+".md"), []byte(body), 0o600); err != nil {
		t.Fatalf("write memory file: %v", err)
	}
	pointer := "- [" + name + "](" + name + ".md)\n"
	if err := os.WriteFile(filepath.Join(memDir, "MEMORY.md"), []byte(pointer), 0o600); err != nil {
		t.Fatalf("write MEMORY.md: %v", err)
	}
	return aiRoot
}

func TestMemoryList_PrintsMEMORYContents(t *testing.T) {
	setupMemoryFixture(t, "feedback_tui_ascii_only", "## body\n")
	out, err := runRootCmd(t, "memory", "list")
	if err != nil {
		t.Fatalf("memory list: %v\noutput:\n%s", err, out)
	}
	if !strings.Contains(out, "feedback_tui_ascii_only") {
		t.Errorf("want list output to include pointer; got:\n%s", out)
	}
}

func TestMemoryList_NoMemoriesWhenAbsent(t *testing.T) {
	aiRoot := t.TempDir()
	t.Setenv("AI_ROOT", aiRoot)
	out, err := runRootCmd(t, "memory", "list")
	if err != nil {
		t.Fatalf("memory list: %v\noutput:\n%s", err, out)
	}
	if !strings.Contains(out, "(no memories)") {
		t.Errorf("want (no memories); got:\n%s", out)
	}
}

func TestMemoryShow_PrintsFileContents(t *testing.T) {
	body := "## body\n\nthis is the memory body.\n"
	setupMemoryFixture(t, "feedback_thing", body)
	out, err := runRootCmd(t, "memory", "show", "feedback_thing")
	if err != nil {
		t.Fatalf("memory show: %v\noutput:\n%s", err, out)
	}
	if !strings.Contains(out, "this is the memory body.") {
		t.Errorf("want body in output; got:\n%s", out)
	}
}

func TestMemoryShow_ErrorWhenMissing(t *testing.T) {
	aiRoot := t.TempDir()
	t.Setenv("AI_ROOT", aiRoot)
	_ = os.MkdirAll(filepath.Join(aiRoot, "memory"), 0o750)
	_, err := runRootCmd(t, "memory", "show", "nonexistent")
	if err == nil {
		t.Errorf("want error for missing memory file, got nil")
	}
}

func TestMemoryArchive_MovesFileAndPrunesPointer(t *testing.T) {
	aiRoot := setupMemoryFixture(t, "feedback_thing", "## body\n")

	if _, err := runRootCmd(t, "memory", "archive", "feedback_thing"); err != nil {
		t.Fatalf("memory archive: %v", err)
	}

	memDir := filepath.Join(aiRoot, "memory")
	// Original file must be gone.
	if _, err := os.Stat(filepath.Join(memDir, "feedback_thing.md")); !os.IsNotExist(err) {
		t.Errorf("original file should be removed; stat err = %v", err)
	}
	// Archive copy must exist.
	archived := filepath.Join(memDir, "archived", "feedback_thing.md")
	if _, err := os.Stat(archived); err != nil {
		t.Errorf("archived copy missing: %v", err)
	}
	// MEMORY.md must no longer mention the archived name.
	mem, err := os.ReadFile(filepath.Join(memDir, "MEMORY.md")) //nolint:gosec // test path
	if err != nil {
		t.Fatalf("read MEMORY.md: %v", err)
	}
	if strings.Contains(string(mem), "feedback_thing") {
		t.Errorf("MEMORY.md still references archived entry; content:\n%s", mem)
	}
}

func TestMemoryArchive_ErrorWhenMissing(t *testing.T) {
	aiRoot := t.TempDir()
	t.Setenv("AI_ROOT", aiRoot)
	_ = os.MkdirAll(filepath.Join(aiRoot, "memory"), 0o750)
	_, err := runRootCmd(t, "memory", "archive", "missing-name")
	if err == nil {
		t.Errorf("want error for missing memory file, got nil")
	}
}
