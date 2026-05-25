package cmd_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/convergent-systems-co/aiConstitution/src/cmd/ai/cmd"
)

// stubCommon is a Common.md fragment with the version line + a section
// that ai amend can target.
const stubCommon = `# Common.md

**Version:** 0.17

## U17 Worktree placement

Body text.

## Changelog

- **0.17** — Initial.
`

// makeViolationFile writes a minimal violation file to dir and returns the path.
func makeViolationFile(t *testing.T, dir, ruleRef, proposedAmendment string) string {
	t.Helper()
	content := "# Violation — 2026-05-24T120000Z\n\n" +
		"- **Tool / Agent:** Claude Code\n" +
		"- **File / Rule violated:** " + ruleRef + "\n" +
		"- **What happened:** test scenario\n" +
		"- **How noticed:** self-detected\n" +
		"- **Remediation:** pending\n" +
		"- **Proposed amendment (if any):** " + proposedAmendment + "\n"
	p := filepath.Join(dir, "violation.md")
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestAmendDraftWritesFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("AI_ROOT", dir)
	t.Setenv("AICONST_CONFIG_DIR", dir)
	t.Setenv("EDITOR", "") // no editor — print path mode

	violationPath := makeViolationFile(t, dir,
		"§3.U17 — Worktree placement",
		"Add enforcement check to ensure canonical paths.",
	)

	root := cmd.NewRootCmd()
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"amend", "draft", violationPath})
	if err := root.Execute(); err != nil {
		t.Fatalf("amend draft error: %v\noutput:%s", err, buf)
	}

	plansDir := filepath.Join(dir, "governance", "plans")
	entries, err := os.ReadDir(plansDir)
	if err != nil {
		t.Fatalf("read plans dir: %v", err)
	}
	if len(entries) == 0 {
		t.Fatalf("no draft created in %s", plansDir)
	}
	if buf.Len() == 0 {
		t.Errorf("expected draft path in output, got empty")
	}
}

func TestAmendDraftWarnsOnMissingSection(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("AI_ROOT", dir)
	t.Setenv("AICONST_CONFIG_DIR", dir)
	t.Setenv("EDITOR", "")

	// Violation file referencing a section that won't exist in constitution
	violationPath := makeViolationFile(t, dir,
		"§99.Nonexistent — does not exist",
		"Add the missing section.",
	)

	root := cmd.NewRootCmd()
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"amend", "draft", violationPath})
	// Should succeed (draft still created even without matching section)
	if err := root.Execute(); err != nil {
		t.Fatalf("amend draft error: %v\noutput:%s", err, buf)
	}
	// Draft should be created regardless
	plansDir := filepath.Join(dir, "governance", "plans")
	entries, _ := os.ReadDir(plansDir)
	if len(entries) == 0 {
		t.Error("expected draft to be created even for nonexistent section ref")
	}
}
