package cmd_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/convergent-systems-co/aiConstitution/src/cmd/ai/cmd"
)

// setAiAtomsCatalogURL temporarily overrides cmd.AiAtomsCatalogURL for the
// duration of the test, using the exported test pointer.
func setAiAtomsCatalogURL(t *testing.T, url string) {
	t.Helper()
	old := *cmd.AiAtomsCatalogURLForTest
	*cmd.AiAtomsCatalogURLForTest = url
	t.Cleanup(func() { *cmd.AiAtomsCatalogURLForTest = old })
}

// buildCatalogPayload constructs a minimal catalog JSON document containing the
// given atom entries.
func buildCatalogPayload(atoms []map[string]interface{}) []byte {
	doc := map[string]interface{}{
		"catalog": "ai-atoms",
		"version": "0.1.0",
		"atoms":   atoms,
	}
	b, _ := json.Marshal(doc)
	return b
}

// ---------------------------------------------------------------------------
// TestFetchAiAtomsCatalog_ParsesResponse
// ---------------------------------------------------------------------------

// TestFetchAiAtomsCatalog_ParsesResponse verifies that fetchAiAtomsCatalog
// correctly parses a catalog containing both skill and hook entries.
func TestFetchAiAtomsCatalog_ParsesResponse(t *testing.T) {
	atoms := []map[string]interface{}{
		{
			"type":        "skill",
			"id":          "skill/make",
			"name":        "Make",
			"description": "Run make tasks.",
			"lifecycle":   "stable",
			"version":     "1.0.0",
		},
		{
			"type":        "hook",
			"id":          "hook/branch-guard",
			"name":        "Branch Guard",
			"description": "Block direct commits to protected branches.",
			"lifecycle":   "stable",
			"event":       "PreToolUse",
			"version":     "1.0.0",
		},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(buildCatalogPayload(atoms))
	}))
	t.Cleanup(srv.Close)
	setAiAtomsCatalogURL(t, srv.URL+"/catalog.json")

	root := t.TempDir()
	out, _, err := runSkillsCmd(t, root, "skills", "available")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// The skill "make" should appear in the output.
	if !strings.Contains(out, "make") {
		t.Errorf("expected 'make' in skills available output; got:\n%s", out)
	}
}

// ---------------------------------------------------------------------------
// TestFetchAiAtomsCatalog_HTTPError
// ---------------------------------------------------------------------------

