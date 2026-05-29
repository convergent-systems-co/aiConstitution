package cmd_test

import (
	"testing"

	cmd "github.com/convergent-systems-co/aiConstitution/src/cmd/ai/cmd"
)

func TestHookSlug(t *testing.T) {
	t.Parallel()
	cases := []struct{ in, want string }{
		{"~/.ai/hooks/branch-guard.py", "branch-guard"},
		{"~/.ai/hooks/audit-command.py", "audit-command"},
		{"/abs/path/secret-precommit.py", "secret-precommit"},
		{"no-verify-strip.py", "no-verify-strip"},
	}
	for _, c := range cases {
		if got := cmd.HookSlugForTest(c.in); got != c.want {
			t.Errorf("hookSlug(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestHookApplies_EmptySubcmds(t *testing.T) {
	t.Parallel()
	h := cmd.NewHookDefForTest("audit.py", nil, nil)
	for _, subCmd := range []string{"commit", "push", "merge", ""} {
		if !cmd.HookAppliesForTest(h, subCmd) {
			t.Errorf("hook with empty subcommands should apply to %q", subCmd)
		}
	}
}

func TestHookApplies_Matching(t *testing.T) {
	t.Parallel()
	h := cmd.NewHookDefForTest("branch-guard.py", []string{"commit", "merge", "push"}, nil)
	for _, subCmd := range []string{"commit", "merge", "push"} {
		if !cmd.HookAppliesForTest(h, subCmd) {
			t.Errorf("hook should apply to %q", subCmd)
		}
	}
	for _, subCmd := range []string{"status", "log", "diff", ""} {
		if cmd.HookAppliesForTest(h, subCmd) {
			t.Errorf("hook should NOT apply to %q", subCmd)
		}
	}
}

func TestHookApplies_MultiWordSubcommand(t *testing.T) {
	t.Parallel()
	// "repo delete" in subcommands should match on the first word "repo"
	h := cmd.NewHookDefForTest("destructive-gh-guard.py", []string{"repo delete", "release delete"}, nil)
	if !cmd.HookAppliesForTest(h, "repo") {
		t.Error("hook should apply to 'repo' (matches 'repo delete' on first word)")
	}
	if !cmd.HookAppliesForTest(h, "release") {
		t.Error("hook should apply to 'release' (matches 'release delete' on first word)")
	}
	if cmd.HookAppliesForTest(h, "delete") {
		t.Error("hook should NOT apply to bare 'delete'")
	}
}

func TestApplyStripArgs(t *testing.T) {
	t.Parallel()
	args := []string{"commit", "--no-verify", "-m", "msg", "-n"}
	strip := []string{"--no-verify", "-n"}
	got := cmd.ApplyStripArgsForTest(args, strip)
	want := []string{"commit", "-m", "msg"}
	if len(got) != len(want) {
		t.Fatalf("applyStripArgs = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("applyStripArgs[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestApplyStripArgs_NoStrip(t *testing.T) {
	t.Parallel()
	args := []string{"push", "origin", "main"}
	got := cmd.ApplyStripArgsForTest(args, nil)
	if len(got) != len(args) {
		t.Fatalf("applyStripArgs with nil strip should return input unchanged, got %v", got)
	}
	for i, a := range args {
		if got[i] != a {
			t.Errorf("applyStripArgs[%d] = %q, want %q", i, got[i], a)
		}
	}
}

func TestApplyStripArgs_EmptyInput(t *testing.T) {
	t.Parallel()
	got := cmd.ApplyStripArgsForTest(nil, []string{"--no-verify"})
	if len(got) != 0 {
		t.Errorf("applyStripArgs on nil input should return empty, got %v", got)
	}
}

func TestFindRealBinary_OverrideUsed(t *testing.T) {
	t.Parallel()
	// When realCommandOverride is non-empty, findRealBinary returns it directly
	// without a PATH search.
	got, err := cmd.FindRealBinaryForTest("git", "/usr/bin/git")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "/usr/bin/git" {
		t.Errorf("FindRealBinary with override = %q, want /usr/bin/git", got)
	}
}

func TestFindRealBinary_FindsOnPath(t *testing.T) {
	t.Parallel()
	// "true" is universally available on POSIX and we're running on macOS/Linux CI.
	got, err := cmd.FindRealBinaryForTest("true", "")
	if err != nil {
		t.Fatalf("FindRealBinary(\"true\") failed: %v", err)
	}
	if got == "" {
		t.Error("FindRealBinary(\"true\") returned empty path")
	}
}
