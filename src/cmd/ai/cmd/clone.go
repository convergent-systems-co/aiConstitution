package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

// newCloneCmd implements `ai clone <url> [<dir>]`. Replaces the
// previous bin/clone shell wrapper with a Go subcommand per
// SPEC.md §6 (Q05/Q35) + Common.md §4.7 (per-repo credential
// helpers).
//
// Identity routing against metadata/projects.json is stubbed in v0.8
// (morning work). The post-clone secret-precommit hook install is
// active when ~/.ai/hooks/secret-precommit.py is present.
func newCloneCmd() *cobra.Command {
	var identity string
	var noPrecommit bool

	c := &cobra.Command{
		Use:   "clone <url> [<dir>]",
		Short: "Clone a repo with identity routing + post-clone hook install",
		Long: `clone runs `+"`"+`git clone`+"`"+` (intentionally not bypassing the
~/.ai/bin/git wrapper) and then installs the canonical pre-commit
secret-scan hook into the freshly-cloned tree, per
SPEC.md §6 + §10.2.

Identity routing against metadata/projects.json is a v0.9+ feature;
v0.8 honors the system git identity.

Args:
  <url>                 Git URL to clone.
  <dir>                 Optional destination directory (default: repo name).

Flags:
  --identity=<name>     Force a specific identity from metadata/projects.json.
  --no-precommit        Skip post-clone secret-precommit hook install.

See SPEC.md §6 + §10.2 + Common.md §4.7.`,
		Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			url := args[0]
			var target string
			if len(args) == 2 {
				target = args[1]
			} else {
				target = repoNameFromURL(url)
			}

			if identity != "" {
				notice("clone:", "identity routing for", identity, "is TBD (v0.9 work); using system git identity.")
			}

			// Run `git clone` — relies on the wrapper if installed.
			gitArgs := []string{"clone", url}
			if len(args) == 2 {
				gitArgs = append(gitArgs, args[1])
			}
			// gosec G204: this command literally IS a wrapper around
			// git. The args originate from `ai clone`'s own argv,
			// which cobra has already parsed and validated; the
			// branch-guard / no-verify-strip preHooks fire inside the
			// underlying ~/.ai/bin/git wrapper. Tainted-arg warning
			// is structural and not actionable here.
			g := exec.Command("git", gitArgs...) //nolint:gosec // G204: wrapper-around-git
			g.Stdin = os.Stdin
			g.Stdout = os.Stdout
			g.Stderr = os.Stderr
			if err := g.Run(); err != nil {
				return fmt.Errorf("git clone: %w", err)
			}

			if noPrecommit {
				return nil
			}
			return installPrecommitHook(target)
		},
	}

	c.Flags().StringVar(&identity, "identity", "", "force a specific identity from metadata/projects.json")
	c.Flags().BoolVar(&noPrecommit, "no-precommit", false, "skip the post-clone secret-precommit hook install")

	return c
}

// repoNameFromURL derives the default clone target — the trailing
// path segment with any `.git` suffix stripped.
func repoNameFromURL(url string) string {
	// Strip query and fragment if present.
	if i := strings.IndexAny(url, "?#"); i >= 0 {
		url = url[:i]
	}
	// Trim trailing slash.
	url = strings.TrimSuffix(url, "/")
	// Take last path segment.
	if i := strings.LastIndexAny(url, "/:"); i >= 0 {
		url = url[i+1:]
	}
	return strings.TrimSuffix(url, ".git")
}

// installPrecommitHook writes .git/hooks/pre-commit into the cloned
// repo that defers to ~/.ai/hooks/secret-precommit.py. Idempotent —
// skips if the hook is already present.
func installPrecommitHook(repoDir string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		notice("clone:", "could not resolve $HOME; skipping pre-commit install:", err)
		return nil
	}
	aiRoot := os.Getenv("AI_ROOT")
	if aiRoot == "" {
		aiRoot = filepath.Join(home, ".ai")
	}
	hookPath := filepath.Clean(filepath.Join(aiRoot, "hooks", "secret-precommit.py"))
	if _, err := os.Stat(hookPath); err != nil {
		notice("clone:", hookPath, "not present; skipping pre-commit install (run `ai hooks install --all` to fix).")
		return nil
	}
	dst := filepath.Clean(filepath.Join(repoDir, ".git", "hooks", "pre-commit"))
	if _, err := os.Stat(dst); err == nil {
		notice("clone:", "pre-commit already exists at", dst, "— leaving in place.")
		return nil
	}
	body := fmt.Sprintf(`#!/usr/bin/env bash
# Installed by `+"`"+`ai clone`+"`"+` (SPEC.md §10.2).
exec python3 %q "$@"
`, hookPath)
	if err := os.MkdirAll(filepath.Dir(dst), 0o750); err != nil {
		return err
	}
	// 0o755 is intentional: this IS a git pre-commit hook; git
	// requires the executable bit to invoke it.
	if err := os.WriteFile(dst, []byte(body), 0o755); err != nil { //nolint:gosec // G306: required executable
		return err
	}
	notice("clone:", "installed pre-commit secret hook at", dst)
	return nil
}
