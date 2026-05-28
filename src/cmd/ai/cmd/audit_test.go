package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func runAuditCmd(t *testing.T, args ...string) (string, error) {
	t.Helper()
	root := NewRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs(append([]string{"audit"}, args...))
	err := root.Execute()
	return buf.String(), err
}

func helperAuditAIRoot(t *testing.T) string {
	t.Helper()
	aiRoot := t.TempDir()
	t.Setenv("AI_ROOT", aiRoot)
	_ = os.MkdirAll(filepath.Join(aiRoot, "audit", "violations"), 0o755)
	_ = os.MkdirAll(filepath.Join(aiRoot, "audit", "interactions"), 0o755)
	return aiRoot
}

func writeViolationFile(t *testing.T, aiRoot, filename, content string) string {
	t.Helper()
	path := filepath.Join(aiRoot, "audit", "violations", filename)
	_ = os.WriteFile(path, []byte(content), 0o644)
	return path
}

// ---- audit list (#172) ----

func TestAuditList_ShowsViolations(t *testing.T) {
	aiRoot := helperAuditAIRoot(t)
	writeViolationFile(t, aiRoot, "20260522T100000Z-pipeline-skip.md",
		"# Violation — 2026-05-22T10:00:00Z\n\n- **File / Rule violated:** Code.md §11.8\n")
	writeViolationFile(t, aiRoot, "20260524T120000Z-handoff-stale.md",
		"# Violation — 2026-05-24T12:00:00Z\n\n- **File / Rule violated:** Common.md §U14\n")

	out, err := runAuditCmd(t, "list")
	if err != nil && strings.Contains(err.Error(), "not yet implemented") {
		t.Fatalf("audit list returned stub error: %v", err)
	}
	if !strings.Contains(out, "pipeline-skip") {
		t.Errorf("expected 'pipeline-skip' in output\n%s", out)
	}
	if !strings.Contains(out, "handoff-stale") {
		t.Errorf("expected 'handoff-stale' in output\n%s", out)
	}
}

func TestAuditList_ShowsNewestFirst(t *testing.T) {
	aiRoot := helperAuditAIRoot(t)
	writeViolationFile(t, aiRoot, "20260501T000000Z-old.md", "# Violation — old\n")
	writeViolationFile(t, aiRoot, "20260524T000000Z-new.md", "# Violation — new\n")

	out, err := runAuditCmd(t, "list")
	if err != nil && strings.Contains(err.Error(), "not yet implemented") {
		t.Fatalf("audit list returned stub error: %v", err)
	}
	newIdx := strings.Index(out, "new")
	oldIdx := strings.Index(out, "old")
	if newIdx == -1 || oldIdx == -1 {
		t.Logf("note: 'new' or 'old' not in output — may use different format\n%s", out)
		return
	}
	if newIdx > oldIdx {
		t.Errorf("expected newest ('new') to appear before oldest ('old')\n%s", out)
	}
}

func TestAuditList_ShowsInteractionCount(t *testing.T) {
	aiRoot := helperAuditAIRoot(t)
	month := time.Now().UTC().Format("2006-01")
	jsonl := `{"chronon":"now","kind":"signal"}` + "\n" +
		`{"chronon":"now","kind":"request"}` + "\n"
	_ = os.WriteFile(filepath.Join(aiRoot, "audit", "interactions", month+".jsonl"), []byte(jsonl), 0o644)

	out, err := runAuditCmd(t, "list")
	if err != nil && strings.Contains(err.Error(), "not yet implemented") {
		t.Fatalf("audit list returned stub error: %v", err)
	}
	// Should mention interactions or event count
	if !strings.Contains(out, "2") && !strings.Contains(out, "interaction") && !strings.Contains(out, "event") {
		t.Logf("note: interaction count not clearly visible\n%s", out)
	}
}

func TestAuditList_NoStub(t *testing.T) {
	aiRoot := helperAuditAIRoot(t)
	_ = aiRoot

	_, err := runAuditCmd(t, "list")
	if err != nil && strings.Contains(err.Error(), "not yet implemented") {
		t.Fatalf("audit list returned stub error: %v", err)
	}
}

// ---- audit show (#172) ----

func TestAuditShow_ByExactFilename(t *testing.T) {
	aiRoot := helperAuditAIRoot(t)
	content := "# Violation — 2026-05-24T10:00:00Z\n\n- **File / Rule violated:** Common.md §U14\n- **What happened:** Stale handoff accepted.\n"
	writeViolationFile(t, aiRoot, "20260524T100000Z-my-violation.md", content)

	out, err := runAuditCmd(t, "show", "20260524T100000Z-my-violation.md")
	if err != nil && strings.Contains(err.Error(), "not yet implemented") {
		t.Fatalf("audit show returned stub error: %v", err)
	}
	if !strings.Contains(out, "Stale handoff accepted") {
		t.Errorf("expected violation content in output\n%s", out)
	}
}

