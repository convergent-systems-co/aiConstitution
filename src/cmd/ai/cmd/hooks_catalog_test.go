package cmd_test

// hooks_catalog_test.go — TDD tests for installHookFromCatalog (story #418)
//
// These tests verify catalog-first install behavior:
//   - Success: hook atom with script field → file written at correct path, mode 0755
//   - NotFound: hook absent from catalog → ErrHookNotInCatalog
//   - NoScript: atom present but script is empty → ErrHookNotInCatalog
//   - NetworkError: catalog returns 500 → wrapped error, NOT ErrHookNotInCatalog

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/convergent-systems-co/aiConstitution/src/cmd/ai/cmd"
)

// startErrorServer starts a test server that always returns the given HTTP status code.
func startErrorServer(t *testing.T, statusCode int) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(statusCode)
		_, _ = w.Write([]byte(`{"error":"server error"}`))
	}))
	t.Cleanup(srv.Close)
	return srv
}

// ---------------------------------------------------------------------------
// TestInstallHookFromCatalog_Success
// ---------------------------------------------------------------------------

// TestInstallHookFromCatalog_Success verifies that when a hook atom has a
// non-empty script field, the script is written to hooksDir/<slug>.<ext>
// with 0755 permissions.
func TestInstallHookFromCatalog_Success(t *testing.T) {
	hooksDir := t.TempDir()

	scriptContent := "#!/usr/bin/env python3\nprint('hello from catalog')\n"
	payload := buildCatalogPayload([]map[string]interface{}{
		{
			"type":        "hook",
			"id":          "hook/my-hook",
			"name":        "My Hook",
			"description": "A test hook from catalog.",
			"lifecycle":   "stable",
			"version":     "1.0.0",
			"language":    "python",
			"script":      scriptContent,
		},
	})
	srv := startCatalogServer(t, payload)
	setAiAtomsCatalogURL(t, srv.URL+"/catalog.json")

	err := cmd.InstallHookFromCatalogForTest("my-hook", hooksDir)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	destPath := filepath.Join(hooksDir, "my-hook.py")
	data, readErr := os.ReadFile(destPath)
	if readErr != nil {
		t.Fatalf("hook file not written at %s: %v", destPath, readErr)
	}
	if string(data) != scriptContent {
		t.Errorf("hook content mismatch:\nwant: %q\ngot:  %q", scriptContent, string(data))
	}

	// Verify the file is executable (mode & 0111 != 0).
	info, statErr := os.Stat(destPath)
	if statErr != nil {
		t.Fatalf("stat %s: %v", destPath, statErr)
	}
	if info.Mode()&0o111 == 0 {
		t.Errorf("hook file is not executable; mode: %v", info.Mode())
	}
}

// TestInstallHookFromCatalog_ShellExtension verifies that a shell hook atom
// (language="sh") gets written with a .sh extension.
func TestInstallHookFromCatalog_ShellExtension(t *testing.T) {
	hooksDir := t.TempDir()

	scriptContent := "#!/usr/bin/env bash\necho 'shell hook'\n"
	payload := buildCatalogPayload([]map[string]interface{}{
		{
			"type":        "hook",
			"id":          "hook/shell-hook",
			"name":        "Shell Hook",
			"description": "A shell test hook.",
			"lifecycle":   "stable",
			"version":     "1.0.0",
			"language":    "sh",
			"script":      scriptContent,
		},
	})
	srv := startCatalogServer(t, payload)
	setAiAtomsCatalogURL(t, srv.URL+"/catalog.json")

	err := cmd.InstallHookFromCatalogForTest("shell-hook", hooksDir)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	destPath := filepath.Join(hooksDir, "shell-hook.sh")
	if _, statErr := os.Stat(destPath); os.IsNotExist(statErr) {
		t.Errorf("shell hook not written at %s", destPath)
	}
}

// ---------------------------------------------------------------------------
// TestInstallHookFromCatalog_NotFound
// ---------------------------------------------------------------------------

// TestInstallHookFromCatalog_NotFound verifies that when the slug does not
// appear in the catalog, ErrHookNotInCatalog is returned.
func TestInstallHookFromCatalog_NotFound(t *testing.T) {
	hooksDir := t.TempDir()

	// Catalog contains an unrelated hook; the target slug is absent.
	payload := buildCatalogPayload([]map[string]interface{}{
		{
			"type":     "hook",
			"id":       "hook/other-hook",
			"name":     "Other Hook",
			"lifecycle": "stable",
			"version":  "1.0.0",
			"script":   "#!/usr/bin/env python3\n",
		},
	})
	srv := startCatalogServer(t, payload)
	setAiAtomsCatalogURL(t, srv.URL+"/catalog.json")

	err := cmd.InstallHookFromCatalogForTest("missing-hook", hooksDir)
	if err == nil {
		t.Fatal("expected ErrHookNotInCatalog, got nil")
	}
	if !errors.Is(err, cmd.ErrHookNotInCatalogForTest) {
		t.Errorf("expected errors.Is(err, ErrHookNotInCatalog); got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// TestInstallHookFromCatalog_NoScript
// ---------------------------------------------------------------------------

// TestInstallHookFromCatalog_NoScript verifies that when the hook atom is
// present in the catalog but has an empty script field, ErrHookNotInCatalog
// is returned (backward-compat: catalog transition period).
func TestInstallHookFromCatalog_NoScript(t *testing.T) {
	hooksDir := t.TempDir()

	payload := buildCatalogPayload([]map[string]interface{}{
		{
			"type":     "hook",
			"id":       "hook/no-script-hook",
			"name":     "No Script Hook",
			"lifecycle": "stable",
			"version":  "1.0.0",
			// script field is intentionally absent (simulates transition period)
		},
	})
	srv := startCatalogServer(t, payload)
	setAiAtomsCatalogURL(t, srv.URL+"/catalog.json")

	err := cmd.InstallHookFromCatalogForTest("no-script-hook", hooksDir)
	if err == nil {
		t.Fatal("expected ErrHookNotInCatalog for atom without script, got nil")
	}
	if !errors.Is(err, cmd.ErrHookNotInCatalogForTest) {
		t.Errorf("expected errors.Is(err, ErrHookNotInCatalog); got: %v", err)
	}

	// No file should have been written.
	entries, _ := os.ReadDir(hooksDir)
	if len(entries) != 0 {
		names := make([]string, 0, len(entries))
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Errorf("unexpected files written to hooksDir: %v", names)
	}
}

// ---------------------------------------------------------------------------
// TestInstallHookFromCatalog_NetworkError
// ---------------------------------------------------------------------------

// TestInstallHookFromCatalog_NetworkError verifies that when the catalog
// server returns a non-200 status, a wrapped error is returned that is NOT
// ErrHookNotInCatalog (so callers can distinguish network failures from
// "hook simply not present").
func TestInstallHookFromCatalog_NetworkError(t *testing.T) {
	hooksDir := t.TempDir()

	srv := startErrorServer(t, http.StatusInternalServerError)
	setAiAtomsCatalogURL(t, srv.URL+"/catalog.json")

	err := cmd.InstallHookFromCatalogForTest("any-hook", hooksDir)
	if err == nil {
		t.Fatal("expected an error for HTTP 500, got nil")
	}
	if errors.Is(err, cmd.ErrHookNotInCatalogForTest) {
		t.Errorf("network error should NOT be ErrHookNotInCatalog; got: %v", err)
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("error should mention HTTP 500; got: %v", err)
	}
}
