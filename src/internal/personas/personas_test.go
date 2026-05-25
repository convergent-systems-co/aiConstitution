package personas_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/convergent-systems-co/aiConstitution/src/internal/personas"
)

// writeFile is a test helper that writes content to a named file inside dir.
func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatalf("writeFile: %v", err)
	}
	return p
}

// --- #250: LoadPersonas reads agentic YAML files ---

func TestLoadPersonas_ReadsAgenticYAML(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "devops-engineer.yaml", `
name: devops-engineer
type: agentic
description: "Singleton DevOps orchestrator"
domain: cli
capabilities:
  - git
  - gh
  - spawn
`)

	got, err := personas.LoadPersonas(dir)
	if err != nil {
		t.Fatalf("LoadPersonas returned error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 persona, got %d", len(got))
	}
	p := got[0]
	if p.Name != "devops-engineer" {
		t.Errorf("Name: want %q, got %q", "devops-engineer", p.Name)
	}
	if p.Type != "agentic" {
		t.Errorf("Type: want %q, got %q", "agentic", p.Type)
	}
	if p.Description != "Singleton DevOps orchestrator" {
		t.Errorf("Description: want %q, got %q", "Singleton DevOps orchestrator", p.Description)
	}
	if len(p.Capabilities) != 3 {
		t.Errorf("Capabilities: want 3, got %d: %v", len(p.Capabilities), p.Capabilities)
	}
}

// --- #251: Load reviewer persona with extra fields ---

func TestLoadPersonas_ReadsReviewerYAML(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "security-reviewer.yaml", `
name: security-reviewer
type: reviewer
role: security
panel_weight: 0.3
domains:
  - cli
  - hooks
`)
	writeFile(t, dir, "docs-reviewer.yaml", `
name: docs-reviewer
type: reviewer
role: documentation
panel_weight: 0.7
domains:
  - docs
`)

	got, err := personas.LoadPersonas(dir)
	if err != nil {
		t.Fatalf("LoadPersonas returned error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 personas, got %d", len(got))
	}

	// Find security-reviewer
	var sec *personas.PersonaFile
	for i := range got {
		if got[i].Name == "security-reviewer" {
			sec = &got[i]
		}
	}
	if sec == nil {
		t.Fatal("security-reviewer not found in results")
	}
	if sec.Role != "security" {
		t.Errorf("Role: want %q, got %q", "security", sec.Role)
	}
	if sec.PanelWeight != 0.3 {
		t.Errorf("PanelWeight: want 0.3, got %f", sec.PanelWeight)
	}
	if len(sec.Domains) != 2 {
		t.Errorf("Domains: want 2, got %d: %v", len(sec.Domains), sec.Domains)
	}
}

// --- #252: domain (string) and domains ([]string) both parse to []string ---

func TestLoadPersonas_DomainString_NormalizesToSlice(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "single-domain.yaml", `
name: single-domain
type: agentic
domain: cli
`)

	got, err := personas.LoadPersonas(dir)
	if err != nil {
		t.Fatalf("LoadPersonas returned error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 persona, got %d", len(got))
	}
	if len(got[0].Domains) != 1 || got[0].Domains[0] != "cli" {
		t.Errorf("Domains: want [cli], got %v", got[0].Domains)
	}
}

func TestLoadPersonas_DomainsArray_NormalizesToSlice(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "multi-domain.yaml", `
name: multi-domain
type: agentic
domains:
  - cli
  - hooks
`)

	got, err := personas.LoadPersonas(dir)
	if err != nil {
		t.Fatalf("LoadPersonas returned error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 persona, got %d", len(got))
	}
	want := []string{"cli", "hooks"}
	if len(got[0].Domains) != len(want) {
		t.Fatalf("Domains len: want %d, got %d: %v", len(want), len(got[0].Domains), got[0].Domains)
	}
	for i, d := range want {
		if got[0].Domains[i] != d {
			t.Errorf("Domains[%d]: want %q, got %q", i, d, got[0].Domains[i])
		}
	}
}

// --- Skip-on-error: invalid YAML alongside valid ---

func TestLoadPersonas_SkipsInvalidYAML(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "good.yaml", `
name: good
type: agentic
domain: cli
`)
	// Intentionally malformed YAML: tab indentation is illegal in YAML
	writeFile(t, dir, "bad.yaml", `
name: bad
	type: broken: yaml: here
`)

	got, err := personas.LoadPersonas(dir)
	if err != nil {
		t.Fatalf("LoadPersonas should not return top-level error for parse failures, got: %v", err)
	}
	if len(got) != 1 {
		t.Errorf("expected 1 valid persona, got %d: %v", len(got), got)
	}
	if got[0].Name != "good" {
		t.Errorf("expected persona name %q, got %q", "good", got[0].Name)
	}
}

// --- Non-YAML files are ignored ---

func TestLoadPersonas_IgnoresNonYAMLFiles(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "persona.yaml", `
name: real
type: agentic
domain: cli
`)
	writeFile(t, dir, "readme.md", "# Not a persona")
	writeFile(t, dir, "config.json", `{"name": "not-a-persona"}`)

	got, err := personas.LoadPersonas(dir)
	if err != nil {
		t.Fatalf("LoadPersonas returned error: %v", err)
	}
	if len(got) != 1 {
		t.Errorf("expected 1 persona (only .yaml), got %d: %v", len(got), got)
	}
}

// --- Empty directory returns empty slice ---

func TestLoadPersonas_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	got, err := personas.LoadPersonas(dir)
	if err != nil {
		t.Fatalf("LoadPersonas returned error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty slice, got %d personas", len(got))
	}
}
