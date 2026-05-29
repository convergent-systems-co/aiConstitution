# Full-Stack Test Infrastructure — Plan 1 of 6: §9 Seams + §2 Sandbox Harness

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Close the four code seams the test spec requires (§9) and build the `sandbox()` test helper that every integration test in Plans 2–6 will call.

**Architecture:** Add `PluginAtomsBaseURL` package var + `pluginHTTPGet` function var to `plugins.go` (mirroring the existing `brandHTTPGet` pattern in `brand.go`). Add four export-test seams to `export_test.go`. Create `harness_test.go` that wires all environment boundaries — dirs, env vars, path overrides, URL package vars, httptest servers, fake bare-git remote — into a single `sandbox(t *testing.T) *Sandbox` call. Finish with a smoke test that asserts every wire is connected.

**Tech Stack:** Go 1.26, `net/http/httptest`, `os/exec` (git init --bare), `archive/tar` + `compress/gzip` (in-memory test tarball builder), `paths.SetOverrides`.

---

## Context: existing seams

Before writing any code, understand what already exists:

| Seam | Status | How |
|---|---|---|
| `AI_ROOT` env → `paths.AIRoot()` | ✓ exists | `paths.SetOverrides(aiRoot, "")` + `t.Setenv("AI_ROOT", ...)` |
| `AICONST_CONFIG_DIR` env | ✓ exists | `paths.SetOverrides("", configDir)` + `t.Setenv` |
| `AiAtomsCatalogURL` (ai-atoms) | ✓ exists | `*cmd.AiAtomsCatalogURLForTest = srv.URL+"/catalog.json"` |
| `SkillAtomsBaseURL` (skills) | var exists, **export missing** | add `var SkillAtomsBaseURLForTest = &SkillAtomsBaseURL` |
| `brandRegistryBaseURL()` (brand) | ✓ env var `AICONST_BRAND_REGISTRY_URL` | `t.Setenv(...)` |
| `brandHTTPGet` (brand HTTP) | var exists, **export missing** | add `var BrandHTTPGetForTest = &brandHTTPGet` |
| Plugin URL resolution | **hard-coded**, needs var | add `var PluginAtomsBaseURL` |
| Plugin HTTP fetch (`fetchArchive`) | **hard-coded** `http.Get`, needs seam | add `var pluginHTTPGet` |
| `AICONST_SEEDS` env (setup seeds) | ✓ exists | `t.Setenv("AICONST_SEEDS", ...)` |
| Sync remote | ✓ `AI_SYNC_REMOTE` env | `t.Setenv("AI_SYNC_REMOTE", localBareRepo)` |

---

## Files

**Modify:**
- `src/cmd/ai/cmd/plugins.go` — add `PluginAtomsBaseURL` var + `pluginHTTPGet` seam
- `src/cmd/ai/cmd/export_test.go` — add 4 new export pointers

**Create:**
- `src/cmd/ai/cmd/harness_test.go` — `Sandbox` struct + `sandbox()` + `testTarball()` + smoke test

---

## Task 1: Add `PluginAtomsBaseURL` and `pluginHTTPGet` seams to plugins.go

`plugins.go` hard-codes `"https://plugin-atoms.com"` in `resolvePluginAtomURL()` and calls `http.Get()` directly in `fetchArchive()`. This task makes both overridable, mirroring the `brandHTTPGet` pattern already present in `brand.go`.

**Files:**
- Modify: `src/cmd/ai/cmd/plugins.go`

- [ ] **Step 1.1: Read the existing function signatures to confirm insertion points**

  ```bash
  grep -n 'func resolvePluginAtomURL\|func fetchArchive\|http\.Get\|var Plugin' \
    src/cmd/ai/cmd/plugins.go
  ```

  Expected output includes:
  - `func resolvePluginAtomURL(source string) string` (≈ line 399)
  - `resp, err := http.Get(source)` inside `fetchArchive` (≈ line 435)

