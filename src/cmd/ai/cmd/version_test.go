package cmd

import (
	"bytes"
	"strings"
	"testing"
)

// TestVersionOutputFormat exercises `ai version` end-to-end and asserts
// the two documented output lines appear: binary version and questions.yaml.
// Code.md was removed in the unified-constitution model (v1.4.x) — it is
// now a section of Constitution.md, not a separate tracked file.
func TestVersionOutputFormat(t *testing.T) {
	buf := &bytes.Buffer{}
	cmd := newVersionCmd()
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs(nil)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	out := buf.String()

	if !strings.HasPrefix(out, "ai ") {
		t.Errorf("want output to start with 'ai ', got:\n%s", out)
	}
	if !strings.Contains(out, "questions.yaml ") {
		t.Errorf("want output to contain 'questions.yaml ', got:\n%s", out)
	}
	if strings.Contains(out, "Code.md") {
		t.Errorf("Code.md should not appear in version output (unified model): got:\n%s", out)
	}
}
