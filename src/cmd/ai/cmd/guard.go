package cmd

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
)

type guardBranchPolicy struct {
	protectedNames map[string]bool
}

var defaultGuardBranchPolicy = guardBranchPolicy{
	protectedNames: map[string]bool{
		"main":   true,
		"master": true,
	},
}

func newGuardCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "guard [--state | --git <args...> | --git-hook <hook>]",
		Short: "Run repo-local constitution guard checks",
		Long: `guard is the installed runtime entrypoint for repo-managed Git hooks
and explicit governance checks. It avoids requiring Go in hook execution paths.`,
		DisableFlagParsing: true,
		SilenceUsage:       true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runGuard(args, cmd.OutOrStdout(), cmd.ErrOrStderr(), os.Stdin)
		},
	}
}

func runGuard(args []string, stdout, stderr io.Writer, stdin io.Reader) error {
	err := runGuardChecks(args, stdout, stdin)
	if err != nil {
		fmt.Fprintf(stderr, "constitution-guard: %v\n", err)
	}
	return err
}

func runGuardChecks(args []string, stdout io.Writer, stdin io.Reader) error {
	switch {
	case len(args) == 0:
		if err := printGuardState(stdout); err != nil {
			return err
		}
		return guardState(requireCleanTree)
	case args[0] == "--state":
		return printGuardState(stdout)
	case args[0] == "--git":
		gitArgs := args[1:]
		if err := guardGitState(gitArgs, requireCleanTree); err != nil {
			return err
		}
		return guardGitCommand(gitArgs)
	case args[0] == "--git-hook":
		return guardGitHook(args[1:], stdin)
	default:
		if err := guardGitState(args, requireCleanTree); err != nil {
			return err
		}
		return guardGitCommand(args)
	}
}

func printGuardState(w io.Writer) error {
	root, err := guardGitOutput("rev-parse", "--show-toplevel")
	if err != nil {
		return fmt.Errorf("not inside a git repository")
	}
	branch := guardCurrentBranch()
	fmt.Fprintf(w, "pwd: %s\n", guardGetwd())
	fmt.Fprintf(w, "branch: %s\n", branch)
	fmt.Fprintf(w, "worktree: %s\n", root)
	if guardDirty() {
		fmt.Fprintln(w, "dirty: yes")
	} else {
		fmt.Fprintln(w, "dirty: no")
	}
	return nil
}

type guardDirtyPolicy bool

const (
	allowDirtyTree   guardDirtyPolicy = false
	requireCleanTree guardDirtyPolicy = true
)

func guardState(policy guardDirtyPolicy) error {
	branch := guardCurrentBranch()
	if guardProtectedBranch(branch) {
		return fmt.Errorf("refusing edits or mutations on protected branch %q", branch)
	}
	if policy == requireCleanTree && guardDirty() && os.Getenv("AI_CONSTITUTION_DIRTY_ACK") != "1" {
		return fmt.Errorf("working tree is dirty; set AI_CONSTITUTION_DIRTY_ACK=1 only after explicitly acknowledging the dirty state")
	}
	return nil
}

func guardGitState(args []string, policy guardDirtyPolicy) error {
	branch := guardCurrentBranch()
	if guardProtectedBranch(branch) && !defaultGuardBranchPolicy.allowsProtectedBranchGit(args) {
		return fmt.Errorf("refusing edits or mutations on protected branch %q", branch)
	}
	if policy == requireCleanTree && guardDirty() && os.Getenv("AI_CONSTITUTION_DIRTY_ACK") != "1" {
		return fmt.Errorf("working tree is dirty; set AI_CONSTITUTION_DIRTY_ACK=1 only after explicitly acknowledging the dirty state")
	}
	return nil
}

func guardGitCommand(args []string) error {
	if len(args) == 0 {
		return nil
	}

	subcommand := args[0]
	switch subcommand {
	case "commit", "merge", "rebase", "cherry-pick", "revert", "am", "pull", "worktree":
		if os.Getenv("AI_CONSTITUTION_APPROVED_MUTATION") != "1" {
			return fmt.Errorf("git %s requires AI_CONSTITUTION_APPROVED_MUTATION=1 and an explicit approved entrypoint", subcommand)
		}
	case "push":
		if os.Getenv("AI_CONSTITUTION_APPROVED_MUTATION") != "1" {
			return fmt.Errorf("git push requires AI_CONSTITUTION_APPROVED_MUTATION=1 and an explicit approved entrypoint")
		}
		return guardPush(args[1:])
	}
	return nil
}

func guardGitHook(args []string, stdin io.Reader) error {
	if len(args) == 0 {
		return nil
	}
	switch args[0] {
	case "pre-commit", "commit-msg":
		return guardState(allowDirtyTree)
	case "pre-push":
		if err := guardState(allowDirtyTree); err != nil {
			return err
		}
		input, err := io.ReadAll(stdin)
		if err != nil {
			return fmt.Errorf("read pre-push stdin: %w", err)
		}
		if len(bytes.TrimSpace(input)) > 0 {
			return guardPrePushInput(string(input))
		}
		return guardPush(args[1:])
	default:
		return nil
	}
}

func guardPrePushInput(input string) error {
	for _, line := range strings.Split(input, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		remoteRef := fields[2]
		if guardProtectedBranch(guardRemoteRefName(remoteRef)) {
			return fmt.Errorf("refusing push targeting protected ref %q", remoteRef)
		}
	}
	return nil
}

func guardPush(args []string) error {
	if len(args) == 0 {
		branch := guardCurrentBranch()
		if guardProtectedBranch(branch) {
			return fmt.Errorf("refusing bare push from protected branch %q", branch)
		}
		return nil
	}

	for _, arg := range args {
		if strings.HasPrefix(arg, "-") || arg == "origin" || arg == "upstream" {
			continue
		}
		if guardProtectedBranch(guardRemoteRefName(arg)) {
			return fmt.Errorf("refusing push targeting protected ref %q", arg)
		}
	}
	return nil
}

func guardCurrentBranch() string {
	branch, err := guardGitOutput("symbolic-ref", "--quiet", "--short", "HEAD")
	if err != nil || branch == "" {
		return "DETACHED"
	}
	return branch
}

func guardDirty() bool {
	out, err := guardGitOutput("status", "--porcelain")
	return err == nil && out != ""
}

func guardProtectedBranch(branch string) bool {
	return defaultGuardBranchPolicy.isProtectedBranch(branch)
}

func (p guardBranchPolicy) isProtectedBranch(branch string) bool {
	return p.protectedNames[branch] || strings.HasPrefix(branch, "release/")
}

func (p guardBranchPolicy) allowsProtectedBranchGit(args []string) bool {
	if len(args) < 2 {
		return false
	}
	return args[0] == "worktree" && args[1] == "add"
}

func guardRemoteRefName(ref string) string {
	if strings.Contains(ref, ":") {
		parts := strings.Split(ref, ":")
		ref = parts[len(parts)-1]
	}
	return strings.TrimPrefix(ref, "refs/heads/")
}

func guardGitOutput(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			return "", err
		}
		return "", fmt.Errorf("%w: %s", err, msg)
	}
	return strings.TrimSpace(string(out)), nil
}

func guardGetwd() string {
	wd, err := os.Getwd()
	if err != nil {
		return "(unknown)"
	}
	return wd
}
