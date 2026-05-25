package cmd

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// violationStub returns the content of a minimal violation file that the
// amend draft command must be able to parse.
func violationStub(ruleRef, whatHappened, proposedAmendment string) string {
	return fmt.Sprintf(`# Violation — 2026-05-24T120000Z

- **Tool / Agent:** Claude Code
- **File / Rule violated:** %s
- **What happened:** %s
- **How noticed:** self-detected
- **Remediation:** pending
- **Proposed amendment (if any):** %s
`, ruleRef, whatHappened, proposedAmendment)
}

// constitutionStub returns a minimal Constitution.md fixture with a
// versioned header and a single section the apply command can patch.
func constitutionStub(version, sectionRef, sectionBody string) string {
	return fmt.Sprintf(`# CONSTITUTION.md — Governance

**Version:** %s

## %s

%s

## 2. Inheritance

Inheritance rules live here.

## Changelog

- **%s** — initial draft
`, version, sectionRef, sectionBody, version)
}

// ─── draft ────────────────────────────────────────────────────────────────────

// TestAmendDraftCreatesFile verifies that `ai amend draft <violation-path>`
// creates a plan file at $AI_ROOT/governance/plans/<UTC>-<slug>.md.
func TestAmendDraftCreatesFile(t *testing.T) {
	root := t.TempDir()
	t.Setenv("AI_ROOT", root)
	t.Setenv("EDITOR", "") // prevent editor from launching during test

	// Write a violation file to a temp location.
	vf := filepath.Join(t.TempDir(), "violation.md")
	if err := os.WriteFile(vf, []byte(violationStub(
		"Common.md/U17 — worktree placement",
		"A worktree was created in an ad-hoc location.",
		"Add explicit check for ad-hoc paths.",
	)), 0o644); err != nil {
		t.Fatalf("setup: write violation file: %v", err)
	}

	cmd := NewRootCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"amend", "draft", vf})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("amend draft returned error: %v", err)
	}

	plansDir := filepath.Join(root, "governance", "plans")
	entries, err := os.ReadDir(plansDir)
	if err != nil {
		t.Fatalf("plans dir not created at %s: %v", plansDir, err)
	}
	if len(entries) == 0 {
		t.Fatal("plans dir is empty — draft did not create a file")
	}

	// Exactly one file should exist.
	if len(entries) != 1 {
		t.Fatalf("expected 1 plan file; got %d", len(entries))
	}

	name := entries[0].Name()
	if !strings.HasSuffix(name, ".md") {
		t.Errorf("plan file %q does not have .md suffix", name)
	}

	// Slug must be derived from the rule reference (kebab-case).
	// "Common.md/U17 — worktree placement" → slug contains "common-md-u17"
	slug := strings.TrimSuffix(name, ".md")
	// Strip UTC prefix (first field is date-time, slug follows after first '-' at char 16+)
	// Name format: <UTC>-<slug>.md  e.g. 20260524T120000Z-common-md-u17....md
	if !strings.Contains(slug, "u17") && !strings.Contains(slug, "common") {
		t.Errorf("slug %q does not appear to be derived from rule ref", slug)
	}
}

// TestAmendDraftFileContent verifies the content format of the created plan.
func TestAmendDraftFileContent(t *testing.T) {
	root := t.TempDir()
	t.Setenv("AI_ROOT", root)
	t.Setenv("EDITOR", "") // prevent editor from launching

	proposed := "Rename the worktree guard hook for clarity."
	happened := "Ad-hoc worktree was created during testing."
	ruleRef := "Common.md/U17 — worktree-placement"

	vf := filepath.Join(t.TempDir(), "violation.md")
	if err := os.WriteFile(vf, []byte(violationStub(ruleRef, happened, proposed)), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	cmd := NewRootCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"amend", "draft", vf})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("amend draft error: %v", err)
	}

	plansDir := filepath.Join(root, "governance", "plans")
	entries, _ := os.ReadDir(plansDir)
	if len(entries) == 0 {
		t.Fatal("no plan file created")
	}

	content, err := os.ReadFile(filepath.Join(plansDir, entries[0].Name()))
	if err != nil {
		t.Fatalf("read plan file: %v", err)
	}

	body := string(content)
	for _, want := range []string{
		"# Amendment Draft",
		"## Target",
		"## Proposed Change",
		proposed,
		"## Rationale",
		happened,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("plan body missing %q\n---\n%s", want, body)
		}
	}
}

