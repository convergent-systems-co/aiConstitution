package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestHooksCopilotInstall_CreatesSymlink verifies that `hooks install --copilot`
// creates ~/.copilot/instructions/constitution.md → <aiRoot>/Constitution.runtime.md.
func TestHooksCopilotInstall_CreatesSymlink(t *testing.T) {
	home := t.TempDir()
	aiRoot := t.TempDir()

	// Create the runtime file so the "happy path" runs cleanly.
	runtimeFile := filepath.Join(aiRoot, "Constitution.runtime.md")
	if err := os.WriteFile(runtimeFile, []byte("# runtime"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := runHooksCopilotInstall(aiRoot, home)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	symlinkPath := filepath.Join(home, ".copilot", "instructions", "constitution.md")
	target, err := os.Readlink(symlinkPath)
	if err != nil {
		t.Fatalf("symlink not created at %s: %v", symlinkPath, err)
	}
	if target != runtimeFile {
		t.Errorf("symlink target = %q, want %q", target, runtimeFile)
	}
}

// TestHooksCopilotInstall_CreatesDirectory verifies that the
// ~/.copilot/instructions/ directory is created if absent.
func TestHooksCopilotInstall_CreatesDirectory(t *testing.T) {
	home := t.TempDir()
	aiRoot := t.TempDir()

	runtimeFile := filepath.Join(aiRoot, "Constitution.runtime.md")
	if err := os.WriteFile(runtimeFile, []byte("# runtime"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Confirm the dir does not pre-exist.
	instructionsDir := filepath.Join(home, ".copilot", "instructions")
	if _, err := os.Stat(instructionsDir); !os.IsNotExist(err) {
		t.Fatal("expected instructions dir to not exist before install")
	}

	if err := runHooksCopilotInstall(aiRoot, home); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if fi, err := os.Stat(instructionsDir); err != nil || !fi.IsDir() {
		t.Errorf("instructions dir was not created at %s", instructionsDir)
	}
}

// TestHooksCopilotInstall_Idempotent verifies that running install twice
// with the symlink already pointing to the right target is a no-op (no error).
func TestHooksCopilotInstall_Idempotent(t *testing.T) {
	home := t.TempDir()
	aiRoot := t.TempDir()

	runtimeFile := filepath.Join(aiRoot, "Constitution.runtime.md")
	if err := os.WriteFile(runtimeFile, []byte("# runtime"), 0o644); err != nil {
		t.Fatal(err)
	}

	// First install.
	if err := runHooksCopilotInstall(aiRoot, home); err != nil {
		t.Fatalf("first install error: %v", err)
	}

	// Second install — must not error.
	if err := runHooksCopilotInstall(aiRoot, home); err != nil {
		t.Fatalf("second install (idempotent) error: %v", err)
	}

	// Symlink must still point to the same target.
	symlinkPath := filepath.Join(home, ".copilot", "instructions", "constitution.md")
	target, err := os.Readlink(symlinkPath)
	if err != nil {
		t.Fatalf("symlink missing after second install: %v", err)
	}
	if target != runtimeFile {
		t.Errorf("symlink target = %q, want %q", target, runtimeFile)
	}
}

// TestHooksCopilotInstall_StaleSymlinkReplaced verifies that a symlink pointing
// to a wrong target is removed and recreated to the correct target.
func TestHooksCopilotInstall_StaleSymlinkReplaced(t *testing.T) {
	home := t.TempDir()
	aiRoot := t.TempDir()

	runtimeFile := filepath.Join(aiRoot, "Constitution.runtime.md")
	if err := os.WriteFile(runtimeFile, []byte("# runtime"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create a stale symlink first.
	instructionsDir := filepath.Join(home, ".copilot", "instructions")
	if err := os.MkdirAll(instructionsDir, 0o750); err != nil {
		t.Fatal(err)
	}
	symlinkPath := filepath.Join(instructionsDir, "constitution.md")
	if err := os.Symlink("/some/old/path", symlinkPath); err != nil {
		t.Fatal(err)
	}

	if err := runHooksCopilotInstall(aiRoot, home); err != nil {
		t.Fatalf("install with stale symlink error: %v", err)
	}

	target, err := os.Readlink(symlinkPath)
	if err != nil {
		t.Fatalf("symlink missing after stale replacement: %v", err)
	}
	if target != runtimeFile {
		t.Errorf("after stale replacement: symlink target = %q, want %q", target, runtimeFile)
	}
}

// TestHooksCopilotInstall_WarnsMissingRuntime verifies that when
// Constitution.runtime.md does not exist, the function returns an error
// containing a message directing the user to run 'ai generate runtime'.
func TestHooksCopilotInstall_WarnsMissingRuntime(t *testing.T) {
	home := t.TempDir()
	aiRoot := t.TempDir()
	// Deliberately do NOT create Constitution.runtime.md.

	err := runHooksCopilotInstall(aiRoot, home)
	if err == nil {
		t.Fatal("expected error when Constitution.runtime.md is absent, got nil")
	}
	if !strings.Contains(err.Error(), "ai generate runtime") {
		t.Errorf("error message = %q, want it to contain 'ai generate runtime'", err.Error())
	}
}
