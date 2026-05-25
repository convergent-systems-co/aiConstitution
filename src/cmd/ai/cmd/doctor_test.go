package cmd_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/convergent-systems-co/aiConstitution/src/cmd/ai/cmd"
)

func TestDoctorReportsAllFilesPresent(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("AI_ROOT", dir)

	for _, name := range []string{"Constitution.md", "Common.md", "Code.md", "Writing.md"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("# "+name+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	// Provide a CLAUDE.md with the personas block so doctor check 2 passes.
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	claudeDir := filepath.Join(homeDir, ".claude")
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	claudeMD := filepath.Join(claudeDir, "CLAUDE.md")
	block := "# Instructions\n<!-- ai:personas — managed by ai cli, do not edit manually -->\n@" + dir + "/Common.md\n<!-- /ai:personas -->\n"
	if err := os.WriteFile(claudeMD, []byte(block), 0o600); err != nil {
		t.Fatal(err)
	}

	root := cmd.NewRootCmd()
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"doctor"})
	if err := root.Execute(); err != nil {
		t.Logf("doctor output: %s", buf)
		t.Errorf("doctor returned error: %v", err)
	}
	if !bytes.Contains(buf.Bytes(), []byte("Constitution.md")) {
		t.Errorf("expected 'Constitution.md' in output, got:\n%s", buf)
	}
}

func TestDoctorReportsMissingFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("AI_ROOT", dir)
	for _, name := range []string{"Constitution.md", "Common.md", "Code.md"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("# "+name+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	// Writing.md absent

	root := cmd.NewRootCmd()
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"doctor"})
	err := root.Execute()
	if err == nil {
		t.Error("doctor should return error when required files are missing")
	}
	if !bytes.Contains(buf.Bytes(), []byte("Writing.md")) {
		t.Errorf("expected 'Writing.md' in output, got:\n%s", buf)
	}
}
