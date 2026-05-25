package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/convergent-systems-co/aiConstitution/src/cmd/ai/embed"
	tui "github.com/convergent-systems-co/aiConstitution/src/cmd/ai/internal/wizard"
	"github.com/convergent-systems-co/aiConstitution/src/internal/config"
	internalwizard "github.com/convergent-systems-co/aiConstitution/src/internal/wizard"
	"github.com/convergent-systems-co/aiConstitution/src/internal/constitution"
	"github.com/convergent-systems-co/aiConstitution/src/internal/paths"
)

// newSetupCmd implements `ai setup [--tui] [--non-interactive] [--profile=…]`.
// See SPEC.md §3.1 and the wizard taxonomy in questions.yaml.
func newSetupCmd() *cobra.Command {
	var tuiFlag bool
	var nonInteractive bool
	var profile string
	var noHooks bool

	c := &cobra.Command{
		Use:   "setup",
		Short: "Run the guided constitution-setup wizard (TUI by default)",
		Long: `setup walks the user through the wizard taxonomy in
governance/wizard/questions.yaml and writes a personalized
Constitution.md / Constitution.runtime.md based on the answers,
saves settings.toml, installs hooks, writes ~/.claude/CLAUDE.md,
and creates the ~/.copilot/instructions/constitution.md symlink.

Flags:
  --tui                  (default) Use the Bubble Tea TUI.
  --non-interactive      Use seeded answers from
                         governance/seed/answers.yaml; fail on any
                         unanswered required question.
  --profile=<starter|developer|writer|both>
                         Bias the question set toward a profile.
  --no-hooks             Skip hook installation and tool wiring
                         (CLAUDE.md, Copilot symlink). Useful for
                         generating a constitution in isolation
                         (e.g. AI_ROOT=/tmp/test ai setup --no-hooks).

See SPEC.md §3.1, §4, and questions.yaml.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if nonInteractive {
				return runSetupNonInteractive(profile)
			}
			return runSetupTUI(noHooks)
		},
	}

	c.Flags().BoolVar(&tuiFlag, "tui", true, "use the Bubble Tea TUI (default)")
	c.Flags().BoolVar(&nonInteractive, "non-interactive", false, "use seeded answers; fail on missing required answers")
	c.Flags().StringVar(&profile, "profile", "", "bias the question set (starter|developer|writer|both)")
	c.Flags().BoolVar(&noHooks, "no-hooks", false, "skip hook install, CLAUDE.md, and Copilot symlink")

	return c
}

// runSetupTUI runs the Bubble Tea wizard and, on completion, wires up the
// constitution files and optionally hooks, CLAUDE.md, and the Copilot symlink.
// When noHooks is true, only the constitution files are written.
func runSetupTUI(noHooks bool) error {
	// Detect and migrate legacy four-file layout before running the wizard.
	// This ensures the wizard sees the existing rules and the user can
	// compare what their original constitution looked like vs the new one.
	aiRootPre := paths.AIRoot()
	status := constitution.FileStatusV2(aiRootPre)
	if !status["v2"] && status["Common.md"] {
		fmt.Println("Detected existing four-file constitution layout.")
		fmt.Println("Migrating your existing rules into a unified Constitution.md first...")
		migrateOut := &cobra.Command{}
		migrateOut.SetOut(os.Stdout)
		if err := runMigrateFlatten(migrateOut, aiRootPre); err != nil {
			return fmt.Errorf("setup: auto-migrate flatten: %w", err)
		}
		if err := runMigrateAddBehavioral(migrateOut, aiRootPre); err != nil {
			return fmt.Errorf("setup: auto-migrate behavioral: %w", err)
		}
		if err := runMigrateGenerateRuntime(migrateOut, aiRootPre); err != nil {
			return fmt.Errorf("setup: auto-migrate runtime: %w", err)
		}
		fmt.Println("Migration complete. Your original rules are in Constitution.md.")
		fmt.Println("The wizard will now generate a fresh personalized constitution.")
		fmt.Println("You can compare the two after setup is complete.")
		fmt.Println()
	}

	// Load the embedded questions taxonomy.
	taxData := embed.QuestionsYAML()
	tax, err := internalwizard.ParseTaxonomy(taxData)
	if err != nil {
		return fmt.Errorf("setup: parse taxonomy: %w", err)
	}

	// Run the Bubble Tea program.
	m := tui.NewModel(*tax)
	prog := tea.NewProgram(m)
	finalModel, err := prog.Run()
	if err != nil {
		return fmt.Errorf("setup: TUI error: %w", err)
	}
	finalWizard, ok := finalModel.(tui.Model)
	if !ok {
		return fmt.Errorf("setup: unexpected model type %T", finalModel)
	}

	if !finalWizard.Done() {
		fmt.Fprintln(os.Stderr, "setup: wizard exited without completing")
		return nil
	}

	aiRoot := paths.AIRoot()
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("setup: resolve HOME: %w", err)
	}
	claudeDir := filepath.Join(home, ".claude")
	copilotDir := filepath.Join(home, ".copilot")

	return runSetupPostWizard(aiRoot, claudeDir, copilotDir, finalWizard.Answers(), noHooks)
}

// runSetupPostWizard executes the post-wizard setup steps:
//  1. Map wizard answers to settings.toml fields and save (#195).
//  2. Extract all embedded hooks into ~/.ai/hooks/ (#196).
//  3. Write ~/.claude/CLAUDE.md with @-include directive (#197).
//  4. Create ~/.copilot/instructions/constitution.md symlink (#198).
//
// Paths are passed as parameters so tests can supply temp dirs.
func runSetupPostWizard(aiRoot, claudeDir, copilotDir string, answers map[string]string, noHooks bool) error {
	// §195 — map answers to settings.toml fields.
	if err := saveWizardSettings(answers); err != nil {
		return fmt.Errorf("setup: save settings: %w", err)
	}

	// Render and write Constitution.md from wizard answers + embedded template.
	as, err := internalwizard.AnswersToAnswerSet(answers)
	if err != nil {
		return fmt.Errorf("setup: map answers to constitution: %w", err)
	}
	tmplBytes, err := embed.ConstitutionTemplate()
	if err != nil {
		return fmt.Errorf("setup: load constitution template: %w", err)
	}
	rendered, err := constitution.Render(as, string(tmplBytes))
	if err != nil {
		return fmt.Errorf("setup: render constitution: %w", err)
	}
	if err := os.MkdirAll(aiRoot, 0o750); err != nil {
		return fmt.Errorf("setup: mkdir airoot: %w", err)
	}
	constitutionPath := filepath.Join(aiRoot, "Constitution.md")
	if err := os.WriteFile(constitutionPath, []byte(rendered), 0o600); err != nil { //nolint:gosec
		return fmt.Errorf("setup: write Constitution.md: %w", err)
	}
	// Generate compact runtime for Copilot/Cursor.
	if rc, err := constitution.ExtractRuntime(rendered); err == nil {
		runtimePath := filepath.Join(aiRoot, "Constitution.runtime.md")
		_ = os.WriteFile(runtimePath, []byte(constitution.FormatRuntime(rc)), 0o600) //nolint:gosec
	}

	if noHooks {
		fmt.Printf("setup: Constitution.md written (%d bytes). Skipping hook install and tool wiring (--no-hooks).\n", len(rendered))
		return nil
	}

	// §196 — extract embedded hooks.
	hooksDir := filepath.Join(aiRoot, "hooks")
	if _, err := embed.ExtractAllHooks(hooksDir, false); err != nil {
		return fmt.Errorf("setup: install hooks: %w", err)
	}

	// §197 — write CLAUDE.md.
	if err := writeClaudeMD(claudeDir, aiRoot); err != nil {
		return fmt.Errorf("setup: write CLAUDE.md: %w", err)
	}

	// §198 — create Copilot symlink.
	if err := installCopilotSymlink(copilotDir, aiRoot); err != nil {
		return fmt.Errorf("setup: install Copilot symlink: %w", err)
	}

	fmt.Println("setup: done — constitution wired. Run `ai doctor` to verify.")
	return nil
}

// saveWizardSettings maps wizard answers to config.Settings fields and
// persists them via config.Save. Only fields that exist in the Settings struct
// are written — no invented struct fields.
//
// Mapping (§195):
//
//	Q01 (principal name) — stored in WizardSettings.LastSeenWizardVersion
//	                       field note: the struct has no PrincipalName field;
//	                       the name is written into Constitution.md instead.
//	wizard version      — WizardSettings.LastSeenWizardVersion stays at "0.2"
//	                       (the embedded questions.yaml version is authoritative).
func saveWizardSettings(answers map[string]string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	// The WizardSettings struct only carries LastSeenWizardVersion.
	// Annotate it with the taxonomy version so `ai review` knows which
	// questions were answered.
	cfg.Wizard.LastSeenWizardVersion = "0.10" // canonical questions.yaml version

	// Map Q09 (autonomy posture) → Focus.DefaultMode as a representative
	// mapping. Both fields exist in the Settings struct.
	if posture, ok := answers["Q09"]; ok && posture == "weaken" {
		cfg.Focus.DefaultMode = "supervised"
	}

	return config.Save(cfg)
}

// writeClaudeMD writes (or updates) ~/.claude/CLAUDE.md so that it contains
// the @-include directive for ~/.ai/Constitution.md.
//
// Idempotent: if the file already contains the directive, it is not added again.
//
// #197 acceptance criteria:
//   - Target: claudeDir/CLAUDE.md
//   - Content: @~/.ai/Constitution.md (and @~/.ai/Constitution.local.md if
//     that file exists)
//   - Idempotent: no duplicate lines on repeated runs
func writeClaudeMD(claudeDir, _ string) error {
	if err := os.MkdirAll(claudeDir, 0o750); err != nil {
		return err
	}
	path := filepath.Join(claudeDir, "CLAUDE.md")

	const primaryInclude = "@~/.ai/Constitution.md"

	// Read existing content if the file already exists.
	existing := ""
	if data, err := os.ReadFile(path); err == nil { //nolint:gosec // G304: path is a controlled value derived from claudeDir
		existing = string(data)
	}

	if strings.Contains(existing, primaryInclude) {
		// Already wired — nothing to do.
		return nil
	}

	// Append to existing content (preserving any user additions).
	var sb strings.Builder
	if existing != "" {
		sb.WriteString(strings.TrimRight(existing, "\n"))
		sb.WriteString("\n")
	}
	sb.WriteString(primaryInclude)
	sb.WriteString("\n")

	return os.WriteFile(path, []byte(sb.String()), 0o640) //nolint:gosec // G306: user config file, 0o640 intentional
}

// installCopilotSymlink creates (or repairs) the symlink:
//
//	~/.copilot/instructions/constitution.md → ~/.ai/Constitution.runtime.md
//
// #198 acceptance criteria:
//   - If the symlink already exists and points to the correct target: no-op.
//   - If the symlink exists but points to a stale target: remove and recreate.
//   - If the symlink does not exist: create it (and any missing parent dirs).
func installCopilotSymlink(copilotDir, aiRoot string) error {
	instructionsDir := filepath.Join(copilotDir, "instructions")
	if err := os.MkdirAll(instructionsDir, 0o750); err != nil {
		return err
	}

	linkPath := filepath.Join(instructionsDir, "constitution.md")
	target := filepath.Join(aiRoot, "Constitution.runtime.md")

	// Check existing symlink.
	existing, err := os.Readlink(linkPath)
	if err == nil {
		if existing == target {
			// Already correct — no-op.
			return nil
		}
		// Stale symlink: remove before recreating.
		if removeErr := os.Remove(linkPath); removeErr != nil {
			return fmt.Errorf("installCopilotSymlink: remove stale link: %w", removeErr)
		}
	} else if !os.IsNotExist(err) {
		// An unexpected error reading the link target (not just "not exist").
		return fmt.Errorf("installCopilotSymlink: readlink: %w", err)
	}

	return os.Symlink(target, linkPath)
}

// runSetupNonInteractive renders the constitution using default answers from
// questions.yaml and wires all tool integrations. Uses the same rendering
// pipeline as the TUI path so the output is always the full template.
func runSetupNonInteractive(_ string) error {
	taxData := embed.QuestionsYAML()
	tax, err := internalwizard.ParseTaxonomy(taxData)
	if err != nil {
		return fmt.Errorf("setup: parse taxonomy: %w", err)
	}
	// Use wizard defaults — every question has a default value.
	answers, err := internalwizard.RunNonInteractive(*tax, nil)
	if err != nil {
		return fmt.Errorf("setup: non-interactive wizard: %w", err)
	}
	aiRoot := paths.AIRoot()
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("setup: resolve HOME: %w", err)
	}
	return runSetupPostWizard(aiRoot,
		filepath.Join(home, ".claude"),
		filepath.Join(home, ".copilot"),
		answers, false)
}
