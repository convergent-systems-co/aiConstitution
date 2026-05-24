package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/convergent-systems-co/aiConstitution/src/cmd/ai/embed"
	"github.com/convergent-systems-co/aiConstitution/src/internal/constitution"
	"github.com/convergent-systems-co/aiConstitution/src/internal/paths"
	"github.com/convergent-systems-co/aiConstitution/src/internal/wizard"
	"github.com/spf13/cobra"
)

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
			if nonInteractive {
				return runSetupNonInteractive(cmd)
			}
			notice("setup:", "would launch Bubble Tea TUI; questions taxonomy at governance/wizard/questions.yaml (v0.8).")
			_ = tui
			_ = profile
			return stub("setup", "§3.1 + §4")
		},
	}

	c.Flags().BoolVar(&tui, "tui", true, "use the Bubble Tea TUI (default)")
	c.Flags().BoolVar(&nonInteractive, "non-interactive", false, "use seeded answers; fail on missing required answers")
	c.Flags().StringVar(&profile, "profile", "", "bias the question set (starter|developer|writer|both)")

	return c
}

// runSetupNonInteractive runs setup with seeded/default answers and writes
// Constitution.md and Constitution.runtime.md to the AI root directory.
func runSetupNonInteractive(cmd *cobra.Command) error {
	// Seed answers: minimal defaults that satisfy all required wizard fields.
	answers := map[string]string{
		"Q01": "Principal",            // required: principal name
		"Q02": "claude-code",          // tools
		"Q03": "software development", // work context
		"Q04": "technical",            // domains
		"Q05": "",                     // personal rules (optional)
		"Q06": "$5",                   // cost ceiling
		"Q07": "100",                  // blast radius
		"Q08": "main",                 // protected branches
		"Q09": "autonomous",           // autonomy posture
		"Q10": "flag-once",            // pushback persistence
		"Q11": "match-complexity",     // response length
		"Q12": "direct-framing",       // disagreement tone
		"Q13": "true",                 // provenance in commits
	}

	// Map answers to AnswerSet and render Constitution.md
	as, err := wizard.AnswersToAnswerSet(answers)
	if err != nil {
		return fmt.Errorf("setup: map answers: %w", err)
	}
	tmplBytes, err := embed.ConstitutionTemplate()
	if err != nil {
		return fmt.Errorf("setup: load template: %w", err)
	}
	rendered, err := constitution.Render(as, string(tmplBytes))
	if err != nil {
		return fmt.Errorf("setup: render constitution: %w", err)
	}
	aiRoot := paths.AIRoot()
	if err := os.MkdirAll(aiRoot, 0o750); err != nil {
		return fmt.Errorf("setup: mkdir airoot: %w", err)
	}
	if err := os.WriteFile(filepath.Join(aiRoot, "Constitution.md"), []byte(rendered), 0o600); err != nil { //nolint:gosec
		return fmt.Errorf("setup: write Constitution.md: %w", err)
	}

	// Generate runtime
	rc, err := constitution.ExtractRuntime(rendered)
	if err != nil {
		return fmt.Errorf("setup: extract runtime: %w", err)
	}
	runtimeOut := constitution.FormatRuntime(rc)
	if err := os.WriteFile(filepath.Join(aiRoot, "Constitution.runtime.md"), []byte(runtimeOut), 0o600); err != nil { //nolint:gosec
		return fmt.Errorf("setup: write runtime: %w", err)
	}

	_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Setup complete.")
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Constitution: %s\n", filepath.Join(aiRoot, "Constitution.md"))
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Runtime:      %s\n", filepath.Join(aiRoot, "Constitution.runtime.md"))
	return nil
}

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
