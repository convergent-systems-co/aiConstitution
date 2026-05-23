package cmd

import (
	"fmt"

	"github.com/convergent-systems-co/aiConstitution/src/internal/constitution"
	"github.com/convergent-systems-co/aiConstitution/src/internal/paths"
	"github.com/spf13/cobra"
)

// newDoctorCmd implements `ai doctor`. See SPEC.md §3.3.
func newDoctorCmd() *cobra.Command {
	var fix bool
	var resetHead string

	c := &cobra.Command{
		Use:   "doctor",
		Short: "Detect and repair structural damage to the ~/.ai/ tree",
		Long: `doctor checks the predictable failure modes of the constitution
tree and either reports them or fixes them.

Checks currently implemented:
  1. Constitution files present (~/.ai/{Constitution,Common,Code,Writing}.md)

See SPEC.md §3.3 for the full 10-point check list (in progress).`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			_ = fix
			_ = resetHead
			root := paths.AIRoot()
			status := constitution.FileStatus(root)

			allOK := true
			for _, name := range constitution.FileNames {
				present := status[name]
				mark := "✓"
				if !present {
					mark = "✗"
					allOK = false
				}
				fmt.Fprintf(cmd.OutOrStdout(), "  [%s] %s\n", mark, name)
			}
			if localPresent := status["Constitution.local.md"]; localPresent {
				fmt.Fprintf(cmd.OutOrStdout(), "  [✓] Constitution.local.md (local override)\n")
			}
			if !allOK {
				return fmt.Errorf("doctor: missing required constitution files in %s", root)
			}
			fmt.Fprintln(cmd.OutOrStdout(), "Constitution files: OK")
			return nil
		},
	}

	c.Flags().BoolVar(&fix, "fix", false, "attempt to repair each detected issue")
	c.Flags().StringVar(&resetHead, "reset-head", "", "reset HEAD to <ref> (use with caution)")
	return c
}
