package cmd_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/convergent-systems-co/aiConstitution/src/cmd/ai/cmd"
)

// writeViolationFile creates a minimal violation markdown file at path and
// returns it. Matches the WriteViolation output format.
func writeViolationFile(t *testing.T, dir, rule, what, how, remediation string) string {
	t.Helper()
	ts := "20260524T120000Z"
	content := "# Violation — " + ts + "\n\n" +
		"- **File / Rule violated:** " + rule + "\n" +
		"- **What happened:** " + what + "\n" +
		"- **How noticed:** " + how + "\n" +
		"- **Remediation:** " + remediation + "\n" +
		"- **Proposed amendment (if any):** \n"

	path := filepath.Join(dir, ts+"-test-violation.md")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("writeViolationFile: %v", err)
	}
	return path
}

func TestMemoryCodify_PrintsCreatedPath(t *testing.T) {
	aiRoot := t.TempDir()
	t.Setenv("AI_ROOT", aiRoot)

	// Create a scratch dir for the violation file (not inside aiRoot).
	scratch := t.TempDir()
	vpath := writeViolationFile(t, scratch,
		"Code.md §11.3 — refactor-protocol",
		"Refactor included a bug fix.",
		"self-detected",
		"Separated the fix into its own commit.",
	)

	root := cmd.NewRootCmd()
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"memory", "codify", vpath})

	if err := root.Execute(); err != nil {
		t.Fatalf("memory codify error = %v; output:\n%s", err, buf)
	}

	out := buf.String()
	if !strings.Contains(out, "feedback_") {
		t.Errorf("expected output to contain 'feedback_' path; got:\n%s", out)
	}
	if !strings.Contains(out, ".md") {
		t.Errorf("expected output to contain '.md'; got:\n%s", out)
	}
}

func TestMemoryCodify_CreatesMemoryFile(t *testing.T) {
	aiRoot := t.TempDir()
	t.Setenv("AI_ROOT", aiRoot)

	scratch := t.TempDir()
	rule := "Common.md §2.2 — destructive-action"
	what := "Dropped a table without approval."
	vpath := writeViolationFile(t, scratch, rule, what, "self-detected", "Restored.")

	root := cmd.NewRootCmd()
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"memory", "codify", vpath})

	if err := root.Execute(); err != nil {
		t.Fatalf("memory codify error = %v; output:\n%s", err, buf)
	}

	// Verify a feedback_ file was created in the memory dir.
	memDir := filepath.Join(aiRoot, "memory")
	entries, err := os.ReadDir(memDir)
	if err != nil {
		t.Fatalf("ReadDir(%q) error = %v", memDir, err)
	}

	var feedbackFiles []string
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "feedback_") {
			feedbackFiles = append(feedbackFiles, e.Name())
		}
	}
	if len(feedbackFiles) == 0 {
		t.Fatalf("expected at least one feedback_ file in %q; found: %v", memDir, entries)
	}

	// Read the file and verify it contains the rule.
	data, err := os.ReadFile(filepath.Join(memDir, feedbackFiles[0]))
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if !strings.Contains(string(data), rule) {
		t.Errorf("feedback file missing rule %q; content:\n%s", rule, string(data))
	}
}

func TestMemoryCodify_UpdatesMEMORYmd(t *testing.T) {
	aiRoot := t.TempDir()
	t.Setenv("AI_ROOT", aiRoot)

	scratch := t.TempDir()
	vpath := writeViolationFile(t, scratch,
		"Code.md §11.2 — squash-merging",
		"Squash-merged a non-release branch.",
		"user-flagged",
		"Reverted and re-merged with merge commit.",
	)

	root := cmd.NewRootCmd()
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"memory", "codify", vpath})
	if err := root.Execute(); err != nil {
		t.Fatalf("memory codify error = %v", err)
	}

	memMD := filepath.Join(aiRoot, "memory", "MEMORY.md")
	data, err := os.ReadFile(memMD)
	if err != nil {
		t.Fatalf("ReadFile(MEMORY.md) error = %v", err)
	}
	if !strings.Contains(string(data), "feedback_") {
		t.Errorf("MEMORY.md does not contain pointer; content:\n%s", string(data))
	}
}

func TestMemoryCodify_ErrorOnMissingFile(t *testing.T) {
	aiRoot := t.TempDir()
	t.Setenv("AI_ROOT", aiRoot)

	root := cmd.NewRootCmd()
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"memory", "codify", "/nonexistent/path/violation.md"})

	err := root.Execute()
	if err == nil {
		t.Error("expected error for missing file, got nil")
	}
}
