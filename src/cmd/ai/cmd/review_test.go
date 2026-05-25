package cmd_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/convergent-systems-co/aiConstitution/src/cmd/ai/cmd"
)

// TestReviewCmd_PRFlag_PrintsReportHeader verifies that `ai review --pr <n>`
// prints the expected report header to stdout.
//
// This test uses NewRootCmd() + cobra execution with a captured writer.
// The gh subprocess is NOT invoked in tests — the command stubs the diff.
func TestReviewCmd_PRFlag_PrintsReportHeader(t *testing.T) {
	root := cmd.NewRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&bytes.Buffer{}) // suppress stderr

	root.SetArgs([]string{"review", "--pr", "42"})
	// Execute returns an error because the stub is not yet implemented,
	// but the report header must be printed before the error is returned.
	// We don't assert on the error; we assert on the stdout content.
	_ = root.Execute()

	out := buf.String()
	if !strings.Contains(out, "## Review: PR #42") {
		t.Errorf("expected report header %q in output, got:\n%s", "## Review: PR #42", out)
	}
}

// TestReviewCmd_PRFlag_PrintsPanelLines verifies that each panel's result
// line appears in the output.
func TestReviewCmd_PRFlag_PrintsPanelLines(t *testing.T) {
	root := cmd.NewRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&bytes.Buffer{})

	root.SetArgs([]string{"review", "--pr", "99"})
	_ = root.Execute()

	out := buf.String()
	// At minimum, one panel line with the format [panel-name] must appear.
	if !strings.Contains(out, "[") {
		t.Errorf("expected at least one panel result line in output, got:\n%s", out)
	}
}

// TestReviewCmd_PRFlag_PrintsOverallLine verifies the overall score/verdict
// appears in the output.
func TestReviewCmd_PRFlag_PrintsOverallLine(t *testing.T) {
	root := cmd.NewRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&bytes.Buffer{})

	root.SetArgs([]string{"review", "--pr", "7"})
	_ = root.Execute()

	out := buf.String()
	if !strings.Contains(out, "Overall:") {
		t.Errorf("expected 'Overall:' in output, got:\n%s", out)
	}
}
