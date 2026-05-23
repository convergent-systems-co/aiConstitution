package cmd

import "github.com/spf13/cobra"

// newUpdateCmd implements `ai update`. See SPEC.md §3.9 + §8.
func newUpdateCmd() *cobra.Command {
	var migrate bool
	var skipMigrate bool
	var blocking bool

	c := &cobra.Command{
		Use:   "update",
		Short: "Update the binary + reconcile new hooks/skills/personas/questions",
		Long: `update runs the upstream reconciliation. The base action is
`+"`"+`git pull --ff-only`+"`"+` on ~/.ai/ plus `+"`"+`go build`+"`"+` of the binary.

On any subsequent `+"`"+`ai`+"`"+` invocation where governance/last-seen-version
differs from the binary version, the migration prompt fires (unless
settings.update.autoMigratePrompt = false). --migrate runs it
immediately; --skip-migrate suppresses it once.

See SPEC.md §3.9 + §8.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			notice("update:", "migrate=", migrate, "skip-migrate=", skipMigrate, "blocking=", blocking)
			return stub("update", "§3.9 + §8")
		},
	}

	c.Flags().BoolVar(&migrate, "migrate", false, "run reconciliation now")
	c.Flags().BoolVar(&skipMigrate, "skip-migrate", false, "one-shot bypass of the migration prompt")
	c.Flags().BoolVar(&blocking, "blocking", false, "opt back into the original blocking behavior of the migration prompt")

	return c
}
