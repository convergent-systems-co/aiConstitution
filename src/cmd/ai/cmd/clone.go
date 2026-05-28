package cmd

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/convergent-systems-co/aiConstitution/src/cmd/ai/internal/identity"
	"github.com/spf13/cobra"
)

// newCloneCmd implements `ai clone <url> [<dir>]`. Replaces the
// previous bin/clone shell wrapper with a Go subcommand per
// SPEC.md §6 (Q05/Q35) + Common.md §4.7 (per-repo credential
// helpers).
//
// Identity routing reads metadata/projects.json from the config dir
// (XDG_CONFIG_HOME/aiConstitution by default) and applies per-project
// git identity settings after a successful clone.
func newCloneCmd() *cobra.Command {
	var identityName string
	var noPrecommit bool

	c := &cobra.Command{
		Use:   "clone <url> [<dir>]",
		Short: "Clone a repo with identity routing + post-clone hook install",
		Long: `clone runs ` + "`" + `git clone` + "`" + ` (intentionally not bypassing the
~/.ai/bin/git wrapper) and then installs the canonical pre-commit
secret-scan hook into the freshly-cloned tree, per
SPEC.md §6 + §10.2.

Identity routing reads metadata/projects.json from the aiConstitution
config directory (XDG_CONFIG_HOME/aiConstitution) and applies git
user.name, user.email, and optionally user.signingkey to the cloned
repo. URL patterns use filepath.Match glob syntax (no ** support).

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

			if err := applyIdentityRouting(cmd.OutOrStdout(), url, target, identityName); err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: identity routing: %v\n", err)
			}

			if noPrecommit {
				return nil
			}
			return installPrecommitHook(target)
		},
	}

	c.Flags().StringVar(&identityName, "identity", "", "force a specific identity from metadata/projects.json")
	c.Flags().BoolVar(&noPrecommit, "no-precommit", false, "skip the post-clone secret-precommit hook install")

	return c
}

// applyIdentityRouting loads metadata/projects.json, resolves the
// applicable identity for cloneURL (or forceName if non-empty), and
// applies git user.name / user.email / user.signingkey inside cloneDir.
//
// Returns nil when no projects.json exists or no project matches —
// both are silent no-ops, not errors.
func applyIdentityRouting(out io.Writer, cloneURL, cloneDir, forceName string) error {
	configDir, err := xdgConfigDir()
	if err != nil {
		return err
	}
	cfg, err := identity.Load(configDir)
	if err != nil {
		return fmt.Errorf("load projects.json: %w", err)
	}
	if cfg == nil {
		return nil // no projects.json — silent
	}

	var proj *identity.Project
	if forceName != "" {
		proj = identity.FindByName(cfg.Projects, forceName)
		if proj == nil {
			return fmt.Errorf("identity %q not found in projects.json", forceName)
		}
	} else {
		proj = identity.Match(cfg.Projects, cloneURL)
		if proj == nil {
			return nil // no match — silent
		}
	}

	// Apply git config in the cloned directory.
	configs := [][2]string{
		{"user.name", proj.GitName},
		{"user.email", proj.GitEmail},
	}
	if proj.SigningKey != "" {
		configs = append(configs, [2]string{"user.signingkey", proj.SigningKey})
	}
	for _, kv := range configs {
		g := exec.Command("git", "-C", cloneDir, "config", kv[0], kv[1]) //nolint:gosec // G204: tainted via argv
		if err := g.Run(); err != nil {
			return fmt.Errorf("git config %s: %w", kv[0], err)
		}
	}
	fmt.Fprintf(out, "Identity applied: %s (%s)\n", proj.Name, proj.GitEmail)
	return nil
}

// xdgConfigDir returns the aiConstitution config directory, following
// the XDG Base Directory specification (os.UserConfigDir).
//
// The AI_CONFIG_DIR environment variable overrides the default, which
// allows tests to point at a temp directory without touching the real
// user config.
func xdgConfigDir() (string, error) {
	if override := os.Getenv("AI_CONFIG_DIR"); override != "" {
		return override, nil
	}
	d, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, "aiConstitution"), nil
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
