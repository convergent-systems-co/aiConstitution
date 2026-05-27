package cmd

// update_test.go — TDD suite for `ai update` (no flags). Issue #346.
//
// Dependency injection: tests replace the package-level runGitUpdate and
// githubLatestRelease vars (same seam pattern as sync_test.go), so no
// real git binary or network is required.

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// withFakeUpdateGit replaces runGitUpdate for the test duration.
// Returns a pointer to the recorded calls slice.
func withFakeUpdateGit(t *testing.T, returnErr error) *[][]string {
	t.Helper()
	calls := [][]string{}
	orig := runGitUpdate
	t.Cleanup(func() { runGitUpdate = orig })
	runGitUpdate = func(_ string, args ...string) error {
		calls = append(calls, args)
		return returnErr
	}
	return &calls
}

// withFakeGitHubRelease replaces githubLatestRelease for the test duration.
func withFakeGitHubRelease(t *testing.T, tag string, err error) {
	t.Helper()
	orig := githubLatestRelease
	t.Cleanup(func() { githubLatestRelease = orig })
	githubLatestRelease = func() (string, error) { return tag, err }
}

// runUpdateCmd is a convenience wrapper: creates the command, wires out/err
// to a buffer, sets args, executes, and returns (output, error).
func runUpdateCmd(t *testing.T, args ...string) (string, error) {
	t.Helper()
	buf := &bytes.Buffer{}
	c := newUpdateCmd()
	c.SetOut(buf)
	c.SetErr(buf)
	c.SetArgs(args)
	err := c.Execute()
	return buf.String(), err
}

// ---- tests ------------------------------------------------------------------

// TestUpdate_NoGitRepo verifies that when ~/.ai/ is not a git repository
// (no .git subdirectory), the command prints the "not a git repo" notice
// and does NOT invoke git. The version check path also runs; we stub it
// to return a non-fatal network error so it does not mask the real signal.
func TestUpdate_NoGitRepo(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("AI_ROOT", tmp)

	calls := withFakeUpdateGit(t, nil)
	withFakeGitHubRelease(t, "", errors.New("simulated network error"))

	out, err := runUpdateCmd(t)
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
	if len(*calls) != 0 {
		t.Errorf("expected zero git calls when not a git repo; got %d: %v", len(*calls), *calls)
	}
	if !strings.Contains(out, "not a git repo") {
		t.Errorf("expected output to mention 'not a git repo'; got:\n%s", out)
	}
}

// TestUpdate_GitPullFails verifies that when git pull returns a non-zero
// exit, the command surfaces the error (non-nil return from RunE).
func TestUpdate_GitPullFails(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("AI_ROOT", tmp)
	// Create .git dir so the "is git repo" check passes.
	if err := os.MkdirAll(filepath.Join(tmp, ".git"), 0o700); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}

	withFakeUpdateGit(t, errors.New("exit status 1"))
	// GitHub check should not be reached on pull failure; stub it anyway.
	withFakeGitHubRelease(t, "v9.9.9", nil)

	_, err := runUpdateCmd(t)
	if err == nil {
		t.Fatal("expected non-nil error when git pull fails, got nil")
	}
	if !strings.Contains(err.Error(), "git pull failed") {
		t.Errorf("expected error to mention 'git pull failed'; got: %v", err)
	}
}

// TestUpdate_VersionUpToDate verifies that when the GitHub API returns the
// same tag as the current binary, the output contains "up to date".
func TestUpdate_VersionUpToDate(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("AI_ROOT", tmp)
	// Not a git repo — skip the pull so this test is focused on the version check.
	withFakeUpdateGit(t, nil)
	// Return the same version that buildinfo.Raw() returns (dev build).
	withFakeGitHubRelease(t, "v0.8.0-dev", nil)

	out, err := runUpdateCmd(t)
	if err != nil {
		t.Fatalf("expected nil error; got: %v", err)
	}
	if !strings.Contains(out, "up to date") {
		t.Errorf("expected 'up to date' in output; got:\n%s", out)
	}
}

// TestUpdate_NewVersionAvailable verifies that when the GitHub API returns
// a tag newer than the current binary, the output contains the new version
// tag and upgrade instructions.
func TestUpdate_NewVersionAvailable(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("AI_ROOT", tmp)
	withFakeUpdateGit(t, nil)
	withFakeGitHubRelease(t, "v99.0.0", nil)

	out, err := runUpdateCmd(t)
	if err != nil {
		t.Fatalf("expected nil error; got: %v", err)
	}
	if !strings.Contains(out, "v99.0.0") {
		t.Errorf("expected output to contain the new version tag 'v99.0.0'; got:\n%s", out)
	}
	if !strings.Contains(out, "brew upgrade") && !strings.Contains(out, "go install") {
		t.Errorf("expected upgrade instructions (brew upgrade / go install) in output; got:\n%s", out)
	}
}
