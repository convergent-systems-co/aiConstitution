package cmd_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/convergent-systems-co/aiConstitution/src/cmd/ai/cmd"
)

// stubCommon is a Common.md fragment with the version line + a section
// that ai amend can target.
const stubCommon = `# Common.md

**Version:** 0.17

## U17 Worktree placement

Body text.

## Changelog

- **0.17** — Initial.
`

func TestAmendDraftWritesFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("AI_ROOT", dir)
	t.Setenv("AICONST_CONFIG_DIR", dir)
	t.Setenv("EDITOR", "") // no editor — print path mode
	if err := os.WriteFile(filepath.Join(dir, "Common.md"), []byte(stubCommon), 0o600); err != nil {
		t.Fatal(err)
	}

	root := cmd.NewRootCmd()
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"amend", "draft", "Common.md/U17"})
	if err := root.Execute(); err != nil {
		t.Fatalf("amend draft error: %v\noutput:%s", err, buf)
	}

	plansDir := filepath.Join(dir, "governance", "plans")
	entries, err := os.ReadDir(plansDir)
	if err != nil {
		t.Fatalf("read plans dir: %v", err)
	}
	if len(entries) == 0 {
		t.Fatalf("no draft created in %s", plansDir)
	}
	if !strings.Contains(buf.String(), "Wrote draft:") {
		t.Errorf("expected 'Wrote draft:' in output, got: %s", buf)
	}
}

func TestAmendDraftWarnsOnMissingSection(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("AI_ROOT", dir)
	t.Setenv("AICONST_CONFIG_DIR", dir)
	t.Setenv("EDITOR", "")
	if err := os.WriteFile(filepath.Join(dir, "Common.md"), []byte(stubCommon), 0o600); err != nil {
		t.Fatal(err)
	}

	root := cmd.NewRootCmd()
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"amend", "draft", "Common.md/U99"})
	if err := root.Execute(); err != nil {
		t.Fatalf("amend draft error: %v\noutput:%s", err, buf)
	}
	if !strings.Contains(buf.String(), "not found") {
		t.Errorf("expected warning about missing section, got: %s", buf)
	}
}