- [ ] **Step 1.2: Add the two package vars near the top of plugins.go, after the `import` block**

  Find the first `var` or `const` block after the imports (around line 15–25 in `plugins.go`). Insert:

  ```go
  // PluginAtomsBaseURL is the base URL for plugin-atoms.com resolution.
  // Tests override this via PluginAtomsBaseURLForTest (see export_test.go).
  var PluginAtomsBaseURL = "https://plugin-atoms.com"

  // pluginHTTPGet is the package-level HTTP GET seam for plugin archive
  // downloads. Tests replace it with a fake. Mirrors brandHTTPGet in brand.go.
  var pluginHTTPGet = func(url string) (*http.Response, error) {
  	return http.Get(url) //nolint:noctx // simple CLI fetch
  }
  ```

- [ ] **Step 1.3: Update `resolvePluginAtomURL()` to use `PluginAtomsBaseURL`**

  In `resolvePluginAtomURL()`, find:
  ```go
  return fmt.Sprintf("https://plugin-atoms.com/%s/%s/plugin.tar.gz", name, version)
  ```
  Replace with:
  ```go
  return fmt.Sprintf("%s/%s/%s/plugin.tar.gz", PluginAtomsBaseURL, name, version)
  ```

- [ ] **Step 1.4: Update `fetchArchive()` to use `pluginHTTPGet`**

  In `fetchArchive()`, find:
  ```go
  resp, err := http.Get(source) //nolint:noctx // simple CLI fetch
  ```
  Replace with:
  ```go
  resp, err := pluginHTTPGet(source)
  ```

- [ ] **Step 1.5: Verify build**

  ```bash
  go build ./src/cmd/ai/...
  ```
  Expected: no output (clean build). If you see `http.Get` import-unused errors, check that the import block in plugins.go still imports `"net/http"` via `pluginHTTPGet`.

- [ ] **Step 1.6: Run existing plugin tests**

  ```bash
  go test -run TestPlugin -v ./src/cmd/ai/cmd/
  ```
  Expected: all pre-existing plugin tests pass. Zero new failures.

- [ ] **Step 1.7: Commit**

  ```bash
  git add src/cmd/ai/cmd/plugins.go
  git commit -m "feat(test): add PluginAtomsBaseURL + pluginHTTPGet seam to plugins.go

  Mirrors the brandHTTPGet pattern already in brand.go.
  Both vars are overridden in tests via export_test.go pointers (next commit).

  Co-Authored-By: Claude Sonnet 4.6 (1M context) <noreply@anthropic.com>"
  ```

---

## Task 2: Add export seams to export_test.go

Three package vars now need test-accessible pointers, and `brandHTTPGet` needs to be exported.

**Files:**
- Modify: `src/cmd/ai/cmd/export_test.go`

- [ ] **Step 2.1: Read the current export_test.go to see what's already exported**

  ```bash
  cat src/cmd/ai/cmd/export_test.go
  ```
  Confirm the file is `package cmd` (not `package cmd_test`). You should see `AiAtomsCatalogURLForTest`, `InstallHookFromCatalogForTest`, and others.

- [ ] **Step 2.2: Append four new export seams at the end of the file**

  ```go
  // SkillAtomsBaseURLForTest allows tests to redirect skill-atom GitHub
  // Contents API calls to an httptest server.
  var SkillAtomsBaseURLForTest = &SkillAtomsBaseURL

  // PluginAtomsBaseURLForTest allows tests to redirect plugin-atoms.com
  // resolution to an httptest server.
  var PluginAtomsBaseURLForTest = &PluginAtomsBaseURL

  // BrandHTTPGetForTest allows tests to replace the brand HTTP GET seam.
  var BrandHTTPGetForTest = &brandHTTPGet

  // PluginHTTPGetForTest allows tests to replace the plugin archive HTTP
  // GET seam.
  var PluginHTTPGetForTest = &pluginHTTPGet
  ```

