package cmd

import "github.com/spf13/cobra"

// newSyncCmd implements `ai sync {push,pull,status}`. See SPEC.md §3.4
// and §12.
func newSyncCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "sync",
		Short: "Push or pull the canonical tree to a user-owned remote",
		Long: `sync moves the canonical tree (memories, audit overrides, audit
violations, governance, hooks, settings.toml — never raw interaction
JSONL, never secrets) between this machine and a user-owned remote.

See SPEC.md §3.4 + §12.`,
	}

	// push
	var pushRemote string
	var pushForce bool
	push := &cobra.Command{
		Use:   "push",
		Short: "Push the canonical tree to the configured (or specified) remote",
		RunE: func(cmd *cobra.Command, _ []string) error {
			notice("sync push:", "would scan for secrets, validate, then push to", pushRemote)
			_ = pushForce
			return stub("sync push", "§12")
		},
	}
	push.Flags().StringVar(&pushRemote, "remote", "", "override the configured sync remote")
	push.Flags().BoolVar(&pushForce, "force", false, "force-push (gated; refuses on protected branch)")

	// pull
	var pullRemote string
	pull := &cobra.Command{
		Use:   "pull",
		Short: "Pull the canonical tree from the configured (or specified) remote",
		RunE: func(cmd *cobra.Command, _ []string) error {
			notice("sync pull:", "would fetch from", pullRemote, "and verify symlink topology")
			return stub("sync pull", "§12")
		},
	}
	pull.Flags().StringVar(&pullRemote, "remote", "", "override the configured sync remote")

	// status
	status := &cobra.Command{
		Use:   "status",
		Short: "Show sync state: configured remote, last push, last pull, dirty count",
		RunE: func(cmd *cobra.Command, _ []string) error {
			notice("sync status:", "would report configured remote + ahead/behind counts.")
			return stub("sync status", "§12")
		},
	}

	c.AddCommand(push, pull, status)
	return c
}
