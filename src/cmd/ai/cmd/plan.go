package cmd

import "github.com/spf13/cobra"

// newPlanCmd implements `ai plan new` (and list/show).
// Plans live under ~/.ai/plans/ per SPEC.md §15 and §17.1 (F-5 carve-out).
// Their shape comes from Code.md §11.1 (Plan template).
func newPlanCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "plan",
		Short: "Manage plans under ~/.ai/plans/",
		Long: `plan operates on the work-product plans at ~/.ai/plans/. These are
authored, edited during work, and edited post-mortem. Per spec §17.1
the plans/specs-in-~/.ai/ placement is a deliberate carve-out from the
"mutable lives in ~/.config/" rule because sync of work products is
load-bearing.

Plans follow the template from Code.md §11.1: Objective, Rationale +
Alternatives Table, Scope, Approach, Testing strategy, Risk, Deps,
Backcompat.

When the `+"`"+`superpowers`+"`"+` Claude plugin is enabled, `+"`"+`plan new`+"`"+` produces
plans with `+"`"+`- [ ]`+"`"+` task lists compatible with subagent-driven-development.`,
	}

	c.AddCommand(
		&cobra.Command{Use: "new <slug>", Short: "Scaffold a new plan at ~/.ai/plans/<UTC>-<slug>.md", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
			notice("plan new:", args[0])
			return stub("plan new", "§15 + §17.1 + Code.md §11.1")
		}},
		&cobra.Command{Use: "list", Short: "List plans under ~/.ai/plans/", RunE: func(cmd *cobra.Command, _ []string) error {
			notice("plan list")
			return stub("plan list", "§15")
		}},
		&cobra.Command{Use: "show <name>", Short: "Print a plan to stdout", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
			notice("plan show:", args[0])
			return stub("plan show", "§15")
		}},
	)
	return c
}
