package cmd

import (
	"bytes"
	"errors"
	"testing"
)

// syncCall captures one invocation of the git runner so tests can
// assert on the directory + args without actually shelling out.
type syncCall struct {
	dir  string
	args []string
}

// withFakeRunGit replaces the package-level runGit / runGitOutput hooks
// for the duration of a test, restoring the originals on cleanup.
// nothingStagedExitCode toggles whether `git diff --cached --quiet`
// reports clean (exit 0 == nothing staged) or dirty (exit 1).
func withFakeRunGit(t *testing.T, nothingStaged bool) *[]syncCall {
	t.Helper()
	calls := []syncCall{}
	origRun := runGit
	origQuery := runGitQuiet
	t.Cleanup(func() {
		runGit = origRun
		runGitQuiet = origQuery
	})
	runGit = func(dir string, args ...string) error {
		calls = append(calls, syncCall{dir: dir, args: args})
		return nil
	}
	runGitQuiet = func(dir string, args ...string) (cleanExit bool, err error) {
		calls = append(calls, syncCall{dir: dir, args: args})
		return nothingStaged, nil
	}
	return &calls
}

func runSync(t *testing.T, args ...string) string {
	t.Helper()
	buf := &bytes.Buffer{}
	cmd := newSyncCmd()
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs(args)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("sync %v: %v", args, err)
	}
	return buf.String()
}

func TestSyncPushWithStagedChangesRunsAddCommitPush(t *testing.T) {
	t.Setenv("AI_ROOT", t.TempDir())
	t.Setenv("AI_SYNC_REMOTE", "origin")
	calls := withFakeRunGit(t, false /* not nothingStaged */)

	_ = runSync(t, "push")

	if len(*calls) < 4 {
		t.Fatalf("want >=4 git invocations (add, diff, commit, push); got %d: %+v", len(*calls), *calls)
	}
	if (*calls)[0].args[0] != "add" {
		t.Errorf("call 0 want add, got %v", (*calls)[0].args)
	}
	if (*calls)[1].args[0] != "diff" {
		t.Errorf("call 1 want diff, got %v", (*calls)[1].args)
	}
	if (*calls)[2].args[0] != "commit" {
		t.Errorf("call 2 want commit, got %v", (*calls)[2].args)
	}
	last := (*calls)[len(*calls)-1]
	if last.args[0] != "push" {
		t.Errorf("last call want push, got %v", last.args)
	}
}

func TestSyncPushSkipsCommitWhenNothingStaged(t *testing.T) {
	t.Setenv("AI_ROOT", t.TempDir())
	t.Setenv("AI_SYNC_REMOTE", "origin")
	calls := withFakeRunGit(t, true /* nothingStaged */)

	_ = runSync(t, "push")

	// No call should be `commit`.
	for _, c := range *calls {
		if len(c.args) > 0 && c.args[0] == "commit" {
			t.Errorf("expected no commit when nothing staged, got call %v", c.args)
		}
	}
	// Push must still happen.
	pushSeen := false
	for _, c := range *calls {
		if len(c.args) > 0 && c.args[0] == "push" {
			pushSeen = true
		}
	}
	if !pushSeen {
		t.Errorf("expected push call even when nothing staged; calls=%+v", *calls)
	}
}

func TestSyncPullRunsGitPull(t *testing.T) {
	t.Setenv("AI_ROOT", t.TempDir())
	t.Setenv("AI_SYNC_REMOTE", "origin")
	calls := withFakeRunGit(t, false)

	_ = runSync(t, "pull")

	if len(*calls) == 0 {
		t.Fatal("want at least 1 call")
	}
	c := (*calls)[0]
	if c.args[0] != "pull" {
		t.Errorf("want pull, got %v", c.args)
	}
	if c.args[1] != "origin" {
		t.Errorf("want remote=origin, got %v", c.args)
	}
}

func TestSyncRemoteHonorsEnvOverride(t *testing.T) {
	t.Setenv("AI_ROOT", t.TempDir())
	t.Setenv("AI_SYNC_REMOTE", "upstream")
	calls := withFakeRunGit(t, true)

	_ = runSync(t, "push")
	last := (*calls)[len(*calls)-1]
	if last.args[1] != "upstream" {
		t.Errorf("want push remote=upstream, got %v", last.args)
	}
}

func TestSyncPushSurfacesRunGitErrors(t *testing.T) {
	t.Setenv("AI_ROOT", t.TempDir())
	t.Setenv("AI_SYNC_REMOTE", "origin")
	origRun := runGit
	origQuery := runGitQuiet
	t.Cleanup(func() {
		runGit = origRun
		runGitQuiet = origQuery
	})
	want := errors.New("boom")
	runGit = func(_ string, _ ...string) error { return want }
	runGitQuiet = func(_ string, _ ...string) (bool, error) { return false, nil }

	buf := &bytes.Buffer{}
	cmd := newSyncCmd()
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"push"})
	if err := cmd.Execute(); err == nil {
		t.Errorf("want error from sync push, got nil")
	}
}