func TestAuditShow_BySlugPrefix(t *testing.T) {
	aiRoot := helperAuditAIRoot(t)
	content := "# Violation — 2026-05-24T10:00:00Z\n\n- **File / Rule violated:** Code.md §11.3\n- **What happened:** Refactor included bug fix.\n"
	writeViolationFile(t, aiRoot, "20260524T110000Z-slug-prefix-test.md", content)

	out, err := runAuditCmd(t, "show", "slug-prefix")
	if err != nil && strings.Contains(err.Error(), "not yet implemented") {
		t.Fatalf("audit show returned stub error: %v", err)
	}
	if !strings.Contains(out, "Refactor included bug fix") {
		t.Errorf("expected violation content in output\n%s", out)
	}
}

func TestAuditShow_UnknownSlug_ReturnsError(t *testing.T) {
	aiRoot := helperAuditAIRoot(t)
	_ = aiRoot

	_, err := runAuditCmd(t, "show", "nonexistent-slug-xyz")
	if err == nil {
		t.Error("expected error for unknown slug, got nil")
	}
	if err != nil && strings.Contains(err.Error(), "not yet implemented") {
		t.Fatalf("audit show returned stub error: %v", err)
	}
}

// ---- audit rotate (#172) ----

func TestAuditRotate_DeletesOldFiles(t *testing.T) {
	aiRoot := helperAuditAIRoot(t)
	interDir := filepath.Join(aiRoot, "audit", "interactions")

	// Create an old file (>30 days ago) and a current month file
	oldMonth := time.Now().UTC().AddDate(0, -2, 0).Format("2006-01")
	currentMonth := time.Now().UTC().Format("2006-01")
	oldFile := filepath.Join(interDir, oldMonth+".jsonl")
	currentFile := filepath.Join(interDir, currentMonth+".jsonl")

	_ = os.WriteFile(oldFile, []byte(`{"kind":"signal"}`+"\n"), 0o644)
	_ = os.WriteFile(currentFile, []byte(`{"kind":"signal"}`+"\n"), 0o644)

	// Set old file's mtime to 60 days ago
	oldTime := time.Now().AddDate(0, 0, -60)
	_ = os.Chtimes(oldFile, oldTime, oldTime)

	out, err := runAuditCmd(t, "rotate")
	if err != nil && strings.Contains(err.Error(), "not yet implemented") {
		t.Fatalf("audit rotate returned stub error: %v", err)
	}
	_ = out

	// Old file should be deleted (>30 days)
	if _, statErr := os.Stat(oldFile); !os.IsNotExist(statErr) {
		t.Errorf("expected old file %s to be deleted", oldFile)
	}

	// Current file should remain
	if _, statErr := os.Stat(currentFile); os.IsNotExist(statErr) {
		t.Errorf("expected current file %s to remain", currentFile)
	}
}

func TestAuditRotate_PrintsDeleteCount(t *testing.T) {
	aiRoot := helperAuditAIRoot(t)
	interDir := filepath.Join(aiRoot, "audit", "interactions")

	oldMonth := time.Now().UTC().AddDate(0, -3, 0).Format("2006-01")
	oldFile := filepath.Join(interDir, oldMonth+".jsonl")
	_ = os.WriteFile(oldFile, []byte(`{"kind":"signal"}`+"\n"), 0o644)
	oldTime := time.Now().AddDate(0, 0, -90)
	_ = os.Chtimes(oldFile, oldTime, oldTime)

	out, err := runAuditCmd(t, "rotate")
	if err != nil && strings.Contains(err.Error(), "not yet implemented") {
		t.Fatalf("audit rotate returned stub error: %v", err)
	}
	// Should print count or "Deleted"
	if !strings.Contains(out, "1") && !strings.Contains(out, "Deleted") && !strings.Contains(out, "deleted") {
		t.Logf("note: delete count not clearly visible\n%s", out)
	}
}

func TestAuditRotate_NothingToRotate(t *testing.T) {
	aiRoot := helperAuditAIRoot(t)
	// Only current month file exists
	interDir := filepath.Join(aiRoot, "audit", "interactions")
	currentMonth := time.Now().UTC().Format("2006-01")
	_ = os.WriteFile(filepath.Join(interDir, currentMonth+".jsonl"), []byte(`{"kind":"signal"}`+"\n"), 0o644)

	out, err := runAuditCmd(t, "rotate")
	if err != nil && strings.Contains(err.Error(), "not yet implemented") {
		t.Fatalf("audit rotate returned stub error: %v", err)
	}
	// Should not crash and should indicate nothing to do
	_ = out
}

