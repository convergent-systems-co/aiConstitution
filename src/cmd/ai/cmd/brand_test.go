package cmd_test

// brand_test.go — TDD suite for `ai brand fetch` and `ai brand list` (#353).
//
// All tests:
//   - redirect AI_ROOT and AICONST_CONFIG_DIR via env vars
//   - mock the GitHub API via httptest.Server
//   - verify behavior through the cobra root command tree

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/convergent-systems-co/aiConstitution/src/cmd/ai/cmd"
)

// ---- helpers ----------------------------------------------------------------

// brandEnv sets AI_ROOT and AICONST_CONFIG_DIR to fresh temp dirs and
// returns the two paths. Call at the start of every brand test.
func brandEnv(t *testing.T) (aiRoot, configDir string) {
	t.Helper()
	aiRoot = t.TempDir()
	configDir = t.TempDir()
	t.Setenv("AI_ROOT", aiRoot)
	t.Setenv("AICONST_CONFIG_DIR", configDir)
	return
}

// runBrandCmd runs `ai brand <args...>` and returns combined stdout+stderr
// output plus the execution error (if any).
func runBrandCmd(t *testing.T, args ...string) (string, error) {
	t.Helper()
	root := cmd.NewRootCmd()
	var out strings.Builder
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs(append([]string{"brand"}, args...))
	err := root.Execute()
	return out.String(), err
}

// githubDirEntry builds a GitHub Contents API directory entry JSON object.
func githubDirEntry(name, typ, downloadURL string) map[string]interface{} {
	e := map[string]interface{}{
		"name": name,
		"type": typ,
		"url":  "https://api.github.com/repos/convergent-systems-co/brand-atoms/contents/brands/" + name,
	}
	if downloadURL != "" {
		e["download_url"] = downloadURL
	}
	return e
}

// ---- TestBrandList_Success --------------------------------------------------