// TestAmendDraftPrintsPathWhenNoEditor verifies that when $EDITOR is unset
// the command prints the created file's path to stdout.
func TestAmendDraftPrintsPathWhenNoEditor(t *testing.T) {
	root := t.TempDir()
	t.Setenv("AI_ROOT", root)
	t.Setenv("EDITOR", "")

	vf := filepath.Join(t.TempDir(), "v.md")
	if err := os.WriteFile(vf, []byte(violationStub(
		"Code.md/§11.3 — refactor-protocol",
		"Refactor modified production behavior.",
		"Add guard clause.",
	)), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	cmd := NewRootCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"amend", "draft", vf})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("amend draft error: %v", err)
	}

	output := out.String()
	plansDir := filepath.Join(root, "governance", "plans")
	if !strings.Contains(output, plansDir) {
		t.Errorf("stdout %q does not contain plans dir path %q", output, plansDir)
	}
}

// ─── apply ────────────────────────────────────────────────────────────────────

// TestAmendApplyPatchesSection verifies that apply replaces the section body
// in Constitution.md.
func TestAmendApplyPatchesSection(t *testing.T) {
	root := t.TempDir()
	t.Setenv("AI_ROOT", root)

	// Write Constitution.md with a known section.
	origBody := "Original body content."
	constPath := filepath.Join(root, "Constitution.md")
	if err := os.WriteFile(constPath, []byte(constitutionStub(
		"1.0", "1. Prime Directives", origBody,
	)), 0o644); err != nil {
		t.Fatalf("setup: write Constitution.md: %v", err)
	}

	// Write a plan stub.
	newBody := "Updated prime directive text."
	planContent := fmt.Sprintf(`# Amendment Draft — prime-directives

## Target
1. Prime Directives

## Proposed Change
%s

## Rationale
Original text was insufficient.
`, newBody)

	plansDir := filepath.Join(root, "governance", "plans")
	if err := os.MkdirAll(plansDir, 0o755); err != nil {
		t.Fatalf("setup: mkdir plans: %v", err)
	}
	slug := "20260524T120000Z-prime-directives"
	planPath := filepath.Join(plansDir, slug+".md")
	if err := os.WriteFile(planPath, []byte(planContent), 0o644); err != nil {
		t.Fatalf("setup: write plan: %v", err)
	}

	cmd := NewRootCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"amend", "apply", slug})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("amend apply error: %v", err)
	}

	got, err := os.ReadFile(constPath)
	if err != nil {
		t.Fatalf("read Constitution.md: %v", err)
	}
	if !strings.Contains(string(got), newBody) {
		t.Errorf("Constitution.md does not contain new body %q\n---\n%s", newBody, string(got))
	}
	if strings.Contains(string(got), origBody) {
		t.Errorf("Constitution.md still contains original body %q after apply", origBody)
	}
}

