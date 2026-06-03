package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/convergent-systems-co/aiConstitution/src/internal/audit"
)

func TestGuardProtectedBranch(t *testing.T) {
	for _, branch := range []string{"main", "master", "release/2026.06"} {
		if !guardProtectedBranch(branch) {
			t.Fatalf("expected %q to be protected", branch)
		}
	}
	if guardProtectedBranch("feature/codex-parity") {
		t.Fatal("feature branch should not be protected")
	}
}

func TestGuardBranchPolicyAllowsApprovedWorktreeAddFromProtectedBranch(t *testing.T) {
	policy := defaultGuardBranchPolicy
	if !policy.isProtectedBranch("main") {
		t.Fatal("main should be protected")
	}
	if !policy.allowsProtectedBranchGit([]string{"worktree", "add", ".worktrees/example", "-b", "example", "main"}) {
		t.Fatal("worktree add should be the protected-branch escape hatch")
	}
}

func TestGuardBranchPolicyBlocksOtherProtectedBranchGitMutations(t *testing.T) {
	policy := defaultGuardBranchPolicy
	blocked := [][]string{
		{"commit"},
		{"merge", "feature"},
		{"worktree", "remove", ".worktrees/example"},
		{"push", "origin", "main"},
	}
	for _, args := range blocked {
		if policy.allowsProtectedBranchGit(args) {
			t.Fatalf("protected branch policy should not allow git args %v", args)
		}
	}
}

func TestGuardRemoteRefName(t *testing.T) {
	cases := map[string]string{
		"main":                    "main",
		"refs/heads/main":         "main",
		"feature/x:main":          "main",
		"feature/x:release/2026":  "release/2026",
		"refs/heads/feature/test": "feature/test",
	}
	for input, want := range cases {
		if got := guardRemoteRefName(input); got != want {
			t.Fatalf("guardRemoteRefName(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestGuardPushBlocksProtectedRef(t *testing.T) {
	if err := guardPush([]string{"origin", "feature/x:main"}); err == nil {
		t.Fatal("expected protected push target to be blocked")
	}
}

func TestGuardPushAllowsFeatureRef(t *testing.T) {
	if err := guardPush([]string{"origin", "feature/x"}); err != nil {
		t.Fatalf("expected feature push target to be allowed: %v", err)
	}
}

func TestGuardPrePushInputBlocksProtectedRemoteRef(t *testing.T) {
	input := "refs/heads/feature abc123 refs/heads/main def456\n"
	if err := guardPrePushInput(input); err == nil {
		t.Fatal("expected protected remote ref to be blocked")
	}
}

func TestGuardPrePushInputAllowsFeatureRemoteRef(t *testing.T) {
	input := "refs/heads/feature abc123 refs/heads/feature def456\n"
	if err := guardPrePushInput(input); err != nil {
		t.Fatalf("expected feature remote ref to be allowed: %v", err)
	}
}

func TestGuardCommandPrintsErrorPrefix(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := runGuard([]string{"--git", "commit"}, &stdout, &stderr, strings.NewReader(""))
	if err == nil {
		t.Fatal("expected guard to block unapproved commit")
	}
	if !strings.Contains(stderr.String(), "constitution-guard:") {
		t.Fatalf("expected guard error prefix, got %q", stderr.String())
	}
}

func TestGuardCobraAcceptsGitModeArgument(t *testing.T) {
	cmd := newGuardCmd()
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetIn(strings.NewReader(""))
	cmd.SetArgs([]string{"--git", "commit"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected guard to block unapproved commit")
	}
	if strings.Contains(err.Error(), "unknown flag") {
		t.Fatalf("guard should treat --git as a mode argument, got %v", err)
	}
}

func TestGuardRequiresScopedApproval(t *testing.T) {
	t.Setenv("AI_CONSTITUTION_APPROVED_MUTATION", "1")
	if err := guardGitCommand([]string{"worktree", "add"}); err == nil {
		t.Fatal("legacy boolean approval env should not authorize mutation")
	}
}

func TestGuardScopedApprovalRecordsAudit(t *testing.T) {
	t.Setenv(guardApprovalScopeEnv, "git:worktree:add")
	var got []audit.Event
	old := guardAppendAuditEvent
	guardAppendAuditEvent = func(e audit.Event) error {
		got = append(got, e)
		return nil
	}
	t.Cleanup(func() { guardAppendAuditEvent = old })

	if err := guardGitCommand([]string{"worktree", "add"}); err != nil {
		t.Fatalf("scoped approval should authorize worktree add: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected one audit event, got %d", len(got))
	}
	if got[0].Probe != "git:worktree:add" {
		t.Fatalf("audit probe = %q, want git:worktree:add", got[0].Probe)
	}
	if got[0].Kind != audit.KindSignal {
		t.Fatalf("audit kind = %q, want %q", got[0].Kind, audit.KindSignal)
	}
}