- [ ] **Step 2.3: Verify the seams compile**

  ```bash
  go test -run='^$' -count=1 ./src/cmd/ai/cmd/
  ```
  Expected: compiles cleanly, 0 tests run, exit 0.

- [ ] **Step 2.4: Commit**

  ```bash
  git add src/cmd/ai/cmd/export_test.go
  git commit -m "feat(test): export SkillAtomsBaseURL, PluginAtomsBaseURL, brandHTTPGet, pluginHTTPGet seams

  Co-Authored-By: Claude Sonnet 4.6 (1M context) <noreply@anthropic.com>"
  ```

---

## Task 3: Implement the sandbox harness

The core deliverable of this plan. One file, one exported function, one struct. Tests that use `sandbox(t)` MUST NOT call `t.Parallel()` because `sandbox` modifies package-level vars.

**Files:**
- Create: `src/cmd/ai/cmd/harness_test.go`

- [ ] **Step 3.1: Create the file with the Sandbox struct and sandbox() function**

  ```go
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
  	// Save originals and restore on cleanup so parallel test suites aren't poisoned.
  	origAtom := *cmd.AiAtomsCatalogURLForTest
  	*cmd.AiAtomsCatalogURLForTest = s.AtomServer.URL + "/catalog.json"
  	t.Cleanup(func() { *cmd.AiAtomsCatalogURLForTest = origAtom })

  	origSkill := *cmd.SkillAtomsBaseURLForTest
  	*cmd.SkillAtomsBaseURLForTest = s.SkillServer.URL
  	t.Cleanup(func() { *cmd.SkillAtomsBaseURLForTest = origSkill })

  	origPlugin := *cmd.PluginAtomsBaseURLForTest
  	*cmd.PluginAtomsBaseURLForTest = s.PluginServer.URL
  	t.Cleanup(func() { *cmd.PluginAtomsBaseURLForTest = origPlugin })

  	// Brand uses an env var seam (AICONST_BRAND_REGISTRY_URL); t.Setenv restores it.
  	t.Setenv("AICONST_BRAND_REGISTRY_URL", s.BrandServer.URL)

  	// --- Environment variables (t.Setenv restores all on cleanup) ---
  	t.Setenv("HOME", s.Home)
  	t.Setenv("AI_ROOT", s.AIRoot)
  	t.Setenv("AICONST_CONFIG_DIR", s.ConfigDir)
  	t.Setenv("CLAUDE_CONFIG_DIR", s.ClaudeDir)
  	t.Setenv("CLAUDE_SKILLS_DIR", filepath.Join(s.ClaudeDir, "skills"))
  	t.Setenv("COPILOT_INSTRUCTIONS_DIR", filepath.Join(s.Home, "copilot"))
  	t.Setenv("PATH", filepath.Join(s.AIRoot, "bin")+string(os.PathListSeparator)+os.Getenv("PATH"))
  	t.Setenv("EDITOR", "true") // no-op editor for any command that opens $EDITOR
  	t.Setenv("GH_TOKEN", "sandbox-fake-not-real")
  	t.Setenv("GITHUB_TOKEN", "sandbox-fake-not-real")
  	t.Setenv("AI_SYNC_REMOTE", s.Remote)

  	return s
  }
  ```

- [ ] **Step 3.2: Add the four fake-server constructors in the same file**

  ```go
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
  // The paths package expects: GET /skills/skill → []skillAtomDirEntry
  //                            GET /skills/skill/<slug>.json → skillAtom JSON
  func sandboxSkillServer(t *testing.T) *httptest.Server {
  	t.Helper()
  	skillJSON, _ := json.Marshal(map[string]any{
  		"id": "example-skill", "version": "1.0.0",
  		"name": "Example Skill", "description": "Fixture skill",
  		"lifecycle": "stable",
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

  // sandboxBrandServer serves a GitHub Contents API directory listing for
  // brand-atoms with one fixture brand entry.
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

  // sandboxPluginServer serves plugin tar.gz archives for any path.
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
  // (filename → content). Returns the raw bytes. Used to build fake plugin
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
  ```

