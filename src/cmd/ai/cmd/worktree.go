package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

// newWorktreeCmd implements `ai worktree {add,remove,list}`.
// See ~/.ai/Common.md §U17 (Worktree placement) and §U17.5
// (preferred surface).
func newWorktreeCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "worktree",
		Short: "Create worktrees in the canonical locations (~/.ai/Common.md §U17)",
		Long: `worktree is the preferred surface for the §U17 lifecycle decision:

  Single-repo, dies-with-repo  → <repo>/.worktrees/<name>/
  Cross-repo or persistent      → ~/.ai/worktrees/<name>/

The CLI computes the canonical path automatically. Raw ` + "`" + `git worktree add` + "`" + `
remains permitted and is policed by ~/.ai/hooks/worktree-guard.py as
defense-in-depth.

See Common.md §U17.`,
	}

	var global bool
	add := &cobra.Command{
		Use:   "add <name>",
		Short: "Create a worktree at the canonical path",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runWorktreeAdd(cmd, args[0], global)
		},
	}
	add.Flags().BoolVar(&global, "global", false, "create under ~/.ai/worktrees/ (cross-repo) instead of <repo>/.worktrees/")

	remove := &cobra.Command{
		Use:   "remove <name>",
		Short: "Remove a canonically-placed worktree",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runWorktreeRemove(cmd, args[0])
		},
	}

	list := &cobra.Command{
		Use:   "list",
		Short: "List worktrees in both canonical roots",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runWorktreeList(cmd)
		},
	}

	c.AddCommand(add, remove, list)
	return c
}

