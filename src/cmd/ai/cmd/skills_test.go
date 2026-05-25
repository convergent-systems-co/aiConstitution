package cmd_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/convergent-systems-co/aiConstitution/src/cmd/ai/cmd"
)

// helper: create a populated skills fixture under a temp AI_ROOT.
// Returns the AI_ROOT dir. Caller responsible for cleanup via t.Cleanup.
func makeSkillsFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	skillsDir := filepath.Join(root, "skills")

	// valid-skill: has name and description
	validDir := filepath.Join(skillsDir, "valid-skill")
	if err := os.MkdirAll(validDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(validDir, "SKILL.md"), []byte(`---
name: valid-skill
description: A well-formed skill for testing.
---

# valid-skill
`), 0o644); err != nil {
		t.Fatal(err)
	}

	// no-md-skill: directory exists but no SKILL.md
	noMDDir := filepath.Join(skillsDir, "no-md-skill")
	if err := os.MkdirAll(noMDDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// partial-skill: has SKILL.md but missing description
	partialDir := filepath.Join(skillsDir, "partial-skill")
	if err := os.MkdirAll(partialDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(partialDir, "SKILL.md"), []byte(`---
name: partial-skill
---

# partial-skill
`), 0o644); err != nil {
		t.Fatal(err)
	}

	// templates-skill: has SKILL.md and a templates/ directory
	templatesSkillDir := filepath.Join(skillsDir, "templates-skill")
	templatesDir := filepath.Join(templatesSkillDir, "templates")
	if err := os.MkdirAll(templatesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(templatesSkillDir, "SKILL.md"), []byte(`---
name: templates-skill
description: A skill with templates.
---
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(templatesDir, "greeting.txt"), []byte(`Hello $NAME, welcome to ${PLACE}!`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(templatesDir, "config.yaml"), []byte(`version: $VERSION`), 0o644); err != nil {
		t.Fatal(err)
	}

	return root
}

// runCmd executes the root command with the given args and AI_ROOT set to root.
// Returns stdout, stderr, and the error (if any).
func runSkillsCmd(t *testing.T, root string, args ...string) (stdout, stderr string, err error) {
	t.Helper()

	// Set AI_ROOT so paths.SkillsManifestDir() resolves to our fixture.
	t.Setenv("AI_ROOT", root)

	var outBuf, errBuf bytes.Buffer
	c := cmd.NewRootCmd()
	c.SetOut(&outBuf)
	c.SetErr(&errBuf)
	c.SetArgs(args)
	err = c.Execute()
	return outBuf.String(), errBuf.String(), err
}

// ---------------------------------------------------------------------------
// #228 — ai skills list
// ---------------------------------------------------------------------------

func TestSkillsList_PrintsNameAndDescription(t *testing.T) {
	root := makeSkillsFixture(t)
	out, _, err := runSkillsCmd(t, root, "skills", "list")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "valid-skill") {
		t.Errorf("output missing 'valid-skill'; got:\n%s", out)
	}
	if !strings.Contains(out, "A well-formed skill for testing.") {
		t.Errorf("output missing description; got:\n%s", out)
	}
}

func TestSkillsList_ShowsNoDirSkill(t *testing.T) {
	root := makeSkillsFixture(t)
	out, _, err := runSkillsCmd(t, root, "skills", "list")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// no-md-skill should still appear, with a fallback description
	if !strings.Contains(out, "no-md-skill") {
		t.Errorf("output missing 'no-md-skill'; got:\n%s", out)
	}
	if !strings.Contains(out, "(no SKILL.md)") {
		t.Errorf("output missing '(no SKILL.md)'; got:\n%s", out)
	}
}

func TestSkillsList_EmptyDir_PrintsNoSkillsMessage(t *testing.T) {
	root := t.TempDir()
	skillsDir := filepath.Join(root, "skills")
	if err := os.MkdirAll(skillsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	out, _, err := runSkillsCmd(t, root, "skills", "list")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "(no skills installed)") {
		t.Errorf("expected '(no skills installed)'; got:\n%s", out)
	}
}

func TestSkillsList_MissingSkillsDir_PrintsNoSkillsMessage(t *testing.T) {
	root := t.TempDir() // no skills/ subdir
	out, _, err := runSkillsCmd(t, root, "skills", "list")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "(no skills installed)") {
		t.Errorf("expected '(no skills installed)'; got:\n%s", out)
	}
}

// ---------------------------------------------------------------------------
// #229 — ai skills show <name>
// ---------------------------------------------------------------------------

func TestSkillsShow_ExactMatch_PrintsContent(t *testing.T) {
	root := makeSkillsFixture(t)
	out, _, err := runSkillsCmd(t, root, "skills", "show", "valid-skill")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "name: valid-skill") {
		t.Errorf("expected SKILL.md content with 'name: valid-skill'; got:\n%s", out)
	}
	if !strings.Contains(out, "A well-formed skill for testing.") {
		t.Errorf("expected SKILL.md content with description; got:\n%s", out)
	}
}

func TestSkillsShow_PrefixMatch_FindsSkill(t *testing.T) {
	root := makeSkillsFixture(t)
	// "valid" is a prefix of "valid-skill"
	out, _, err := runSkillsCmd(t, root, "skills", "show", "valid")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "name: valid-skill") {
		t.Errorf("expected SKILL.md content for prefix match; got:\n%s", out)
	}
}

func TestSkillsShow_NotFound_ReturnsError(t *testing.T) {
	root := makeSkillsFixture(t)
	_, errOut, err := runSkillsCmd(t, root, "skills", "show", "nonexistent-skill")
	if err == nil {
		t.Fatal("expected error for unknown skill, got nil")
	}
	combined := errOut + err.Error()
	if !strings.Contains(combined, "nonexistent-skill") {
		t.Errorf("error should mention the skill name; got err=%v stderr=%s", err, errOut)
	}
	if !strings.Contains(combined, "not found") {
		t.Errorf("error should say 'not found'; got err=%v stderr=%s", err, errOut)
	}
}

// ---------------------------------------------------------------------------
// #231 — ai skills validate
// ---------------------------------------------------------------------------

func TestSkillsValidate_ReportsValid(t *testing.T) {
	root := makeSkillsFixture(t)
	out, _, err := runSkillsCmd(t, root, "skills", "validate")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// valid-skill and templates-skill should show [✓]
	if !strings.Contains(out, "[✓] valid-skill") {
		t.Errorf("expected '[✓] valid-skill'; got:\n%s", out)
	}
}

func TestSkillsValidate_ReportsMissingSKILLMD(t *testing.T) {
	root := makeSkillsFixture(t)
	out, _, err := runSkillsCmd(t, root, "skills", "validate")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "[⚠] no-md-skill") {
		t.Errorf("expected '[⚠] no-md-skill'; got:\n%s", out)
	}
	if !strings.Contains(out, "SKILL.md missing") {
		t.Errorf("expected 'SKILL.md missing'; got:\n%s", out)
	}
}

func TestSkillsValidate_ReportsMissingFrontmatterField(t *testing.T) {
	root := makeSkillsFixture(t)
	out, _, err := runSkillsCmd(t, root, "skills", "validate")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// partial-skill has name but no description
	if !strings.Contains(out, "[⚠] partial-skill") {
		t.Errorf("expected '[⚠] partial-skill'; got:\n%s", out)
	}
	if !strings.Contains(out, "missing frontmatter field 'description'") {
		t.Errorf("expected missing description warning; got:\n%s", out)
	}
}

func TestSkillsValidate_ExitZeroAlways(t *testing.T) {
	root := makeSkillsFixture(t)
	_, _, err := runSkillsCmd(t, root, "skills", "validate")
	if err != nil {
		t.Errorf("validate must exit 0 (warnings are informational); got error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// #232 — ai skills templates list <skill>
// ---------------------------------------------------------------------------

func TestSkillsTemplatesList_PrintsFileNames(t *testing.T) {
	root := makeSkillsFixture(t)
	out, _, err := runSkillsCmd(t, root, "skills", "templates", "list", "templates-skill")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "greeting.txt") {
		t.Errorf("expected 'greeting.txt'; got:\n%s", out)
	}
	if !strings.Contains(out, "config.yaml") {
		t.Errorf("expected 'config.yaml'; got:\n%s", out)
	}
}

func TestSkillsTemplatesList_SkillNotFound_ReturnsError(t *testing.T) {
	root := makeSkillsFixture(t)
	_, _, err := runSkillsCmd(t, root, "skills", "templates", "list", "ghost-skill")
	if err == nil {
		t.Fatal("expected error for unknown skill")
	}
}

func TestSkillsTemplatesList_NoTemplatesDir_ReturnsError(t *testing.T) {
	root := makeSkillsFixture(t)
	// valid-skill exists but has no templates/ dir
	_, _, err := runSkillsCmd(t, root, "skills", "templates", "list", "valid-skill")
	if err == nil {
		t.Fatal("expected error when no templates/ dir exists")
	}
}

// ---------------------------------------------------------------------------
// #233 — ai skills templates show <skill> <template>
// ---------------------------------------------------------------------------

func TestSkillsTemplatesShow_SubstitutesVarFromFlag(t *testing.T) {
	root := makeSkillsFixture(t)
	out, _, err := runSkillsCmd(t, root, "skills", "templates", "show", "templates-skill", "greeting.txt",
		"--var", "NAME=World", "--var", "PLACE=Earth")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "Hello World") {
		t.Errorf("expected 'Hello World'; got:\n%s", out)
	}
	if !strings.Contains(out, "welcome to Earth") {
		t.Errorf("expected 'welcome to Earth'; got:\n%s", out)
	}
}

func TestSkillsTemplatesShow_UnresolvedVarsLeftAsIs(t *testing.T) {
	root := makeSkillsFixture(t)
	// No --var flags, no matching env vars
	t.Setenv("NAME", "")
	t.Setenv("PLACE", "")
	out, _, err := runSkillsCmd(t, root, "skills", "templates", "show", "templates-skill", "greeting.txt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Unresolved vars stay as-is
	if !strings.Contains(out, "$NAME") && !strings.Contains(out, "${PLACE}") {
		t.Errorf("expected unresolved vars to remain; got:\n%s", out)
	}
}

func TestSkillsTemplatesShow_SubstitutesVarFromEnv(t *testing.T) {
	root := makeSkillsFixture(t)
	t.Setenv("VERSION", "v2.0")
	out, _, err := runSkillsCmd(t, root, "skills", "templates", "show", "templates-skill", "config.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "v2.0") {
		t.Errorf("expected VERSION substituted from env; got:\n%s", out)
	}
}

func TestSkillsTemplatesShow_FlagOverridesEnv(t *testing.T) {
	root := makeSkillsFixture(t)
	t.Setenv("VERSION", "env-version")
	out, _, err := runSkillsCmd(t, root, "skills", "templates", "show", "templates-skill", "config.yaml",
		"--var", "VERSION=flag-version")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "flag-version") {
		t.Errorf("expected flag value to override env; got:\n%s", out)
	}
	if strings.Contains(out, "env-version") {
		t.Errorf("env value should be overridden by flag; got:\n%s", out)
	}
}

func TestSkillsTemplatesShow_TemplateNotFound_ReturnsError(t *testing.T) {
	root := makeSkillsFixture(t)
	_, _, err := runSkillsCmd(t, root, "skills", "templates", "show", "templates-skill", "nonexistent.txt")
	if err == nil {
		t.Fatal("expected error for nonexistent template file")
	}
}