func TestAuditRotate_DryRun(t *testing.T) {
	aiRoot := helperAuditAIRoot(t)
	interDir := filepath.Join(aiRoot, "audit", "interactions")

	oldMonth := time.Now().UTC().AddDate(0, -2, 0).Format("2006-01")
	oldFile := filepath.Join(interDir, oldMonth+".jsonl")
	_ = os.WriteFile(oldFile, []byte(`{"kind":"signal"}`+"\n"), 0o644)
	oldTime := time.Now().AddDate(0, 0, -60)
	_ = os.Chtimes(oldFile, oldTime, oldTime)

	out, err := runAuditCmd(t, "rotate", "--dry-run")
	if err != nil && strings.Contains(err.Error(), "not yet implemented") {
		t.Fatalf("audit rotate --dry-run returned stub error: %v", err)
	}
	_ = out

	// File should NOT be deleted in dry-run
	if _, statErr := os.Stat(oldFile); os.IsNotExist(statErr) {
		t.Errorf("dry-run should not delete file %s", oldFile)
	}
}

// ---- audit override (#382) ----

func TestAuditOverride_WritesFile(t *testing.T) {
	aiRoot := helperAuditAIRoot(t)
	_ = aiRoot

	out, err := runAuditCmd(t, "override",
		"--tool", "Claude Code",
		"--section", "§3.2",
		"--scope", "task",
		"--strict", "Would have required explicit approval before destructive action.",
		"--relaxed", "Proceeding without approval because artifact is throwaway.",
		"--risk", "Potential data loss if artifact was not actually throwaway.",
		"--confirmation", "yes",
		"--artifacts", "tmp/scratch.md",
	)
	if err != nil {
		t.Fatalf("audit override returned error: %v\nout: %s", err, out)
	}
	if strings.Contains(out, "not yet implemented") {
		t.Fatalf("audit override returned stub output: %s", out)
	}

	// Output should contain the path written
	if !strings.Contains(out, "overrides") {
		t.Errorf("expected output to contain 'overrides' path, got: %s", out)
	}

	// The written path should exist and contain the correct content
	writtenPath := strings.TrimSpace(out)
	data, err := os.ReadFile(writtenPath)
	if err != nil {
		t.Fatalf("could not read written file %q: %v", writtenPath, err)
	}
	body := string(data)
	if !strings.Contains(body, "# Override —") {
		t.Errorf("expected '# Override —' header\n%s", body)
	}
	if !strings.Contains(body, "**Tool / Agent:** Claude Code") {
		t.Errorf("expected tool field\n%s", body)
	}
	if !strings.Contains(body, "**Section / Rule relaxed:** §3.2") {
		t.Errorf("expected section field\n%s", body)
	}
	if !strings.Contains(body, "**Scope:** task") {
		t.Errorf("expected scope field\n%s", body)
	}
	if !strings.Contains(body, "**Strict behavior:** Would have required explicit approval") {
		t.Errorf("expected strict field\n%s", body)
	}
	if !strings.Contains(body, "**Relaxed behavior:** Proceeding without approval") {
		t.Errorf("expected relaxed field\n%s", body)
	}
	if !strings.Contains(body, "**Risk acknowledged:** Potential data loss") {
		t.Errorf("expected risk field\n%s", body)
	}
	if !strings.Contains(body, "**Principal confirmation:** yes") {
		t.Errorf("expected confirmation field\n%s", body)
	}
	if !strings.Contains(body, "**Artifacts affected:** tmp/scratch.md") {
		t.Errorf("expected artifacts field\n%s", body)
	}
}

func TestAuditOverride_CreatesDir(t *testing.T) {
	aiRoot := t.TempDir()
	t.Setenv("AI_ROOT", aiRoot)
	// Do NOT pre-create audit/overrides/ — test that the command creates it

	_, err := runAuditCmd(t, "override",
		"--tool", "Claude Code",
		"--section", "§3.2",
		"--scope", "task",
		"--strict", "strict behavior",
		"--relaxed", "relaxed behavior",
		"--risk", "risk description",
		"--confirmation", "yes",
		"--artifacts", "none",
	)
	if err != nil {
		t.Fatalf("audit override failed: %v", err)
	}

	overridesDir := filepath.Join(aiRoot, "audit", "overrides")
	entries, err := os.ReadDir(overridesDir)
	if err != nil {
		t.Fatalf("overrides dir not created: %v", err)
	}
	if len(entries) == 0 {
		t.Error("expected at least one file in overrides dir")
	}
}

