// Package cmd is the cobra command tree for `ai`.
package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestSetupNonInteractive_WritesConstitution(t *testing.T) {
	aiRoot := t.TempDir()
	t.Setenv("AI_ROOT", aiRoot)

	root := NewRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"setup", "--non-interactive"})
	if err := root.Execute(); err != nil {
		t.Fatalf("setup --non-interactive: %v\n%s", err, buf.String())
	}

	constitutionPath := filepath.Join(aiRoot, "Constitution.md")
	data, err := os.ReadFile(constitutionPath)
	if err != nil {
		t.Fatalf("Constitution.md not written: %v", err)
	}
	if len(data) < 1000 {
		t.Errorf("Constitution.md too short (%d bytes) — likely not rendered", len(data))
	}

	runtimePath := filepath.Join(aiRoot, "Constitution.runtime.md")
	if _, err := os.Stat(runtimePath); err != nil {
		t.Error("Constitution.runtime.md not written")
	}
}
