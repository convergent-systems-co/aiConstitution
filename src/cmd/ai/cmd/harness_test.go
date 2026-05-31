// harness_test.go — sandbox() integration-test harness.
//
// Every integration test calls sandbox(t) as its first line. The helper
// wires every external boundary (filesystem paths, env vars, URL package
// vars, HTTP servers, git remote) to temp resources and restores them
// via t.Cleanup.
//
// Tests that use sandbox MUST NOT call t.Parallel() because sandbox
// modifies package-level vars (URL pointers, paths overrides).
package cmd_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	cmd "github.com/convergent-systems-co/aiConstitution/src/cmd/ai/cmd"
	"github.com/convergent-systems-co/aiConstitution/src/internal/paths"
)

// Sandbox holds the isolated environment for one integration test.
type Sandbox struct {
	Home      string // temp dir wired as $HOME
	AIRoot    string // temp dir wired as $AI_ROOT and paths override
	ConfigDir string // temp dir wired as $AICONST_CONFIG_DIR
	ClaudeDir string // temp dir wired as $CLAUDE_CONFIG_DIR
	Remote    string // path to a local bare git repo (AI_SYNC_REMOTE)

	// Fake HTTP servers — each closed on t.Cleanup.
	AtomServer   *httptest.Server // serves ai-atoms catalog JSON
	SkillServer  *httptest.Server // serves GitHub Contents API for skills
	BrandServer  *httptest.Server // serves brand-atoms directory listing
	PluginServer *httptest.Server // serves plugin tar.gz archives
}

// sandbox wires every external boundary to isolated temp resources and
// returns the Sandbox. All modifications are undone via t.Cleanup, so the
// test does not need to clean up manually.
func sandbox(t *testing.T) *Sandbox {
	t.Helper()

	tmp := t.TempDir() // auto-removed by t.Cleanup

	s := &Sandbox{
		Home:      filepath.Join(tmp, "home"),
		AIRoot:    filepath.Join(tmp, "ai"),
		ConfigDir: filepath.Join(tmp, "config"),
		ClaudeDir: filepath.Join(tmp, "claude"),
		Remote:    filepath.Join(tmp, "remote.git"),
	}

	// Create directory tree.
	for _, d := range []string{
		s.Home,
		s.AIRoot,
		s.ConfigDir,
		s.ClaudeDir,
		filepath.Join(s.ClaudeDir, "skills"),
		filepath.Join(s.Home, "copilot"),
		filepath.Join(s.AIRoot, "bin"),
	} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatalf("sandbox: mkdir %s: %v", d, err)
		}
	}

	// Initialize a bare git repo as the sync remote.
	if out, err := exec.Command("git", "init", "--bare", s.Remote).CombinedOutput(); err != nil {
		t.Fatalf("sandbox: git init --bare: %v\n%s", err, out)
	}

	// Start fake HTTP servers.
	s.AtomServer = sandboxAtomServer(t)
	s.SkillServer = sandboxSkillServer(t)
	s.BrandServer = sandboxBrandServer(t)
	s.PluginServer = sandboxPluginServer(t)

	// --- paths package overrides ---
	paths.SetOverrides(s.AIRoot, s.ConfigDir)
	t.Cleanup(func() { paths.SetOverrides("", "") })

	// --- Package-level URL overrides ---
	// Save originals and restore on cleanup so packages aren't poisoned.
	origAtom := *cmd.AiAtomsCatalogURLForTest
	*cmd.AiAtomsCatalogURLForTest = s.AtomServer.URL + "/catalog.json"
	t.Cleanup(func() { *cmd.AiAtomsCatalogURLForTest = origAtom })

	origSkill := *cmd.SkillAtomsBaseURLForTest
	*cmd.SkillAtomsBaseURLForTest = s.SkillServer.URL
	t.Cleanup(func() { *cmd.SkillAtomsBaseURLForTest = origSkill })

	origPlugin := *cmd.PluginAtomsBaseURLForTest
	*cmd.PluginAtomsBaseURLForTest = s.PluginServer.URL
	t.Cleanup(func() { *cmd.PluginAtomsBaseURLForTest = origPlugin })

	// Brand uses an env var seam; t.Setenv restores it automatically.
	t.Setenv("AICONST_BRAND_REGISTRY_URL", s.BrandServer.URL)

	// --- Environment variables (t.Setenv restores all on cleanup) ---
	t.Setenv("HOME", s.Home)
	t.Setenv("AI_ROOT", s.AIRoot)
	t.Setenv("AICONST_CONFIG_DIR", s.ConfigDir)
	t.Setenv("CLAUDE_CONFIG_DIR", s.ClaudeDir)
	t.Setenv("CLAUDE_SKILLS_DIR", filepath.Join(s.ClaudeDir, "skills"))
	t.Setenv("COPILOT_SKILLS_DIR", filepath.Join(s.Home, "copilot", "skills"))
	t.Setenv("PATH", filepath.Join(s.AIRoot, "bin")+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("EDITOR", "true") // no-op editor for any command that opens $EDITOR
	t.Setenv("GH_TOKEN", "sandbox-fake-not-real")
	t.Setenv("GITHUB_TOKEN", "sandbox-fake-not-real")
	t.Setenv("AI_SYNC_REMOTE", s.Remote)

	return s
}