- [ ] **Step 3.3: Verify it compiles**

  ```bash
  go test -run='^$' -count=1 ./src/cmd/ai/cmd/
  ```
  Expected: exit 0, no compilation errors, 0 tests run.

  Common errors and fixes:
  - `undefined: cmd.SkillAtomsBaseURLForTest` → Task 2 was not committed yet
  - `cannot use http.HandlerFunc...` → import `"net/http"` is missing
  - `undefined: paths.SetOverrides` → check import path is `github.com/convergent-systems-co/aiConstitution/src/internal/paths`

---

## Task 4: Write the sandbox smoke test and verify

A single test that asserts every wire the sandbox sets is actually connected. Runs in under 1 second.

**Files:**
- Modify: `src/cmd/ai/cmd/harness_test.go` (append at end of file)

- [ ] **Step 4.1: Append `TestSandboxWiring` to harness_test.go**

  ```go
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

  	// GH_TOKEN must be the fake value (never a real token).
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

  	// AtomServer must serve a valid JSON catalog.
  	resp, err := http.Get(s.AtomServer.URL + "/catalog.json")
  	if err != nil {
  		t.Fatalf("AtomServer GET /catalog.json: %v", err)
  	}
  	resp.Body.Close()
  	if resp.StatusCode != http.StatusOK {
  		t.Errorf("AtomServer status = %d, want 200", resp.StatusCode)
  	}

  	// SkillServer must serve a directory listing.
  	resp2, err := http.Get(s.SkillServer.URL + "/skills/skill")
  	if err != nil {
  		t.Fatalf("SkillServer GET /skills/skill: %v", err)
  	}
  	resp2.Body.Close()
  	if resp2.StatusCode != http.StatusOK {
  		t.Errorf("SkillServer status = %d, want 200", resp2.StatusCode)
  	}

  	// PluginServer must serve a non-empty body (tarball bytes).
  	resp3, err := http.Get(s.PluginServer.URL + "/example-plugin/latest/plugin.tar.gz")
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
  ```

- [ ] **Step 4.2: Run the smoke test**

  ```bash
  go test -run TestSandboxWiring -v ./src/cmd/ai/cmd/
  ```

  Expected output:
  ```
  === RUN   TestSandboxWiring
  --- PASS: TestSandboxWiring (0.00s)
  PASS
  ok      github.com/convergent-systems-co/aiConstitution/src/cmd/ai/cmd  X.XXXs
  ```

  If it fails, common causes:
  - `cmd.SkillAtomsBaseURLForTest undefined` → export_test.go seam missing from Task 2
  - `paths.AIRoot() = "" want <sandbox>` → `paths.SetOverrides` called after `paths.AIRoot()` in test; check order in sandbox()
  - `AtomServer status = 404` → check the path in sandboxAtomServer matches `/catalog.json`
  - `git init --bare` fails on CI → git not installed; add `exec.LookPath("git")` guard and t.Skip

- [ ] **Step 4.3: Run the full test suite to confirm no regressions**

  ```bash
  make test
  ```

  Expected: all packages pass, 0 failures. The smoke test adds ≤1s to the suite.

- [ ] **Step 4.4: Commit**

  ```bash
  git add src/cmd/ai/cmd/harness_test.go
  git commit -m "feat(test): add sandbox() harness, fake servers, testTarball(), and smoke test

  Implements §2 of the full-stack test spec. Every integration test in
  Plans 2–6 calls sandbox(t) as its first line.

  Co-Authored-By: Claude Sonnet 4.6 (1M context) <noreply@anthropic.com>"
  ```

---

## Self-review

**Spec §9 coverage:**

