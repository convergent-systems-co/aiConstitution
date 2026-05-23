package cmd

import "github.com/spf13/cobra"

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

The CLI computes the canonical path automatically. Raw `+"`"+`git worktree add`+"`"+`
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
			notice("worktree add:", args[0], "global=", global)
			return stub("worktree add", "Common.md §U17.5")
		},
	}
	add.Flags().BoolVar(&global, "global", false, "create under ~/.ai/worktrees/ (cross-repo) instead of <repo>/.worktrees/")

	c.AddCommand(
		add,
		&cobra.Command{Use: "remove <name>", Short: "Remove a canonically-placed worktree", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
			notice("worktree remove:", args[0])
			return stub("worktree remove", "Common.md §U17.3")
		}},
		&cobra.Command{Use: "list", Short: "List worktrees in both canonical roots", RunE: func(cmd *cobra.Command, _ []string) error {
			notice("worktree list")
			return stub("worktree list", "Common.md §U17")
		}},
	)
	return c
}
