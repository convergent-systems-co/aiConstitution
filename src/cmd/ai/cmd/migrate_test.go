package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func helperMigrateAIRoot(t *testing.T) string {
	t.Helper()
	aiRoot := t.TempDir()
	t.Setenv("AI_ROOT", aiRoot)
	files := map[string]string{
		"Constitution.md": "# Constitution\n\n## 1. The File System\n\nFour files.\n\n## 8. Changelog\n- 0.3\n",
		"Common.md":       "# Common\n\n## 1. Prime Directives\n\nP1. Civilization.\n\n## 2. Autonomy Gates\n\nCommon.md §U17 worktree rule.\n\n## 6. Changelog\n- 0.17\n",
		"Code.md":         "# Code\n\n## 1. Clean Code\n\nNames reveal intent.\n\n## 12. Changelog\n- 0.7\n",
		"Writing.md":      "# Writing\n\n## 1. Voice\n\nMatch the voice.\n\n## 13. Changelog\n- 0.4\n",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(aiRoot, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return aiRoot
}

func runMigrateCmd(t *testing.T, args ...string) (string, error) {
	t.Helper()
	root := NewRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs(append([]string{"migrate"}, args...))
	err := root.Execute()
	return buf.String(), err
}

func TestMigrateFlatten_ProducesUnifiedFile(t *testing.T) {
	aiRoot := helperMigrateAIRoot(t)

	_, err := runMigrateCmd(t, "--flatten")
	if err != nil {
		t.Fatalf("migrate --flatten: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(aiRoot, "Constitution.md"))
	if err != nil {
		t.Fatal("Constitution.md not present after flatten")
	}
	body := string(data)
	for _, want := range []string{"§1", "§3", "§4", "§5", "Clean Code", "Prime Directives"} {
		if !strings.Contains(body, want) {
			t.Errorf("unified Constitution.md missing %q", want)
		}
	}
	// Cross-references should be rewritten
	if strings.Contains(body, "Common.md §U17") {
		t.Error("Common.md §U17 cross-reference not rewritten")
	}
}

func TestMigrateFlatten_ArchivesOriginals(t *testing.T) {
	aiRoot := helperMigrateAIRoot(t)
	_, _ = runMigrateCmd(t, "--flatten")

	archiveDir := filepath.Join(aiRoot, "archive", "pre-v2")
	for _, name := range []string{"Constitution.md", "Common.md", "Code.md", "Writing.md"} {
		if _, err := os.Stat(filepath.Join(archiveDir, name)); err != nil {
			t.Errorf("original file %s not archived: %v", name, err)
		}
	}
}

func TestMigrateAddBehavioral_InsertsSection(t *testing.T) {
	aiRoot := helperMigrateAIRoot(t)
	_, _ = runMigrateCmd(t, "--flatten")

	_, err := runMigrateCmd(t, "--add-behavioral")
	if err != nil {
		t.Fatalf("migrate --add-behavioral: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(aiRoot, "Constitution.md"))
	body := string(data)
	if !strings.Contains(body, "§2 Behavioral Standards") {
		t.Error("§2 Behavioral Standards section not inserted")
	}
	if !strings.Contains(body, "§2.1") {
		t.Error("§2.1 Conviction not present")
	}
}

func TestMigrateGenerateRuntime_WritesRuntimeFile(t *testing.T) {
	aiRoot := helperMigrateAIRoot(t)
	_, _ = runMigrateCmd(t, "--flatten")
	_, _ = runMigrateCmd(t, "--add-behavioral")

	_, err := runMigrateCmd(t, "--generate-runtime")
	if err != nil {
		t.Fatalf("migrate --generate-runtime: %v", err)
	}

	if _, err := os.Stat(filepath.Join(aiRoot, "Constitution.runtime.md")); err != nil {
		t.Error("Constitution.runtime.md not written by --generate-runtime")
	}
}
