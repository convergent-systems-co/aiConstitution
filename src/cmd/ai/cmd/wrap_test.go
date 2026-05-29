package cmd_test

import (
	"os"
	"path/filepath"
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

// --- Enforcement tests ---

// TestRunHookForWrap_BlockingMissingHook verifies that when a blocking hook
// file is not installed, runHookForWrap returns 1 (not 0).
func TestRunHookForWrap_BlockingMissingHook(t *testing.T) {
	s := sandbox(t)
	_ = os.MkdirAll(filepath.Join(s.AIRoot, "hooks"), 0o755)
	code := cmd.RunHookForWrapForTest("branch-guard", nil, nil, true)
	if code != 1 {
		t.Errorf("blocking missing hook: want exit 1, got %d", code)
	}
}

// TestRunHookForWrap_AdvisoryMissingHook verifies that when an advisory hook
// file is not installed, runHookForWrap returns 0 (skip silently).
func TestRunHookForWrap_AdvisoryMissingHook(t *testing.T) {
	s := sandbox(t)
	_ = os.MkdirAll(filepath.Join(s.AIRoot, "hooks"), 0o755)
	code := cmd.RunHookForWrapForTest("worktree-guard", nil, nil, false)
	if code != 0 {
		t.Errorf("advisory missing hook: want exit 0, got %d", code)
	}
}

// TestHookDef_IsBlocking verifies the isBlocking() semantics via the
// NewHookDefForTest constructor (enforcement is the fourth field).
func TestHookDef_IsBlocking(t *testing.T) {
	t.Parallel()
	cases := []struct {
		enforcement string
		want        bool
	}{
		{"", true},          // default: blocking
		{"blocking", false}, // explicit — but isBlocking checks != "advisory", so "blocking" is also true
		{"advisory", false},
	}
	// Re-derive: isBlocking returns true when enforcement != "advisory"
	for _, c := range cases {
		got := cmd.IsBlockingForTest(c.enforcement)
		wantBlocking := c.enforcement != "advisory"
		if got != wantBlocking {
			t.Errorf("isBlocking(%q) = %v, want %v", c.enforcement, got, wantBlocking)
		}
	}
}

// --- Config error test ---

// TestLoadCommandWrappers_CorruptTOML verifies that a corrupt TOML on disk
// returns an error (so the caller can fail closed rather than pass through).
func TestLoadCommandWrappers_CorruptTOML(t *testing.T) {
	s := sandbox(t)
	hooksDir := filepath.Join(s.AIRoot, "hooks")
	if err := os.MkdirAll(hooksDir, 0o755); err != nil {
		t.Fatal(err)
	}
	corrupt := []byte("not [ valid toml ][[[")
	if err := os.WriteFile(filepath.Join(hooksDir, "command-wrappers.toml"), corrupt, 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := cmd.LoadCommandWrappersForTest()
	if err == nil {
		t.Errorf("expected error for corrupt TOML, got config: %+v", cfg)
	}
}
