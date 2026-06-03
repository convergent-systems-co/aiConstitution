package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func main() {
	branch := flag.String("branch", "", "branch/worktree name")
	flag.Parse()

	if err := run(*branch); err != nil {
		fmt.Fprintf(os.Stderr, "create-worktree: %v\n", err)
		os.Exit(1)
	}
}

func run(branch string) error {
	branch = strings.TrimSpace(branch)
	if branch == "" {
		return fmt.Errorf("BRANCH is required: make worktree BRANCH=feature-name")
	}
	if strings.Contains(branch, "..") || filepath.IsAbs(branch) {
		return fmt.Errorf("branch name must be relative and must not contain '..'")
	}

	if err := runGuard("worktree", "add"); err != nil {
		return err
	}

	path := filepath.Join(".worktrees", filepath.FromSlash(branch))
	cmd := exec.Command("git", "worktree", "add", path, "-b", branch, "main")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func runGuard(args ...string) error {
	guardArgs := append([]string{"run", "./src/cmd/ai", "guard", "--git"}, args...)
	cmd := exec.Command("go", guardArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = append(os.Environ(), "AI_CONSTITUTION_APPROVAL_SCOPE=git:worktree:add")
	return cmd.Run()
}
