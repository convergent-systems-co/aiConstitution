package cmd_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// fakeHookAtom builds a minimal ai-hook atom JSON payload for testing.
func fakeHookAtom(slug, description, lifecycle string, events []string) []byte {
	payload := map[string]interface{}{
		"type":        "ai-hook",
		"id":          "ai-hook/" + slug,
		"name":        slug,
		"description": description,
		"lifecycle":   lifecycle,
		"events":      events,
	}
	b, _ := json.Marshal(payload)
	return b
}

// startHookAtomsServer starts an httptest.Server that mimics the GitHub
// Contents API for the skills/ai-hook directory. entries maps slug → atom
// JSON body. The directory listing is synthesised from the keys.
func startHookAtomsServer(t *testing.T, entries map[string][]byte) *httptest.Server {
	t.Helper()
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		// Directory listing endpoint: /skills/ai-hook
		if r.URL.Path == "/skills/ai-hook" {
			var listing []map[string]interface{}
			for slug := range entries {
				listing = append(listing, map[string]interface{}{
					"name":         slug + ".json",
					"download_url": srv.URL + "/atoms/" + slug + ".json",
				})
			}
			b, _ := json.Marshal(listing)
			_, _ = w.Write(b)
			return
		}

		// Individual atom endpoint: /atoms/<slug>.json
		if strings.HasPrefix(r.URL.Path, "/atoms/") {
			slug := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/atoms/"), ".json")
			if body, ok := entries[slug]; ok {
				_, _ = w.Write(body)
				return
			}
		}

		// Anything else → 404.
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"Not Found"}`))
	}))
	t.Cleanup(srv.Close)
	return srv
}

// ---------------------------------------------------------------------------
// #398 — ai hooks available registry integration
// ---------------------------------------------------------------------------

// TestHooksAvailable_RegistryFetched verifies that when the registry is
// reachable and returns ai-hook atoms, the output includes a "Registry hooks"
// section with the atom's slug and description.
func TestHooksAvailable_RegistryFetched(t *testing.T) {
	root := t.TempDir()
	entries := map[string][]byte{
		"audit":        fakeHookAtom("audit", "Append JSONL interaction records to ~/.ai/audit.", "stable", []string{"PostToolUse", "PreToolUse"}),
		"branch-guard": fakeHookAtom("branch-guard", "Block direct commits to protected branches.", "stable", []string{"PreToolUse"}),
	}
	srv := startHookAtomsServer(t, entries)
	setSkillAtomBaseURL(t, srv.URL)

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

// TestHooksAvailable_RegistryUnreachable verifies that when the registry
// returns an error, the embedded hooks are still listed and a warning is shown.
func TestHooksAvailable_RegistryUnreachable(t *testing.T) {
	root := t.TempDir()
	// Serve 500 for every request to simulate an unreachable registry.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"message":"Internal Server Error"}`))
	}))
	t.Cleanup(srv.Close)
	setSkillAtomBaseURL(t, srv.URL)

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

// TestHooksAvailable_RegistryEmpty verifies that when the registry returns an
// empty directory listing, the embedded hooks are shown and no registry section
// is added.
func TestHooksAvailable_RegistryEmpty(t *testing.T) {
	root := t.TempDir()
	srv := startHookAtomsServer(t, map[string][]byte{})
	setSkillAtomBaseURL(t, srv.URL)

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