// repoRoot returns the git repository root for the current working directory,
// or an error if not inside a git repo.
func repoRoot() (string, error) {
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return "", fmt.Errorf("not inside a git repository (git rev-parse failed): %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// runWorktreeAdd creates a worktree at the canonical path.
func runWorktreeAdd(cmd *cobra.Command, name string, global bool) error {
	out := cmd.OutOrStdout()
	var worktreePath string

	if global {
		// Cross-repo or persistent: ~/.ai/worktrees/<name>/
		home, err := os.UserHomeDir()
		if err != nil {
			home = os.Getenv("HOME")
		}
		worktreePath = filepath.Join(home, ".ai", "worktrees", name)
		if err := os.MkdirAll(filepath.Dir(worktreePath), 0o750); err != nil {
			return fmt.Errorf("worktree add: mkdir parent: %w", err)
		}
	} else {
		// Single-repo: <repo>/.worktrees/<name>/
		root, err := repoRoot()
		if err != nil {
			return fmt.Errorf("worktree add: %w", err)
		}
		worktreePath = filepath.Join(root, ".worktrees", name)
	}

	// Determine base branch: try main, fall back to HEAD
	baseBranch := "main"
	checkMain := exec.Command("git", "rev-parse", "--verify", "main")
	if err := checkMain.Run(); err != nil {
		baseBranch = "HEAD"
	}

	// Run: git worktree add <path> -b <name> <baseBranch>
	gitArgs := []string{"worktree", "add", worktreePath, "-b", name, baseBranch}
	gitCmd := exec.Command("git", gitArgs...)
	gitCmd.Stdout = out
	gitCmd.Stderr = cmd.ErrOrStderr()
	if err := gitCmd.Run(); err != nil {
		return fmt.Errorf("worktree add: git worktree add failed: %w", err)
	}

	fmt.Fprintf(out, "Created: %s\n", worktreePath)
	return nil
}

// runWorktreeList lists worktrees from both canonical roots using `git worktree list`.
func runWorktreeList(cmd *cobra.Command) error {
	out := cmd.OutOrStdout()

	// Use git worktree list --porcelain as the source of truth.
	gitOut, err := exec.Command("git", "worktree", "list", "--porcelain").Output()
	if err != nil {
		// Not in a git repo — try listing global worktrees only
		home, _ := os.UserHomeDir()
		if home == "" {
			home = os.Getenv("HOME")
		}
		globalDir := filepath.Join(home, ".ai", "worktrees")
		return listDirectoryWorktrees(out, globalDir)
	}

	// Parse porcelain output:
	// worktree /path
	// HEAD <sha>
	// branch refs/heads/<name>
	// (blank line separates entries)
	type worktreeEntry struct {
		path   string
		head   string
		branch string
	}
	var entries []worktreeEntry
	var current worktreeEntry

	lines := strings.Split(strings.TrimSpace(string(gitOut)), "\n")
	for _, line := range lines {
		switch {
		case strings.HasPrefix(line, "worktree "):
			current.path = strings.TrimPrefix(line, "worktree ")
		case strings.HasPrefix(line, "HEAD "):
			sha := strings.TrimPrefix(line, "HEAD ")
			if len(sha) > 8 {
				sha = sha[:8]
			}
			current.head = sha
		case strings.HasPrefix(line, "branch "):
			ref := strings.TrimPrefix(line, "branch ")
			ref = strings.TrimPrefix(ref, "refs/heads/")
			current.branch = ref
		case line == "":
			if current.path != "" {
				entries = append(entries, current)
				current = worktreeEntry{}
			}
		}
	}
	if current.path != "" {
		entries = append(entries, current)
	}

	for _, e := range entries {
		branch := e.branch
		if branch == "" {
			branch = "(detached)"
		}
		fmt.Fprintf(out, "%s  [%s]  %s\n", e.path, branch, e.head)
	}

	// Also list global worktrees not tracked by current repo's git
	home, _ := os.UserHomeDir()
	if home == "" {
		home = os.Getenv("HOME")
	}
	globalDir := filepath.Join(home, ".ai", "worktrees")
	if entries2, err := os.ReadDir(globalDir); err == nil {
		for _, e := range entries2 {
			if !e.IsDir() {
				continue
			}
			p := filepath.Join(globalDir, e.Name())
			// Check if already listed
			alreadyListed := false
			for _, listed := range entries {
				if listed.path == p {
					alreadyListed = true
					break
				}
			}
			if !alreadyListed {
				branch := readWorktreeBranch(p)
				sha := readWorktreeShortSHA(p)
				fmt.Fprintf(out, "%s  [%s]  %s\n", p, branch, sha)
			}
		}
	}

	return nil
}

// listDirectoryWorktrees lists worktrees from a directory (fallback for non-git cwd).
func listDirectoryWorktrees(out interface{ Write([]byte) (int, error) }, dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil // Directory doesn't exist; silent
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		p := filepath.Join(dir, e.Name())
		branch := readWorktreeBranch(p)
		sha := readWorktreeShortSHA(p)
		fmt.Fprintf(out, "%s  [%s]  %s\n", p, branch, sha)
	}
	return nil
}

// readWorktreeBranch reads the branch name from a worktree's HEAD file.
func readWorktreeBranch(worktreePath string) string {
	headFile := filepath.Join(worktreePath, ".git")
	// In a worktree, .git is a file containing "gitdir: /path/.git/worktrees/<name>"
	data, err := os.ReadFile(filepath.Clean(headFile))
	if err != nil {
		// Try reading .git/HEAD directly (main worktree)
		headFile = filepath.Join(worktreePath, ".git", "HEAD")
		data, err = os.ReadFile(filepath.Clean(headFile))
		if err != nil {
			return "(unknown)"
		}
		return parseBranchFromHEAD(string(data))
	}

	// .git file in worktree points to the gitdir
	gitdirLine := strings.TrimSpace(string(data))
	if !strings.HasPrefix(gitdirLine, "gitdir: ") {
		return "(unknown)"
	}
	gitdir := strings.TrimPrefix(gitdirLine, "gitdir: ")
	headPath := filepath.Join(gitdir, "HEAD")
	headData, err := os.ReadFile(filepath.Clean(headPath))
	if err != nil {
		return "(unknown)"
	}
	return parseBranchFromHEAD(string(headData))
}

func parseBranchFromHEAD(content string) string {
	line := strings.TrimSpace(content)
	if strings.HasPrefix(line, "ref: refs/heads/") {
		return strings.TrimPrefix(line, "ref: refs/heads/")
	}
	if len(line) >= 8 {
		return "(detached:" + line[:8] + ")"
	}
	return "(detached)"
}

// readWorktreeShortSHA returns the short HEAD commit SHA for a worktree.
func readWorktreeShortSHA(worktreePath string) string {
	out, err := exec.Command("git", "-C", worktreePath, "rev-parse", "--short", "HEAD").Output()
	if err != nil {
		return "(unknown)"
	}
	return strings.TrimSpace(string(out))
}

// runWorktreeRemove removes a worktree from either canonical location.
func runWorktreeRemove(cmd *cobra.Command, name string) error {
	out := cmd.OutOrStdout()

	// Search canonical locations: repo first, then global
	worktreePath, err := findCanonicalWorktree(name)
	if err != nil {
		return err
	}

	// Run: git worktree remove <path>
	removeCmd := exec.Command("git", "worktree", "remove", worktreePath)
	removeCmd.Stdout = out
	removeCmd.Stderr = cmd.ErrOrStderr()
	if err := removeCmd.Run(); err != nil {
		// Try with --force for clean worktrees that git refuses to remove
		forceCmd := exec.Command("git", "worktree", "remove", "--force", worktreePath)
		forceCmd.Stdout = out
		forceCmd.Stderr = cmd.ErrOrStderr()
		if err2 := forceCmd.Run(); err2 != nil {
			return fmt.Errorf("worktree remove: %w (also tried --force: %v)", err, err2)
		}
	}

	// Prune stale metadata
	pruneCmd := exec.Command("git", "worktree", "prune")
	_ = pruneCmd.Run() // Best-effort; ignore error

	fmt.Fprintf(out, "Removed: %s\n", worktreePath)
	return nil
}

// findCanonicalWorktree finds the absolute path for a worktree by name,
// checking repo .worktrees/ first, then ~/.ai/worktrees/.
func findCanonicalWorktree(name string) (string, error) {
	// Check repo .worktrees/<name>/
	if root, err := repoRoot(); err == nil {
		candidate := filepath.Join(root, ".worktrees", name)
		if _, statErr := os.Stat(candidate); statErr == nil {
			return candidate, nil
		}
	}

	// Check ~/.ai/worktrees/<name>/
	home, _ := os.UserHomeDir()
	if home == "" {
		home = os.Getenv("HOME")
	}
	candidate := filepath.Join(home, ".ai", "worktrees", name)
	if _, statErr := os.Stat(candidate); statErr == nil {
		return candidate, nil
	}

	return "", fmt.Errorf("worktree %q not found in <repo>/.worktrees/ or ~/.ai/worktrees/", name)
}
