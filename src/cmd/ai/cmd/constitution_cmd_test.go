package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConstitutionBackup_CreatesBackupDir(t *testing.T) {
	aiRoot := t.TempDir()
	home := t.TempDir()
	t.Setenv("AI_ROOT", aiRoot)

	// Write a minimal Constitution.md
	_ = os.WriteFile(filepath.Join(aiRoot, "Constitution.md"),
		[]byte("# AI Constitution\n\n## §1 Governance\n\nTest content.\n"), 0o600)

	root := NewRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetArgs([]string{"constitution", "backup"})
	_ = os.Setenv("HOME", home)
	if err := root.Execute(); err != nil {
		t.Fatalf("constitution backup: %v\n%s", err, buf.String())
	}

	backupsDir := filepath.Join(aiRoot, "backups")
	entries, err := os.ReadDir(backupsDir)
	if err != nil || len(entries) == 0 {
		t.Fatalf("expected backup directory created under %s", backupsDir)
	}

	backupDir := filepath.Join(backupsDir, entries[0].Name())
	if _, err := os.Stat(filepath.Join(backupDir, "Constitution.md")); err != nil {
		t.Error("Constitution.md not found in backup")
	}
	if _, err := os.Stat(filepath.Join(backupDir, "manifest.json")); err != nil {
		t.Error("manifest.json not found in backup")
	}
}

func TestConstitutionBackup_ClearLinks_RemovesClaudeMDIncludes(t *testing.T) {
	aiRoot := t.TempDir()
	home := t.TempDir()
	t.Setenv("AI_ROOT", aiRoot)
	_ = os.Setenv("HOME", home)

	// Write Constitution.md
	_ = os.WriteFile(filepath.Join(aiRoot, "Constitution.md"),
		[]byte("# AI Constitution\n"), 0o600)

	// Write CLAUDE.md with @-include
	claudeDir := filepath.Join(home, ".claude")
	_ = os.MkdirAll(claudeDir, 0o750)
	claudeMD := filepath.Join(claudeDir, "CLAUDE.md")
	_ = os.WriteFile(claudeMD, []byte("@~/.ai/Constitution.md\n@~/.ai/Constitution.local.md\n"), 0o600)

	root := NewRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetArgs([]string{"constitution", "backup", "--clear-links"})
	if err := root.Execute(); err != nil {
		t.Fatalf("constitution backup --clear-links: %v\n%s", err, buf.String())
	}

	// CLAUDE.md should have @-includes stripped
	data, _ := os.ReadFile(claudeMD)
	if strings.Contains(string(data), "@~/.ai/") {
		t.Errorf("expected @-includes removed from CLAUDE.md, got:\n%s", string(data))
	}
}

func TestConstitutionRestore_RestoresConstitution(t *testing.T) {
	aiRoot := t.TempDir()
	home := t.TempDir()
	t.Setenv("AI_ROOT", aiRoot)
	_ = os.Setenv("HOME", home)

	// Create a backup manually
	backupDir := filepath.Join(aiRoot, "backups", "20260524T120000Z")
	_ = os.MkdirAll(backupDir, 0o750)
	originalContent := "# AI Constitution — Original\n\n## §1 Governance\n\nOriginal content.\n"
	_ = os.WriteFile(filepath.Join(backupDir, "Constitution.md"), []byte(originalContent), 0o600)
	manifest := `{"created_at":"2026-05-24T12:00:00Z","ai_root":"` + aiRoot + `","claude_md_path":"` + filepath.Join(home, ".claude", "CLAUDE.md") + `","hooks_installed":false,"clear_links_applied":true}`
	_ = os.WriteFile(filepath.Join(backupDir, "manifest.json"), []byte(manifest), 0o600)
	_ = os.WriteFile(filepath.Join(backupDir, "CLAUDE.md"), []byte("@~/.ai/Constitution.md\n"), 0o600)

	// Create a hooks dir so ExtractAllHooks has somewhere to write
	_ = os.MkdirAll(filepath.Join(aiRoot, "hooks"), 0o750)

	root := NewRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetArgs([]string{"constitution", "restore"})
	if err := root.Execute(); err != nil {
		t.Fatalf("constitution restore: %v\n%s", err, buf.String())
	}

	// Constitution.md should be restored
	data, err := os.ReadFile(filepath.Join(aiRoot, "Constitution.md"))
	if err != nil {
		t.Fatalf("Constitution.md not restored: %v", err)
	}
	if !strings.Contains(string(data), "Original content.") {
		t.Errorf("unexpected Constitution.md content:\n%s", string(data))
	}

	// CLAUDE.md should be restored
	claudeData, _ := os.ReadFile(filepath.Join(home, ".claude", "CLAUDE.md"))
	if !strings.Contains(string(claudeData), "@~/.ai/Constitution.md") {
		t.Errorf("CLAUDE.md not restored correctly:\n%s", string(claudeData))
	}
}
