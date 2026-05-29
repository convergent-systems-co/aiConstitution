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
	// Isolate symlink operations from the real home directory, but only when
	// the test hasn't already configured a specific target directory.
	if _, ok := os.LookupEnv("CLAUDE_SKILLS_DIR"); !ok {
		t.Setenv("CLAUDE_SKILLS_DIR", t.TempDir())
	}
	if _, ok := os.LookupEnv("COPILOT_INSTRUCTIONS_DIR"); !ok {
		t.Setenv("COPILOT_INSTRUCTIONS_DIR", t.TempDir())
	}

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
	// Redirect Claude symlink target to a temp dir so tests don't pollute the
	// real ~/.claude/skills/ with symlinks pointing into t.TempDir() paths that
	// are cleaned up after the test completes.
	t.Setenv("CLAUDE_SKILLS_DIR", t.TempDir())
	atoms := []map[string]interface{}{
		{
			"type":                   "skill",
			"id":                     "skill/commit",
			"version":                "1.2.0",
			"name":                   "commit",
			"description":            "Generate a conventional commit message.",
			"system_prompt_fragment": "You are a commit message assistant.",
			"lifecycle":              "stable",
		},
	}
	srv := startSkillsCatalogServer(t, atoms)
	setAiAtomsCatalogURL(t, srv.URL+"/catalog.json")

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
	// Catalog is empty — "nonexistent-skill" will not be found.
	srv := startSkillsCatalogServer(t, nil)
	setAiAtomsCatalogURL(t, srv.URL+"/catalog.json")

	_, errStr, err := runSkillsCmd(t, root, "skills", "install", "nonexistent-skill")
	if err == nil {
		t.Fatal("expected error when skill is not in catalog, got nil")
	}
	combined := errStr + err.Error()
	if !strings.Contains(combined, "nonexistent-skill") {
		t.Errorf("error should mention the skill name; got: %s", combined)
	}
	if !strings.Contains(combined, "not found") {
		t.Errorf("error should say 'not found'; got: %s", combined)
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

// ---------------------------------------------------------------------------
// #365 / #415 — ai skills available (ai-atoms.com catalog)
// ---------------------------------------------------------------------------

// startSkillsCatalogServer starts a mock ai-atoms.com catalog server that
// serves a catalog JSON document built from the given skill entries.
//
// entries maps slug → (description, lifecycle). All entries are emitted as
// "skill" type atoms in the catalog.
func startSkillsCatalogServer(t *testing.T, atoms []map[string]interface{}) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(buildCatalogPayload(atoms))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestSkillsAvailable_Success(t *testing.T) {
	root := t.TempDir()
	atoms := []map[string]interface{}{
		{"type": "skill", "id": "skill/commit", "name": "commit", "description": "Generate commit messages.", "lifecycle": "stable", "version": "1.2.0"},
		{"type": "skill", "id": "skill/review", "name": "review", "description": "AI code review assistant.", "lifecycle": "stable", "version": "0.5.1"},
	}
	srv := startSkillsCatalogServer(t, atoms)
	setAiAtomsCatalogURL(t, srv.URL+"/catalog.json")

	out, _, err := runSkillsCmd(t, root, "skills", "available")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Table must include both skills.
	if !strings.Contains(out, "commit") {
		t.Errorf("output missing 'commit'; got:\n%s", out)
	}
	if !strings.Contains(out, "Generate commit messages.") {
		t.Errorf("output missing description; got:\n%s", out)
	}
	if !strings.Contains(out, "review") {
		t.Errorf("output missing 'review'; got:\n%s", out)
	}
}

func TestSkillsAvailable_Empty(t *testing.T) {
	root := t.TempDir()
	srv := startSkillsCatalogServer(t, nil)
	setAiAtomsCatalogURL(t, srv.URL+"/catalog.json")

	out, _, err := runSkillsCmd(t, root, "skills", "available")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "(no skills available)") {
		t.Errorf("expected '(no skills available)'; got:\n%s", out)
	}
}

