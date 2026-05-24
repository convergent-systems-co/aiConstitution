package cmd_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/convergent-systems-co/aiConstitution/src/cmd/ai/cmd"
)

func TestStatusShowsConstitutionFiles(t *testing.T) {
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
	root.SetArgs([]string{"status"})
	root.Execute() //nolint:errcheck
	if !bytes.Contains(buf.Bytes(), []byte("Constitution")) {
		t.Errorf("expected 'Constitution' in status output, got:\n%s", buf)
	}
}

func TestStatusShowsAIRoot(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("AI_ROOT", dir)

	root := cmd.NewRootCmd()
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"status"})
	root.Execute() //nolint:errcheck
	if !bytes.Contains(buf.Bytes(), []byte(dir)) {
		t.Errorf("expected AI root path %q in status output, got:\n%s", dir, buf)
	}
}
