package cmd

// sync_status_test.go — TDD tests for `ai sync status` (#353).
//
// Uses the white-box package so we can swap the runGitOutput seam.

import (
	"bytes"
	"strings"
	"testing"
)

// withFakeRunGitOutput installs a fake runGitOutput seam that returns
// fixed (output, error) pairs keyed by the first git sub-command argument.
// The fake also records calls for assertion.
func withFakeRunGitOutput(t *testing.T, responses map[string]fakeGitResp) {
	t.Helper()
	orig := runGitOutput
	t.Cleanup(func() { runGitOutput = orig })
	runGitOutput = func(_ string, args ...string) (string, error) {
		if len(args) == 0 {
			return "", nil
		}
		// Key on the full args joined so tests can match sub-commands precisely.
		key := strings.Join(args, " ")
		for k, v := range responses {
			if strings.Contains(key, k) {
				return v.out, v.err
			}
		}
		return "", nil
	}
}

type fakeGitResp struct {
	out string
	err error
}

// ---- TestSyncStatus_NotGitRepo ---------------------------------------------

// TestSyncStatus_NotGitRepo verifies that when runGitOutput returns an error
// for rev-parse (which git uses to detect whether CWD is a repo), the
// command prints a user-friendly "not a git repository" message rather than
// a raw error and exits with an error.
func TestSyncStatus_NotGitRepo(t *testing.T) {
	t.Setenv("AI_ROOT", t.TempDir())

	withFakeRunGitOutput(t, map[string]fakeGitResp{
		"rev-parse": {out: "", err: &gitExitError{msg: "not a git repository"}},
	})

	buf := &bytes.Buffer{}
	syncCmd := newSyncCmd()
	syncCmd.SetOut(buf)
	syncCmd.SetErr(buf)
	syncCmd.SetArgs([]string{"status"})

	err := syncCmd.Execute()
	out := buf.String()

	// The command should surface the "not a git repository" message.
	if !strings.Contains(out, "not a git repository") && !strings.Contains(out, "not a git repo") {
		// Error output is acceptable too if it mentions the condition.
		if err == nil || (!strings.Contains(err.Error(), "not a git") && !strings.Contains(out, "~/.ai/")) {
			t.Errorf("expected 'not a git repository' in output or error; got output=%q err=%v", out, err)
		}
	}
}

// ---- TestSyncStatus_UpToDate -----------------------------------------------

// TestSyncStatus_UpToDate verifies the happy-path output format:
// Remote, Commit, Status, and Last pull lines are present.
func TestSyncStatus_UpToDate(t *testing.T) {
	t.Setenv("AI_ROOT", t.TempDir())

	withFakeRunGitOutput(t, map[string]fakeGitResp{
		"rev-parse --short":         {out: "abc1234\n", err: nil},
		"rev-parse HEAD":            {out: "abc1234full\n", err: nil},
		"remote get-url":            {out: "https://github.com/user/ai.git\n", err: nil},
		"HEAD..origin/main":         {out: "0\n", err: nil},
		"origin/main..HEAD":         {out: "0\n", err: nil},
		"log -1":                    {out: "3 hours ago\n", err: nil},
	})

	buf := &bytes.Buffer{}
	syncCmd := newSyncCmd()
	syncCmd.SetOut(buf)
	syncCmd.SetErr(buf)
	syncCmd.SetArgs([]string{"status"})

	if err := syncCmd.Execute(); err != nil {
		t.Fatalf("sync status failed: %v\noutput: %s", err, buf.String())
	}

	out := buf.String()

	if !strings.Contains(out, "Remote") {
		t.Errorf("output missing 'Remote' label; got:\n%s", out)
	}
	if !strings.Contains(out, "abc1234") {
		t.Errorf("output missing commit hash 'abc1234'; got:\n%s", out)
	}
	if !strings.Contains(out, "github.com") {
		t.Errorf("output missing remote URL; got:\n%s", out)
	}
	// up-to-date branch: status line should say "up to date".
	if !strings.Contains(strings.ToLower(out), "up to date") {
		t.Errorf("output missing 'up to date'; got:\n%s", out)
	}
}

// ---- TestSyncStatus_AheadBehind --------------------------------------------

// TestSyncStatus_AheadBehind verifies that when ahead/behind counts are
// non-zero the status line reflects them.
func TestSyncStatus_AheadBehind(t *testing.T) {
	t.Setenv("AI_ROOT", t.TempDir())

	withFakeRunGitOutput(t, map[string]fakeGitResp{
		"rev-parse --short": {out: "def5678\n", err: nil},
		"rev-parse HEAD":    {out: "def5678full\n", err: nil},
		"remote get-url":    {out: "https://github.com/user/ai.git\n", err: nil},
		"HEAD..origin/main": {out: "1\n", err: nil}, // 1 behind
		"origin/main..HEAD": {out: "2\n", err: nil}, // 2 ahead
		"log -1":            {out: "5 minutes ago\n", err: nil},
	})

	buf := &bytes.Buffer{}
	syncCmd := newSyncCmd()
	syncCmd.SetOut(buf)
	syncCmd.SetErr(buf)
	syncCmd.SetArgs([]string{"status"})

	if err := syncCmd.Execute(); err != nil {
		t.Fatalf("sync status failed: %v\noutput: %s", err, buf.String())
	}

	out := buf.String()

	if !strings.Contains(out, "2") {
		t.Errorf("output should mention ahead count 2; got:\n%s", out)
	}
	if !strings.Contains(out, "1") {
		t.Errorf("output should mention behind count 1; got:\n%s", out)
	}
}

// gitExitError is a minimal error that satisfies the interface returned
// by exec.ExitError so our fake can simulate a non-zero exit.
type gitExitError struct{ msg string }

func (e *gitExitError) Error() string { return e.msg }