func TestSkillsAvailable_FiltersDeprecated(t *testing.T) {
	root := t.TempDir()
	atoms := []map[string]interface{}{
		{"type": "skill", "id": "skill/active", "name": "active", "description": "Active skill.", "lifecycle": "stable", "version": "1.0.0"},
		{"type": "skill", "id": "skill/deprecated", "name": "deprecated", "description": "Old skill.", "lifecycle": "deprecated", "version": "0.9.0"},
		{"type": "skill", "id": "skill/retired", "name": "retired", "description": "Retired skill.", "lifecycle": "retired", "version": "0.1.0"},
	}
	srv := startSkillsCatalogServer(t, atoms)
	setAiAtomsCatalogURL(t, srv.URL+"/catalog.json")

	out, _, err := runSkillsCmd(t, root, "skills", "available")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(out, "active") {
		t.Errorf("output should contain 'active'; got:\n%s", out)
	}
	if strings.Contains(out, "deprecated") {
		t.Errorf("output should NOT contain 'deprecated' skill; got:\n%s", out)
	}
	if strings.Contains(out, "retired") {
		t.Errorf("output should NOT contain 'retired' skill; got:\n%s", out)
	}
}

// fakeSkillAtomWithLifecycle is kept for use in install/upgrade tests which
// still rely on SkillAtomsBaseURL and the per-atom GitHub API endpoint.
func fakeSkillAtomWithLifecycle(slug, version, description, lifecycle string) []byte {
	payload := map[string]interface{}{
		"id":                     "skill/" + slug,
		"version":                version,
		"name":                   slug,
		"description":            description,
		"system_prompt_fragment": "",
		"lifecycle":              lifecycle,
	}
	b, _ := json.Marshal(payload)
	return b
}

// ---------------------------------------------------------------------------
// #371 — ai skills link
// ---------------------------------------------------------------------------