// TestAmendApplyBumpsVersion verifies that apply increments the minor version.
func TestAmendApplyBumpsVersion(t *testing.T) {
	root := t.TempDir()
	t.Setenv("AI_ROOT", root)

	constPath := filepath.Join(root, "Constitution.md")
	if err := os.WriteFile(constPath, []byte(constitutionStub(
		"1.3", "1. Prime Directives", "Old body.",
	)), 0o644); err != nil {
		t.Fatalf("setup: write Constitution.md: %v", err)
	}

	plansDir := filepath.Join(root, "governance", "plans")
	if err := os.MkdirAll(plansDir, 0o755); err != nil {
		t.Fatalf("setup: mkdir plans: %v", err)
	}
	slug := "20260524T130000Z-prime-directives"
	planContent := `# Amendment Draft — prime-directives

## Target
1. Prime Directives

## Proposed Change
New body text.

## Rationale
Needed update.
`
	if err := os.WriteFile(filepath.Join(plansDir, slug+".md"), []byte(planContent), 0o644); err != nil {
		t.Fatalf("setup: write plan: %v", err)
	}

	cmd := NewRootCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"amend", "apply", slug})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("amend apply error: %v", err)
	}

	got, err := os.ReadFile(constPath)
	if err != nil {
		t.Fatalf("read Constitution.md: %v", err)
	}
	body := string(got)
	if !strings.Contains(body, "**Version:** 1.4") {
		t.Errorf("expected version bumped to 1.4; got:\n%s", body)
	}
	// Old version string should be gone.
	if strings.Contains(body, "**Version:** 1.3") {
		t.Errorf("old version 1.3 still present after apply:\n%s", body)
	}
}

// TestAmendApplyAppendsChangelog verifies that apply appends a changelog entry.
func TestAmendApplyAppendsChangelog(t *testing.T) {
	root := t.TempDir()
	t.Setenv("AI_ROOT", root)

	constPath := filepath.Join(root, "Constitution.md")
	if err := os.WriteFile(constPath, []byte(constitutionStub(
		"2.0", "1. Prime Directives", "Old body.",
	)), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	plansDir := filepath.Join(root, "governance", "plans")
	if err := os.MkdirAll(plansDir, 0o755); err != nil {
		t.Fatalf("setup: mkdir: %v", err)
	}
	slug := "20260524T140000Z-prime-directives"
	planContent := `# Amendment Draft — prime-directives

## Target
1. Prime Directives

## Proposed Change
Clarified prime directive wording.

## Rationale
Prior wording was ambiguous.
`
	if err := os.WriteFile(filepath.Join(plansDir, slug+".md"), []byte(planContent), 0o644); err != nil {
		t.Fatalf("setup: write plan: %v", err)
	}

	cmd := NewRootCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"amend", "apply", slug})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("amend apply error: %v", err)
	}

	got, err := os.ReadFile(constPath)
	if err != nil {
		t.Fatalf("read Constitution.md: %v", err)
	}
	body := string(got)
	// Changelog entry must contain new version and slug.
	if !strings.Contains(body, "**2.1**") {
		t.Errorf("no changelog entry for 2.1 found:\n%s", body)
	}
	if !strings.Contains(body, "prime-directives") {
		t.Errorf("changelog entry missing slug 'prime-directives':\n%s", body)
	}
}

// ─── list ─────────────────────────────────────────────────────────────────────

// TestAmendListNewestFirst verifies that `ai amend list` outputs plan files
// sorted newest-first based on the UTC-timestamp filename prefix.
func TestAmendListNewestFirst(t *testing.T) {
	root := t.TempDir()
	t.Setenv("AI_ROOT", root)

	plansDir := filepath.Join(root, "governance", "plans")
	if err := os.MkdirAll(plansDir, 0o755); err != nil {
		t.Fatalf("setup: mkdir: %v", err)
	}

	// Create three plans with different timestamps.
	files := []struct {
		name string
		body string
	}{
		{"20260520T000000Z-alpha-rule.md", "# Amendment Draft — alpha-rule\n\nOldest plan."},
		{"20260522T000000Z-beta-rule.md", "# Amendment Draft — beta-rule\n\nMiddle plan."},
		{"20260524T000000Z-gamma-rule.md", "# Amendment Draft — gamma-rule\n\nNewest plan."},
	}
	for _, f := range files {
		if err := os.WriteFile(filepath.Join(plansDir, f.name), []byte(f.body), 0o644); err != nil {
			t.Fatalf("setup: write %s: %v", f.name, err)
		}
	}

	cmd := NewRootCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"amend", "list"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("amend list error: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	// Expect 3 lines.
	if len(lines) != 3 {
		t.Fatalf("expected 3 output lines; got %d:\n%s", len(lines), out.String())
	}
	// First line (newest) must reference gamma.
	if !strings.Contains(lines[0], "gamma") {
		t.Errorf("line[0] (newest) expected to contain 'gamma'; got %q", lines[0])
	}
	// Last line (oldest) must reference alpha.
	if !strings.Contains(lines[2], "alpha") {
		t.Errorf("line[2] (oldest) expected to contain 'alpha'; got %q", lines[2])
	}
}

// TestAmendListEmptyDir verifies graceful output when no plans exist.
func TestAmendListEmptyDir(t *testing.T) {
	root := t.TempDir()
	t.Setenv("AI_ROOT", root)

	cmd := NewRootCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"amend", "list"})
	// Should not error — just print nothing (or "no plans").
	if err := cmd.Execute(); err != nil {
		t.Fatalf("amend list on empty dir returned error: %v", err)
	}
}

