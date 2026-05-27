package cmd_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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

// ---------------------------------------------------------------------------
// Helpers for install/uninstall/upgrade/upgrade-all tests (#347)
// ---------------------------------------------------------------------------

// fakeSkillAtom builds the JSON payload that a real skill-atoms GitHub API
// endpoint would return.
func fakeSkillAtom(slug, version, description, fragment string) []byte {
	payload := map[string]interface{}{
		"id":                     "skill/" + slug,
		"version":                version,
		"name":                   slug,
		"description":            description,
		"system_prompt_fragment": fragment,
	}
	b, _ := json.Marshal(payload)
	return b
}

// startSkillAtomServer starts an httptest.Server that returns fakePayload for
// any request. Returns the server; caller should t.Cleanup(srv.Close) or
// rely on t.Cleanup registered here.
func startSkillAtomServer(t *testing.T, status int, fakePayload []byte) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write(fakePayload)
	}))
	t.Cleanup(srv.Close)
	return srv
}

// setSkillAtomBaseURL temporarily overrides cmd.SkillAtomsBaseURL for the
// duration of the test.
func setSkillAtomBaseURL(t *testing.T, baseURL string) {
	t.Helper()
	old := cmd.SkillAtomsBaseURL
	cmd.SkillAtomsBaseURL = baseURL
	t.Cleanup(func() { cmd.SkillAtomsBaseURL = old })
}

// ---------------------------------------------------------------------------
// #347 — ai skills install
// ---------------------------------------------------------------------------

func TestSkillsInstall_Success(t *testing.T) {
	root := t.TempDir()
	payload := fakeSkillAtom("commit", "1.2.0", "Generate a conventional commit message.", "You are a commit message assistant.")
	srv := startSkillAtomServer(t, http.StatusOK, payload)
	setSkillAtomBaseURL(t, srv.URL)

	out, _, err := runSkillsCmd(t, root, "skills", "install", "commit")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// SKILL.md must exist at ~/.ai/skills/commit/SKILL.md
	skillMDPath := filepath.Join(root, "skills", "commit", "SKILL.md")
	data, readErr := os.ReadFile(skillMDPath)
	if readErr != nil {
		t.Fatalf("SKILL.md not written: %v", readErr)
	}

	content := string(data)
	if !strings.Contains(content, "name: commit") {
		t.Errorf("SKILL.md missing 'name: commit'; got:\n%s", content)
	}
	if !strings.Contains(content, "version: 1.2.0") {
		t.Errorf("SKILL.md missing 'version: 1.2.0'; got:\n%s", content)
	}
	if !strings.Contains(content, "Generate a conventional commit message.") {
		t.Errorf("SKILL.md missing description; got:\n%s", content)
	}
	if !strings.Contains(content, "You are a commit message assistant.") {
		t.Errorf("SKILL.md missing system_prompt_fragment; got:\n%s", content)
	}

	if !strings.Contains(out, "Installed commit v1.2.0") {
		t.Errorf("expected 'Installed commit v1.2.0' in output; got:\n%s", out)
	}
}

func TestSkillsInstall_NotFound(t *testing.T) {
	root := t.TempDir()
	srv := startSkillAtomServer(t, http.StatusNotFound, []byte(`{"message":"Not Found"}`))
	setSkillAtomBaseURL(t, srv.URL)

	_, errStr, err := runSkillsCmd(t, root, "skills", "install", "nonexistent-skill")
	if err == nil {
		t.Fatal("expected error for 404, got nil")
	}
	combined := errStr + err.Error()
	if !strings.Contains(combined, "nonexistent-skill") {
		t.Errorf("error should mention the skill name; got: %s", combined)
	}
}

// ---------------------------------------------------------------------------
// #347 — ai skills uninstall
// ---------------------------------------------------------------------------

func TestSkillsUninstall_Success(t *testing.T) {
	root := t.TempDir()
	// Pre-install a skill manually.
	skillDir := filepath.Join(root, "skills", "commit")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("---\nname: commit\ndescription: test\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Also create a symlink in a fake ~/.claude/skills/ directory.
	claudeSkillsDir := filepath.Join(root, "claude-skills")
	if err := os.MkdirAll(claudeSkillsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	symlinkPath := filepath.Join(claudeSkillsDir, "commit")
	if err := os.Symlink(skillDir, symlinkPath); err != nil {
		t.Fatal(err)
	}

	t.Setenv("AI_ROOT", root)
	t.Setenv("CLAUDE_SKILLS_DIR", claudeSkillsDir)

	out, _, err := runSkillsCmd(t, root, "skills", "uninstall", "commit")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "Uninstalled commit") {
		t.Errorf("expected 'Uninstalled commit'; got:\n%s", out)
	}

	// Skill dir and symlink must be gone.
	if _, statErr := os.Stat(skillDir); !os.IsNotExist(statErr) {
		t.Error("skill dir should have been removed")
	}
	if _, lstatErr := os.Lstat(symlinkPath); !os.IsNotExist(lstatErr) {
		t.Error("symlink should have been removed")
	}
}

func TestSkillsUninstall_NotInstalled(t *testing.T) {
	root := t.TempDir()
	_, errStr, err := runSkillsCmd(t, root, "skills", "uninstall", "ghost-skill")
	if err == nil {
		t.Fatal("expected error when skill is not installed")
	}
	combined := errStr + err.Error()
	if !strings.Contains(combined, "ghost-skill") {
		t.Errorf("error should mention the skill name; got: %s", combined)
	}
}

// ---------------------------------------------------------------------------
// #347 — ai skills upgrade
// ---------------------------------------------------------------------------

func TestSkillsUpgrade_Success(t *testing.T) {
	root := t.TempDir()
	// Pre-install skill at v1.0.0.
	skillDir := filepath.Join(root, "skills", "commit")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	oldMD := "---\nname: commit\ndescription: old\nversion: 1.0.0\n---\n# commit\nold fragment\n"
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(oldMD), 0o644); err != nil {
		t.Fatal(err)
	}

	// Mock server returns v1.2.0.
	payload := fakeSkillAtom("commit", "1.2.0", "New description.", "New fragment.")
	srv := startSkillAtomServer(t, http.StatusOK, payload)
	setSkillAtomBaseURL(t, srv.URL)

	out, _, err := runSkillsCmd(t, root, "skills", "upgrade", "commit")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(out, "1.0.0") {
		t.Errorf("output should mention old version 1.0.0; got:\n%s", out)
	}
	if !strings.Contains(out, "1.2.0") {
		t.Errorf("output should mention new version 1.2.0; got:\n%s", out)
	}

	// SKILL.md should now contain the new fragment.
	data, _ := os.ReadFile(filepath.Join(skillDir, "SKILL.md"))
	if !strings.Contains(string(data), "New fragment.") {
		t.Errorf("SKILL.md not updated; got:\n%s", string(data))
	}
}

// ---------------------------------------------------------------------------
// #347 — ai skills upgrade-all
// ---------------------------------------------------------------------------

func TestSkillsUpgradeAll_Empty(t *testing.T) {
	root := t.TempDir()
	// Create empty skills dir.
	if err := os.MkdirAll(filepath.Join(root, "skills"), 0o755); err != nil {
		t.Fatal(err)
	}

	// No server needed — there is nothing to upgrade.
	out, _, err := runSkillsCmd(t, root, "skills", "upgrade-all")
	if err != nil {
		t.Fatalf("upgrade-all on empty dir should not error; got: %v", err)
	}
	// Should emit a "nothing to do" style message.
	if out == "" {
		t.Error("expected some output from upgrade-all on empty dir")
	}
}
