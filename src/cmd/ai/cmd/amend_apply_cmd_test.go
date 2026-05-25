package cmd_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/convergent-systems-co/aiConstitution/src/cmd/ai/cmd"
)

func TestAmendApplyEndToEnd(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("AI_ROOT", dir)
	t.Setenv("AICONST_CONFIG_DIR", dir)
	t.Setenv("EDITOR", "")

	// Write a constitution file with a target section
	constitutionContent := `# AI Constitution

**Version:** 1.0

## §3.U17 — Worktree placement

Original body text here.

## §3.U18 Other rule

Other content.

## Changelog

- **1.0** — initial
`
	if err := os.WriteFile(filepath.Join(dir, "Constitution.md"), []byte(constitutionContent), 0o600); err != nil {
		t.Fatal(err)
	}

	// 1) Create a violation file
	violationContent := `# Violation — 2026-05-24T120000Z

- **Tool / Agent:** Claude Code
- **File / Rule violated:** §3.U17 — Worktree placement
- **What happened:** Worktree created at ad-hoc path.
- **How noticed:** self-detected
- **Remediation:** moved to canonical path
- **Proposed amendment (if any):** Strengthen rule to forbid all non-canonical placement.
`
	violationPath := filepath.Join(dir, "violation.md")
	if err := os.WriteFile(violationPath, []byte(violationContent), 0o600); err != nil {
		t.Fatal(err)
	}

	// 2) Draft
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
	if err != nil || len(entries) == 0 {
		t.Fatalf("no draft created")
	}
	draftPath := filepath.Join(plansDir, entries[0].Name())

	// 3) Apply
	root2 := cmd.NewRootCmd()
	buf2 := &bytes.Buffer{}
	root2.SetOut(buf2)
	root2.SetErr(buf2)
	root2.SetArgs([]string{"amend", "apply", draftPath})
	if err := root2.Execute(); err != nil {
		t.Fatalf("amend apply error: %v\noutput:%s", err, buf2)
	}

	// Verify constitution was updated
	updated, err := os.ReadFile(filepath.Join(dir, "Constitution.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(updated), "Strengthen rule") {
		t.Errorf("expected amendment applied to constitution, got:\n%s", string(updated))
	}
}
