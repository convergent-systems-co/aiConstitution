package memory_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/convergent-systems-co/aiConstitution/src/internal/memory"
)

// --- WriteFinding tests ---

func TestWriteFinding_CreatesFile(t *testing.T) {
	root := t.TempDir()

	path, err := memory.WriteFinding(
		root,
		"Code.md §11.3 — refactor-protocol",
		"A refactor included a bug fix.",
		"Separated the fix into its own commit.",
	)
	if err != nil {
		t.Fatalf("WriteFinding() error = %v", err)
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatalf("expected file %q to exist", path)
	}
}

func TestWriteFinding_ReturnedPathIsUnderRoot(t *testing.T) {
	root := t.TempDir()

	path, err := memory.WriteFinding(root, "some-rule", "happened", "fixed")
	if err != nil {
		t.Fatalf("WriteFinding() error = %v", err)
	}
	if !strings.HasPrefix(path, root) {
		t.Errorf("path %q does not start with root %q", path, root)
	}
}

func TestWriteFinding_FilenameSlugFromRule(t *testing.T) {
	root := t.TempDir()

	path, err := memory.WriteFinding(root, "Code.md §11.3 Refactor Rule", "w", "r")
	if err != nil {
		t.Fatalf("WriteFinding() error = %v", err)
	}
	base := filepath.Base(path)

	// Must start with "feedback_" and end with ".md".
	if !strings.HasPrefix(base, "feedback_") {
		t.Errorf("filename %q does not start with 'feedback_'", base)
	}
	if !strings.HasSuffix(base, ".md") {
		t.Errorf("filename %q does not end with '.md'", base)
	}
	// Slug must be lowercase with no raw spaces.
	slug := strings.TrimSuffix(strings.TrimPrefix(base, "feedback_"), ".md")
	if strings.Contains(slug, " ") {
		t.Errorf("slug %q contains spaces", slug)
	}
	if slug != strings.ToLower(slug) {
		t.Errorf("slug %q is not lowercase", slug)
	}
}

func TestWriteFinding_SlugMaxLength(t *testing.T) {
	root := t.TempDir()
	// A rule longer than 32 chars.
	rule := "Code.md §11.3 This Is A Very Long Rule Name That Exceeds Thirty-Two Characters"
	path, err := memory.WriteFinding(root, rule, "w", "r")
	if err != nil {
		t.Fatalf("WriteFinding() error = %v", err)
	}
	base := filepath.Base(path)
	slug := strings.TrimSuffix(strings.TrimPrefix(base, "feedback_"), ".md")
	if len(slug) > 32 {
		t.Errorf("slug %q exceeds 32 characters (len=%d)", slug, len(slug))
	}
}

func TestWriteFinding_ContainsFrontmatter(t *testing.T) {
	root := t.TempDir()
	rule := "Common.md §2.2 — destructive-action"
	what := "Dropped a table without approval."
	rem := "Restored from backup."

	path, err := memory.WriteFinding(root, rule, what, rem)
	if err != nil {
		t.Fatalf("WriteFinding() error = %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", path, err)
	}
	body := string(data)

	// Frontmatter delimiters.
	if !strings.HasPrefix(body, "---\n") {
		t.Errorf("expected frontmatter opening '---', got:\n%s", body[:min(len(body), 40)])
	}
	if !strings.Contains(body, "type: feedback") {
		t.Errorf("missing 'type: feedback' in frontmatter:\n%s", body)
	}
	if !strings.Contains(body, "rule:") {
		t.Errorf("missing 'rule:' in frontmatter:\n%s", body)
	}
}

func TestWriteFinding_ContainsBody(t *testing.T) {
	root := t.TempDir()
	rule := "Common.md §2.2 — destructive-action"
	what := "Dropped a table without approval."
	rem := "Restored from backup."

	path, err := memory.WriteFinding(root, rule, what, rem)
	if err != nil {
		t.Fatalf("WriteFinding() error = %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", path, err)
	}
	body := string(data)

	checks := []string{rule, what, rem}
	for _, c := range checks {
		if !strings.Contains(body, c) {
			t.Errorf("finding file missing %q\nFull content:\n%s", c, body)
		}
	}
}

func TestWriteFinding_AppendsMEMORYmd(t *testing.T) {
	root := t.TempDir()

	path, err := memory.WriteFinding(root, "my-rule", "what happened", "remediation")
	if err != nil {
		t.Fatalf("WriteFinding() error = %v", err)
	}

	memPath := filepath.Join(root, "MEMORY.md")
	data, err := os.ReadFile(memPath)
	if err != nil {
		t.Fatalf("ReadFile(MEMORY.md) error = %v; expected file to be created/updated", err)
	}

	// The pointer should include the filename.
	base := filepath.Base(path)
	if !strings.Contains(string(data), base) {
		t.Errorf("MEMORY.md does not contain pointer to %q\nContent:\n%s", base, string(data))
	}
}

func TestWriteFinding_AppendsMEMORYmd_MultipleEntries(t *testing.T) {
	root := t.TempDir()

	for i, rule := range []string{"rule-alpha", "rule-beta"} {
		if _, err := memory.WriteFinding(root, rule, "w", "r"); err != nil {
			t.Fatalf("WriteFinding() [%d] error = %v", i, err)
		}
	}

	memPath := filepath.Join(root, "MEMORY.md")
	data, err := os.ReadFile(memPath)
	if err != nil {
		t.Fatalf("ReadFile(MEMORY.md) error = %v", err)
	}
	if !strings.Contains(string(data), "rule-alpha") {
		t.Errorf("MEMORY.md missing first entry; content:\n%s", string(data))
	}
	if !strings.Contains(string(data), "rule-beta") {
		t.Errorf("MEMORY.md missing second entry; content:\n%s", string(data))
	}
}

// min is a local helper to avoid Go 1.21 stdlib dependency in older compiles.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
