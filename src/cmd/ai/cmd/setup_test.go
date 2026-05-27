package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestWriteClaudeMDCreatesFile verifies that writeClaudeMD writes a file
// to ~/.claude/CLAUDE.md containing the @-include directive.
func TestWriteClaudeMDCreatesFile(t *testing.T) {
	tmp := t.TempDir()
	claudeDir := filepath.Join(tmp, ".claude")
	if err := os.MkdirAll(claudeDir, 0o750); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	aiRoot := filepath.Join(tmp, ".ai")
	if err := writeClaudeMD(claudeDir, aiRoot); err != nil {
		t.Fatalf("writeClaudeMD: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(claudeDir, "CLAUDE.md"))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !strings.Contains(string(content), "@~/.ai/Constitution.md") {
		t.Errorf("CLAUDE.md does not contain @~/.ai/Constitution.md; got:\n%s", content)
	}
}

// TestWriteClaudeMDIsIdempotent verifies that calling writeClaudeMD twice
// does not duplicate the @-include line.
func TestWriteClaudeMDIsIdempotent(t *testing.T) {
	tmp := t.TempDir()
	claudeDir := filepath.Join(tmp, ".claude")
	if err := os.MkdirAll(claudeDir, 0o750); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	aiRoot := filepath.Join(tmp, ".ai")
	if err := writeClaudeMD(claudeDir, aiRoot); err != nil {
		t.Fatalf("first writeClaudeMD: %v", err)
	}
	if err := writeClaudeMD(claudeDir, aiRoot); err != nil {
		t.Fatalf("second writeClaudeMD: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(claudeDir, "CLAUDE.md"))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	count := strings.Count(string(content), "@~/.ai/Constitution.md")
	if count != 1 {
		t.Errorf("Idempotent: @-include appears %d times, want exactly 1;\n%s", count, content)
	}
}

// TestInstallCopilotSymlinkCreatesSymlink verifies that installCopilotSymlink
// creates a symlink from ~/.copilot/instructions/constitution.md to
// ~/.ai/Constitution.runtime.md.
func TestInstallCopilotSymlinkCreatesSymlink(t *testing.T) {
	tmp := t.TempDir()
	aiRoot := filepath.Join(tmp, ".ai")
	copilotDir := filepath.Join(tmp, ".copilot")

	// Create the symlink.
	if err := installCopilotSymlink(copilotDir, aiRoot); err != nil {
		t.Fatalf("installCopilotSymlink: %v", err)
	}

	linkPath := filepath.Join(copilotDir, "instructions", "constitution.md")
	target, err := os.Readlink(linkPath)
	if err != nil {
		t.Fatalf("Readlink: %v", err)
	}

	wantTarget := filepath.Join(aiRoot, "Constitution.runtime.md")
	if target != wantTarget {
		t.Errorf("symlink target = %q, want %q", target, wantTarget)
	}
}

// TestInstallCopilotSymlinkStaleSymlink verifies that a stale symlink is
// removed and recreated pointing to the correct target.
func TestInstallCopilotSymlinkStaleSymlink(t *testing.T) {
	tmp := t.TempDir()
	aiRoot := filepath.Join(tmp, ".ai")
	copilotDir := filepath.Join(tmp, ".copilot")
	instructionsDir := filepath.Join(copilotDir, "instructions")

	// Create the instructions directory and a stale symlink.
	if err := os.MkdirAll(instructionsDir, 0o750); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	linkPath := filepath.Join(instructionsDir, "constitution.md")
	if err := os.Symlink("/stale/target", linkPath); err != nil {
		t.Fatalf("Symlink (stale): %v", err)
	}

	// installCopilotSymlink should fix the stale link.
	if err := installCopilotSymlink(copilotDir, aiRoot); err != nil {
		t.Fatalf("installCopilotSymlink (stale): %v", err)
	}

	target, err := os.Readlink(linkPath)
	if err != nil {
		t.Fatalf("Readlink after fix: %v", err)
	}
	wantTarget := filepath.Join(aiRoot, "Constitution.runtime.md")
	if target != wantTarget {
		t.Errorf("symlink target after fix = %q, want %q", target, wantTarget)
	}
}

// TestRunSetupTUI_NonTTY_FallsBack verifies that runSetupTUI does not attempt
// to launch the Bubble Tea TUI when stdout is not a terminal (e.g. in CI or
// when output is piped). It should fall back to the non-interactive path and
// complete without error.
//
// In test environments os.Stdout is never a TTY, so this test exercises the
// real TTY-detection branch unconditionally.
func TestRunSetupTUI_NonTTY_FallsBack(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("AI_ROOT", tmp)
	t.Setenv("HOME", tmp)

	// runSetupTUI falls back to runSetupNonInteractive when not a TTY.
	// runSetupNonInteractive uses AICONST_SEEDS to fill required answers
	// and then calls runSetupPostWizard — which writes Constitution.md,
	// CLAUDE.md, and the Copilot symlink.
	t.Setenv("AICONST_SEEDS", "Q01=Test User,Q07=both")

	err := runSetupTUI(true /* noHooks=true to avoid hook extraction */)
	if err != nil {
		t.Fatalf("runSetupTUI non-TTY fallback: unexpected error: %v", err)
	}

	// Constitution.md must have been written.
	if _, statErr := os.Stat(filepath.Join(tmp, "Constitution.md")); statErr != nil {
		t.Error("Constitution.md not written by non-TTY fallback path")
	}
}

// TestSetupCreatesDirectories verifies that runSetupPostWizard creates all
// required subdirectories under aiRoot so that hooks and commands can write
// to them on first use without hitting "no such file or directory" errors.
func TestSetupCreatesDirectories(t *testing.T) {
	tmp := t.TempDir()
	claudeDir := filepath.Join(tmp, ".claude")
	aiRoot := filepath.Join(tmp, ".ai")
	copilotDir := filepath.Join(tmp, ".copilot")

	answers := map[string]string{"Q01": "Test Principal", "Q07": "both"}
	if err := runSetupPostWizard(aiRoot, claudeDir, copilotDir, answers, true /* noHooks */); err != nil {
		t.Fatalf("runSetupPostWizard: %v", err)
	}

	requiredDirs := []string{
		"audit",
		"audit/overrides",
		"audit/violations",
		"audit/interactions",
		"memory",
		"governance",
		"governance/plans",
		"governance/schemas",
		"governance/personas",
		"governance/agentic",
		"checkpoints",
	}
	for _, d := range requiredDirs {
		path := filepath.Join(aiRoot, d)
		info, err := os.Stat(path)
		if err != nil {
			t.Errorf("expected directory %q to exist: %v", d, err)
			continue
		}
		if !info.IsDir() {
			t.Errorf("expected %q to be a directory, got file mode %v", d, info.Mode())
		}
	}
}

// TestRunSetupWritesConstitutionFiles is an integration-style test that
// exercises the setup helpers end-to-end using temp dirs for all paths.
// It only verifies that config.Save is called without error (the stub
// implementation always returns nil) and that the CLAUDE.md file is created.
func TestRunSetupWritesConstitutionFiles(t *testing.T) {
	tmp := t.TempDir()
	claudeDir := filepath.Join(tmp, ".claude")
	aiRoot := filepath.Join(tmp, ".ai")
	if err := os.MkdirAll(claudeDir, 0o750); err != nil {
		t.Fatalf("MkdirAll claudeDir: %v", err)
	}
	if err := os.MkdirAll(aiRoot, 0o750); err != nil {
		t.Fatalf("MkdirAll aiRoot: %v", err)
	}

	// Run the helpers directly (not through cobra since cobra setup would
	// require a real terminal for the TUI).
	answers := map[string]string{"Q01": "Test Principal", "Q07": "both"}
	if err := runSetupPostWizard(aiRoot, claudeDir, filepath.Join(tmp, ".copilot"), answers, false); err != nil {
		t.Fatalf("runSetupPostWizard: %v", err)
	}

	// CLAUDE.md must exist and contain the @-include.
	content, err := os.ReadFile(filepath.Join(claudeDir, "CLAUDE.md"))
	if err != nil {
		t.Fatalf("CLAUDE.md missing: %v", err)
	}
	if !strings.Contains(string(content), "@~/.ai/Constitution.md") {
		t.Errorf("CLAUDE.md missing @-include; got:\n%s", content)
	}
}