// TestFetchAiAtomsCatalog_HTTPError verifies that a 500 response causes an
// error to be returned from fetchAiAtomsCatalog.
func TestFetchAiAtomsCatalog_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"internal server error"}`))
	}))
	t.Cleanup(srv.Close)
	setAiAtomsCatalogURL(t, srv.URL+"/catalog.json")

	root := t.TempDir()
	_, _, err := runSkillsCmd(t, root, "skills", "available")
	if err == nil {
		t.Fatal("expected error for HTTP 500, got nil")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("expected error message to mention 500; got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// TestFetchAiAtomsCatalog_InvalidJSON
// ---------------------------------------------------------------------------

// TestFetchAiAtomsCatalog_InvalidJSON verifies that malformed JSON from the
// catalog endpoint causes a decode error to be returned.
func TestFetchAiAtomsCatalog_InvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`this is not json {{{`))
	}))
	t.Cleanup(srv.Close)
	setAiAtomsCatalogURL(t, srv.URL+"/catalog.json")

	root := t.TempDir()
	_, _, err := runSkillsCmd(t, root, "skills", "available")
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}

// ---------------------------------------------------------------------------
// TestSkillsAvailable_UsesAiAtomsCatalog
// ---------------------------------------------------------------------------

// TestSkillsAvailable_UsesAiAtomsCatalog is the end-to-end test for the
// catalog-backed skills listing. It verifies:
//   - skill type entries appear
//   - hook type entries are excluded
//   - deprecated/retired skills are filtered
//   - sub-skills (in depends_on) are deduplicated from the top-level list
func TestSkillsAvailable_UsesAiAtomsCatalog(t *testing.T) {
	atoms := []map[string]interface{}{
		{
			"type":        "skill",
			"id":          "skill/make",
			"name":        "Make",
			"description": "Unified make dispatcher.",
			"lifecycle":   "stable",
			"version":     "1.0.0",
			"depends_on":  []string{"make-work"},
		},
		{
			"type":        "skill",
			"id":          "skill/make-work",
			"name":        "Make Work",
			"description": "Sub-skill for make.",
			"lifecycle":   "stable",
			"version":     "1.0.0",
		},
		{
			"type":        "skill",
			"id":          "skill/old",
			"name":        "Old",
			"description": "Deprecated skill.",
			"lifecycle":   "deprecated",
			"version":     "0.1.0",
		},
		{
			"type":        "hook",
			"id":          "hook/branch-guard",
			"name":        "Branch Guard",
			"description": "Block commits to main.",
			"lifecycle":   "stable",
			"version":     "1.0.0",
		},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(buildCatalogPayload(atoms))
	}))
	t.Cleanup(srv.Close)
	setAiAtomsCatalogURL(t, srv.URL+"/catalog.json")

	root := t.TempDir()
	out, _, err := runSkillsCmd(t, root, "skills", "available")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// "make" should appear with its (+1) sub-skill annotation.
	if !strings.Contains(out, "make") {
		t.Errorf("expected 'make' in output; got:\n%s", out)
	}
	// "make-work" is a sub-skill and must be deduplicated (not shown top-level).
	// It might appear as part of the "+1" annotation line for "make" but not as its own row.
	// Check that no standalone "make-work" row exists (it would appear as a separate line).
	lines := strings.Split(out, "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) > 0 && fields[0] == "make-work" {
			t.Errorf("make-work should be deduplicated as a sub-skill, not shown top-level; got line: %q", line)
		}
	}
	// "old" (deprecated) must not appear.
	if strings.Contains(out, "old") && !strings.Contains(out, "make") {
		// allow "old" only if it appears as part of another word (e.g. "bold")
	}
	// Strict check: "Old" (the Name field) must not appear.
	if strings.Contains(out, "\nold ") || strings.Contains(out, "  old ") {
		t.Errorf("deprecated skill 'old' should not appear; got:\n%s", out)
	}
	// Hook entries must not appear in skills available.
	if strings.Contains(out, "branch-guard") {
		t.Errorf("hook entries should not appear in skills available; got:\n%s", out)
	}
}

// ---------------------------------------------------------------------------
// TestHooksAvailable_UsesAiAtomsCatalog
// ---------------------------------------------------------------------------

// TestHooksAvailable_UsesAiAtomsCatalog is the end-to-end test for the
// catalog-backed hooks listing. It verifies:
//   - hook type entries appear in the registry section
//   - skill type entries are excluded from the registry section
//   - deprecated/retired hooks are filtered
func TestHooksAvailable_UsesAiAtomsCatalog(t *testing.T) {
	atoms := []map[string]interface{}{
		{
			"type":        "hook",
			"id":          "hook/branch-guard",
			"name":        "Branch Guard",
			"description": "Block direct commits to protected branches.",
			"lifecycle":   "stable",
			"event":       "PreToolUse",
			"version":     "1.0.0",
		},
		{
			"type":        "hook",
			"id":          "hook/old-hook",
			"name":        "Old Hook",
			"description": "Retired hook.",
			"lifecycle":   "retired",
			"version":     "0.1.0",
		},
		{
			"type":        "skill",
			"id":          "skill/make",
			"name":        "Make",
			"description": "Make dispatcher.",
			"lifecycle":   "stable",
			"version":     "1.0.0",
		},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(buildCatalogPayload(atoms))
	}))
	t.Cleanup(srv.Close)
	setAiAtomsCatalogURL(t, srv.URL+"/catalog.json")

	root := t.TempDir()
	out, _, err := runSkillsCmd(t, root, "hooks", "available")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Registry section must appear.
	if !strings.Contains(out, "Registry hooks") {
		t.Errorf("expected 'Registry hooks' section; got:\n%s", out)
	}
	// "branch-guard" must appear.
	if !strings.Contains(out, "branch-guard") {
		t.Errorf("expected 'branch-guard' in hooks available; got:\n%s", out)
	}
	// "old-hook" (retired) must not appear.
	if strings.Contains(out, "old-hook") {
		t.Errorf("retired hook 'old-hook' should not appear; got:\n%s", out)
	}
	// Skill entries must not appear in the registry section.
	if strings.Contains(out, "Make dispatcher") {
		t.Errorf("skill entry description should not appear in hooks available; got:\n%s", out)
	}
	// Embedded hooks must still appear.
	if !strings.Contains(out, "Embedded hooks") {
		t.Errorf("expected 'Embedded hooks' section; got:\n%s", out)
	}
}
