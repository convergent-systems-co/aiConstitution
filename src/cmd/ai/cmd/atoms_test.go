package cmd_test

// atoms_test.go — TDD suite for `ai atoms fetch/fork/publish/list` (#261–#265).
//
// All tests:
//   - use real temp directories (no mocks)
//   - redirect AI_ROOT and AICONST_CONFIG_DIR env vars so production dirs
//     are never touched
//   - build tar.gz fixtures in-process (no external fixtures needed)
//   - verify behavior end-to-end through the cobra command tree

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/convergent-systems-co/aiConstitution/src/cmd/ai/cmd"
)

// ---- helpers ----------------------------------------------------------------

// buildAtomTarGz creates an in-memory tar.gz containing a single atom.toml
// (and optionally extra files). Returns the raw bytes.
func buildAtomTarGz(t *testing.T, name, version, sha256hex, upstreamURL, upstreamRef string, extra map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)

	// Build atom.toml content
	tomlLines := []string{
		fmt.Sprintf(`name = %q`, name),
		fmt.Sprintf(`version = %q`, version),
		fmt.Sprintf(`sha256 = %q`, sha256hex),
		fmt.Sprintf(`upstream_url = %q`, upstreamURL),
	}
	if upstreamRef != "" {
		tomlLines = append(tomlLines, fmt.Sprintf(`upstream_ref = %q`, upstreamRef))
	}
	tomlContent := strings.Join(tomlLines, "\n") + "\n"

	writeEntry := func(relPath, content string) {
		hdr := &tar.Header{
			Name: filepath.Join(name, relPath),
			Mode: 0644,
			Size: int64(len(content)),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatalf("tar write header: %v", err)
		}
		if _, err := tw.Write([]byte(content)); err != nil {
			t.Fatalf("tar write content: %v", err)
		}
	}

	writeEntry("atom.toml", tomlContent)
	for rel, content := range extra {
		writeEntry(rel, content)
	}

	if err := tw.Close(); err != nil {
		t.Fatalf("tar close: %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("gzip close: %v", err)
	}
	return buf.Bytes()
}

// sha256OfBytes returns the hex-encoded SHA256 of b.
func sha256OfBytes(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

// serveFile starts an httptest.Server that serves b as a single endpoint GET /atom.tar.gz.
func serveFile(t *testing.T, b []byte) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write(b)
	}))
	t.Cleanup(srv.Close)
	return srv
}

// atomsEnv sets AI_ROOT and AICONST_CONFIG_DIR to temp dirs and returns a
// cleanup function. Call it at the start of every atoms test.
func atomsEnv(t *testing.T) (aiRoot, configDir string) {
	t.Helper()
	aiRoot = t.TempDir()
	configDir = t.TempDir()
	t.Setenv("AI_ROOT", aiRoot)
	t.Setenv("AICONST_CONFIG_DIR", configDir)
	return
}

// runCmd builds a fresh root command, injects args, captures stdout/stderr,
// and returns combined output plus error.
func runCmd(t *testing.T, args ...string) (string, error) {
	t.Helper()
	root := cmd.NewRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs(args)
	err := root.Execute()
	return out.String(), err
}

// ---- #261: fetch extracts tar.gz to ~/.ai/atoms/<name>/ -------------------

func TestAtomsFetchExtractsFiles(t *testing.T) {
	aiRoot, _ := atomsEnv(t)

	atomBytes := buildAtomTarGz(t,
		"convergent-systems-core", "1.0.0",
		"", // sha256 empty — hash verification tested separately
		"https://example.com/atom.tar.gz", "",
		map[string]string{"README.md": "# test atom\n"},
	)
	srv := serveFile(t, atomBytes)

	_, err := runCmd(t, "atoms", "fetch", srv.URL+"/atom.tar.gz")
	if err != nil {
		t.Fatalf("fetch failed: %v", err)
	}

	// atom.toml must exist at ~/.ai/atoms/convergent-systems-core/atom.toml
	atomTOML := filepath.Join(aiRoot, "atoms", "convergent-systems-core", "atom.toml")
	if _, statErr := os.Stat(atomTOML); statErr != nil {
		t.Fatalf("atom.toml not extracted to expected path %q: %v", atomTOML, statErr)
	}

	// README.md must also have been extracted
	readme := filepath.Join(aiRoot, "atoms", "convergent-systems-core", "README.md")
	if _, statErr := os.Stat(readme); statErr != nil {
		t.Fatalf("README.md not extracted to expected path %q: %v", readme, statErr)
	}
}

// ---- #262: fetch reads atom.toml name/version after extraction -------------

