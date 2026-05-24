package amend_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/convergent-systems-co/aiConstitution/src/internal/amend"
)

// sampleCommon is a stub Common.md containing a version line, a few
// sections, and a Changelog. Mirrors the real file's shape closely
// enough for Apply to exercise its three mutations.
const sampleCommon = `# Common.md — Universal Operating Rules

**Version:** 0.17

## U17 Worktree placement

Some prose about worktrees.

## U18 Next thing

Some other prose.

## 6. Changelog

- **0.17** — Initial entry.
`

func TestLoadDraftRoundTrip(t *testing.T) {
	dir := t.TempDir()
	d := amend.Draft{
		File:           "Common.md",
		Section:        "U17",
		ProposedChange: "New rule body.",
		Rationale:      "Closes recurring issue.",
		AuditRef:       "audit/violations/foo.md",
	}
	path, err := amend.WriteDraft(d, dir)
	if err != nil {
		t.Fatal(err)
	}
	got, err := amend.LoadDraft(path)
	if err != nil {
		t.Fatalf("LoadDraft error: %v", err)
	}
	if got.File != d.File || got.Section != d.Section {
		t.Errorf("file/section mismatch: %+v", got)
	}
	if got.AuditRef != d.AuditRef {
		t.Errorf("AuditRef = %q, want %q", got.AuditRef, d.AuditRef)
	}
	if !strings.Contains(got.ProposedChange, "New rule body.") {
		t.Errorf("ProposedChange = %q", got.ProposedChange)
	}
}

func TestBumpVersionMinor(t *testing.T) {
	out, oldV, newV := amend.BumpVersion("**Version:** 0.17 (draft)\n")
	if oldV != "0.17" || newV != "0.18" {
		t.Errorf("got old=%q new=%q, want 0.17/0.18", oldV, newV)
	}
	if !strings.Contains(out, "**Version:** 0.18") {
		t.Errorf("bump did not appear in output: %q", out)
	}
}

func TestBumpVersionNoVersionLine(t *testing.T) {
	out, oldV, newV := amend.BumpVersion("no version here\n")
	if oldV != "" || newV != "" {
		t.Errorf("expected empty versions, got %q/%q", oldV, newV)
	}
	if out != "no version here\n" {
		t.Errorf("content mutated: %q", out)
	}
}

func TestAppendChangelogPrepends(t *testing.T) {
	content := "## Changelog\n\n- **0.17** — Initial.\n"
	out := amend.AppendChangelog(content, "- **0.18** — New entry.")
	// New entry should appear before old.
	idxNew := strings.Index(out, "0.18")
	idxOld := strings.Index(out, "0.17")
	if idxNew < 0 || idxOld < 0 || idxNew > idxOld {
		t.Errorf("ordering wrong; new must precede old:\n%s", out)
	}
}

func TestAppendChangelogCreatesSection(t *testing.T) {
	content := "Some doc with no changelog.\n"
	out := amend.AppendChangelog(content, "- **0.2** — First.")
	if !strings.Contains(out, "## Changelog") {
		t.Errorf("Changelog header not added:\n%s", out)
	}
}

func TestApplyEndToEnd(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "Common.md")
	if err := os.WriteFile(target, []byte(sampleCommon), 0o600); err != nil {
		t.Fatal(err)
	}

	d := amend.Draft{
		File:           "Common.md",
		Section:        "U17",
		ProposedChange: "Added detail about worktree-guard hook.",
		Rationale:      "Strengthen U17 with hook reference.",
		AuditRef:       "audit/violations/example.md",
	}
	res, err := amend.Apply(d, dir)
	if err != nil {
		t.Fatalf("Apply error: %v", err)
	}

	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	body := string(got)

	if !strings.Contains(body, "Added detail about worktree-guard hook.") {
		t.Errorf("proposed change not appended to section:\n%s", body)
	}
	if !strings.Contains(body, "**Version:** 0.18") {
		t.Errorf("version not bumped to 0.18:\n%s", body)
	}
	if !strings.Contains(body, "**0.18**") || !strings.Contains(body, "Strengthen U17 with hook reference") {
		t.Errorf("changelog entry missing:\n%s", body)
	}
	if !strings.Contains(body, "audit/violations/example.md") {
		t.Errorf("changelog missing audit_ref:\n%s", body)
	}

	if res.OldVersion != "0.17" || res.NewVersion != "0.18" {
		t.Errorf("result versions wrong: %+v", res)
	}
	if res.AuditPath == "" {
		t.Errorf("audit path not returned: %+v", res)
	}
	if _, err := os.Stat(res.AuditPath); err != nil {
		t.Errorf("audit file missing: %v", err)
	}
}

func TestApplyOnSectionNotFoundStillBumps(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "Common.md")
	if err := os.WriteFile(target, []byte(sampleCommon), 0o600); err != nil {
		t.Fatal(err)
	}
	d := amend.Draft{
		File:           "Common.md",
		Section:        "U99", // doesn't exist
		ProposedChange: "New section content.",
	}
	res, err := amend.Apply(d, dir)
	if err != nil {
		t.Fatalf("Apply error: %v", err)
	}
	if res.NewVersion != "0.18" {
		t.Errorf("version should still bump even if section not found: %s", res.NewVersion)
	}
}

func TestApplyRefusesMissingVersionLine(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "Common.md")
	if err := os.WriteFile(target, []byte("# Common\n\nNo version here.\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	d := amend.Draft{File: "Common.md", Section: "U17", ProposedChange: "x"}
	if _, err := amend.Apply(d, dir); err == nil {
		t.Error("expected error when target has no **Version:** line")
	}
}
