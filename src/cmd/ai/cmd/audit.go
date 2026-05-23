package cmd

import "github.com/spf13/cobra"

// newAuditCmd implements `ai audit {override,violation}`.
// Mentioned in SPEC.md §11.2 as part of the existing/stays-in-CLI set.
// The override/violation file shape is governed by
// ~/.ai/Constitution.md §5.1 + §5.2.
func newAuditCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "audit",
		Short: "Record overrides and violations into ~/.ai/audit/",
		Long: `audit is the canonical surface for adding override and violation
records. The file format is defined by Constitution.md §5.1 (overrides)
and §5.2 (violations).`,
	}

	c.AddCommand(
		&cobra.Command{Use: "override", Short: "Record an override (writes audit/overrides/<UTC>.md)", RunE: func(cmd *cobra.Command, _ []string) error {
			notice("audit override:", "would prompt for the canonical override fields")
			return stub("audit override", "Constitution.md §5.1")
		}},
		&cobra.Command{Use: "violation", Short: "Record a self-noticed violation (writes audit/violations/<UTC>.md)", RunE: func(cmd *cobra.Command, _ []string) error {
			notice("audit violation:", "would prompt for the canonical violation fields")
			return stub("audit violation", "Constitution.md §5.2")
		}},
		&cobra.Command{Use: "list", Short: "List recent overrides and violations", RunE: func(cmd *cobra.Command, _ []string) error {
			notice("audit list:", "would tail ~/.ai/audit/overrides/ and ~/.ai/audit/violations/")
			return stub("audit list", "Constitution.md §5")
		}},
		newAuditRotateCmd(),
	)
	return c
}
