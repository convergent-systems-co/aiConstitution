package cmd

import "github.com/spf13/cobra"

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
			notice("hooks list")
			return stub("hooks list", "§3.10")
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

	// install
	var installRepo string
	var installAll bool
	install := &cobra.Command{
		Use:   "install <name>",
		Short: "Install a hook into the wiring (idempotent)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			notice("hooks install:", args[0], "repo:", installRepo, "all:", installAll)
			return stub("hooks install", "§3.10 + §10.2")
		},
	}
	install.Flags().StringVar(&installRepo, "repo", "", "install pre-commit hook into a specific repo")
	install.Flags().BoolVar(&installAll, "all-future-clones", false, "wire into bin/clone so every fresh clone gets the hook")

	c.AddCommand(propose, install)
	return c
}
