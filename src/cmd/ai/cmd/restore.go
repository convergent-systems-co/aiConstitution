package cmd

import "github.com/spf13/cobra"

// newRestoreCmd implements `ai restore <url>`. See SPEC.md §3.5 + §12.1.
func newRestoreCmd() *cobra.Command {
	var dest string
	var noHooks bool

	c := &cobra.Command{
		Use:   "restore <url>",
		Short: "Restore the canonical tree from a sync remote (use on fresh machines)",
		Long: `restore is `+"`"+`ai setup`+"`"+` re-bound to a sync URL: it pulls the
previously-synced tree and re-asserts the symlink/hook topology on a
fresh machine. Used to reproduce the system on a borrowed laptop, dev
container, or recovered backup.

The quarterly drill from Q45 invokes this with --dest=/tmp/...; see
SPEC.md §12.1 for the verification checklist output.

Args:
  <url>                  Sync remote (git URL, S3-compatible URL, or
                         file path).

Flags:
  --dest=<path>          Restore to a non-default path (default: ~/.ai).
                         Required for drill mode (/tmp/ai-restore-drill-…).
  --no-hooks             Skip hook installation (rare; for inspection).

See SPEC.md §3.5 + §12.1.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			notice("restore:", "would clone", args[0], "to", dest, "and re-assert symlink topology.")
			_ = noHooks
			return stub("restore", "§3.5")
		},
	}

	c.Flags().StringVar(&dest, "dest", "", "destination path (default: ~/.ai)")
	c.Flags().BoolVar(&noHooks, "no-hooks", false, "skip hook installation")

	return c
}
