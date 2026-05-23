package cmd

import "github.com/spf13/cobra"

// newIssueCmd implements `ai issue file`. See SPEC.md §3.12 + §9.5.
func newIssueCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "issue",
		Short: "File hook / finding issues upstream",
		Long: `issue is the direct surface for upstreaming hooks and major
findings. Bodies are redacted against hooks/patterns.json before
submission; the user reviews the body unless
settings.upstream.skipReviewWindow=true.

See SPEC.md §3.12 + §9.5.`,
	}

	var fileType string
	var major bool
	var fromAudit string
	file := &cobra.Command{
		Use:   "file",
		Short: "File a hook or finding issue upstream",
		RunE: func(cmd *cobra.Command, _ []string) error {
			notice("issue file:", "type=", fileType, "major=", major, "audit:", fromAudit)
			return stub("issue file", "§3.12 + §9.5")
		},
	}
	file.Flags().StringVar(&fileType, "type", "", "issue type (hook|finding)")
	file.Flags().BoolVar(&major, "major", false, "mark as major (escalates the upstream prompt)")
	file.Flags().StringVar(&fromAudit, "from-audit", "", "originating audit record path")
	_ = file.MarkFlagRequired("type")

	c.AddCommand(file)
	return c
}
