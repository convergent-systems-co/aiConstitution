package cmd_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
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

func TestCheckBinPathOK(t *testing.T) {
	binDir := "/Users/example/.ai/bin"
	pathVar := binDir + ":/usr/local/bin:/opt/homebrew/bin:/usr/bin"
	status, msg := cmd.CheckBinPathForTest(binDir, pathVar)
	if status != cmd.PathOK {
		t.Errorf("status = %d, want PathOK; msg=%s", status, msg)
	}
}

func TestCheckBinPathMissing(t *testing.T) {
	binDir := "/Users/example/.ai/bin"
	pathVar := "/usr/local/bin:/opt/homebrew/bin:/usr/bin"
	status, msg := cmd.CheckBinPathForTest(binDir, pathVar)
	if status != cmd.PathMissing {
		t.Errorf("status = %d, want PathMissing; msg=%s", status, msg)
	}
	if !strings.Contains(msg, "not on PATH") {
		t.Errorf("expected 'not on PATH' in message, got: %s", msg)
	}
}

func TestCheckBinPathShadowed(t *testing.T) {
	binDir := "/Users/example/.ai/bin"
	pathVar := "/usr/local/bin:" + binDir + ":/usr/bin"
	status, msg := cmd.CheckBinPathForTest(binDir, pathVar)
	if status != cmd.PathShadowed {
		t.Errorf("status = %d, want PathShadowed; msg=%s", status, msg)
	}
	if !strings.Contains(msg, "appears earlier") {
		t.Errorf("expected 'appears earlier' in message, got: %s", msg)
	}
}

func TestDoctorPrintsPATHRow(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("AI_ROOT", dir)
	for _, name := range []string{"Constitution.md", "Common.md", "Code.md", "Writing.md"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("# "+name+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	// PATH does not include ~/.ai/bin, so we expect a "!" warning row.
	t.Setenv("PATH", "/usr/local/bin:/usr/bin")

	root := cmd.NewRootCmd()
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"doctor"})
	if err := root.Execute(); err != nil {
		t.Logf("doctor output: %s", buf)
		t.Errorf("doctor unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), "not on PATH") {
		t.Errorf("expected 'not on PATH' warning in doctor output, got:\n%s", buf)
	}
}
