package plugins_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/convergent-systems-co/aiConstitution/src/internal/plugins"
)

// TestParseManifest_ValidFields verifies that ParseManifest reads all
// required and optional fields from a well-formed manifest.yaml.
func TestParseManifest_ValidFields(t *testing.T) {
	dir := t.TempDir()
	content := `name: my-plugin
version: "1.0.0"
description: "What it does"
source: "https://example.com/plugin.tar.gz"
skills:
  - skill-name
  - another-skill
`
	path := filepath.Join(dir, "manifest.yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("setup: write manifest: %v", err)
	}

	m, err := plugins.ParseManifest(path)
	if err != nil {
		t.Fatalf("ParseManifest returned error: %v", err)
	}
	if m.Name != "my-plugin" {
		t.Errorf("Name: got %q, want %q", m.Name, "my-plugin")
	}
	if m.Version != "1.0.0" {
		t.Errorf("Version: got %q, want %q", m.Version, "1.0.0")
	}
	if m.Description != "What it does" {
		t.Errorf("Description: got %q, want %q", m.Description, "What it does")
	}
	if m.Source != "https://example.com/plugin.tar.gz" {
		t.Errorf("Source: got %q, want %q", m.Source, "https://example.com/plugin.tar.gz")
	}
	if len(m.Skills) != 2 {
		t.Errorf("Skills len: got %d, want 2", len(m.Skills))
	} else {
		if m.Skills[0] != "skill-name" {
			t.Errorf("Skills[0]: got %q, want %q", m.Skills[0], "skill-name")
		}
		if m.Skills[1] != "another-skill" {
			t.Errorf("Skills[1]: got %q, want %q", m.Skills[1], "another-skill")
		}
	}
}

// TestParseManifest_MissingNameErrors verifies that ParseManifest returns
// an error when the required name field is absent.
func TestParseManifest_MissingNameErrors(t *testing.T) {
	dir := t.TempDir()
	content := `version: "1.0.0"
description: "no name"
source: "https://example.com/plugin.tar.gz"
`
	path := filepath.Join(dir, "manifest.yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	_, err := plugins.ParseManifest(path)
	if err == nil {
		t.Fatal("expected error for missing name, got nil")
	}
}

// TestParseManifest_MissingVersionErrors verifies that ParseManifest
// returns an error when the required version field is absent.
func TestParseManifest_MissingVersionErrors(t *testing.T) {
	dir := t.TempDir()
	content := `name: my-plugin
description: "no version"
source: "https://example.com/plugin.tar.gz"
`
	path := filepath.Join(dir, "manifest.yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	_, err := plugins.ParseManifest(path)
	if err == nil {
		t.Fatal("expected error for missing version, got nil")
	}
}

// TestParseManifest_FileNotFound verifies that ParseManifest returns
// an error when the given path does not exist.
func TestParseManifest_FileNotFound(t *testing.T) {
	_, err := plugins.ParseManifest("/nonexistent/path/manifest.yaml")
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}

// TestParseManifest_NoSkillsField verifies that Skills defaults to nil
// (or empty slice) when omitted — not an error.
func TestParseManifest_NoSkillsField(t *testing.T) {
	dir := t.TempDir()
	content := `name: minimal-plugin
version: "0.1.0"
`
	path := filepath.Join(dir, "manifest.yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	m, err := plugins.ParseManifest(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.Name != "minimal-plugin" {
		t.Errorf("Name: got %q", m.Name)
	}
	if len(m.Skills) != 0 {
		t.Errorf("Skills: expected empty, got %v", m.Skills)
	}
}
