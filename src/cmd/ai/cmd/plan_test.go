package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// runPlanCmd is a test helper that wires a fresh root command, redirects
// stdout/stderr to a buffer, and executes `ai plan <args...>`.
func runPlanCmd(t *testing.T, args ...string) (string, error) {
	t.Helper()
	root := NewRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs(append([]string{"plan"}, args...))
	err := root.Execute()
	return buf.String(), err
}

// helperPlanAIRoot sets up a temp AI_ROOT and returns its path.
func helperPlanAIRoot(t *testing.T) string {
	t.Helper()
	aiRoot := t.TempDir()
	t.Setenv("AI_ROOT", aiRoot)
	t.Setenv("EDITOR", "") // prevent editor from launching during tests
	return aiRoot
}

// ─── plan new ────────────────────────────────────────────────────────────────

// TestPlanNew_CreatesFile verifies that `ai plan new` (no --title flag)
// creates a plan file in <ai_root>/governance/plans/ with the MADR template.
func TestPlanNew_CreatesFile(t *testing.T) {
	aiRoot := helperPlanAIRoot(t)

	out, err := runPlanCmd(t, "new")
	if err != nil {
		t.Fatalf("plan new returned error: %v", err)
	}

	// Plans dir must exist.
	plansDir := filepath.Join(aiRoot, "governance", "plans")
	entries, readErr := os.ReadDir(plansDir)
	if readErr != nil {
		t.Fatalf("plans dir not created at %s: %v", plansDir, readErr)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 plan file, got %d", len(entries))
	}

	// Output must include "Created plan:" message.
	if !strings.Contains(out, "Created plan:") {
		t.Errorf("expected 'Created plan:' in output, got:\n%s", out)
	}

	// File must contain MADR template markers.
	data, readErr := os.ReadFile(filepath.Join(plansDir, entries[0].Name()))
	if readErr != nil {
		t.Fatalf("reading plan file: %v", readErr)
	}
	body := string(data)
	for _, marker := range []string{"## Context", "## Decision", "## Consequences", "## Alternatives Considered"} {
		if !strings.Contains(body, marker) {
			t.Errorf("expected MADR marker %q in plan file body:\n%s", marker, body)
		}
	}
}

// TestPlanNew_WithTitle verifies that the slug in the filename is derived
// from a --title flag value.
func TestPlanNew_WithTitle(t *testing.T) {
	aiRoot := helperPlanAIRoot(t)
	title := "Add Widget Feature"

	_, err := runPlanCmd(t, "new", "--title", title)
	if err != nil {
		t.Fatalf("plan new --title returned error: %v", err)
	}

	plansDir := filepath.Join(aiRoot, "governance", "plans")
	entries, readErr := os.ReadDir(plansDir)
	if readErr != nil || len(entries) == 0 {
		t.Fatalf("plans dir empty or missing: %v", readErr)
	}

	name := entries[0].Name()
	// Slug is derived from title: lower-case, non-alphanum → hyphens.
	if !strings.Contains(name, "add-widget-feature") {
		t.Errorf("expected filename to contain slug 'add-widget-feature', got %q", name)
	}
}

// TestPlanNew_DefaultTitle verifies that omitting --title uses "new-plan" as slug.
func TestPlanNew_DefaultTitle(t *testing.T) {
	aiRoot := helperPlanAIRoot(t)

	_, err := runPlanCmd(t, "new")
	if err != nil {
		t.Fatalf("plan new (no title) returned error: %v", err)
	}

	plansDir := filepath.Join(aiRoot, "governance", "plans")
	entries, readErr := os.ReadDir(plansDir)
	if readErr != nil || len(entries) == 0 {
		t.Fatalf("plans dir empty or missing: %v", readErr)
	}

	name := entries[0].Name()
	if !strings.Contains(name, "new-plan") {
		t.Errorf("expected filename to contain 'new-plan', got %q", name)
	}
}

// TestPlanNew_TemplateContainsTitle verifies that the plan file's first line
// is a Markdown H1 with the provided title.
func TestPlanNew_TemplateContainsTitle(t *testing.T) {
	aiRoot := helperPlanAIRoot(t)
	title := "My Important Plan"

	_, err := runPlanCmd(t, "new", "--title", title)
	if err != nil {
		t.Fatalf("plan new --title returned error: %v", err)
	}

	plansDir := filepath.Join(aiRoot, "governance", "plans")
	entries, _ := os.ReadDir(plansDir)
	if len(entries) == 0 {
		t.Fatal("no plan file created")
	}

	data, readErr := os.ReadFile(filepath.Join(plansDir, entries[0].Name()))
	if readErr != nil {
		t.Fatalf("reading plan file: %v", readErr)
	}
	body := string(data)
	if !strings.Contains(body, "# "+title) {
		t.Errorf("expected H1 title %q in plan body:\n%s", "# "+title, body)
	}
}

// ─── plan list ────────────────────────────────────────────────────────────────

// TestPlanList_Empty verifies that `ai plan list` on an empty plans dir
// prints "(no plans yet)".
func TestPlanList_Empty(t *testing.T) {
	helperPlanAIRoot(t)

	out, err := runPlanCmd(t, "list")
	if err != nil {
		t.Fatalf("plan list returned error: %v", err)
	}
	if !strings.Contains(out, "(no plans yet)") {
		t.Errorf("expected '(no plans yet)' output, got:\n%s", out)
	}
}

