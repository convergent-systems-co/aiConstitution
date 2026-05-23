package cmd

import "github.com/spf13/cobra"

// newBackupCmd implements `ai backup`. Mentioned in SPEC.md §11.2 as
// part of the existing/stays-in-CLI set, and invoked transactionally
// by migrations (e.g., §7.9.7 v0.4 → v0.5 atoms migration).
func newBackupCmd() *cobra.Command {
	var dest string
	c := &cobra.Command{
		Use:   "backup",
		Short: "Snapshot the canonical tree to a local archive (used by migrations)",
		Long: `backup writes a tarball snapshot of ~/.ai/ (excluding
audit/interactions/) to the configured backup directory. Migrations
run this first so a failed migration can be rolled back.

See SPEC.md §11.2 + §7.9.7.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			notice("backup:", "dest=", dest)
			return stub("backup", "§11.2 + §7.9.7")
		},
	}
	c.Flags().StringVar(&dest, "dest", "", "destination directory (default: ~/.config/aiConstitution/backups/)")
	return c
}
