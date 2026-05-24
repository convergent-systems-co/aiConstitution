package cmd

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestBackupCreatesArchiveExcludingGitAndInteractions(t *testing.T) {
	aiRoot := t.TempDir()
	cfg := t.TempDir()
	t.Setenv("AI_ROOT", aiRoot)
	t.Setenv("AICONST_CONFIG_DIR", cfg)

	// Files that MUST end up in the archive.
	mustInclude := map[string]string{
		"Code.md":               "**Version:** 0.7\n",
		"audit/violations/x.md": "# violation\n",
		"memory/MEMORY.md":      "- entry\n",
	}
	// Files that MUST be excluded.
	mustExclude := map[string]string{
		".git/HEAD":                       "ref: refs/heads/main\n",
		"audit/interactions/2026-05.jsonl": "{\"kind\":\"request\"}\n",
	}

	writeAll := func(m map[string]string) {
		for rel, body := range m {
			full := filepath.Join(aiRoot, rel)
			if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
				t.Fatalf("mkdir %s: %v", rel, err)
			}
			if err := os.WriteFile(full, []byte(body), 0o600); err != nil {
				t.Fatalf("write %s: %v", rel, err)
			}
		}
	}
	writeAll(mustInclude)
	writeAll(mustExclude)

	buf := &bytes.Buffer{}
	cmd := newBackupCmd()
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs(nil)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("backup: %v", err)
	}

	// Extract the printed archive path. backup prints it on stdout.
	out := strings.TrimSpace(buf.String())
	pathRe := regexp.MustCompile(`\S+\.tar\.gz`)
	match := pathRe.FindString(out)
	if match == "" {
		t.Fatalf("could not find archive path in output:\n%s", out)
	}
	archive := match

	if _, err := os.Stat(archive); err != nil {
		t.Fatalf("archive not on disk: %v", err)
	}

	got := tarballEntries(t, archive)
	for rel := range mustInclude {
		if !got[rel] {
			t.Errorf("archive missing %q. entries: %v", rel, sortedKeys(got))
		}
	}
	for rel := range mustExclude {
		if got[rel] {
			t.Errorf("archive should not contain %q. entries: %v", rel, sortedKeys(got))
		}
	}
}

func TestBackupCreatesDestDirIfMissing(t *testing.T) {
	aiRoot := t.TempDir()
	cfg := t.TempDir()
	t.Setenv("AI_ROOT", aiRoot)
	t.Setenv("AICONST_CONFIG_DIR", cfg)
	if err := os.WriteFile(filepath.Join(aiRoot, "Code.md"), []byte("x"), 0o600); err != nil {
		t.Fatalf("write Code.md: %v", err)
	}

	buf := &bytes.Buffer{}
	cmd := newBackupCmd()
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs(nil)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("backup: %v", err)
	}

	if _, err := os.Stat(filepath.Join(cfg, "backups")); err != nil {
		t.Errorf("backups dir not created: %v", err)
	}
}

// tarballEntries returns a set of paths inside the .tar.gz at archive.
func tarballEntries(t *testing.T, archive string) map[string]bool {
	t.Helper()
	f, err := os.Open(archive) //nolint:gosec // test fixture path
	if err != nil {
		t.Fatalf("open archive: %v", err)
	}
	defer func() { _ = f.Close() }()
	gz, err := gzip.NewReader(f)
	if err != nil {
		t.Fatalf("gzip reader: %v", err)
	}
	defer func() { _ = gz.Close() }()
	tr := tar.NewReader(gz)
	out := map[string]bool{}
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("tar.Next: %v", err)
		}
		out[hdr.Name] = true
	}
	return out
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
