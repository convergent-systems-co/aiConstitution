package cmd

import (
	"fmt"

	"github.com/convergent-systems-co/aiConstitution/src/internal/constitution"
	"github.com/convergent-systems-co/aiConstitution/src/internal/paths"
	"github.com/spf13/cobra"
)

// newStatusCmd implements `ai status`. The existing surface from v0.1,
// extended in §3.2 and §3.3 to include the review-cadence nag and the
// last-seen-version check.
func newStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Print a short status report (sync state, review cadence, doctor warnings)",
		Long: `status prints a one-screen status report:
  - AI root path
  - Constitution file presence (~/.ai/{Constitution,Common,Code,Writing}.md)
  - Active mode + composing atoms (TODO: §3.2)
  - Sync state — last push, last pull (TODO: §3.2)
  - Review cadence — count of pending review candidates (TODO: §3.2)
  - Doctor warnings — broken symlinks, missing hooks, stale binary (TODO: §3.3)

See SPEC.md §3.2 + §3.3.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			root := paths.AIRoot()
			fmt.Fprintf(cmd.OutOrStdout(), "AI Root: %s\n\n", root)

			fmt.Fprintln(cmd.OutOrStdout(), "Constitution files:")
			status := constitution.FileStatus(root)
			for _, name := range constitution.FileNames {
				mark := "present"
				if !status[name] {
					mark = "MISSING"
				}
				fmt.Fprintf(cmd.OutOrStdout(), "  %-30s %s\n", name, mark)
			}
			if status["Constitution.local.md"] {
				fmt.Fprintf(cmd.OutOrStdout(), "  %-30s %s\n", "Constitution.local.md", "present (local override)")
			}
			return nil
		},
	}
}
