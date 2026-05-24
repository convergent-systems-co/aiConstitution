package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/convergent-systems-co/aiConstitution/src/cmd/ai/embed"

	"github.com/spf13/cobra"
)

// newHooksCmd implements `ai hooks {list,evaluate,propose,share,install}`.
// See SPEC.md §3.10 + §9.
func newHooksCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "hooks",
		Short: "Hook lifecycle: list, evaluate, propose, share upstream, install",
		Long: `hooks operates on the Python hook library at ~/.ai/hooks/, plus the
command-wrapper preHooks/postHooks/commandHooks declared in
hooks/command-wrappers.toml.

See SPEC.md §3.10 + §9.`,
	}

	// list
	c.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "All installed hooks + wiring status",
		RunE: func(cmd *cobra.Command, _ []string) error {
			names, err := embed.HookNames()
			if err != nil {
				return err
			}
			sort.Strings(names)
			fmt.Println("Embedded hooks available for `ai hooks install`:")
			for _, n := range names {
				fmt.Println("  " + n)
			}
			fmt.Println()
			fmt.Println("Wrappers available for `ai hooks install command-wrappers`:")
			fmt.Println("  git, gh")
			return nil
		},
	})

	// evaluate
	c.AddCommand(&cobra.Command{
		Use:   "evaluate",
		Short: "Run each hook's --self-check; emit findings",
		RunE: func(cmd *cobra.Command, _ []string) error {
			notice("hooks evaluate:", "would run --self-check on every installed hook")
			return stub("hooks evaluate", "§9 + §3.10")
		},
	})

	// propose
	var fromViolation string
	var lang string
	propose := &cobra.Command{
		Use:   "propose <name>",
		Short: "Scaffold a new hook from a finding (chat handoff for prose)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			notice("hooks propose:", args[0], "from:", fromViolation, "lang:", lang)
			return stub("hooks propose", "§9.2")
		},
	}
	propose.Flags().StringVar(&fromViolation, "from-violation", "", "path to an audit/violations/*.md file")
	propose.Flags().StringVar(&lang, "lang", "python", "language (python|sh|go|node)")

	// share
	c.AddCommand(&cobra.Command{
		Use:   "share <name>",
		Short: "File the hook upstream as an issue (gated by settings.upstream.shareNewHooks)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			notice("hooks share:", args[0])
			return stub("hooks share", "§9.3")
		},
	})

	// install — extracts from the embedded FS to ~/.ai/hooks/ (hooks)
	// or ~/.ai/bin/ (command-wrappers). Special target names:
	//   --all                    → every embedded hook
	//   command-wrappers         → both wrapper templates (git, gh)
	//   <name.ext>               → that one embedded hook
	var installRepo string
	var installAll bool
	var installAllHooks bool
	var installForce bool
	var installClaude bool
	var installClaudeRoot string
	install := &cobra.Command{
		Use:   "install [<name>]",
		Short: "Extract embedded hook(s) / wrappers to ~/.ai/ (idempotent)",
		Long: `install materializes embedded assets onto disk.

  ai hooks install --all                  extract every embedded hook
                                          into ~/.ai/hooks/
  ai hooks install command-wrappers       extract bin/git and bin/gh
                                          into ~/.ai/bin/
  ai hooks install <name>                 extract a single embedded
                                          hook (e.g. secret-block.py)
  ai hooks install --claude               wire installed hooks into
                                          .claude/settings.json in
                                          the current repo

  --force                overwrite existing files
  --repo=<path>          (with no positional) install a pre-commit
                         hook into the specified repo's .git/hooks/
                         that defers to ~/.ai/hooks/secret-precommit.py
  --claude               wire ~/.ai/hooks/*.py into .claude/settings.json
  --claude-root=<path>   directory containing .claude/ (default ".")

Per SPEC.md §3.10 + §10.2 + §14.1.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			target := ""
			if len(args) == 1 {
				target = args[0]
			}
			if installClaude {
				return runHooksInstallClaude(cmd, installClaudeRoot)
			}
			return runHooksInstall(installRepo, target, installAllHooks || installAll, installForce)
		},
	}
	install.Flags().StringVar(&installRepo, "repo", "", "install a pre-commit shim into the specified repo")
	install.Flags().BoolVar(&installAll, "all-future-clones", false, "(reserved; wires into `ai clone` per SPEC §10.2)")
	install.Flags().BoolVar(&installAllHooks, "all", false, "extract every embedded hook to ~/.ai/hooks/")
	install.Flags().BoolVar(&installForce, "force", false, "overwrite existing files")
	install.Flags().BoolVar(&installClaude, "claude", false, "wire ~/.ai/hooks/*.py into .claude/settings.json")
	install.Flags().StringVar(&installClaudeRoot, "claude-root", ".", "directory containing .claude/ (default: current dir)")

	c.AddCommand(propose, install)
	return c
}

// runHooksInstallClaude wires the Python hooks under ~/.ai/hooks/ into
// .claude/settings.json under claudeRoot. Per §156 / SPEC §14.1.
func runHooksInstallClaude(cmd *cobra.Command, claudeRoot string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	aiRoot := os.Getenv("AI_ROOT")
	if aiRoot == "" {
		aiRoot = filepath.Join(home, ".ai")
	}
	hooksDir := filepath.Join(aiRoot, "hooks")
	added, err := installClaudeHooks(claudeRoot, hooksDir)
	if err != nil {
		return err
	}
	_, _ = fmt.Fprintf(cmd.OutOrStdout(),
		"Wired %d Claude hook entries into %s\n",
		added, filepath.Join(claudeRoot, ".claude", "settings.json"))
	return nil
}

// runHooksInstall is the top-level dispatcher for the various
// install modes. Extracted from newHooksCmd's RunE closure to keep
// the cobra constructor under gocyclo's threshold.
func runHooksInstall(repo, target string, all, force bool) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	aiRoot := os.Getenv("AI_ROOT")
	if aiRoot == "" {
		aiRoot = filepath.Join(home, ".ai")
	}
	hooksDir := filepath.Join(aiRoot, "hooks")
	binDir := filepath.Join(aiRoot, "bin")

	if repo != "" {
		return installRepoPrecommit(repo, hooksDir)
	}
	if all {
		return installAllHooks(hooksDir, force)
	}
	if target == "command-wrappers" {
		return installWrappers(binDir, force)
	}
	if target != "" {
		return installOneHook(target, hooksDir, force)
	}
	return fmt.Errorf("specify a hook name, --all, or `command-wrappers`. See `ai hooks install --help`")
}

func installAllHooks(hooksDir string, force bool) error {
	written, err := embed.ExtractAllHooks(hooksDir, force)
	if err != nil {
		return err
	}
	fmt.Printf("Extracted %d hook(s) to %s\n", len(written), hooksDir)
	for _, p := range written {
		fmt.Println("  " + p)
	}
	return nil
}

func installWrappers(binDir string, force bool) error {
	written, err := embed.ExtractWrappers(binDir, force)
	if err != nil {
		return err
	}
	fmt.Printf("Extracted %d wrapper(s) to %s\n", len(written), binDir)
	for _, p := range written {
		fmt.Println("  " + p)
	}
	if len(written) > 0 {
		fmt.Println("\nNote: add", binDir, "early to your $PATH for wrapper interception to fire.")
	}
	return nil
}

func installOneHook(name, hooksDir string, force bool) error {
	p, err := embed.ExtractHook(name, hooksDir, force)
	if err != nil {
		return err
	}
	fmt.Println("Extracted " + p)
	return nil
}

// installRepoPrecommit writes <repo>/.git/hooks/pre-commit that defers
// to the canonical ~/.ai/hooks/secret-precommit.py. Idempotent.
func installRepoPrecommit(repoDir, hooksDir string) error {
	gitDir := filepath.Join(repoDir, ".git")
	if _, err := os.Stat(gitDir); err != nil {
		return fmt.Errorf("%s is not a git repo (.git/ missing)", repoDir)
	}
	hookPath := filepath.Clean(filepath.Join(hooksDir, "secret-precommit.py"))
	if _, err := os.Stat(hookPath); err != nil {
		return fmt.Errorf("canonical %s missing — run `ai hooks install --all` first", hookPath)
	}
	dst := filepath.Clean(filepath.Join(gitDir, "hooks", "pre-commit"))
	if _, err := os.Stat(dst); err == nil {
		fmt.Println("pre-commit already present at", dst, "— leaving in place")
		return nil
	}
	body := fmt.Sprintf(`#!/usr/bin/env bash
# Installed by `+"`"+`ai hooks install --repo=%s`+"`"+` (SPEC.md §10.2).
exec python3 %q "$@"
`, repoDir, hookPath)
	if err := os.MkdirAll(filepath.Dir(dst), 0o750); err != nil {
		return err
	}
	// 0o755 is intentional: this IS a git pre-commit hook; git
	// requires the executable bit to invoke it.
	if err := os.WriteFile(dst, []byte(body), 0o755); err != nil { //nolint:gosec // G306: required executable
		return err
	}
	fmt.Println("installed", dst)
	return nil
}
