package cmd

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"time"

	"github.com/convergent-systems-co/aiConstitution/src/internal/paths"

	"github.com/spf13/cobra"
)

// runGit is the package-level shell-out helper for sync. Tests
// substitute a fake so the assertions can check the recorded calls
// without invoking the real git binary. Output is streamed to the
// caller's stdout/stderr.
var runGit = func(dir string, args ...string) error {
	return runGitTo(os.Stdout, os.Stderr, dir, args...)
}

// runGitQuiet runs a git command whose exit code carries information
// (e.g., `git diff --cached --quiet` returns 0 when nothing is staged,
// 1 when there are staged changes). Returns (cleanExit, err) where
// cleanExit==true means exit code 0 and err is non-nil only for
// failures other than a non-zero exit.
var runGitQuiet = func(dir string, args ...string) (bool, error) {
	cmd := exec.Command("git", args...) //nolint:gosec // G204: args come from CLI; dir from paths
	cmd.Dir = dir
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			// Non-zero exit is the *signal* this helper reports, not an error.
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// runGitTo is the underlying exec wrapper used by the default runGit.
// Kept separate so the test seam can swap runGit without needing to
// model stdout/stderr plumbing.
func runGitTo(stdout, stderr io.Writer, dir string, args ...string) error {
	cmd := exec.Command("git", args...) //nolint:gosec // G204: args come from CLI; dir from paths
	cmd.Dir = dir
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git %v: %w", args, err)
	}
	return nil
}

// syncRemote returns the configured sync remote. Reads AI_SYNC_REMOTE
// first; falls back to "origin". The settings.toml [sync].remote key
// will subsume this in a later release (per SPEC.md §12).
func syncRemote() string {
	if env := os.Getenv("AI_SYNC_REMOTE"); env != "" {
		return env
	}
	return "origin"
}

// newSyncCmd implements `ai sync {push,pull,status}`. See SPEC.md §3.4
// and §12.
func newSyncCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "sync",
		Short: "Push or pull the canonical tree to a user-owned remote",
		Long: `sync moves the canonical tree (memories, audit overrides, audit
violations, governance, hooks, settings.toml — never raw interaction
JSONL, never secrets) between this machine and a user-owned remote.

See SPEC.md §3.4 + §12.`,
	}

	var pushRemote string
	var pushForce bool
	push := &cobra.Command{
		Use:   "push",
		Short: "Push the canonical tree to the configured (or specified) remote",
		RunE: func(cmd *cobra.Command, _ []string) error {
			remote := pushRemote
			if remote == "" {
				remote = syncRemote()
			}
			_ = pushForce // reserved for a future protected-branch override
			return doSyncPush(cmd.OutOrStdout(), remote)
		},
	}
	push.Flags().StringVar(&pushRemote, "remote", "", "override the configured sync remote")
	push.Flags().BoolVar(&pushForce, "force", false, "force-push (gated; refuses on protected branch)")

	var pullRemote string
	pull := &cobra.Command{
		Use:   "pull",
		Short: "Pull the canonical tree from the configured (or specified) remote",
		RunE: func(cmd *cobra.Command, _ []string) error {
			remote := pullRemote
			if remote == "" {
				remote = syncRemote()
			}
			return doSyncPull(cmd.OutOrStdout(), remote)
		},
	}
	pull.Flags().StringVar(&pullRemote, "remote", "", "override the configured sync remote")

	status := &cobra.Command{
		Use:   "status",
		Short: "Show sync state: configured remote, last push, last pull, dirty count",
		RunE: func(cmd *cobra.Command, _ []string) error {
			notice("sync status:", "would report configured remote + ahead/behind counts.")
			return stub("sync status", "§12")
		},
	}

	c.AddCommand(push, pull, status)
	return c
}

// doSyncPush implements the push pipeline. Each step is a separate
// runGit call so the test seam can verify the ordering without
// re-deriving it from shell strings.
func doSyncPush(_ io.Writer, remote string) error {
	dir := paths.AIRoot()

	if err := runGit(dir, "add", "-A"); err != nil {
		return err
	}

	// `git diff --cached --quiet` exits 0 when nothing is staged.
	// We only commit when there IS something staged (clean exit == false).
	clean, err := runGitQuiet(dir, "diff", "--cached", "--quiet")
	if err != nil {
		return err
	}
	if !clean {
		msg := "chore: sync push " + time.Now().UTC().Format(time.RFC3339)
		if err := runGit(dir, "commit", "-m", msg); err != nil {
			return err
		}
	}

	return runGit(dir, "push", remote, "HEAD:main")
}

// doSyncPull runs `git pull <remote> main` in the canonical tree.
func doSyncPull(_ io.Writer, remote string) error {
	dir := paths.AIRoot()
	return runGit(dir, "pull", remote, "main")
}
