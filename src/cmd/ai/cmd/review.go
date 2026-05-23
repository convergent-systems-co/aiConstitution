package cmd

import (
	"time"

	"github.com/spf13/cobra"
)

// newReviewCmd implements `ai review`. See SPEC.md §3.2 and §6.
func newReviewCmd() *cobra.Command {
	var check bool
	var since time.Duration
	var apply bool
	var dryRun bool

	c := &cobra.Command{
		Use:   "review",
		Short: "Memory-to-amendment review loop (default cadence: 30 days)",
		Long: `review walks ~/.ai/memory/ for patterns that have crystallized
into rules, proposes amendments against the four canonical files,
and retires the memory once the rule is codified.

Flags:
  --check                Cheap dry-run; emits a one-line nag with the
                         count of pending review candidates and exits 0.
                         Suitable for invocation from ai status.
  --since=<duration>     Only consider memory entries newer than this.
  --apply                Apply the proposed amendments (after per-item
                         approval).
  --dry-run              Print the proposed amendments but do not write.

See SPEC.md §3.2 + §6 + §6.5 (30-day idle prompt).`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			notice("review:", "would walk ~/.ai/memory/ and propose amendments.")
			_ = check
			_ = since
			_ = apply
			_ = dryRun
			return stub("review", "§3.2 + §6")
		},
	}

	c.Flags().BoolVar(&check, "check", false, "cheap dry-run; emit a one-line nag and exit 0")
	c.Flags().DurationVar(&since, "since", 0, "only consider memory entries newer than this (e.g. 30d, 168h)")
	c.Flags().BoolVar(&apply, "apply", false, "apply approved amendments (per-item confirmation)")
	c.Flags().BoolVar(&dryRun, "dry-run", false, "print proposed amendments without writing")

	return c
}