// TestPlanList_WithPlans verifies the table output when plans exist.
func TestPlanList_WithPlans(t *testing.T) {
	aiRoot := helperPlanAIRoot(t)

	plansDir := filepath.Join(aiRoot, "governance", "plans")
	if err := os.MkdirAll(plansDir, 0o755); err != nil {
		t.Fatalf("setup: mkdir plans dir: %v", err)
	}

	// Write two plan files with predictable names.
	plan1 := "20260101T120000Z-my-first-plan.md"
	plan2 := "20260102T090000Z-second-plan.md"
	content1 := "# My First Plan\n\n**Status:** draft\n"
	content2 := "# Second Plan\n\n**Status:** draft\n"
	if err := os.WriteFile(filepath.Join(plansDir, plan1), []byte(content1), 0o644); err != nil {
		t.Fatalf("setup: write plan1: %v", err)
	}
	if err := os.WriteFile(filepath.Join(plansDir, plan2), []byte(content2), 0o644); err != nil {
		t.Fatalf("setup: write plan2: %v", err)
	}

	out, err := runPlanCmd(t, "list")
	if err != nil {
		t.Fatalf("plan list returned error: %v", err)
	}

	// Date column.
	if !strings.Contains(out, "2026-01-01") {
		t.Errorf("expected date '2026-01-01' in output:\n%s", out)
	}
	// Slug column.
	if !strings.Contains(out, "my-first-plan") {
		t.Errorf("expected slug 'my-first-plan' in output:\n%s", out)
	}
	// Title column.
	if !strings.Contains(out, "My First Plan") {
		t.Errorf("expected title 'My First Plan' in output:\n%s", out)
	}
	// Second plan present.
	if !strings.Contains(out, "second-plan") {
		t.Errorf("expected 'second-plan' in output:\n%s", out)
	}
	// Should NOT show "(no plans yet)".
	if strings.Contains(out, "(no plans yet)") {
		t.Errorf("unexpected '(no plans yet)' in output with existing plans:\n%s", out)
	}
}

// ─── plan show ────────────────────────────────────────────────────────────────

// TestPlanShow_Found verifies that `ai plan show <slug>` prints the file content.
func TestPlanShow_Found(t *testing.T) {
	aiRoot := helperPlanAIRoot(t)

	plansDir := filepath.Join(aiRoot, "governance", "plans")
	if err := os.MkdirAll(plansDir, 0o755); err != nil {
		t.Fatalf("setup: mkdir plans dir: %v", err)
	}

	content := "# Widget Feature Plan\n\n**Status:** draft\n\n## Context\nWidget needed.\n"
	filename := "20260115T083000Z-widget-feature.md"
	if err := os.WriteFile(filepath.Join(plansDir, filename), []byte(content), 0o644); err != nil {
		t.Fatalf("setup: write plan file: %v", err)
	}

	out, err := runPlanCmd(t, "show", "widget-feature")
	if err != nil {
		t.Fatalf("plan show returned error: %v", err)
	}
	if !strings.Contains(out, "Widget Feature Plan") {
		t.Errorf("expected plan title in output:\n%s", out)
	}
	if !strings.Contains(out, "Widget needed.") {
		t.Errorf("expected plan body in output:\n%s", out)
	}
}

// TestPlanShow_ByExactFilename verifies that `ai plan show` resolves
// a slug that matches an exact filename (without the date prefix).
func TestPlanShow_ByExactFilename(t *testing.T) {
	aiRoot := helperPlanAIRoot(t)

	plansDir := filepath.Join(aiRoot, "governance", "plans")
	if err := os.MkdirAll(plansDir, 0o755); err != nil {
		t.Fatalf("setup: mkdir plans dir: %v", err)
	}

	content := "# Direct Slug Plan\n\n## Context\nDirect access.\n"
	filename := "direct-slug.md"
	if err := os.WriteFile(filepath.Join(plansDir, filename), []byte(content), 0o644); err != nil {
		t.Fatalf("setup: write plan file: %v", err)
	}

	out, err := runPlanCmd(t, "show", "direct-slug")
	if err != nil {
		t.Fatalf("plan show returned error: %v", err)
	}
	if !strings.Contains(out, "Direct Slug Plan") {
		t.Errorf("expected plan title in output:\n%s", out)
	}
}

// TestPlanShow_NotFound verifies that `ai plan show` returns a clear error
// when the slug doesn't match any plan file.
func TestPlanShow_NotFound(t *testing.T) {
	helperPlanAIRoot(t)

	_, err := runPlanCmd(t, "show", "nonexistent-plan")
	if err == nil {
		t.Error("expected error for nonexistent slug, got nil")
	}
	if err != nil && strings.Contains(err.Error(), "not yet implemented") {
		t.Fatalf("plan show returned stub error: %v", err)
	}
}

// TestPlanNew_NoStub verifies that `ai plan new` does NOT return a stub error.
func TestPlanNew_NoStub(t *testing.T) {
	helperPlanAIRoot(t)

	_, err := runPlanCmd(t, "new")
	if err != nil && strings.Contains(err.Error(), "not yet implemented") {
		t.Fatalf("plan new returned stub error: %v", err)
	}
}

// TestPlanList_NoStub verifies that `ai plan list` does NOT return a stub error.
func TestPlanList_NoStub(t *testing.T) {
	helperPlanAIRoot(t)

	_, err := runPlanCmd(t, "list")
	if err != nil && strings.Contains(err.Error(), "not yet implemented") {
		t.Fatalf("plan list returned stub error: %v", err)
	}
}
