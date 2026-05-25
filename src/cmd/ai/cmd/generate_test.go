// Package cmd is the cobra command tree for `ai`.
package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestGenerateRuntime_WritesFile(t *testing.T) {
	aiRoot := t.TempDir()
	t.Setenv("AI_ROOT", aiRoot)

	constitutionContent := `# AI Constitution — TestUser

**Principal:** TestUser
**Version:** 1.0

## §1 Governance

Override protocol.

## §2 Behavioral Standards

### §2.1 Conviction

Agreement is not the goal.

## §3 Universal Rules

### §3.1 Prime Directives

P1. Civilization-grade output.

### §3.2 Autonomy Gates

Cost ceiling: $5

## §4 Technical Work

Code domain.
`
	if err := os.WriteFile(filepath.Join(aiRoot, "Constitution.md"),
		[]byte(constitutionContent), 0o600); err != nil {
		t.Fatal(err)
	}

	root := NewRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"generate", "runtime"})
	if err := root.Execute(); err != nil {
		t.Fatalf("generate runtime: %v\n%s", err, buf.String())
	}

	runtimePath := filepath.Join(aiRoot, "Constitution.runtime.md")
	data, err := os.ReadFile(runtimePath)
	if err != nil {
		t.Fatalf("Constitution.runtime.md not written: %v", err)
	}
	if len(data) == 0 {
		t.Error("Constitution.runtime.md is empty")
	}
}

func TestGenerateRuntime_MissingConstitution_ReturnsError(t *testing.T) {
	aiRoot := t.TempDir()
	t.Setenv("AI_ROOT", aiRoot)

	root := NewRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"generate", "runtime"})
	err := root.Execute()
	if err == nil {
		t.Error("expected error when Constitution.md missing")
	}
}