// sandboxAtomServer serves the ai-atoms catalog JSON with one fixture skill
// atom and one fixture hook atom.
func sandboxAtomServer(t *testing.T) *httptest.Server {
	t.Helper()
	body, _ := json.Marshal(map[string]any{
		"atoms": []map[string]any{
			{
				"type": "skill", "id": "example-skill", "version": "1.0.0",
				"name": "Example Skill", "description": "Fixture skill for tests",
				"lifecycle": "stable",
			},
			{
				"type": "hook", "id": "secret-precommit", "version": "1.0.0",
				"name": "Secret Pre-Commit", "description": "Blocks secrets",
				"lifecycle": "stable", "event": "pre-commit", "language": "python",
			},
		},
	})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/catalog.json" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)
	return srv
}

// sandboxSkillServer serves a GitHub Contents API directory listing for
// skills/skill and individual skill JSON files.
//
// Paths served:
//
//	GET /skills/skill                       → []skillAtomDirEntry (directory listing)
//	GET /skills/skill/example-skill.json    → skillAtom JSON
func sandboxSkillServer(t *testing.T) *httptest.Server {
	t.Helper()
	skillJSON, _ := json.Marshal(map[string]any{
		"id": "example-skill", "version": "1.0.0",
		"name": "Example Skill", "description": "Fixture skill",
		"lifecycle":              "stable",
		"system_prompt_fragment": "# Example Skill\nDo example things.",
	})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/skills/skill":
			listing, _ := json.Marshal([]map[string]any{{
				"name":         "example-skill.json",
				"download_url": "http://" + r.Host + "/skills/skill/example-skill.json",
			}})
			_, _ = w.Write(listing)
		case "/skills/skill/example-skill.json":
			_, _ = w.Write(skillJSON)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

// sandboxBrandServer serves a minimal GitHub Contents API directory listing
// for brand-atoms.
func sandboxBrandServer(t *testing.T) *httptest.Server {
	t.Helper()
	listing, _ := json.Marshal([]map[string]any{
		{"name": "default-brand", "type": "dir", "download_url": ""},
	})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(listing)
	}))
	t.Cleanup(srv.Close)
	return srv
}

// sandboxPluginServer serves a valid plugin tar.gz for any request path.
// The tarball contains a minimal manifest.yaml as required by the plugin
// install command.
func sandboxPluginServer(t *testing.T) *httptest.Server {
	t.Helper()
	pluginTar := testTarball(t, map[string]string{
		"manifest.yaml": "name: example-plugin\nversion: 1.0.0\ndescription: Test plugin\n",
		"SKILL.md":      "# Example Plugin\nA fixture plugin for tests.\n",
	})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/gzip")
		_, _ = w.Write(pluginTar)
	}))
	t.Cleanup(srv.Close)
	return srv
}