// ─── show ─────────────────────────────────────────────────────────────────────

// TestAmendShowBySlugPrefix verifies that `ai amend show <prefix>` finds and
// prints the matching plan file.
func TestAmendShowBySlugPrefix(t *testing.T) {
	root := t.TempDir()
	t.Setenv("AI_ROOT", root)

	plansDir := filepath.Join(root, "governance", "plans")
	if err := os.MkdirAll(plansDir, 0o755); err != nil {
		t.Fatalf("setup: mkdir: %v", err)
	}

	body := "# Amendment Draft — worktree-guard\n\nContent of the plan."
	if err := os.WriteFile(
		filepath.Join(plansDir, "20260524T120000Z-worktree-guard.md"),
		[]byte(body), 0o644,
	); err != nil {
		t.Fatalf("setup: write plan: %v", err)
	}

	cmd := NewRootCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	// Prefix match: "worktree" should find "worktree-guard".
	cmd.SetArgs([]string{"amend", "show", "worktree"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("amend show error: %v", err)
	}

	if !strings.Contains(out.String(), "worktree-guard") {
		t.Errorf("output does not contain slug; got:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "Content of the plan.") {
		t.Errorf("output does not contain plan content; got:\n%s", out.String())
	}
}

// TestAmendShowNotFound verifies that show returns an error when no match.
func TestAmendShowNotFound(t *testing.T) {
	root := t.TempDir()
	t.Setenv("AI_ROOT", root)

	cmd := NewRootCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"amend", "show", "nonexistent"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for missing slug; got nil")
	}
}

// ─── publish --dry-run ────────────────────────────────────────────────────────

// TestAmendPublishDryRunPrintsCommand verifies that `ai amend publish --dry-run`
// outputs the would-be gh release create command without executing it.
func TestAmendPublishDryRunPrintsCommand(t *testing.T) {
	root := t.TempDir()
	t.Setenv("AI_ROOT", root)

	// Write Constitution.md so publish can read its version.
	constPath := filepath.Join(root, "Constitution.md")
	if err := os.WriteFile(constPath, []byte(constitutionStub(
		"3.2", "1. Prime Directives", "Current prime directives.",
	)), 0o644); err != nil {
		t.Fatalf("setup: write Constitution.md: %v", err)
	}

	// Write a plan that matches the applied section.
	plansDir := filepath.Join(root, "governance", "plans")
	if err := os.MkdirAll(plansDir, 0o755); err != nil {
		t.Fatalf("setup: mkdir plans: %v", err)
	}
	slug := "20260524T120000Z-prime-directives"
	planContent := `# Amendment Draft — prime-directives

## Target
1. Prime Directives

## Proposed Change
Current prime directives.

## Rationale
Already applied.
`
	if err := os.WriteFile(filepath.Join(plansDir, slug+".md"), []byte(planContent), 0o644); err != nil {
		t.Fatalf("setup: write plan: %v", err)
	}

	cmd := NewRootCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"amend", "publish", "--dry-run", slug})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("amend publish --dry-run error: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "gh release create") {
		t.Errorf("dry-run output does not contain 'gh release create'; got:\n%s", output)
	}
	if !strings.Contains(output, "v3.2") {
		t.Errorf("dry-run output does not contain version 'v3.2'; got:\n%s", output)
	}
}

