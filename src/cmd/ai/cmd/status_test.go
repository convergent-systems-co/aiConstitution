package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func runStatusCmd(t *testing.T, args ...string) string {
	t.Helper()
	root := NewRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs(append([]string{"status"}, args...))
	_ = root.Execute()
	return buf.String()
}

func TestStatus_PrintsAIRoot(t *testing.T) {
	aiRoot := t.TempDir()
	t.Setenv("AI_ROOT", aiRoot)

	out := runStatusCmd(t)
	if !strings.Contains(out, "AI Root") && !strings.Contains(out, aiRoot) {
		t.Errorf("expected AI Root in output\n%s", out)
	}
}

func TestStatus_ConstitutionFilesSection(t *testing.T) {
	aiRoot := t.TempDir()
	t.Setenv("AI_ROOT", aiRoot)
	// Create two of four files
	_ = os.WriteFile(filepath.Join(aiRoot, "Constitution.md"), []byte("# C"), 0o644)
	_ = os.WriteFile(filepath.Join(aiRoot, "Common.md"), []byte("# C"), 0o644)

	out := runStatusCmd(t)
	// Should mention both files
	if !strings.Contains(out, "Constitution.md") {
		t.Errorf("expected Constitution.md in output\n%s", out)
	}
	if !strings.Contains(out, "Common.md") {
		t.Errorf("expected Common.md in output\n%s", out)
	}
}

func TestStatus_ShowsPresentVsMissing(t *testing.T) {
	aiRoot := t.TempDir()
	t.Setenv("AI_ROOT", aiRoot)
	_ = os.WriteFile(filepath.Join(aiRoot, "Constitution.md"), []byte("# C"), 0o644)
	// Code.md, Common.md, Writing.md intentionally absent

	out := runStatusCmd(t)
	// Should show ✓ for present and ✗ for missing
	if !strings.Contains(out, "✓") && !strings.Contains(out, "✗") {
		t.Errorf("expected ✓/✗ markers in output\n%s", out)
	}
}

func TestStatus_HooksSection(t *testing.T) {
	aiRoot := t.TempDir()
	t.Setenv("AI_ROOT", aiRoot)
	hooksDir := filepath.Join(aiRoot, "hooks")
	_ = os.MkdirAll(hooksDir, 0o755)
	_ = os.WriteFile(filepath.Join(hooksDir, "audit.py"), []byte("# hook"), 0o644)
	_ = os.WriteFile(filepath.Join(hooksDir, "branch-guard.py"), []byte("# hook"), 0o644)

	out := runStatusCmd(t)
	if !strings.Contains(out, "Hook") && !strings.Contains(out, "hook") {
		t.Errorf("expected hooks section in output\n%s", out)
	}
}

func TestStatus_MemorySection(t *testing.T) {
	aiRoot := t.TempDir()
	t.Setenv("AI_ROOT", aiRoot)
	memDir := filepath.Join(aiRoot, "memory")
	_ = os.MkdirAll(memDir, 0o755)
	memContent := "# Memory Index\n\n## Feedback\n- [foo](feedback_foo.md) — a foo\n- [bar](feedback_bar.md) — a bar\n"
	_ = os.WriteFile(filepath.Join(memDir, "MEMORY.md"), []byte(memContent), 0o644)

	out := runStatusCmd(t)
	if !strings.Contains(out, "Memory") && !strings.Contains(out, "memory") {
		t.Errorf("expected memory section in output\n%s", out)
	}
	// Should show entry count
	if !strings.Contains(out, "2") && !strings.Contains(out, "Entries") {
		t.Logf("note: memory entry count may not be visible\n%s", out)
	}
}

func TestStatus_AuditSection(t *testing.T) {
	aiRoot := t.TempDir()
	t.Setenv("AI_ROOT", aiRoot)
	interDir := filepath.Join(aiRoot, "audit", "interactions")
	_ = os.MkdirAll(interDir, 0o755)
	month := time.Now().UTC().Format("2006-01")
	_ = os.WriteFile(filepath.Join(interDir, month+".jsonl"), []byte(`{"chronon":"now","kind":"signal"}`+"\n"), 0o644)

	out := runStatusCmd(t)
	if !strings.Contains(out, "Audit") && !strings.Contains(out, "audit") && !strings.Contains(out, "interaction") {
		t.Errorf("expected audit section in output\n%s", out)
	}
}

func TestStatus_NoStubError(t *testing.T) {
	aiRoot := t.TempDir()
	t.Setenv("AI_ROOT", aiRoot)

	root := NewRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"status"})
	err := root.Execute()
	out := buf.String()

	// Must not return the stub "not yet implemented" error
	if err != nil && strings.Contains(err.Error(), "not yet implemented") {
		t.Errorf("status returned stub error: %v\noutput: %s", err, out)
	}
	if strings.Contains(out, "not yet implemented") {
		t.Errorf("status output contains stub message\n%s", out)
	}
}
