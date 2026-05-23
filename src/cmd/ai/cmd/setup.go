package cmd

import "github.com/spf13/cobra"

// newSetupCmd implements `ai setup [--tui] [--non-interactive] [--profile=…]`.
// See SPEC.md §3.1 and the wizard taxonomy in questions.yaml.
func newSetupCmd() *cobra.Command {
	var tui bool
	var nonInteractive bool
	var profile string

	c := &cobra.Command{
		Use:   "setup",
		Short: "Run the guided constitution-setup wizard (TUI by default)",
		Long: `setup walks the user through the wizard taxonomy in
governance/wizard/questions.yaml and writes a personalized
Constitution.md / Common.md / Code.md / Writing.md based on the
answers.

Flags:
  --tui                  (default) Use the Bubble Tea TUI.
  --non-interactive      Use seeded answers from
                         governance/seed/answers.yaml; fail on any
                         unanswered required question.
  --profile=<starter|developer|writer|both>
                         Bias the question set toward a profile.

See SPEC.md §3.1, §4, and questions.yaml.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			notice("setup:", "would launch Bubble Tea TUI; questions taxonomy at governance/wizard/questions.yaml (v0.8).")
			_ = tui
			_ = nonInteractive
			_ = profile
			return stub("setup", "§3.1 + §4")
		},
	}

	c.Flags().BoolVar(&tui, "tui", true, "use the Bubble Tea TUI (default)")
	c.Flags().BoolVar(&nonInteractive, "non-interactive", false, "use seeded answers; fail on missing required answers")
	c.Flags().StringVar(&profile, "profile", "", "bias the question set (starter|developer|writer|both)")

	return c
}
