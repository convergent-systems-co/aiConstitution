package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Cursor integration tests (#222)
// ---------------------------------------------------------------------------

// TestIntegrateCursor_CreatesSymlink verifies that runIntegrateCursor creates
// .cursor/rules/constitution.md → <aiRoot>/Constitution.runtime.md in cwd.
func TestIntegrateCursor_CreatesSymlink(t *testing.T) {
	cwd := t.TempDir()
	aiRoot := t.TempDir()

	runtimeFile := filepath.Join(aiRoot, "Constitution.runtime.md")
	if err := os.WriteFile(runtimeFile, []byte("# runtime"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := runIntegrateCursor(cwd, aiRoot); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	symlinkPath := filepath.Join(cwd, ".cursor", "rules", "constitution.md")
	target, err := os.Readlink(symlinkPath)
	if err != nil {
		t.Fatalf("symlink not created at %s: %v", symlinkPath, err)
	}
	if target != runtimeFile {
		t.Errorf("symlink target = %q, want %q", target, runtimeFile)
	}
}

// TestIntegrateCursor_CreatesRulesDir verifies .cursor/rules/ is created
// when absent.
func TestIntegrateCursor_CreatesRulesDir(t *testing.T) {
	cwd := t.TempDir()
	aiRoot := t.TempDir()

	runtimeFile := filepath.Join(aiRoot, "Constitution.runtime.md")
	if err := os.WriteFile(runtimeFile, []byte("# runtime"), 0o644); err != nil {
		t.Fatal(err)
	}

	rulesDir := filepath.Join(cwd, ".cursor", "rules")
	if _, err := os.Stat(rulesDir); !os.IsNotExist(err) {
		t.Fatal("expected .cursor/rules to not exist before install")
	}

	if err := runIntegrateCursor(cwd, aiRoot); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if fi, err := os.Stat(rulesDir); err != nil || !fi.IsDir() {
		t.Errorf(".cursor/rules dir was not created")
	}
}

// TestIntegrateCursor_Idempotent verifies a second call with the correct
// symlink in place is a no-op and does not error.
func TestIntegrateCursor_Idempotent(t *testing.T) {
	cwd := t.TempDir()
	aiRoot := t.TempDir()

	runtimeFile := filepath.Join(aiRoot, "Constitution.runtime.md")
	if err := os.WriteFile(runtimeFile, []byte("# runtime"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := runIntegrateCursor(cwd, aiRoot); err != nil {
		t.Fatalf("first call error: %v", err)
	}
	if err := runIntegrateCursor(cwd, aiRoot); err != nil {
		t.Fatalf("second call (idempotent) error: %v", err)
	}

	symlinkPath := filepath.Join(cwd, ".cursor", "rules", "constitution.md")
	target, err := os.Readlink(symlinkPath)
	if err != nil {
		t.Fatalf("symlink missing after second call: %v", err)
	}
	if target != runtimeFile {
		t.Errorf("symlink target = %q, want %q", target, runtimeFile)
	}
}

// TestIntegrateCursor_PrintsPath verifies the printed message contains the
// symlink path (captured via cobra command execution against a temp env).
// This is a lightweight smoke-test via cobra Execute.
func TestIntegrateCursor_WarnsMissingRuntime(t *testing.T) {
	cwd := t.TempDir()
	aiRoot := t.TempDir()
	// Deliberately do NOT create Constitution.runtime.md.

	err := runIntegrateCursor(cwd, aiRoot)
	if err == nil {
		t.Fatal("expected error when Constitution.runtime.md is absent, got nil")
	}
	if !strings.Contains(err.Error(), "Constitution.runtime.md") {
		t.Errorf("error message = %q, want mention of Constitution.runtime.md", err.Error())
	}
}

// ---------------------------------------------------------------------------
// Codex integration tests (#223)
// ---------------------------------------------------------------------------

// TestIntegrateCodex_WritesAgentsMD verifies AGENTS.md is created with the
// required @-include when it does not exist.
func TestIntegrateCodex_WritesAgentsMD(t *testing.T) {
	cwd := t.TempDir()

	if err := runIntegrateCodex(cwd); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	agentsPath := filepath.Join(cwd, "AGENTS.md")
	content, err := os.ReadFile(agentsPath)
	if err != nil {
		t.Fatalf("AGENTS.md not created: %v", err)
	}
	if !strings.Contains(string(content), "@~/.ai/Constitution.md") {
		t.Errorf("AGENTS.md content = %q, want @~/.ai/Constitution.md", string(content))
	}
}

// TestIntegrateCodex_Idempotent verifies that running twice does NOT
// duplicate the @-include line.
func TestIntegrateCodex_Idempotent(t *testing.T) {
	cwd := t.TempDir()

	if err := runIntegrateCodex(cwd); err != nil {
		t.Fatalf("first call error: %v", err)
	}
	if err := runIntegrateCodex(cwd); err != nil {
		t.Fatalf("second call error: %v", err)
	}

	agentsPath := filepath.Join(cwd, "AGENTS.md")
	content, err := os.ReadFile(agentsPath)
	if err != nil {
		t.Fatalf("AGENTS.md missing: %v", err)
	}

	count := strings.Count(string(content), "@~/.ai/Constitution.md")
	if count != 1 {
		t.Errorf("@-include appears %d times, want exactly 1", count)
	}
}

// TestIntegrateCodex_AppendsToExistingFile verifies that when AGENTS.md
// exists but lacks the @-include, the Constitution section is appended
// rather than overwriting the existing content.
func TestIntegrateCodex_AppendsToExistingFile(t *testing.T) {
	cwd := t.TempDir()
	agentsPath := filepath.Join(cwd, "AGENTS.md")

	existing := "# Existing Content\n\nSome pre-existing agent instructions.\n"
	if err := os.WriteFile(agentsPath, []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := runIntegrateCodex(cwd); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	content, err := os.ReadFile(agentsPath)
	if err != nil {
		t.Fatalf("AGENTS.md missing: %v", err)
	}
	body := string(content)

	// Must preserve original content.
	if !strings.Contains(body, "Some pre-existing agent instructions.") {
		t.Error("existing content was lost after append")
	}
	// Must add @-include.
	if !strings.Contains(body, "@~/.ai/Constitution.md") {
		t.Error("@-include not appended to existing AGENTS.md")
	}
}

// TestIntegrateCodex_SkipsAlreadyPresent verifies that if AGENTS.md already
// contains the @-include, neither content is duplicated nor modified.
func TestIntegrateCodex_SkipsAlreadyPresent(t *testing.T) {
	cwd := t.TempDir()
	agentsPath := filepath.Join(cwd, "AGENTS.md")

	existing := "# AI Agents\n\n@~/.ai/Constitution.md\n\nSome extra text.\n"
	if err := os.WriteFile(agentsPath, []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := runIntegrateCodex(cwd); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	content, err := os.ReadFile(agentsPath)
	if err != nil {
		t.Fatalf("AGENTS.md missing: %v", err)
	}
	if string(content) != existing {
		t.Errorf("AGENTS.md was modified when @-include already present\ngot: %q\nwant: %q", string(content), existing)
	}
}

// ---------------------------------------------------------------------------
// Doctor checks tests (#224)
// ---------------------------------------------------------------------------

// TestDoctorCopilotCheck_ValidSymlink verifies the Copilot check returns [✓]
// when ~/.copilot/instructions/constitution.md is a valid symlink.
func TestDoctorCopilotCheck_ValidSymlink(t *testing.T) {
	home := t.TempDir()
	aiRoot := t.TempDir()

	// Create runtime file.
	runtimeFile := filepath.Join(aiRoot, "Constitution.runtime.md")
	if err := os.WriteFile(runtimeFile, []byte("# runtime"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create valid symlink.
	instructionsDir := filepath.Join(home, ".copilot", "instructions")
	if err := os.MkdirAll(instructionsDir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(runtimeFile, filepath.Join(instructionsDir, "constitution.md")); err != nil {
		t.Fatal(err)
	}

	result := checkDoctorCopilot(home)
	if result.status != doctorOK {
		t.Errorf("expected doctorOK, got status=%v msg=%q", result.status, result.message)
	}
}

// TestDoctorCopilotCheck_MissingSymlink verifies the Copilot check returns [⚠]
// when ~/.copilot/instructions/ exists but constitution.md symlink is missing.
func TestDoctorCopilotCheck_MissingSymlink(t *testing.T) {
	home := t.TempDir()

	// Create the directory but NOT the symlink.
	instructionsDir := filepath.Join(home, ".copilot", "instructions")
	if err := os.MkdirAll(instructionsDir, 0o750); err != nil {
		t.Fatal(err)
	}

	result := checkDoctorCopilot(home)
	if result.status != doctorWarn {
		t.Errorf("expected doctorWarn, got status=%v msg=%q", result.status, result.message)
	}
}

// TestDoctorCopilotCheck_NoDirectory verifies that when ~/.copilot/instructions/
// does not exist, the check is skipped (returns doctorSkip).
func TestDoctorCopilotCheck_NoDirectory(t *testing.T) {
	home := t.TempDir()
	// No ~/.copilot/ directory at all.

	result := checkDoctorCopilot(home)
	if result.status != doctorSkip {
		t.Errorf("expected doctorSkip, got status=%v msg=%q", result.status, result.message)
	}
}

// TestDoctorCursorCheck_ValidSymlink verifies the Cursor check returns [✓]
// when .cursor/rules/constitution.md is a valid symlink.
func TestDoctorCursorCheck_ValidSymlink(t *testing.T) {
	cwd := t.TempDir()
	aiRoot := t.TempDir()

	runtimeFile := filepath.Join(aiRoot, "Constitution.runtime.md")
	if err := os.WriteFile(runtimeFile, []byte("# runtime"), 0o644); err != nil {
		t.Fatal(err)
	}

	rulesDir := filepath.Join(cwd, ".cursor", "rules")
	if err := os.MkdirAll(rulesDir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(runtimeFile, filepath.Join(rulesDir, "constitution.md")); err != nil {
		t.Fatal(err)
	}

	result := checkDoctorCursor(cwd)
	if result.status != doctorOK {
		t.Errorf("expected doctorOK, got status=%v msg=%q", result.status, result.message)
	}
}

// TestDoctorCursorCheck_NoCursorDir verifies that when .cursor/ does not exist,
// the check is skipped.
func TestDoctorCursorCheck_NoCursorDir(t *testing.T) {
	cwd := t.TempDir()

	result := checkDoctorCursor(cwd)
	if result.status != doctorSkip {
		t.Errorf("expected doctorSkip, got status=%v msg=%q", result.status, result.message)
	}
}

// TestDoctorAgentsMDCheck_ValidInclude verifies AGENTS.md check returns [✓]
// when AGENTS.md contains @~/.ai/Constitution.md.
func TestDoctorAgentsMDCheck_ValidInclude(t *testing.T) {
	cwd := t.TempDir()
	agentsPath := filepath.Join(cwd, "AGENTS.md")
	if err := os.WriteFile(agentsPath, []byte("@~/.ai/Constitution.md\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	result := checkDoctorAgentsMD(cwd)
	if result.status != doctorOK {
		t.Errorf("expected doctorOK, got status=%v msg=%q", result.status, result.message)
	}
}

// TestDoctorAgentsMDCheck_MissingInclude verifies AGENTS.md check returns [⚠]
// when AGENTS.md exists but lacks the @-include.
func TestDoctorAgentsMDCheck_MissingInclude(t *testing.T) {
	cwd := t.TempDir()
	agentsPath := filepath.Join(cwd, "AGENTS.md")
	if err := os.WriteFile(agentsPath, []byte("# Some existing content\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	result := checkDoctorAgentsMD(cwd)
	if result.status != doctorWarn {
		t.Errorf("expected doctorWarn, got status=%v msg=%q", result.status, result.message)
	}
}

// TestDoctorAgentsMDCheck_NoFile verifies AGENTS.md check returns doctorSkip
// when AGENTS.md does not exist.
func TestDoctorAgentsMDCheck_NoFile(t *testing.T) {
	cwd := t.TempDir()
	// No AGENTS.md.

	result := checkDoctorAgentsMD(cwd)
	if result.status != doctorSkip {
		t.Errorf("expected doctorSkip, got status=%v msg=%q", result.status, result.message)
	}
}
