package cmd

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"os"
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
