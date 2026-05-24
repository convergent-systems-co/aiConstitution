// Package cmd is the cobra command tree for `ai`.
package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func runReviewCmd(t *testing.T, args ...string) (string, error) {
	t.Helper()
	root := NewRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs(append([]string{"review"}, args...))
	err := root.Execute()
	return buf.String(), err
}

func helperReviewAIRoot(t *testing.T) string {
	t.Helper()
	aiRoot := t.TempDir()
	t.Setenv("AI_ROOT", aiRoot)
	_ = os.MkdirAll(filepath.Join(aiRoot, "audit", "violations"), 0o750)
	_ = os.MkdirAll(filepath.Join(aiRoot, "audit", "overrides"), 0o750)
	_ = os.MkdirAll(filepath.Join(aiRoot, "audit", "drift"), 0o750)
	_ = os.MkdirAll(filepath.Join(aiRoot, "memory"), 0o750)
	_ = os.WriteFile(filepath.Join(aiRoot, "memory", "MEMORY.md"), []byte("# Memory Index\n"), 0o600)
	return aiRoot
}

func TestReviewCheck_ProducesReport(t *testing.T) {
	aiRoot := helperReviewAIRoot(t)

	// Write a violation file
	_ = os.WriteFile(
		filepath.Join(aiRoot, "audit", "violations", "20260522T173810Z-branch-commit.md"),
		[]byte("# Violation — 2026-05-22T17:38:10Z\n\n- **File / Rule violated:** §3.2 — protected branch\n- **What happened:** Committed to main.\n"),
		0o600)

	// Write a drift file
	_ = os.WriteFile(
		filepath.Join(aiRoot, "audit", "drift", "20260524T120000Z-blast.md"),
		[]byte("# Drift — 2026-05-24T12:00:00Z\n\n- **Rule:** §3.U17\n- **Trigger:** near-miss\n"),
		0o600)

	out, _ := runReviewCmd(t, "--check")
	if !strings.Contains(strings.ToLower(out), "violation") {
		t.Errorf("expected violations in review output:\n%s", out)
	}
}

func TestReviewCheck_WritesGovernanceReport(t *testing.T) {
	aiRoot := helperReviewAIRoot(t)
	_, _ = runReviewCmd(t, "--check")

	reportsDir := filepath.Join(aiRoot, "governance", "reports")
	entries, err := os.ReadDir(reportsDir)
	if err != nil || len(entries) == 0 {
		t.Error("governance report not written to governance/reports/")
	}
}

func TestReviewCheck_ReportContainsFourSections(t *testing.T) {
	aiRoot := helperReviewAIRoot(t)
	_, _ = runReviewCmd(t, "--check")

	reportsDir := filepath.Join(aiRoot, "governance", "reports")
	entries, _ := os.ReadDir(reportsDir)
	if len(entries) == 0 {
		t.Fatal("no report written")
	}
	data, _ := os.ReadFile(filepath.Join(reportsDir, entries[0].Name()))
	body := string(data)
	for _, section := range []string{"Violation", "Override", "Drift", "Dead"} {
		if !strings.Contains(body, section) {
			t.Errorf("governance report missing %q section:\n%s", section, body)
		}
	}
}
