package cmd

import "github.com/spf13/cobra"

// newDoctorCmd implements `ai doctor`. See SPEC.md §3.3.
func newDoctorCmd() *cobra.Command {
	var fix bool
	var resetHead string

	c := &cobra.Command{
		Use:   "doctor",
		Short: "Detect and repair structural damage to the ~/.ai/ tree",
		Long: `doctor checks the predictable failure modes of the constitution
tree and either reports them or fixes them:

  1.  Broken symlinks under ~/.claude/, ~/.copilot/, .cursor/, etc.
  2.  Missing or misregistered hooks.
  3.  Dirty working tree on ~/.ai/.
  4.  Divergent HEAD vs origin.
  5.  Stale ai binary vs governance/last-seen-version.
  6.  Missing brand-cache; missing persona/profile/skill cache for
      pinned atoms.
  7.  Audit/interactions log writable.
  8.  Mutable state in ~/.config/aiConstitution/ exists and parses.
  9.  Settings file present and within validation ranges.
  10. last-seen-version marker matches the binary.

Flags:
  --fix                  Attempt to repair each detected issue.
  --reset-head=<ref>     If the tree is dirty or HEAD is divergent,
                         reset to <ref> (refuses on dirty tree
                         without --force-hard-reset).

See SPEC.md §3.3.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			notice("doctor:", "would scan the tree for broken symlinks, missing hooks, stale binary, etc.")
			_ = fix
			_ = resetHead
			return stub("doctor", "§3.3")
		},
	}

	c.Flags().BoolVar(&fix, "fix", false, "attempt to repair each detected issue")
	c.Flags().StringVar(&resetHead, "reset-head", "", "reset HEAD to <ref> (use with caution)")

	return c
}