func TestAtomsFetchReadsManifest(t *testing.T) {
	aiRoot, _ := atomsEnv(t)

	atomBytes := buildAtomTarGz(t,
		"my-test-atom", "2.3.1",
		"", "https://example.com/atom.tar.gz", "",
		nil,
	)
	srv := serveFile(t, atomBytes)

	out, err := runCmd(t, "atoms", "fetch", srv.URL+"/atom.tar.gz")
	if err != nil {
		t.Fatalf("fetch failed: %v", err)
	}

	// Output must mention the name and version
	if !strings.Contains(out, "my-test-atom") {
		t.Errorf("output missing atom name; got: %q", out)
	}
	if !strings.Contains(out, "2.3.1") {
		t.Errorf("output missing atom version; got: %q", out)
	}

	// The extracted dir must be named after the atom (from atom.toml), not from the URL
	atomDir := filepath.Join(aiRoot, "atoms", "my-test-atom")
	if _, statErr := os.Stat(atomDir); statErr != nil {
		t.Fatalf("expected atom dir %q not found: %v", atomDir, statErr)
	}
}

// ---- #262: fetch rejects hash mismatch ------------------------------------

func TestAtomsFetchRejectsHashMismatch(t *testing.T) {
	atomsEnv(t)

	// Build a tar.gz, then put a wrong sha256 in the atom.toml inside it.
	atomBytes := buildAtomTarGz(t,
		"bad-hash-atom", "1.0.0",
		"deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
		"https://example.com/atom.tar.gz", "",
		nil,
	)
	srv := serveFile(t, atomBytes)

	_, err := runCmd(t, "atoms", "fetch", srv.URL+"/atom.tar.gz")
	if err == nil {
		t.Fatal("expected error on hash mismatch, got nil")
	}
	if !strings.Contains(err.Error(), "Hash mismatch") && !strings.Contains(err.Error(), "hash mismatch") {
		t.Errorf("error should mention 'Hash mismatch'; got: %v", err)
	}
}

// ---- #262: fetch writes atoms.json index ----------------------------------

func TestAtomsFetchWritesIndex(t *testing.T) {
	_, configDir := atomsEnv(t)

	atomBytes := buildAtomTarGz(t,
		"indexed-atom", "3.0.0",
		"", "https://upstream.example.com/atom.tar.gz", "",
		nil,
	)
	srv := serveFile(t, atomBytes)

	_, err := runCmd(t, "atoms", "fetch", srv.URL+"/atom.tar.gz")
	if err != nil {
		t.Fatalf("fetch failed: %v", err)
	}

	indexPath := filepath.Join(configDir, "atoms.json")
	data, readErr := os.ReadFile(indexPath)
	if readErr != nil {
		t.Fatalf("atoms.json not written at %q: %v", indexPath, readErr)
	}

	// atoms.json must be a JSON array containing an entry with our atom
	var entries []map[string]interface{}
	if jsonErr := json.Unmarshal(data, &entries); jsonErr != nil {
		t.Fatalf("atoms.json is not valid JSON: %v\ncontent: %s", jsonErr, data)
	}
	if len(entries) == 0 {
		t.Fatal("atoms.json is empty after fetch")
	}
	found := false
	for _, e := range entries {
		if e["name"] == "indexed-atom" {
			found = true
			if e["version"] != "3.0.0" {
				t.Errorf("wrong version in index; got %v", e["version"])
			}
		}
	}
	if !found {
		t.Errorf("indexed-atom not found in atoms.json; content: %s", data)
	}
}

// ---- #263: fork copies dir -------------------------------------------------

