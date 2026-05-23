package cmd

import "github.com/spf13/cobra"

// newStatusCmd implements `ai status`. The existing surface from v0.1,
// extended in §3.2 and §3.3 to include the review-cadence nag and the
// last-seen-version check.
func newStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Print a short status report (sync state, review cadence, doctor warnings)",
		Long: `status prints a one-screen status report:
  - Active mode + composing atoms
  - Sync state (last push, last pull)
  - Review cadence (count of pending review candidates)
  - Doctor warnings (broken symlinks, missing hooks, stale binary)
  - Drift since last drill

See SPEC.md §3.2 + §3.3.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			notice("status:", "would emit a one-screen status report")
			return stub("status", "§3.2 + §3.3")
		},
	}
}
