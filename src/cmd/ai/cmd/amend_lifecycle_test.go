package cmd_test

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/convergent-systems-co/aiConstitution/src/cmd/ai/cmd"
)

func writeCompactLifecycleFixture(t *testing.T, dir string) {
	t.Helper()

	var body strings.Builder
	body.WriteString("# AI Constitution\n\n**Version:** 1.0\n\n## §3 Core Autonomous Rules\n\n")
	body.WriteString("### §3.1 Foundations\n\n")
	for i := 1; i <= 30; i++ {
		fmt.Fprintf(&body, "- **3.1.%d. Filler rule %d.** Filler text.\n\n", i, i)
	}
	body.WriteString("### §3.2 Autonomy Gates\n\n")
	for i := 1; i <= 6; i++ {
		fmt.Fprintf(&body, "- **3.2.%d. Gate rule %d.** Gate text.\n\n", i, i)
	}
	body.WriteString("### §3.3 Universal Operating Rules\n\n")
	body.WriteString("**U9. Self-knowledge limits.** Keep this rule.\n\n")
	body.WriteString("**U10. Tool handoff and checkpointing.** Existing U10 text.\n\n")
	body.WriteString("Supporting paragraph for U10.\n\n")
	body.WriteString("**U11. Self-correction on violation.** Keep this rule too.\n\n")
	body.WriteString("## Changelog\n\n- **1.0** — initial\n")

	if err := os.WriteFile(filepath.Join(dir, "Constitution.md"), []byte(body.String()), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestAmendDraftCanonicalizesCompactAuditTarget(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("AI_ROOT", dir)
	t.Setenv("AICONST_CONFIG_DIR", dir)
	t.Setenv("EDITOR", "")

	writeCompactLifecycleFixture(t, dir)

	violationContent := `# Violation — 2026-05-24T120000Z

- **Tool / Agent:** Claude Code
- **Section / Rule violated:** Constitution.md §3.38
- **What happened:** Handoff data was dropped between tools.
- **How noticed:** self-detected
- **Remediation:** rebuilt the missing context
- **Proposed amendment (if any):** Clarify the required checkpoint contents.
`
	violationPath := filepath.Join(dir, "violation.md")
	if err := os.WriteFile(violationPath, []byte(violationContent), 0o600); err != nil {
		t.Fatal(err)
	}

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
		t.Fatalf("expected a draft plan, got err=%v entries=%d", err, len(entries))
	}

	draftPath := filepath.Join(plansDir, entries[0].Name())
	draft, err := os.ReadFile(draftPath)
	if err != nil {
		t.Fatal(err)
	}
	content := string(draft)
	if !strings.Contains(content, "File: Constitution.md") {
		t.Fatalf("draft target did not record canonical file:\n%s", content)
	}
	if !strings.Contains(content, "Section: §3.3 Universal Operating Rules") {
		t.Fatalf("draft target did not resolve canonical section:\n%s", content)
	}
	if !strings.Contains(content, "Rule: U10") {
		t.Fatalf("draft target did not resolve canonical rule:\n%s", content)
	}
}

func TestAmendApplyLegacyCompactTargetAppendsWithinRule(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("AI_ROOT", dir)
	t.Setenv("AICONST_CONFIG_DIR", dir)
	t.Setenv("EDITOR", "")

	writeCompactLifecycleFixture(t, dir)

	plansDir := filepath.Join(dir, "governance", "plans")
	if err := os.MkdirAll(plansDir, 0o755); err != nil {
		t.Fatal(err)
	}

	planPath := filepath.Join(plansDir, "20260524T120000Z-tool-handoff.md")
	planContent := `# Amendment Draft — tool-handoff

## Target
Constitution.md §3.38

## Proposed Change
Clarify that the handoff must include active worktree paths and the last verified commit.

## Rationale
Compact rule references should still resolve to the canonical rule body.
`
	if err := os.WriteFile(planPath, []byte(planContent), 0o600); err != nil {
		t.Fatal(err)
	}

	root := cmd.NewRootCmd()
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"amend", "apply", planPath})
	if err := root.Execute(); err != nil {
		t.Fatalf("amend apply error: %v\noutput:%s", err, buf)
	}

	updated, err := os.ReadFile(filepath.Join(dir, "Constitution.md"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(updated)
	if !strings.Contains(text, "**U9. Self-knowledge limits.** Keep this rule.") {
		t.Fatalf("apply removed the preceding rule:\n%s", text)
	}
	if !strings.Contains(text, "**U10. Tool handoff and checkpointing.** Existing U10 text.") {
		t.Fatalf("apply removed the targeted rule body:\n%s", text)
	}
	if !strings.Contains(text, "**U11. Self-correction on violation.** Keep this rule too.") {
		t.Fatalf("apply removed the following rule:\n%s", text)
	}

	insert := "Clarify that the handoff must include active worktree paths and the last verified commit."
	idxU10 := strings.Index(text, "**U10. Tool handoff and checkpointing.**")
	idxInsert := strings.Index(text, insert)
	idxU11 := strings.Index(text, "**U11. Self-correction on violation.**")
	if idxU10 < 0 || idxInsert < 0 || idxU11 < 0 || !(idxU10 < idxInsert && idxInsert < idxU11) {
		t.Fatalf("expected proposed change between U10 and U11, got:\n%s", text)
	}
}

func TestAmendApplyRejectsNonCanonicalTargetFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("AI_ROOT", dir)
	t.Setenv("AICONST_CONFIG_DIR", dir)
	t.Setenv("EDITOR", "")

	writeCompactLifecycleFixture(t, dir)

	plansDir := filepath.Join(dir, "governance", "plans")
	if err := os.MkdirAll(plansDir, 0o755); err != nil {
		t.Fatal(err)
	}

	planPath := filepath.Join(plansDir, "20260524T120000Z-bad-target.md")
	planContent := `# Amendment Draft — bad-target

## Target
File: ../secrets.txt
Section: §3.3 Universal Operating Rules
Rule: U10

## Proposed Change
This should never be written.

## Rationale
Defense in depth.
`
	if err := os.WriteFile(planPath, []byte(planContent), 0o600); err != nil {
		t.Fatal(err)
	}

	root := cmd.NewRootCmd()
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"amend", "apply", planPath})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected amend apply to reject non-canonical target file")
	}
	if !strings.Contains(err.Error(), "not a canonical governance file") && !strings.Contains(err.Error(), "not a supported canonical governance file") {
		t.Fatalf("unexpected error: %v", err)
	}
}