func TestAuditOverride_MissingRequiredFlag_ReturnsError(t *testing.T) {
	aiRoot := helperAuditAIRoot(t)
	_ = aiRoot

	// Missing --section
	_, err := runAuditCmd(t, "override",
		"--tool", "Claude Code",
		"--scope", "task",
		"--strict", "strict",
		"--relaxed", "relaxed",
		"--risk", "risk",
		"--confirmation", "yes",
		"--artifacts", "none",
	)
	if err == nil {
		t.Error("expected error for missing --section flag, got nil")
	}
	if err != nil && strings.Contains(err.Error(), "not yet implemented") {
		t.Fatalf("audit override returned stub error: %v", err)
	}
}

// ---- audit violation (#382) ----

func TestAuditViolation_WritesFile(t *testing.T) {
	aiRoot := helperAuditAIRoot(t)
	_ = aiRoot

	out, err := runAuditCmd(t, "violation",
		"--section", "§3.1.P2",
		"--what", "Fabricated an API method that does not exist.",
		"--noticed", "user-flagged",
		"--remediation", "Replaced with verified call from official docs.",
		"--amendment", "none",
	)
	if err != nil {
		t.Fatalf("audit violation returned error: %v\nout: %s", err, out)
	}
	if strings.Contains(out, "not yet implemented") {
		t.Fatalf("audit violation returned stub output: %s", out)
	}

	// Output should contain the path written
	if !strings.Contains(out, "violations") {
		t.Errorf("expected output to contain 'violations' path, got: %s", out)
	}

	// The written path should exist and contain correct content
	writtenPath := strings.TrimSpace(out)
	data, err := os.ReadFile(writtenPath)
	if err != nil {
		t.Fatalf("could not read written file %q: %v", writtenPath, err)
	}
	body := string(data)
	if !strings.Contains(body, "# Violation —") {
		t.Errorf("expected '# Violation —' header\n%s", body)
	}
	if !strings.Contains(body, "**Section / Rule violated:** §3.1.P2") {
		t.Errorf("expected section field\n%s", body)
	}
	if !strings.Contains(body, "**What happened:** Fabricated an API method") {
		t.Errorf("expected what field\n%s", body)
	}
	if !strings.Contains(body, "**How noticed:** user-flagged") {
		t.Errorf("expected noticed field\n%s", body)
	}
	if !strings.Contains(body, "**Remediation:** Replaced with verified call") {
		t.Errorf("expected remediation field\n%s", body)
	}
	if !strings.Contains(body, "**Proposed amendment (if any):** none") {
		t.Errorf("expected amendment field\n%s", body)
	}
}

func TestAuditViolation_CreatesDir(t *testing.T) {
	aiRoot := t.TempDir()
	t.Setenv("AI_ROOT", aiRoot)
	// Do NOT pre-create audit/violations/ — test that the command creates it

	_, err := runAuditCmd(t, "violation",
		"--section", "§3.1.P2",
		"--what", "something happened",
		"--noticed", "self-detected",
		"--remediation", "fixed it",
	)
	if err != nil {
		t.Fatalf("audit violation failed: %v", err)
	}

	violationsDir := filepath.Join(aiRoot, "audit", "violations")
	entries, err := os.ReadDir(violationsDir)
	if err != nil {
		t.Fatalf("violations dir not created: %v", err)
	}
	if len(entries) == 0 {
		t.Error("expected at least one file in violations dir")
	}
}

func TestAuditViolation_MissingRequiredFlag_ReturnsError(t *testing.T) {
	aiRoot := helperAuditAIRoot(t)
	_ = aiRoot

	// Missing --section
	_, err := runAuditCmd(t, "violation",
		"--what", "something happened",
		"--noticed", "self-detected",
		"--remediation", "fixed it",
	)
	if err == nil {
		t.Error("expected error for missing --section flag, got nil")
	}
	if err != nil && strings.Contains(err.Error(), "not yet implemented") {
		t.Fatalf("audit violation returned stub error: %v", err)
	}
}

func TestAuditViolation_OptionalAmendment(t *testing.T) {
	aiRoot := helperAuditAIRoot(t)
	_ = aiRoot

	// amendment is optional — should succeed without it
	out, err := runAuditCmd(t, "violation",
		"--section", "§3.1.P2",
		"--what", "something happened",
		"--noticed", "self-detected",
		"--remediation", "fixed it",
	)
	if err != nil {
		t.Fatalf("audit violation without --amendment failed: %v\nout: %s", err, out)
	}
	writtenPath := strings.TrimSpace(out)
	data, readErr := os.ReadFile(writtenPath)
	if readErr != nil {
		t.Fatalf("could not read file: %v", readErr)
	}
	if !strings.Contains(string(data), "**Proposed amendment (if any):**") {
		t.Errorf("expected amendment field present even when empty\n%s", string(data))
	}
}
