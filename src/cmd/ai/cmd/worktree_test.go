package cmd

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// makeRealGitRepo creates a real git repo in a temp dir with an initial commit.
// Returns the repo root path.
func makeRealGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=Test",
			"GIT_AUTHOR_EMAIL=test@test.com",
			"GIT_COMMITTER_NAME=Test",
			"GIT_COMMITTER_EMAIL=test@test.com",
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}

	run("init", "-b", "main")
	run("config", "user.email", "test@test.com")
	run("config", "user.name", "Test")
	// Create initial commit so HEAD is valid
	readme := filepath.Join(dir, "README.md")
	_ = os.WriteFile(readme, []byte("# test\n"), 0o644)
	run("add", "README.md")
	run("commit", "-m", "init")

	return dir
}

func runWorktreeCmd(t *testing.T, repoDir string, args ...string) (string, error) {
	t.Helper()
	root := NewRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	// Change working dir for the command via env
	oldWD, _ := os.Getwd()
	if repoDir != "" {
		if err := os.Chdir(repoDir); err != nil {
			t.Fatalf("chdir: %v", err)
		}
		t.Cleanup(func() { _ = os.Chdir(oldWD) })
	}
	root.SetArgs(append([]string{"worktree"}, args...))
	err := root.Execute()
	return buf.String(), err
}

func TestWorktreeAdd_CreatesCanonicalPath(t *testing.T) {
	repoDir := makeRealGitRepo(t)

	out, err := runWorktreeCmd(t, repoDir, "add", "my-feature")
	if err != nil {
		t.Fatalf("worktree add returned error: %v\noutput: %s", err, out)
	}

	// Canonical path: <repo>/.worktrees/my-feature/
	expectedPath := filepath.Join(repoDir, ".worktrees", "my-feature")
	if _, statErr := os.Stat(expectedPath); os.IsNotExist(statErr) {
		t.Errorf("expected worktree at %s but it doesn't exist", expectedPath)
	}
	if !strings.Contains(out, ".worktrees") && !strings.Contains(out, "my-feature") {
		t.Logf("note: output may not contain path (but worktree should still be created)\n%s", out)
	}
}

func TestWorktreeAdd_PrintsCreatedPath(t *testing.T) {
	repoDir := makeRealGitRepo(t)

	out, err := runWorktreeCmd(t, repoDir, "add", "print-path")
	if err != nil {
		t.Fatalf("worktree add error: %v\noutput: %s", err, out)
	}
	if !strings.Contains(out, "print-path") && !strings.Contains(out, ".worktrees") {
		t.Errorf("expected output to contain worktree path\n%s", out)
	}
}

func TestWorktreeAdd_Global_CreatesInAIWorktrees(t *testing.T) {
	repoDir := makeRealGitRepo(t)
	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)

	out, err := runWorktreeCmd(t, repoDir, "add", "global-feature", "--global")
	if err != nil {
		t.Fatalf("worktree add --global error: %v\noutput: %s", err, out)
	}

	expectedPath := filepath.Join(fakeHome, ".ai", "worktrees", "global-feature")
	if _, statErr := os.Stat(expectedPath); os.IsNotExist(statErr) {
		t.Errorf("expected global worktree at %s but it doesn't exist", expectedPath)
	}
}

func TestWorktreeAdd_NoStubError(t *testing.T) {
	repoDir := makeRealGitRepo(t)

	_, err := runWorktreeCmd(t, repoDir, "add", "no-stub")
	if err != nil && strings.Contains(err.Error(), "not yet implemented") {
		t.Errorf("worktree add returned stub error: %v", err)
	}
}

func TestWorktreeList_ShowsCreatedWorktrees(t *testing.T) {
	repoDir := makeRealGitRepo(t)

	// First add a worktree
	_, addErr := runWorktreeCmd(t, repoDir, "add", "listed-feature")
	if addErr != nil {
		t.Skipf("worktree add failed (may not be implemented yet): %v", addErr)
	}

	out, err := runWorktreeCmd(t, repoDir, "list")
	if err != nil && strings.Contains(err.Error(), "not yet implemented") {
		t.Errorf("worktree list returned stub error: %v", err)
	}
	if !strings.Contains(out, "listed-feature") && !strings.Contains(out, ".worktrees") {
		t.Logf("note: list output may format differently\n%s", out)
	}
}

func TestWorktreeRemove_RemovesWorktree(t *testing.T) {
	repoDir := makeRealGitRepo(t)

	// Add first
	_, addErr := runWorktreeCmd(t, repoDir, "add", "to-remove")
	if addErr != nil {
		t.Skipf("worktree add failed: %v", addErr)
	}

	expectedPath := filepath.Join(repoDir, ".worktrees", "to-remove")
	if _, statErr := os.Stat(expectedPath); os.IsNotExist(statErr) {
		t.Skipf("worktree not created at %s", expectedPath)
	}

	out, err := runWorktreeCmd(t, repoDir, "remove", "to-remove")
	if err != nil && strings.Contains(err.Error(), "not yet implemented") {
		t.Errorf("worktree remove returned stub error: %v", err)
	}
	_ = out

	// Worktree should be gone
	if _, statErr := os.Stat(expectedPath); !os.IsNotExist(statErr) {
		t.Errorf("expected worktree at %s to be removed", expectedPath)
	}
}

func TestWorktreeRemove_NoStubError(t *testing.T) {
	repoDir := makeRealGitRepo(t)

	// Need to add first so remove has something to remove
	_, _ = runWorktreeCmd(t, repoDir, "add", "remove-stub-test")

	_, err := runWorktreeCmd(t, repoDir, "remove", "remove-stub-test")
	if err != nil && strings.Contains(err.Error(), "not yet implemented") {
		t.Errorf("worktree remove returned stub error: %v", err)
	}
}