// testTarball builds a valid .tar.gz in memory from the given files map
// (filename → content) and returns the raw bytes. Used to build fake plugin
// and skill tarballs served by sandbox servers.
func testTarball(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for name, content := range files {
		hdr := &tar.Header{
			Name: name,
			Mode: 0o644,
			Size: int64(len(content)),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatalf("testTarball: write header %s: %v", name, err)
		}
		if _, err := tw.Write([]byte(content)); err != nil {
			t.Fatalf("testTarball: write body %s: %v", name, err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("testTarball: close tar: %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("testTarball: close gzip: %v", err)
	}
	return buf.Bytes()
}

// TestSandboxWiring verifies that sandbox() correctly wires every boundary:
// directories exist, env vars point at sandbox dirs, URL vars point at
// sandbox servers, paths.AIRoot() returns the sandbox root, and the bare
// git remote is initialized.
func TestSandboxWiring(t *testing.T) {
	s := sandbox(t)

	// All sandbox directories must be created.
	for _, d := range []string{s.Home, s.AIRoot, s.ConfigDir, s.ClaudeDir} {
		if _, err := os.Stat(d); err != nil {
			t.Errorf("sandbox dir missing %s: %v", d, err)
		}
	}

	// Env vars must point at sandbox dirs.
	if got, want := os.Getenv("HOME"), s.Home; got != want {
		t.Errorf("HOME = %q, want %q", got, want)
	}
	if got, want := os.Getenv("AI_ROOT"), s.AIRoot; got != want {
		t.Errorf("AI_ROOT = %q, want %q", got, want)
	}
	if got, want := os.Getenv("AICONST_CONFIG_DIR"), s.ConfigDir; got != want {
		t.Errorf("AICONST_CONFIG_DIR = %q, want %q", got, want)
	}
	if got, want := os.Getenv("CLAUDE_CONFIG_DIR"), s.ClaudeDir; got != want {
		t.Errorf("CLAUDE_CONFIG_DIR = %q, want %q", got, want)
	}

	// GH_TOKEN must be the fake value (never a real token in tests).
	if got := os.Getenv("GH_TOKEN"); got != "sandbox-fake-not-real" {
		t.Errorf("GH_TOKEN = %q, want sandbox-fake-not-real", got)
	}

	// paths.AIRoot() must return the sandbox root (SetOverrides takes precedence).
	if got, want := paths.AIRoot(), s.AIRoot; got != want {
		t.Errorf("paths.AIRoot() = %q, want %q", got, want)
	}

	// URL package vars must point at sandbox servers.
	if got, want := *cmd.AiAtomsCatalogURLForTest, s.AtomServer.URL+"/catalog.json"; got != want {
		t.Errorf("AiAtomsCatalogURL = %q, want %q", got, want)
	}
	if got, want := *cmd.SkillAtomsBaseURLForTest, s.SkillServer.URL; got != want {
		t.Errorf("SkillAtomsBaseURL = %q, want %q", got, want)
	}
	if got, want := *cmd.PluginAtomsBaseURLForTest, s.PluginServer.URL; got != want {
		t.Errorf("PluginAtomsBaseURL = %q, want %q", got, want)
	}

	// AtomServer must serve a valid JSON catalog at /catalog.json.
	resp, err := http.Get(s.AtomServer.URL + "/catalog.json") //nolint:noctx
	if err != nil {
		t.Fatalf("AtomServer GET /catalog.json: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("AtomServer status = %d, want 200", resp.StatusCode)
	}

	// SkillServer must serve a directory listing at the skills path.
	resp2, err := http.Get(s.SkillServer.URL + "/skills/skill") //nolint:noctx
	if err != nil {
		t.Fatalf("SkillServer GET /skills/skill: %v", err)
	}
	resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		t.Errorf("SkillServer status = %d, want 200", resp2.StatusCode)
	}

	// PluginServer must serve a non-empty response (the tarball).
	resp3, err := http.Get(s.PluginServer.URL + "/example-plugin/latest/plugin.tar.gz") //nolint:noctx
	if err != nil {
		t.Fatalf("PluginServer GET: %v", err)
	}
	resp3.Body.Close()
	if resp3.StatusCode != http.StatusOK {
		t.Errorf("PluginServer status = %d, want 200", resp3.StatusCode)
	}

	// Bare git remote must be initialized (HEAD file is the marker).
	if _, err := os.Stat(filepath.Join(s.Remote, "HEAD")); err != nil {
		t.Errorf("bare remote HEAD missing: %v", err)
	}

	// AICONST_BRAND_REGISTRY_URL must point at the brand server.
	if got, want := os.Getenv("AICONST_BRAND_REGISTRY_URL"), s.BrandServer.URL; got != want {
		t.Errorf("AICONST_BRAND_REGISTRY_URL = %q, want %q", got, want)
	}
}
