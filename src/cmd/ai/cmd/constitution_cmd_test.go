package cmd

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func makeTestTarGz(t *testing.T, dir, content string) string {
	t.Helper()
	archivePath := filepath.Join(dir, "20260524T120000Z.tar.gz")
	f, err := os.Create(archivePath)
	if err != nil { t.Fatal(err) }
	defer func() { _ = f.Close() }()
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)
	body := []byte(content)
	_ = tw.WriteHeader(&tar.Header{Name: "Constitution.md", Mode: 0o600, Size: int64(len(body))})
	_, _ = tw.Write(body)
	_ = tw.Close()
	_ = gz.Close()
	return archivePath
}

func TestConstitutionBackup_CreatesArchive(t *testing.T) {
	aiRoot := t.TempDir()
	home := t.TempDir()
	t.Setenv("AI_ROOT", aiRoot)
	t.Setenv("HOME", home)
	_ = os.WriteFile(filepath.Join(aiRoot, "Constitution.md"), []byte("# AI Constitution\n"), 0o600)

	root := NewRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetArgs([]string{"constitution", "backup"})
	if err := root.Execute(); err != nil {
		t.Fatalf("constitution backup: %v\n%s", err, buf.String())
	}
	aiBackups := filepath.Join(home, ".ai-backups")
	entries, err := os.ReadDir(aiBackups)
	if err != nil || len(entries) == 0 {
		t.Fatalf("expected archive in %s: %v", aiBackups, err)
	}
	var found bool
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".tar.gz") { found = true }
	}
	if !found { t.Errorf("no .tar.gz archive in %s", aiBackups) }
}

func TestConstitutionBackup_ClearLinks_RemovesIncludes(t *testing.T) {
	aiRoot := t.TempDir()
	home := t.TempDir()
	t.Setenv("AI_ROOT", aiRoot)
	t.Setenv("HOME", home)
	_ = os.WriteFile(filepath.Join(aiRoot, "Constitution.md"), []byte("# X\n"), 0o600)
	claudeDir := filepath.Join(home, ".claude")
	_ = os.MkdirAll(claudeDir, 0o750)
	claudeMD := filepath.Join(claudeDir, "CLAUDE.md")
	_ = os.WriteFile(claudeMD, []byte("@~/.ai/Constitution.md\nother line\n"), 0o600)

	root := NewRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetArgs([]string{"constitution", "backup", "--clear-links"})
	if err := root.Execute(); err != nil {
		t.Fatalf("backup --clear-links: %v\n%s", err, buf.String())
	}
	data, _ := os.ReadFile(claudeMD)
	if strings.Contains(string(data), "@~/.ai/") {
		t.Errorf("@-includes not removed:\n%s", string(data))
	}
}

func TestConstitutionRestore_ExtractsAndRewires(t *testing.T) {
	aiRoot := t.TempDir()
	home := t.TempDir()
	t.Setenv("AI_ROOT", aiRoot)
	t.Setenv("HOME", home)
	aiBackups := filepath.Join(home, ".ai-backups")
	_ = os.MkdirAll(aiBackups, 0o750)
	makeTestTarGz(t, aiBackups, "# AI Constitution — Original\n\n## §1 Governance\n\nOriginal content.\n")
	_ = os.MkdirAll(filepath.Join(aiRoot, "hooks"), 0o750)
	_ = os.MkdirAll(filepath.Join(home, ".claude"), 0o750)

	root := NewRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetArgs([]string{"constitution", "restore"})
	if err := root.Execute(); err != nil {
		t.Fatalf("constitution restore: %v\n%s", err, buf.String())
	}
	data, err := os.ReadFile(filepath.Join(aiRoot, "Constitution.md"))
	if err != nil { t.Fatalf("Constitution.md not restored: %v", err) }
	if !strings.Contains(string(data), "Original content.") {
		t.Errorf("wrong Constitution.md content:\n%s", string(data))
	}
	claudeData, _ := os.ReadFile(filepath.Join(home, ".claude", "CLAUDE.md"))
	if !strings.Contains(string(claudeData), "@~/.ai/Constitution.md") {
		t.Errorf("CLAUDE.md missing @-include:\n%s", string(claudeData))
	}
}

