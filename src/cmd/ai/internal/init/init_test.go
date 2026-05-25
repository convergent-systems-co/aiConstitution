package init_test

import (
	"os"
	"path/filepath"
	"testing"

	initpkg "github.com/convergent-systems-co/aiConstitution/src/cmd/ai/internal/init"
)

func TestEnsureToolFilesCreatesAllThree(t *testing.T) {
	dir := t.TempDir()
	written, err := initpkg.EnsureToolFiles(dir)
	if err != nil {
		t.Fatalf("EnsureToolFiles error: %v", err)
	}
	if len(written) != 3 {
		t.Errorf("written = %d files, want 3 (%v)", len(written), written)
	}
	for _, rel := range []string{"CLAUDE.md", ".github/copilot-instructions.md", "AGENTS.md"} {
		p := filepath.Join(dir, rel)
		if _, err := os.Stat(p); err != nil {
			t.Errorf("missing %s: %v", p, err)
		}
	}
}

func TestEnsureToolFilesPreservesExisting(t *testing.T) {
	dir := t.TempDir()
	custom := []byte("# Custom CLAUDE.md — do not overwrite\n")
	if err := os.WriteFile(filepath.Join(dir, "CLAUDE.md"), custom, 0o600); err != nil {
		t.Fatal(err)
	}

	written, err := initpkg.EnsureToolFiles(dir)
	if err != nil {
		t.Fatalf("EnsureToolFiles error: %v", err)
	}
	if len(written) != 2 {
		t.Errorf("written = %d files, want 2 (CLAUDE.md should be skipped)", len(written))
	}

	got, err := os.ReadFile(filepath.Join(dir, "CLAUDE.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(custom) {
		t.Errorf("CLAUDE.md was overwritten — got %q, want %q", got, custom)
	}
}

func TestEnsureToolFilesIdempotentReRun(t *testing.T) {
	dir := t.TempDir()
	if _, err := initpkg.EnsureToolFiles(dir); err != nil {
		t.Fatal(err)
	}
	written, err := initpkg.EnsureToolFiles(dir)
	if err != nil {
		t.Fatalf("second EnsureToolFiles error: %v", err)
	}
	if len(written) != 0 {
		t.Errorf("second run wrote %d files, want 0 (%v)", len(written), written)
	}
}

func TestEnsureToolFilesEmptyRootError(t *testing.T) {
	_, err := initpkg.EnsureToolFiles("")
	if err == nil {
		t.Error("expected error for empty aiRoot")
	}
}