// ─── update --migrate ─────────────────────────────────────────────────────────

// TestUpdateMigrateAlreadyV2 verifies that --migrate reports "Already v2"
// when Constitution.md exists at $AI_ROOT.
func TestUpdateMigrateAlreadyV2(t *testing.T) {
	root := t.TempDir()
	t.Setenv("AI_ROOT", root)

	// Presence of Constitution.md signals v2 layout.
	if err := os.WriteFile(
		filepath.Join(root, "Constitution.md"),
		[]byte("**Version:** 0.3\n"),
		0o644,
	); err != nil {
		t.Fatalf("setup: %v", err)
	}

	cmd := NewRootCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"update", "--migrate", "--non-interactive"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("update --migrate error: %v", err)
	}

	if !strings.Contains(out.String(), "v2") {
		t.Errorf("expected 'v2' in output; got:\n%s", out.String())
	}
}

// TestUpdateMigrateV1DetectsLayout verifies that --migrate detects v1 layout
// and (in --non-interactive mode) prints what it would do.
func TestUpdateMigrateV1DetectsLayout(t *testing.T) {
	root := t.TempDir()
	t.Setenv("AI_ROOT", root)
	// No Constitution.md → v1 layout.

	cmd := NewRootCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"update", "--migrate", "--non-interactive"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("update --migrate v1 error: %v", err)
	}

	output := out.String()
	// Should mention v1 or migration.
	if !strings.Contains(strings.ToLower(output), "migrat") {
		t.Errorf("expected migration output; got:\n%s", output)
	}
}

// ─── helpers ──────────────────────────────────────────────────────────────────

// TestSlugDerivedFromRule is a unit-level check of slug derivation logic.
// Since derivation is internal, we test it indirectly via draft output.
func TestSlugDerivedFromRule(t *testing.T) {
	root := t.TempDir()
	t.Setenv("AI_ROOT", root)
	t.Setenv("EDITOR", "")

	vf := filepath.Join(t.TempDir(), "v.md")
	// Rule ref with special chars: slashes, dots, em-dash.
	if err := os.WriteFile(vf, []byte(violationStub(
		"Code.md/§11.8 — agentic dispatch protocol",
		"Step skipped.",
		"Add mandatory check.",
	)), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	cmd := NewRootCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"amend", "draft", vf})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("amend draft error: %v", err)
	}

	plansDir := filepath.Join(root, "governance", "plans")
	entries, _ := os.ReadDir(plansDir)
	if len(entries) == 0 {
		t.Fatal("no file created")
	}

	name := entries[0].Name()
	// Slug must be kebab-case: no slashes, no dots, no spaces, no em-dash.
	// Strip .md before checking — the extension is not part of the slug.
	nameWithoutExt := strings.TrimSuffix(name, ".md")
	for _, bad := range []string{"/", ".", " ", "—", "§"} {
		if strings.Contains(nameWithoutExt, bad) {
			t.Errorf("filename (without .md) %q contains invalid char %q", nameWithoutExt, bad)
		}
	}
	// Max 32 chars after the UTC prefix (UTC prefix is ~16 chars + dash).
	// File name format: <UTC>-<slug>.md
	// UTC part: 20260524T120000Z = 16 chars + "-" = 17.
	parts := strings.SplitN(strings.TrimSuffix(name, ".md"), "-", 2)
	if len(parts) < 2 {
		t.Fatalf("filename %q has no slug part", name)
	}
	slugPart := parts[1]
	if len(slugPart) > 32 {
		t.Errorf("slug part %q is longer than 32 chars (%d)", slugPart, len(slugPart))
	}
}

// Compile-time check: time is used in the test helper (UTC timestamp).
var _ = time.Now
