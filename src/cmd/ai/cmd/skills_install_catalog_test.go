package cmd_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeCatalog builds a catalog JSON document containing the given skill entries.
// Each entry should include at minimum "type", "id", "version", "name",
// "description", and optionally "system_prompt_fragment", "depends_on",
// and "lifecycle".
func fakeCatalog(skills []map[string]any) []byte {
	atoms := make([]any, 0, len(skills))
	for _, s := range skills {
		atoms = append(atoms, s)
	}
	b, _ := json.Marshal(map[string]any{"atoms": atoms})
	return b
}

// startCatalogServer starts an httptest.Server that serves the given catalog
// payload for any request. Registers t.Cleanup to close the server.
func startCatalogServer(t *testing.T, payload []byte) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(payload)
	}))
	t.Cleanup(srv.Close)
	return srv
}

// ---------------------------------------------------------------------------
// TestSkillsInstall_WritesSkillMD — install from catalog, verify SKILL.md
// ---------------------------------------------------------------------------

func TestSkillsInstall_WritesSkillMD(t *testing.T) {
	root := t.TempDir()

	payload := fakeCatalog([]map[string]any{
		{
			"type":                   "skill",
			"id":                     "skill/commit",
			"version":                "1.0.0",
			"name":                   "commit",
			"description":            "Generate commit messages.",
			"system_prompt_fragment": "You are a commit assistant.",
			"lifecycle":              "stable",
		},
	})
	srv := startCatalogServer(t, payload)
	setAiAtomsCatalogURL(t, srv.URL+"/catalog.json")

	out, _, err := runSkillsCmd(t, root, "skills", "install", "commit")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	skillMDPath := filepath.Join(root, "skills", "commit", "SKILL.md")
	data, readErr := os.ReadFile(skillMDPath)
	if readErr != nil {
		t.Fatalf("SKILL.md not written at %s: %v", skillMDPath, readErr)
	}

	content := string(data)
	if !strings.Contains(content, "name: commit") {
		t.Errorf("SKILL.md missing 'name: commit'; got:\n%s", content)
	}
	if !strings.Contains(content, "version: 1.0.0") {
		t.Errorf("SKILL.md missing 'version: 1.0.0'; got:\n%s", content)
	}
	if !strings.Contains(content, "Generate commit messages.") {
		t.Errorf("SKILL.md missing description; got:\n%s", content)
	}
	if !strings.Contains(content, "You are a commit assistant.") {
		t.Errorf("SKILL.md missing system_prompt_fragment content; got:\n%s", content)
	}

	if !strings.Contains(out, "Installed commit v1.0.0") {
		t.Errorf("expected 'Installed commit v1.0.0' in output; got:\n%s", out)
	}
}

// ---------------------------------------------------------------------------
// TestSkillsInstall_NotFoundError — slug absent from catalog returns an error
// ---------------------------------------------------------------------------

func TestSkillsInstall_NotFoundError(t *testing.T) {
	root := t.TempDir()

	// Catalog contains an unrelated skill; "ghost" is absent.
	payload := fakeCatalog([]map[string]any{
		{
			"type":      "skill",
			"id":        "skill/other",
			"version":   "1.0.0",
			"name":      "other",
			"lifecycle": "stable",
		},
	})
	srv := startCatalogServer(t, payload)
	setAiAtomsCatalogURL(t, srv.URL+"/catalog.json")

	_, errStr, err := runSkillsCmd(t, root, "skills", "install", "ghost")
	if err == nil {
		t.Fatal("expected an error when skill is not in catalog, got nil")
	}
	combined := errStr + err.Error()
	if !strings.Contains(combined, "not found") {
		t.Errorf("error should contain 'not found'; got: %s", combined)
	}
}

// ---------------------------------------------------------------------------
// TestSkillsInstall_InstallsDependencies_Catalog — depends_on resolved via catalog
// ---------------------------------------------------------------------------

func TestSkillsInstall_InstallsDependencies_Catalog(t *testing.T) {
	root := t.TempDir()

	payload := fakeCatalog([]map[string]any{
		{
			"type":        "skill",
			"id":          "skill/make",
			"version":     "1.0.0",
			"name":        "make",
			"description": "Unified make dispatcher.",
			"lifecycle":   "stable",
			"depends_on":  []string{"make-work"},
		},
		{
			"type":        "skill",
			"id":          "skill/make-work",
			"version":     "1.0.0",
			"name":        "make-work",
			"description": "Sub-skill for make.",
			"lifecycle":   "stable",
		},
	})
	srv := startCatalogServer(t, payload)
	setAiAtomsCatalogURL(t, srv.URL+"/catalog.json")

	// stdout is non-interactive in tests → deps installed automatically.
	out, _, err := runSkillsCmd(t, root, "skills", "install", "make")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(out, "Installed make v1.0.0") {
		t.Errorf("expected 'Installed make v1.0.0'; got:\n%s", out)
	}

	// Dependency SKILL.md must exist.
	depMDPath := filepath.Join(root, "skills", "make-work", "SKILL.md")
	if _, statErr := os.Stat(depMDPath); os.IsNotExist(statErr) {
		t.Errorf("dependency make-work was not installed (SKILL.md missing at %s)", depMDPath)
	}
}
