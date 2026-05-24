package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestVersionPrintsCodeMdVersion exercises `ai version` end-to-end. We
// stage a mock ~/.ai/ root containing a Code.md whose **Version:** line
// declares 0.7, point AI_ROOT at it, capture stdout, and assert the
// three documented lines appear (binary, Code.md, questions.yaml).
func TestVersionPrintsCodeMdVersion(t *testing.T) {
	tmp := t.TempDir()
	codemd := "# Code.md\n\n**Version:** 0.7\n\nbody\n"
	if err := os.WriteFile(filepath.Join(tmp, "Code.md"), []byte(codemd), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	t.Setenv("AI_ROOT", tmp)

	buf := &bytes.Buffer{}
	cmd := newVersionCmd()
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs(nil)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "Code.md 0.7") {
		t.Errorf("want output to contain %q, got:\n%s", "Code.md 0.7", out)
	}
	if !strings.HasPrefix(out, "ai ") {
		t.Errorf("want output to start with %q, got:\n%s", "ai ", out)
	}
	if !strings.Contains(out, "questions.yaml ") {
		t.Errorf("want output to contain %q, got:\n%s", "questions.yaml ", out)
	}
}

// TestVersionMissingCodeMdReportsNotFound covers the case where AI_ROOT
// has no Code.md (fresh install). The output must still render but the
// Code.md line says "not found" rather than fabricating a version.
func TestVersionMissingCodeMdReportsNotFound(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("AI_ROOT", tmp)

	buf := &bytes.Buffer{}
	cmd := newVersionCmd()
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs(nil)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "Code.md not found") {
		t.Errorf("want output to contain %q, got:\n%s", "Code.md not found", out)
	}
}
