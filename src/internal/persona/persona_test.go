package persona_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/convergent-systems-co/aiConstitution/src/internal/config"
	"github.com/convergent-systems-co/aiConstitution/src/internal/persona"
)

func TestResolveUsesSettingsDefault(t *testing.T) {
	s := config.Defaults()
	s.Personas.Default = []string{"common", "code"}
	got, err := persona.Resolve(s, "")
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if len(got) != 2 || got[0] != "common" || got[1] != "code" {
		t.Errorf("Resolve() = %v, want [common code]", got)
	}
}

func TestResolveProjectYAMLOverridesSettings(t *testing.T) {
	dir := t.TempDir()
	projYAML := filepath.Join(dir, "project.yaml")
	if err := os.WriteFile(projYAML, []byte("personas:\n  load:\n    - common\n    - writing\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	s := config.Defaults()
	s.Personas.Default = []string{"common", "code"}

	got, err := persona.Resolve(s, projYAML)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if len(got) != 2 || got[1] != "writing" {
		t.Errorf("Resolve() = %v, want [common writing] from project.yaml", got)
	}
}

func TestResolveProjectYAMLMissingFallsBackToSettings(t *testing.T) {
	s := config.Defaults()
	s.Personas.Default = []string{"common"}
	got, err := persona.Resolve(s, "/nonexistent/project.yaml")
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if len(got) != 1 || got[0] != "common" {
		t.Errorf("Resolve() = %v, want [common]", got)
	}
}

func TestRewriteBlockCreatesBlock(t *testing.T) {
	dir := t.TempDir()
	claudeMD := filepath.Join(dir, "CLAUDE.md")
	if err := os.WriteFile(claudeMD, []byte("# Instructions\n\n@~/.ai/Constitution.md\n\n@~/Documents/Prompts/Instructions.md\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := persona.RewriteBlock(claudeMD, []string{"common", "code"}, "/fake/.ai"); err != nil {
		t.Fatalf("RewriteBlock() error = %v", err)
	}

	content, _ := os.ReadFile(claudeMD)
	s := string(content)
	if !strings.Contains(s, "<!-- ai:personas") {
		t.Error("RewriteBlock: missing opening comment")
	}
	if !strings.Contains(s, "<!-- /ai:personas -->") {
		t.Error("RewriteBlock: missing closing comment")
	}
	if !strings.Contains(s, "@/fake/.ai/Common.md") {
		t.Errorf("RewriteBlock: missing Common.md include, got:\n%s", s)
	}
	if !strings.Contains(s, "@/fake/.ai/Code.md") {
		t.Errorf("RewriteBlock: missing Code.md include, got:\n%s", s)
	}
}

func TestRewriteBlockReplacesExistingBlock(t *testing.T) {
	dir := t.TempDir()
	claudeMD := filepath.Join(dir, "CLAUDE.md")
	initial := "# Instructions\n\n" +
		"<!-- ai:personas — managed by ai cli, do not edit manually -->\n" +
		"@/fake/.ai/Common.md\n" +
		"@/fake/.ai/Code.md\n" +
		"<!-- /ai:personas -->\n\n" +
		"@~/Documents/Prompts/Instructions.md\n"
	if err := os.WriteFile(claudeMD, []byte(initial), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := persona.RewriteBlock(claudeMD, []string{"common", "writing"}, "/fake/.ai"); err != nil {
		t.Fatalf("RewriteBlock() error = %v", err)
	}

	content, _ := os.ReadFile(claudeMD)
	s := string(content)
	if strings.Contains(s, "Code.md") {
		t.Error("RewriteBlock: old Code.md include should be replaced, got:\n" + s)
	}
	if !strings.Contains(s, "Writing.md") {
		t.Errorf("RewriteBlock: missing Writing.md include, got:\n%s", s)
	}
}
