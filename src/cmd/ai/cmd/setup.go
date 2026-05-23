package cmd

import (
	"fmt"

	"github.com/convergent-systems-co/aiConstitution/src/cmd/ai/embed"
	"github.com/convergent-systems-co/aiConstitution/src/internal/config"
	"github.com/convergent-systems-co/aiConstitution/src/internal/wizard"
	"github.com/spf13/cobra"
)

// newSetupCmd implements `ai setup [--tui] [--non-interactive] [--profile=…]`.
func newSetupCmd() *cobra.Command {
	var tui bool
	var nonInteractive bool
	var profile string

	c := &cobra.Command{
		Use:   "setup",
		Short: "Run the guided constitution-setup wizard",
		Long: `setup walks the user through the wizard taxonomy in
questions.yaml and writes settings.toml.

Flags:
  --non-interactive      Use all defaults from questions.yaml; no prompts.
  --tui                  (default) Use the Bubble Tea TUI (not yet implemented).
  --profile=<name>       Bias the question set toward a profile.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			_ = tui
			_ = profile
			if nonInteractive {
				return runSetupNonInteractive(cmd)
			}
			// TUI path: deferred to wizard TUI plan.
			notice("setup:", "TUI not yet implemented; running with defaults (--non-interactive behavior)")
			return runSetupNonInteractive(cmd)
		},
	}

	c.Flags().BoolVar(&tui, "tui", true, "use the Bubble Tea TUI (default)")
	c.Flags().BoolVar(&nonInteractive, "non-interactive", false, "use all defaults; no prompts")
	c.Flags().StringVar(&profile, "profile", "", "bias the question set (starter|developer|writer|both)")

	return c
}

func runSetupNonInteractive(cmd *cobra.Command) error {
	taxData := embed.QuestionsYAML()
	tax, err := wizard.ParseTaxonomy(taxData)
	if err != nil {
		return fmt.Errorf("setup: load question taxonomy: %w", err)
	}

	// No seeds: use all defaults. Answers are collected but not yet persisted
	// to settings fields (future work); the call validates required question
	// defaults before saving.
	if _, err := wizard.RunNonInteractive(tax, nil); err != nil {
		return fmt.Errorf("setup: non-interactive wizard: %w", err)
	}

	s := config.Defaults()
	// Use the taxonomy version to record which wizard schema was run.
	if tax.Version != "" {
		s.Wizard.LastSeenWizardVersion = tax.Version
	}

	if err := config.Save(s); err != nil {
		return fmt.Errorf("setup: save settings: %w", err)
	}

	fmt.Fprintln(cmd.OutOrStdout(), "Setup complete. Run 'ai status' to verify.")
	return nil
}
