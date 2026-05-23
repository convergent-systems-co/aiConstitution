package cmd

import "github.com/spf13/cobra"

// newAmendCmd implements `ai amend <file>/<section>`. See SPEC.md §3.6
// and §6 (Memory → Amendment Lifecycle).
func newAmendCmd() *cobra.Command {
	var message string
	var breaking bool

	c := &cobra.Command{
		Use:   "amend <file>/<section>",
		Short: "Open or apply an amendment against a canonical file",
		Long: `amend writes a versioned change against one of the four canonical
files, bumping the file's version, appending the Changelog entry, and
recording the amendment in the audit log.

Args:
  <file>/<section>       e.g. "Common.md/U17", "Code.md/§11.2".

Flags:
  --message=<text>       Amendment prose (otherwise opens $EDITOR).
  --breaking             Mark as a breaking change in the Changelog.

See SPEC.md §3.6 and §6.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			notice("amend:", args[0], "— would bump file version and append Changelog entry.")
			_ = message
			_ = breaking
			return stub("amend", "§3.6 + §6")
		},
	}

	c.Flags().StringVar(&message, "message", "", "amendment prose (default: open $EDITOR)")
	c.Flags().BoolVar(&breaking, "breaking", false, "mark as a BREAKING change in the Changelog")

	return c
}