// TestBrandList_Success verifies that `ai brand list` queries the GitHub
// Contents API, reads atom.json for each brand slug, and prints a SLUG/VERSION
// table via tabwriter.
func TestBrandList_Success(t *testing.T) {
	brandEnv(t)

	// atom.json content served by the mock server for "acme" brand.
	atomJSON := `{"version":"1.2.3"}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/brands"):
			// Directory listing: two brand dirs.
			dirs := []map[string]interface{}{
				githubDirEntry("acme", "dir", ""),
				githubDirEntry("widget-co", "dir", ""),
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(dirs)
		case strings.Contains(r.URL.Path, "/brands/acme/atom.json"):
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(atomJSON))
		case strings.Contains(r.URL.Path, "/brands/widget-co/atom.json"):
			// widget-co has no atom.json — 404.
			w.WriteHeader(http.StatusNotFound)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)

	// Override the brand registry URL so the command hits our mock server.
	t.Setenv("AICONST_BRAND_REGISTRY_URL", srv.URL)

	out, err := runBrandCmd(t, "list")
	if err != nil {
		t.Fatalf("brand list failed: %v\noutput: %s", err, out)
	}

	if !strings.Contains(out, "acme") {
		t.Errorf("output missing brand slug 'acme'; got:\n%s", out)
	}
	if !strings.Contains(out, "1.2.3") {
		t.Errorf("output missing version '1.2.3'; got:\n%s", out)
	}
	if !strings.Contains(out, "widget-co") {
		t.Errorf("output missing brand slug 'widget-co'; got:\n%s", out)
	}
	// widget-co has no atom.json — version should fall back to "unknown".
	if !strings.Contains(out, "unknown") {
		t.Errorf("output missing 'unknown' for brand with no atom.json; got:\n%s", out)
	}
}

// ---- TestBrandList_APIFail --------------------------------------------------

// TestBrandList_APIFail verifies that a non-2xx response from the GitHub API
// causes `ai brand list` to return an error (non-zero exit).
func TestBrandList_APIFail(t *testing.T) {
	brandEnv(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden) // 403
	}))
	t.Cleanup(srv.Close)

	t.Setenv("AICONST_BRAND_REGISTRY_URL", srv.URL)

	_, err := runBrandCmd(t, "list")
	if err == nil {
		t.Fatal("expected error on HTTP 403 from brand list, got nil")
	}
}

// ---- TestBrandFetch_CreatesCache -------------------------------------------

// TestBrandFetch_CreatesCache verifies that `ai brand fetch <slug>` downloads
// brand atom files from the GitHub Contents API and writes them to
// ~/.ai/atoms/cache/brand-atoms/<slug>/.
func TestBrandFetch_CreatesCache(t *testing.T) {
	aiRoot, _ := brandEnv(t)

	const slug = "acme"
	fileContent := "# Brand Guidelines\n"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/brands/"+slug) && !strings.Contains(r.URL.Path, "/brands/"+slug+"/"):
			// Directory listing for the brand slug.
			files := []map[string]interface{}{
				{
					"name":         "README.md",
					"type":         "file",
					"download_url": "http://" + r.Host + "/download/README.md",
				},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(files)
		case strings.HasSuffix(r.URL.Path, "/download/README.md"):
			_, _ = w.Write([]byte(fileContent))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)

	t.Setenv("AICONST_BRAND_REGISTRY_URL", srv.URL)

	out, err := runBrandCmd(t, "fetch", slug)
	if err != nil {
		t.Fatalf("brand fetch failed: %v\noutput: %s", err, out)
	}

	// File must have been written to cache.
	cachedFile := filepath.Join(aiRoot, "atoms", "cache", "brand-atoms", slug, "README.md")
	data, readErr := os.ReadFile(cachedFile)
	if readErr != nil {
		t.Fatalf("cached file not found at %q: %v", cachedFile, readErr)
	}
	if string(data) != fileContent {
		t.Errorf("cached file content mismatch; want %q, got %q", fileContent, string(data))
	}

	// Output must mention the slug.
	if !strings.Contains(out, slug) {
		t.Errorf("output missing slug %q; got:\n%s", slug, out)
	}
}

// ---- TestBrandFetch_AppliesBrandTOML ----------------------------------------

// TestBrandFetch_AppliesBrandTOML verifies that when the brand directory
// contains a brand.toml, its voice/tone/name settings are written to
// ~/.config/aiConstitution/settings.toml.
func TestBrandFetch_AppliesBrandTOML(t *testing.T) {
	_, configDir := brandEnv(t)

	const slug = "acme-branded"
	brandTOML := `name = "Acme Corp"` + "\n" + `voice = "friendly"` + "\n" + `tone = "casual"` + "\n"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/brands/"+slug) && !strings.Contains(r.URL.Path, "/brands/"+slug+"/"):
			files := []map[string]interface{}{
				{
					"name":         "brand.toml",
					"type":         "file",
					"download_url": "http://" + r.Host + "/download/brand.toml",
				},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(files)
		case strings.HasSuffix(r.URL.Path, "/download/brand.toml"):
			_, _ = w.Write([]byte(brandTOML))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)

	t.Setenv("AICONST_BRAND_REGISTRY_URL", srv.URL)

	out, err := runBrandCmd(t, "fetch", slug)
	if err != nil {
		t.Fatalf("brand fetch with brand.toml failed: %v\noutput: %s", err, out)
	}

	// settings.toml must have been written (or updated).
	settingsPath := filepath.Join(configDir, "settings.toml")
	if _, statErr := os.Stat(settingsPath); statErr != nil {
		t.Fatalf("settings.toml not written to %q: %v", settingsPath, statErr)
	}

	// Output must mention that brand.toml was applied.
	if !strings.Contains(out, "brand.toml") && !strings.Contains(out, "applied") && !strings.Contains(out, "Applied") {
		t.Errorf("output should mention brand.toml application; got:\n%s", out)
	}
}