// ─── Issue #406: ai constitution setup ───────────────────────────────────────

func TestConstitutionSetup_CommandExists(t *testing.T) {
	root := NewRootCmd()
	constitutionCmd, _, err := root.Find([]string{"constitution"})
	if err != nil {
		t.Fatalf("constitution command not found: %v", err)
	}
	var setupCmd interface{ Use() string }
	_ = setupCmd
	var found bool
	for _, sub := range constitutionCmd.Commands() {
		if strings.HasPrefix(sub.Use, "setup") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("'constitution setup' subcommand not registered on constitution command")
	}
}

// ─── Issue #407: ai constitution restore --url ────────────────────────────────

// makeTestGitRepo creates a temporary git repository at dir with the given
// files written into it. Returns a file:// URL suitable for git clone.
func makeTestGitRepo(t *testing.T, files map[string]string) string {
	t.Helper()
	repoDir := t.TempDir()
	mustRun := func(args ...string) {
		t.Helper()
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = repoDir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git command %v failed: %v\n%s", args, err, out)
		}
	}
	mustRun("git", "init")
	mustRun("git", "config", "user.email", "test@test.com")
	mustRun("git", "config", "user.name", "Test")
	for name, content := range files {
		p := filepath.Join(repoDir, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		mustRun("git", "add", name)
	}
	mustRun("git", "commit", "-m", "init")
	return "file://" + repoDir
}

func TestConstitutionRestoreURL_DryRun(t *testing.T) {
	aiRoot := t.TempDir()
	t.Setenv("AI_ROOT", aiRoot)
	// Write a sentinel file so we can confirm it is NOT overwritten.
	sentinel := filepath.Join(aiRoot, "Constitution.md")
	_ = os.WriteFile(sentinel, []byte("sentinel\n"), 0o600)

	repoURL := makeTestGitRepo(t, map[string]string{
		"Constitution.md": "# From Git\n",
		"Common.md":       "# Common\n",
	})

	root := NewRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"constitution", "restore", "--url", repoURL, "--dry-run"})
	if err := root.Execute(); err != nil {
		t.Fatalf("constitution restore --url --dry-run: %v\n%s", err, buf.String())
	}

	out := buf.String()
	if !strings.Contains(out, "would copy") {
		t.Errorf("expected 'would copy' in dry-run output, got:\n%s", out)
	}

	// Sentinel must be unchanged — dry-run must not write.
	data, _ := os.ReadFile(sentinel)
	if string(data) != "sentinel\n" {
		t.Errorf("dry-run must not modify existing files; got: %s", data)
	}
}

func TestConstitutionRestoreURL_Restores(t *testing.T) {
	aiRoot := t.TempDir()
	t.Setenv("AI_ROOT", aiRoot)

	repoURL := makeTestGitRepo(t, map[string]string{
		"Constitution.md": "# Restored Constitution\n",
		"GOALS.md":        "# Goals\n",
	})

	root := NewRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"constitution", "restore", "--url", repoURL})
	if err := root.Execute(); err != nil {
		t.Fatalf("constitution restore --url: %v\n%s", err, buf.String())
	}

	data, err := os.ReadFile(filepath.Join(aiRoot, "Constitution.md"))
	if err != nil {
		t.Fatalf("Constitution.md not written after --url restore: %v", err)
	}
	if !strings.Contains(string(data), "Restored Constitution") {
		t.Errorf("unexpected Constitution.md content:\n%s", data)
	}

	// GOALS.md must also be present.
	if _, err := os.Stat(filepath.Join(aiRoot, "GOALS.md")); err != nil {
		t.Errorf("GOALS.md not restored: %v", err)
	}
}