| Gap | Covered? | Task |
|---|---|---|
| §9.1 PluginAtomsBaseURL var | ✓ | Task 1 |
| §9.2 Centralized HTTP client | ⚠️ Partial | `pluginHTTPGet` + `brandHTTPGet` are per-package seams; a single global transport is deferred to Plan 6 (the §6 network guard needs it) |
| §9.3 Non-interactive setup + AICONST_SEEDS | ✓ existing | `--non-interactive` flag + `AICONST_SEEDS` env already present in `setup.go`; sandbox wires `AICONST_SEEDS` via `t.Setenv`. Plan 2 writes the T1 test that exercises this. |
| §9.4 `ai hooks install` honors AI_ROOT | ✓ existing | `hooks.go` already reads `AI_ROOT` env; sandbox wires it. Plan 2 verifies. |
| §9.5 Stable status output | ✓ existing | Status output is deterministic (fixed section order, no random elements); dynamic content (git hash, dates) is stripped in the golden-snapshot helper in Plan 5/6. |

**§2 sandbox coverage:**

All 14 boundaries from the spec table are wired:
- `AI_ROOT`, `AICONST_CONFIG_DIR`, `CLAUDE_CONFIG_DIR`, `CLAUDE_SKILLS_DIR`, `COPILOT_INSTRUCTIONS_DIR`, `HOME` → env vars ✓
- `PATH` → prepend `AIRoot/bin` ✓
- `EDITOR` → `"true"` no-op ✓
- Skill atoms, ai-atoms catalog → httptest servers ✓
- Brand atoms → env var `AICONST_BRAND_REGISTRY_URL` + httptest ✓
- Plugin atoms → `PluginAtomsBaseURL` package var + httptest ✓
- Sync remote → local bare git repo ✓
- `GITHUB_TOKEN` / `GH_TOKEN` → fake value ✓
- `AICONST_SEEDS` → callers set this per-test via `t.Setenv` (not in sandbox() since it's test-specific) ✓
- `AICONST_REVIEW_CADENCE_DAYS` → callers set per-test ✓

**One known gap:** The `brandHTTPGet` seam is exported but the sandbox doesn't replace it — it relies on the env var `AICONST_BRAND_REGISTRY_URL` which `brandRegistryBaseURL()` already reads. Plan 3 will verify this is sufficient for the brand integration tests.

**Placeholder scan:** None found. All steps have exact code, commands, and expected output.

**Type consistency:** `Sandbox` struct fields match across the constructor (`sandbox()`) and the smoke test. `testTarball` signature is `(t *testing.T, files map[string]string) []byte` — consistent between the definition in Task 3 and usage in `sandboxPluginServer`.

---

## What's next

After this plan is merged, write and implement:

- **Plan 2** (`2026-05-28-fullstack-test-plan-2-setup-hooks.md`): T1/T2 tests for `ai setup`, `ai hooks install`, `ai hooks install --claude`, command-wrapper extraction, hook behavioral tests (§4.1–4.3).
- **Plan 3** (`2026-05-28-fullstack-test-plan-3-skills-plugins.md`): T1/T2 tests for `ai skills available/install/validate`, `ai plugins install/enable/disable/status`, cache-hit verification, 404 negative paths (§4.4–4.5).
- **Plan 4** (`2026-05-28-fullstack-test-plan-4-atoms-sync.md`): Brand atoms, persona/profile/mode, sync push/pull, restore into second sandbox, backup (§4.6–4.7).
- **Plan 5** (`2026-05-28-fullstack-test-plan-5-doctor-audit.md`): Doctor repair loop, status golden snapshot, audit rotate, update/migrate (§4.8–4.9).
- **Plan 6** (`2026-05-28-fullstack-test-plan-6-e2e-ci.md`): T3 binary journey test, T4 hook self-checks, §6 safety guards (no-real-home, no-real-network), CI job additions (§5–7).