func TestAtomsForkCopiesDir(t *testing.T) {
	aiRoot, _ := atomsEnv(t)

	// First, manually install an atom into aiRoot so fork has something to work with.
	srcDir := filepath.Join(aiRoot, "atoms", "source-atom")
	if err := os.MkdirAll(srcDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	tomlContent := `name = "source-atom"` + "\n" + `version = "1.0.0"` + "\n" + `sha256 = ""` + "\n" + `upstream_url = ""` + "\n"
	if err := os.WriteFile(filepath.Join(srcDir, "atom.toml"), []byte(tomlContent), 0644); err != nil {
		t.Fatalf("write atom.toml: %v", err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "extra.md"), []byte("extra file\n"), 0644); err != nil {
		t.Fatalf("write extra.md: %v", err)
	}

	_, err := runCmd(t, "atoms", "fork", "source-atom")
	if err != nil {
		t.Fatalf("fork failed: %v", err)
	}

	// Default local name is <name>-local
	dstDir := filepath.Join(aiRoot, "atoms", "source-atom-local")
	if _, statErr := os.Stat(dstDir); statErr != nil {
		t.Fatalf("forked dir %q not created: %v", dstDir, statErr)
	}

	// extra.md must have been copied
	if _, statErr := os.Stat(filepath.Join(dstDir, "extra.md")); statErr != nil {
		t.Fatalf("extra.md not copied to forked dir: %v", statErr)
	}
}

// ---- #263: fork adds upstream_ref to atom.toml ----------------------------

func TestAtomsForkAddsUpstreamRef(t *testing.T) {
	aiRoot, _ := atomsEnv(t)

	srcDir := filepath.Join(aiRoot, "atoms", "core-atom")
	if err := os.MkdirAll(srcDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	tomlContent := `name = "core-atom"` + "\n" + `version = "2.5.0"` + "\n" + `sha256 = ""` + "\n" + `upstream_url = ""` + "\n"
	if err := os.WriteFile(filepath.Join(srcDir, "atom.toml"), []byte(tomlContent), 0644); err != nil {
		t.Fatalf("write atom.toml: %v", err)
	}

	_, err := runCmd(t, "atoms", "fork", "core-atom", "--as", "my-fork")
	if err != nil {
		t.Fatalf("fork failed: %v", err)
	}

	dstTOML := filepath.Join(aiRoot, "atoms", "my-fork", "atom.toml")
	data, readErr := os.ReadFile(dstTOML)
	if readErr != nil {
		t.Fatalf("forked atom.toml not found at %q: %v", dstTOML, readErr)
	}

	// Must contain upstream_ref = "core-atom@2.5.0"
	if !strings.Contains(string(data), "upstream_ref") {
		t.Errorf("forked atom.toml missing upstream_ref; content:\n%s", data)
	}
	if !strings.Contains(string(data), "core-atom@2.5.0") {
		t.Errorf("forked atom.toml upstream_ref does not reference core-atom@2.5.0; content:\n%s", data)
	}
}

// ---- #264: publish --dry-run writes atom.toml with correct fields ---------

func TestAtomsPublishDryRun(t *testing.T) {
	aiRoot, _ := atomsEnv(t)

	// Create a file under aiRoot for publish to hash
	govDir := filepath.Join(aiRoot, "governance")
	if err := os.MkdirAll(govDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(govDir, "Constitution.md"), []byte("# Governance\n"), 0644); err != nil {
		t.Fatalf("write Constitution.md: %v", err)
	}

	out, err := runCmd(t, "atoms", "publish", "--name", "my-constitution", "--version", "0.1.0", "--dry-run")
	if err != nil {
		t.Fatalf("publish --dry-run failed: %v", err)
	}

	// Output must say "Would publish: my-constitution@0.1.0"
	if !strings.Contains(out, "Would publish") {
		t.Errorf("dry-run output missing 'Would publish'; got: %q", out)
	}
	if !strings.Contains(out, "my-constitution") {
		t.Errorf("dry-run output missing atom name; got: %q", out)
	}
	if !strings.Contains(out, "0.1.0") {
		t.Errorf("dry-run output missing version; got: %q", out)
	}

	// atom.toml must have been written in dry-run (preview only, no upload)
	atomTOML := filepath.Join(aiRoot, "atoms", "my-constitution", "atom.toml")
	data, readErr := os.ReadFile(atomTOML)
	if readErr != nil {
		t.Fatalf("atom.toml not written by publish --dry-run at %q: %v", atomTOML, readErr)
	}
	content := string(data)
	if !strings.Contains(content, `name = "my-constitution"`) {
		t.Errorf("atom.toml missing name field; content:\n%s", content)
	}
	if !strings.Contains(content, `version = "0.1.0"`) {
		t.Errorf("atom.toml missing version field; content:\n%s", content)
	}
	if !strings.Contains(content, "sha256") {
		t.Errorf("atom.toml missing sha256 field; content:\n%s", content)
	}
}

// ---- #265: list prints (no atoms installed) when index is absent -----------

func TestAtomsListEmpty(t *testing.T) {
	atomsEnv(t)
	// No atoms.json created — list should gracefully report empty

	out, err := runCmd(t, "atoms", "list")
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}
	if !strings.Contains(out, "no atoms installed") {
		t.Errorf("expected '(no atoms installed)' in output; got: %q", out)
	}
}

// ---- #265: list prints table when atoms are installed ----------------------

func TestAtomsListTable(t *testing.T) {
	_, configDir := atomsEnv(t)

	// Write a pre-populated atoms.json
	entries := []map[string]interface{}{
		{
			"name":      "my-atom",
			"version":   "1.2.3",
			"upstream":  "https://upstream.example.com",
			"path":      "~/.ai/atoms/my-atom",
		},
	}
	data, _ := json.MarshalIndent(entries, "", "  ")
	indexPath := filepath.Join(configDir, "atoms.json")
	if err := os.WriteFile(indexPath, data, 0644); err != nil {
		t.Fatalf("write atoms.json: %v", err)
	}

	out, err := runCmd(t, "atoms", "list")
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}
	if !strings.Contains(out, "my-atom") {
		t.Errorf("list output missing atom name; got: %q", out)
	}
	if !strings.Contains(out, "1.2.3") {
		t.Errorf("list output missing version; got: %q", out)
	}
}

// ---- #261: fetch errors on network failure --------------------------------

func TestAtomsFetchNetworkError(t *testing.T) {
	atomsEnv(t)

	// Point at a server that immediately closes the connection
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		// Hijack and close without writing a response
		w.WriteHeader(http.StatusInternalServerError)
	}))
	srv.Close() // close before fetching

	_, err := runCmd(t, "atoms", "fetch", srv.URL+"/atom.tar.gz")
	if err == nil {
		t.Fatal("expected error on network failure, got nil")
	}
}
