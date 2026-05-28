package cmd_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// #398 / #415 — ai hooks available registry integration (ai-atoms.com catalog)
// ---------------------------------------------------------------------------

// TestHooksAvailable_RegistryFetched verifies that when the catalog is
// reachable and returns hook atoms, the output includes a "Registry hooks"
// section with the atom's slug and description.
func TestHooksAvailable_RegistryFetched(t *testing.T) {
	atoms := []map[string]interface{}{
		{
			"type":        "hook",
			"id":          "hook/audit",
			"name":        "Audit",
			"description": "Append JSONL interaction records to ~/.ai/audit.",
			"lifecycle":   "stable",
			"event":       "PostToolUse",
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
	out, _, err := runSkillsCmd(t, root, "hooks", "available")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(out, "Registry hooks") {
		t.Errorf("expected 'Registry hooks' section header; got:\n%s", out)
	}
	if !strings.Contains(out, "audit") {
		t.Errorf("expected 'audit' in registry section; got:\n%s", out)
	}
	if !strings.Contains(out, "branch-guard") {
		t.Errorf("expected 'branch-guard' in registry section; got:\n%s", out)
	}
	// Embedded hooks must still appear.
	if !strings.Contains(out, "Embedded hooks") {
		t.Errorf("expected 'Embedded hooks' section header; got:\n%s", out)
	}
}

// TestHooksAvailable_RegistryUnreachable verifies that when the catalog
// returns an error, the embedded hooks are still listed and a warning is shown.
func TestHooksAvailable_RegistryUnreachable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"message":"Internal Server Error"}`))
	}))
	t.Cleanup(srv.Close)
	setAiAtomsCatalogURL(t, srv.URL+"/catalog.json")

	root := t.TempDir()
	out, _, err := runSkillsCmd(t, root, "hooks", "available")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Embedded hooks must still appear.
	if !strings.Contains(out, "Embedded hooks") {
		t.Errorf("expected 'Embedded hooks' section header even on registry failure; got:\n%s", out)
	}
	// A warning about the registry must appear.
	if !strings.Contains(out, "could not reach skill-atoms.com") {
		t.Errorf("expected registry warning; got:\n%s", out)
	}
	// No registry section on failure.
	if strings.Contains(out, "Registry hooks") {
		t.Errorf("registry section should not appear on failure; got:\n%s", out)
	}
}

// TestHooksAvailable_RegistryEmpty verifies that when the catalog returns no
// hook atoms, the embedded hooks are shown and no registry section is added.
func TestHooksAvailable_RegistryEmpty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(buildCatalogPayload(nil))
	}))
	t.Cleanup(srv.Close)
	setAiAtomsCatalogURL(t, srv.URL+"/catalog.json")

	root := t.TempDir()
	out, _, err := runSkillsCmd(t, root, "hooks", "available")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Embedded hooks must still appear.
	if !strings.Contains(out, "Embedded hooks") {
		t.Errorf("expected 'Embedded hooks' section header; got:\n%s", out)
	}
	// No registry section when empty.
	if strings.Contains(out, "Registry hooks") {
		t.Errorf("registry section should not appear for empty listing; got:\n%s", out)
	}
}
