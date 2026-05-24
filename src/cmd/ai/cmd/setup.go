package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/convergent-systems-co/aiConstitution/src/cmd/ai/embed"
	initpkg "github.com/convergent-systems-co/aiConstitution/src/cmd/ai/internal/init"
	"github.com/convergent-systems-co/aiConstitution/src/internal/config"
	"github.com/convergent-systems-co/aiConstitution/src/internal/paths"
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
  --profile=<name>       Bias the question set toward a profile.

Environment:
  AICONST_SEEDS          Comma-separated key=value pairs used to seed
                         wizard answers in --non-interactive mode.
                         Example: defaultMode=writer,reviewCadenceDays=14`,
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

	seeds := parseSeedsEnv(os.Getenv("AICONST_SEEDS"))
	answers, err := wizard.RunNonInteractive(tax, seeds)
	if err != nil {
		return fmt.Errorf("setup: non-interactive wizard: %w", err)
	}

	s := config.Defaults()
	if tax.Version != "" {
		s.Wizard.LastSeenWizardVersion = tax.Version
	}
	// Map answer keys → Settings fields (per SPEC §13.6). The wizard's
	// flat answer map keys are the qid-derived answer keys (defaultMode,
	// shareNewHooks, reviewCadenceDays, syncIncludeSettings) — not the
	// qid itself. Seeds use the same keys.
	config.ApplyAnswers(&s, answers)
	config.ApplyAnswers(&s, seeds)

	if err := config.Save(s); err != nil {
		return fmt.Errorf("setup: save settings: %w", err)
	}

	// Create ~/.ai/ tree and the three AI-tool integration files.
	// Pre-existing files (CLAUDE.md, copilot-instructions.md, AGENTS.md)
	// are never overwritten.
	written, err := initpkg.EnsureToolFiles(paths.AIRoot())
	if err != nil {
		return fmt.Errorf("setup: ensure tool files: %w", err)
	}
	for _, p := range written {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Wrote %s\n", p)
	}

	_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Setup complete. Run 'ai status' to verify.")
	return nil
}

// parseSeedsEnv parses the AICONST_SEEDS env var, which uses the form
// "key1=value1,key2=value2". Empty input returns nil. Whitespace
// around keys and values is trimmed. Malformed entries (no '=') are
// silently skipped.
func parseSeedsEnv(env string) map[string]string {
	env = strings.TrimSpace(env)
	if env == "" {
		return nil
	}
	out := make(map[string]string)
	for _, pair := range strings.Split(env, ",") {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}
		eq := strings.IndexByte(pair, '=')
		if eq <= 0 {
			continue
		}
		k := strings.TrimSpace(pair[:eq])
		v := strings.TrimSpace(pair[eq+1:])
		if k == "" {
			continue
		}
		out[k] = v
	}
	return out
}