// makeSkillsWithSKILLMD creates skill directories that each contain a SKILL.md
// under <root>/skills/<slug>/SKILL.md. Returns the AI_ROOT dir.
func makeSkillsWithSKILLMD(t *testing.T, slugs ...string) string {
	t.Helper()
	root := t.TempDir()
	for _, slug := range slugs {
		dir := filepath.Join(root, "skills", slug)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		md := "---\nname: " + slug + "\ndescription: test\nversion: 1.0.0\n---\n# " + slug + "\n"
		if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(md), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestSkillsLink_LinkedToBoth(t *testing.T) {
	root := makeSkillsWithSKILLMD(t, "alpha", "beta")

	claudeDir := t.TempDir()
	copilotDir := t.TempDir()

	t.Setenv("AI_ROOT", root)
	t.Setenv("CLAUDE_SKILLS_DIR", claudeDir)
	t.Setenv("COPILOT_INSTRUCTIONS_DIR", copilotDir)

	out, _, err := runSkillsCmd(t, root, "skills", "link")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Output must report 2 Claude links and 2 Copilot links.
	if !strings.Contains(out, "2") {
		t.Errorf("expected count 2 in output; got:\n%s", out)
	}

	// Claude symlinks: ~/.claude/skills/alpha and ~/.claude/skills/beta
	for _, slug := range []string{"alpha", "beta"} {
		linkPath := filepath.Join(claudeDir, slug)
		if _, err := os.Lstat(linkPath); err != nil {
			t.Errorf("expected Claude symlink %s; got err: %v", linkPath, err)
		}
	}

	// Copilot symlinks: ~/.copilot/instructions/alpha.md and beta.md
	for _, slug := range []string{"alpha", "beta"} {
		linkPath := filepath.Join(copilotDir, slug+".md")
		if _, err := os.Lstat(linkPath); err != nil {
			t.Errorf("expected Copilot symlink %s; got err: %v", linkPath, err)
		}
	}
}

func TestSkillsLink_NoDirs(t *testing.T) {
	// No CLAUDE_SKILLS_DIR and no existing ~/.copilot/instructions → 0, 0
	root := makeSkillsWithSKILLMD(t, "alpha", "beta")

	t.Setenv("AI_ROOT", root)
	// Point CLAUDE_SKILLS_DIR at a non-existent directory.
	t.Setenv("CLAUDE_SKILLS_DIR", filepath.Join(t.TempDir(), "nonexistent"))
	// COPILOT_INSTRUCTIONS_DIR not set; no ~/.copilot/instructions exists.
	t.Setenv("COPILOT_INSTRUCTIONS_DIR", "")

	out, _, err := runSkillsCmd(t, root, "skills", "link")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Both counts should be 0.
	if !strings.Contains(out, "0") {
		t.Errorf("expected 0 links reported; got:\n%s", out)
	}
}

func TestSkillsLink_Idempotent(t *testing.T) {
	// Running link twice should not error and should produce the same symlinks.
	root := makeSkillsWithSKILLMD(t, "gamma")

	claudeDir := t.TempDir()
	copilotDir := t.TempDir()

	t.Setenv("AI_ROOT", root)
	t.Setenv("CLAUDE_SKILLS_DIR", claudeDir)
	t.Setenv("COPILOT_INSTRUCTIONS_DIR", copilotDir)

	// First run.
	if _, _, err := runSkillsCmd(t, root, "skills", "link"); err != nil {
		t.Fatalf("first link run failed: %v", err)
	}
	// Second run — must not fail.
	if _, _, err := runSkillsCmd(t, root, "skills", "link"); err != nil {
		t.Fatalf("second link run (idempotent) failed: %v", err)
	}

	// Symlinks must still exist after both runs.
	if _, err := os.Lstat(filepath.Join(claudeDir, "gamma")); err != nil {
		t.Errorf("Claude symlink missing after idempotent re-link: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(copilotDir, "gamma.md")); err != nil {
		t.Errorf("Copilot symlink missing after idempotent re-link: %v", err)
	}
}

// ---------------------------------------------------------------------------
// #371 — Copilot wiring on install / uninstall
// ---------------------------------------------------------------------------

func TestCopilotWiringOnInstall(t *testing.T) {
	root := t.TempDir()
	copilotDir := t.TempDir()

	atoms := []map[string]interface{}{
		{
			"type":                   "skill",
			"id":                     "skill/commit",
			"version":                "1.2.0",
			"name":                   "commit",
			"description":            "Generate a commit message.",
			"system_prompt_fragment": "You are a commit assistant.",
			"lifecycle":              "stable",
		},
	}
	srv := startSkillsCatalogServer(t, atoms)
	setAiAtomsCatalogURL(t, srv.URL+"/catalog.json")

	t.Setenv("AI_ROOT", root)
	// Redirect Claude symlink target to a temp dir so tests don't pollute the
	// real ~/.claude/skills/ with symlinks pointing into t.TempDir() paths that
	// are cleaned up after the test completes.
	t.Setenv("CLAUDE_SKILLS_DIR", t.TempDir())
	t.Setenv("COPILOT_INSTRUCTIONS_DIR", copilotDir)

	out, _, err := runSkillsCmd(t, root, "skills", "install", "commit")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "Installed commit v1.2.0") {
		t.Errorf("expected install confirmation; got:\n%s", out)
	}

	// Copilot symlink must exist at ~/.copilot/instructions/commit.md
	linkPath := filepath.Join(copilotDir, "commit.md")
	info, err := os.Lstat(linkPath)
	if err != nil {
		t.Fatalf("expected Copilot symlink at %s; got err: %v", linkPath, err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Errorf("expected %s to be a symlink; got mode %v", linkPath, info.Mode())
	}
}

func TestCopilotWiringOnUninstall(t *testing.T) {
	root := t.TempDir()

	// Pre-create the skill dir with a SKILL.md.
	skillDir := filepath.Join(root, "skills", "commit")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"),
		[]byte("---\nname: commit\ndescription: test\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create a fake Copilot instructions dir with a pre-existing symlink.
	copilotDir := t.TempDir()
	linkPath := filepath.Join(copilotDir, "commit.md")
	if err := os.Symlink(filepath.Join(skillDir, "SKILL.md"), linkPath); err != nil {
		t.Fatal(err)
	}

	t.Setenv("AI_ROOT", root)
	t.Setenv("COPILOT_INSTRUCTIONS_DIR", copilotDir)

	out, _, err := runSkillsCmd(t, root, "skills", "uninstall", "commit")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "Uninstalled commit") {
		t.Errorf("expected uninstall confirmation; got:\n%s", out)
	}

	// Copilot symlink must be gone.
	if _, lstatErr := os.Lstat(linkPath); !os.IsNotExist(lstatErr) {
		t.Errorf("Copilot symlink should have been removed; lstat: %v", lstatErr)
	}
}

// fakeSkillAtomWithDeps builds an atom JSON payload that includes a depends_on list.
func fakeSkillAtomWithDeps(slug, version, description string, deps []string) []byte {
	payload := map[string]interface{}{
		"id":                     "skill/" + slug,
		"version":                version,
		"name":                   slug,
		"description":            description,
		"system_prompt_fragment": "",
		"depends_on":             deps,
	}
	b, _ := json.Marshal(payload)
	return b
}

// ---------------------------------------------------------------------------
// #375 — ai skills available: slug appears in output
// ---------------------------------------------------------------------------

func TestSkillsAvailable_ShowsSlugColumn(t *testing.T) {
	root := t.TempDir()
	atoms := []map[string]interface{}{
		{"type": "skill", "id": "skill/commit", "name": "commit", "description": "Generate commit messages.", "lifecycle": "stable", "version": "1.0.0"},
	}
	srv := startSkillsCatalogServer(t, atoms)
	setAiAtomsCatalogURL(t, srv.URL+"/catalog.json")

	out, _, err := runSkillsCmd(t, root, "skills", "available")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// New format: two-line display with slug on its own line.
	if !strings.Contains(out, "commit") {
		t.Errorf("expected slug 'commit' in output; got:\n%s", out)
	}
	// Description must appear.
	if !strings.Contains(out, "Generate commit messages") {
		t.Errorf("expected description in output; got:\n%s", out)
	}
}

// ---------------------------------------------------------------------------
// #375 — ai skills install: depends_on resolution
// ---------------------------------------------------------------------------

// TestSkillsInstall_InstallsDependencies verifies that when a skill atom
// declares depends_on and stdout is non-interactive, all listed dependencies
// are installed automatically.
func TestSkillsInstall_InstallsDependencies(t *testing.T) {
	root := t.TempDir()
	// Redirect Claude symlink target to a temp dir so tests don't pollute the
	// real ~/.claude/skills/ with symlinks pointing into t.TempDir() paths that
	// are cleaned up after the test completes.
	t.Setenv("CLAUDE_SKILLS_DIR", t.TempDir())

	// "make" depends on "make-commit" and "make-review"; all in the catalog.
	atoms := []map[string]interface{}{
		{
			"type":        "skill",
			"id":          "skill/make",
			"version":     "1.0.0",
			"name":        "make",
			"description": "Run make tasks.",
			"lifecycle":   "stable",
			"depends_on":  []string{"make-commit", "make-review"},
		},
		{
			"type":        "skill",
			"id":          "skill/make-commit",
			"version":     "1.0.0",
			"name":        "make-commit",
			"description": "make commit sub-skill.",
			"lifecycle":   "stable",
		},
		{
			"type":        "skill",
			"id":          "skill/make-review",
			"version":     "1.0.0",
			"name":        "make-review",
			"description": "make review sub-skill.",
			"lifecycle":   "stable",
		},
	}
	srv := startSkillsCatalogServer(t, atoms)
	setAiAtomsCatalogURL(t, srv.URL+"/catalog.json")

	// stdout is not a TTY in test (non-interactive) → deps installed automatically.
	out, _, err := runSkillsCmd(t, root, "skills", "install", "make")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Primary skill installed.
	if !strings.Contains(out, "Installed make v1.0.0") {
		t.Errorf("expected 'Installed make v1.0.0'; got:\n%s", out)
	}

	// Both dependencies installed automatically.
	for _, dep := range []string{"make-commit", "make-review"} {
		depMDPath := filepath.Join(root, "skills", dep, "SKILL.md")
		if _, statErr := os.Stat(depMDPath); os.IsNotExist(statErr) {
			t.Errorf("dependency %s was not installed (SKILL.md missing at %s)", dep, depMDPath)
		}
	}
}

// TestSkillsInstall_NoDependencies verifies that a skill with no depends_on
// does not emit any dependency-related output.
func TestSkillsInstall_NoDependencies(t *testing.T) {
	root := t.TempDir()
	// Redirect Claude symlink target to a temp dir so tests don't pollute the
	// real ~/.claude/skills/ with symlinks pointing into t.TempDir() paths that
	// are cleaned up after the test completes.
	t.Setenv("CLAUDE_SKILLS_DIR", t.TempDir())
	atoms := []map[string]interface{}{
		{
			"type":        "skill",
			"id":          "skill/commit",
			"version":     "1.0.0",
			"name":        "commit",
			"description": "Generate commit messages.",
			"lifecycle":   "stable",
		},
	}
	srv := startSkillsCatalogServer(t, atoms)
	setAiAtomsCatalogURL(t, srv.URL+"/catalog.json")

	out, _, err := runSkillsCmd(t, root, "skills", "install", "commit")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if strings.Contains(out, "depends on") {
		t.Errorf("expected no dependency output for skill without depends_on; got:\n%s", out)
	}
	if strings.Contains(out, "Install dependencies") {
		t.Errorf("expected no dependency prompt for skill without depends_on; got:\n%s", out)
	}
}
