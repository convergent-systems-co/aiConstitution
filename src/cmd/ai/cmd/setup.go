package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	cbterm "github.com/charmbracelet/x/term"
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
			return runSetupTUI(cmd, noHooks)
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
// cmd is passed so that runSkillSelectionPromptReal can use it for output routing.
func runSetupTUI(cmd *cobra.Command, noHooks bool) error {
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

	// TTY detection: Bubble Tea requires an interactive terminal. If stdout is
	// not a TTY (e.g. piped output, CI, restricted terminal emulators), fall
	// back to the non-interactive path rather than failing with a cryptic error.
	if !cbterm.IsTerminal(os.Stdout.Fd()) {
		fmt.Fprintln(os.Stderr, "Note: non-interactive terminal detected; running in non-interactive mode.")
		fmt.Fprintln(os.Stderr, "For the full TUI wizard, run: ai setup  (in an interactive terminal)")
		return runSetupNonInteractive("") // skill selection is skipped in non-interactive mode
	}

	// Load the embedded questions taxonomy.
	taxData := embed.QuestionsYAML()
	tax, err := internalwizard.ParseTaxonomy(taxData)
	if err != nil {
		return fmt.Errorf("setup: parse taxonomy: %w", err)
	}

	// Reference-first intro: print before launching TUI.
	fmt.Println("┌─────────────────────────────────────────────────────────┐")
	fmt.Println("│  Your AI Constitution is ready.                        │")
	fmt.Println("│                                                         │")
	fmt.Println("│  It already contains:                                   │")
	fmt.Println("│  · 17 universal operating rules (U1–U17)                │")
	fmt.Println("│  · Complete Technical Work practices                    │")
	fmt.Println("│  · Complete Prose & Writing guidelines                  │")
	fmt.Println("│  · Full governance, audit, and amendment protocol       │")
	fmt.Println("│                                                         │")
	fmt.Println("│  We just need 8 things to make it yours.               │")
	fmt.Println("└─────────────────────────────────────────────────────────┘")
	fmt.Println()

	// Run the Bubble Tea program.
	m := tui.NewModel(*tax)
	prog := tea.NewProgram(m)
	finalModel, err := prog.Run()
	if err != nil {
		// TUI failed after TTY check passed (e.g. restricted terminal emulator
		// that reports as a TTY but cannot drive a full TUI). Fall back to the
		// non-interactive path rather than surfacing a confusing Bubble Tea error.
		fmt.Fprintf(os.Stderr, "TUI unavailable (%v); running non-interactive setup.\n", err)
		return runSetupNonInteractive("")
	}
	finalWizard, ok := finalModel.(tui.Model)
	if !ok {
		return fmt.Errorf("setup: unexpected model type %T", finalModel)
	}

	if !finalWizard.Done() {
		fmt.Fprintln(os.Stderr, "setup: wizard exited without completing")
		return nil
	}

	// Hook selection — shown when stdout is a TTY; no-op otherwise.
	// runSetupPostWizard still calls ExtractAllHooks(overwrite=false) so any
	// hooks the user skipped here are installed automatically afterward.
	if !noHooks {
		if selErr := runHookSelectionPromptReal(); selErr != nil {
			fmt.Fprintf(os.Stderr, "setup: hook selection: %v (continuing)\n", selErr)
		}
	}

	// Offer skill selection before wiring constitution files. Non-fatal: if the
	// fetch or any install fails, setup continues to runSetupPostWizard.
	// runSkillSelectionPromptReal is a no-op when stdout is not a terminal.
	if selErr := runSkillSelectionPromptReal(cmd); selErr != nil {
		fmt.Fprintf(os.Stderr, "setup: skill selection: %v (continuing)\n", selErr)
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
	// Create all directories the system writes to on first use so that hooks
	// and commands never hit "no such file or directory" on a fresh install.
	dirs := []string{
		"audit",
		"audit/overrides",
		"audit/violations",
		"audit/interactions",
		"memory",
		"governance",
		"governance/plans",
		"governance/schemas",
		"governance/personas",
		"governance/agentic",
		"checkpoints",
	}
	for _, d := range dirs {
		if err := os.MkdirAll(filepath.Join(aiRoot, d), 0o750); err != nil {
			return fmt.Errorf("setup: mkdir %s: %w", d, err)
		}
	}
	constitutionPath := filepath.Join(aiRoot, "Constitution.md")
	if err := os.WriteFile(constitutionPath, []byte(rendered), 0o600); err != nil { //nolint:gosec
		return fmt.Errorf("setup: write Constitution.md: %w", err)
	}

	// Generate Constitution.compact.md — the compressed form that clients load.
	// This is what Claude Code and Copilot receive; the full Constitution.md is
	// the human-readable source of truth.
	values, _ := extractPersonalValues(aiRoot)
	compact := renderCompactConstitution(values, rendered)
	compactPath := filepath.Join(aiRoot, "Constitution.compact.md")
	if err := os.WriteFile(compactPath, []byte(compact), 0o600); err != nil { //nolint:gosec
		return fmt.Errorf("setup: write Constitution.compact.md: %w", err)
	}

	if noHooks {
		fmt.Printf("setup: Constitution.md written (%d bytes), compact: %d bytes. Skipping wiring (--no-hooks).\n",
			len(rendered), len(compact))
		return nil
	}

	// Fix Python prerequisites before attempting hook installation.
	// On Windows, the Microsoft Store may place zero-byte python.exe stubs that
	// shadow a real Python installation — silently fix them so hooks can run.
	ensurePythonWorks()

	// §196 — full hook install: fetch from catalog + extract infrastructure +
	// wire ~/.claude/settings.json. Non-fatal: a network failure is surfaced as
	// a warning; the user can run 'ai hooks install --all' to retry.
	hooksDir := filepath.Join(aiRoot, "hooks")
	home := homeDir()
	fmt.Println("setup: installing hooks...")
	if err := installAllHooksAndWire(hooksDir, home, false); err != nil {
		fmt.Fprintf(os.Stderr, "setup: warning: hook install incomplete (%v)\n", err)
		fmt.Fprintln(os.Stderr, "       Run 'ai hooks install --all' to retry when online.")
	}

	// Install command-wrappers: git/gh shims appropriate for this OS.
	// On Windows this extracts git.cmd + git.ps1; on POSIX the bash shims.
	fmt.Println("setup: installing command-wrappers...")
	binDir := filepath.Join(aiRoot, "bin")
	if _, err := embed.ExtractWrappers(binDir, false); err != nil {
		fmt.Fprintf(os.Stderr, "setup: warning: command-wrapper install failed: %v\n", err)
	} else {
		fmt.Printf("setup: add %s early to PATH for git/gh interception.\n", binDir)
	}

	// §197-198 — wire each client the user selected in Q36.
	// Q36 is a multi-select: values like "claude-code,copilot-cli,cursor,codex".
	// Default to Claude Code when Q36 is absent (e.g. non-interactive/seeds).
	wireClients(answers, claudeDir, copilotDir, aiRoot)

	// Install Claude Code plugins selected in Q36b/Q36c.
	// Requires the `claude` binary on PATH; non-fatal if absent.
	if tools := answers["Q36"]; strings.Contains(tools, "claude-code") || tools == "" {
		installClaudePlugins(answers)
	}

	fmt.Println("setup: done — constitution wired. Run `ai doctor` to verify.")
	return nil
}

// ensurePythonWorks silently fixes Windows Python App Execution Alias stubs
// before setup attempts to install hooks. On non-Windows this is a no-op.
// It reuses fixWindowsPythonStubs from doctor.go (same package).
func ensurePythonWorks() {
	pyArgs := discoverPythonArgs() // defined in hooks.go
	if pyArgs == nil {
		return // no Python; doctor will surface this later
	}
	out, err := exec.Command(pyArgs[0], "--version").CombinedOutput() //nolint:gosec
	if err != nil || len(out) == 0 {
		if runtime.GOOS == "windows" {
			removed := fixWindowsPythonStubs(os.Stderr)
			if removed > 0 {
				fmt.Fprintf(os.Stderr, "setup: removed %d Python App Execution Alias stub(s).\n", removed)
			}
		}
	}
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

	// Wire the compact form — clients (Claude Code, Copilot) receive the
	// compressed ~8KB form. Constitution.md remains the human-readable source.
	const primaryInclude = "@~/.ai/Constitution.compact.md"

	// Stale includes from earlier layouts that cause silent context failures.
	staleIncludes := map[string]bool{
		"@~/.ai/Constitution.md":  true, // superseded by compact form
		"@~/.ai/Common.md":        true,
		"@~/.ai/Code.md":          true,
		"@~/.ai/Writing.md":       true,
	}

	existing := ""
	if data, err := os.ReadFile(path); err == nil { //nolint:gosec // G304: controlled path
		existing = string(data)
	}

	// Strip stale lines.
	if existing != "" {
		var cleaned []string
		for _, line := range strings.Split(existing, "\n") {
			if !staleIncludes[strings.TrimSpace(line)] {
				cleaned = append(cleaned, line)
			}
		}
		existing = strings.Join(cleaned, "\n")
	}

	if strings.Contains(existing, primaryInclude) {
		// Already wired; write back cleaned version to remove any stale lines.
		return os.WriteFile(path, []byte(existing), 0o640) //nolint:gosec
	}

	var sb strings.Builder
	if existing != "" {
		sb.WriteString(strings.TrimRight(existing, "\n"))
		sb.WriteString("\n")
	}
	sb.WriteString(primaryInclude)
	sb.WriteString("\n")

	return os.WriteFile(path, []byte(sb.String()), 0o640) //nolint:gosec // G306: 0o640 intentional
}

// installCopilotSymlink creates (or repairs) the symlink:
//
//	~/.copilot/instructions/constitution.md → ~/.ai/Constitution.compact.md
//
// Copilot receives the compact form — same as Claude Code — keeping both
// clients in sync and within context budget.
func installCopilotSymlink(copilotDir, aiRoot string) error {
	instructionsDir := filepath.Join(copilotDir, "instructions")
	if err := os.MkdirAll(instructionsDir, 0o750); err != nil {
		return err
	}

	linkPath := filepath.Join(instructionsDir, "constitution.md")
	target := filepath.Join(aiRoot, "Constitution.compact.md")

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

	return symlinkOrCopy(target, linkPath)
}

// runSetupNonInteractive renders the constitution using default answers from
// questions.yaml and wires all tool integrations. Uses the same rendering
// pipeline as the TUI path so the output is always the full template.

// parseSeedsEnv parses AICONST_SEEDS env var into a seed map.
// Format: "Q01=Thomas Polliard,Q02=claude-code,Q06=$5"
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
		if k != "" {
			out[k] = v
		}
	}
	return out
}

func runSetupNonInteractive(_ string) error {
	taxData := embed.QuestionsYAML()
	tax, err := internalwizard.ParseTaxonomy(taxData)
	if err != nil {
		return fmt.Errorf("setup: parse taxonomy: %w", err)
	}
	// Seeds from AICONST_SEEDS env var override defaults.
	// Format: Q01=Thomas Polliard,Q02=claude-code,Q06=$5
	seeds := parseSeedsEnv(os.Getenv("AICONST_SEEDS"))
	answers, err := internalwizard.RunNonInteractive(*tax, seeds)
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

// wireClients reads the Q36 multi-select answer and wires each selected client.
//
//   - claude-code → ~/.claude/CLAUDE.md @-include (global; always wired when selected or when Q36 absent)
//   - copilot-cli → ~/.copilot/instructions/constitution.md symlink (global)
//   - cursor      → per-project: prints a reminder to run `ai init-integrate --cursor` in each repo
//   - codex       → per-project: prints a reminder to run `ai init-integrate --codex` in each repo
//
// If Q36 is absent (non-interactive / seeds that omit it), Claude Code is wired
// as the safe default so setup never produces an unwired installation.
func wireClients(answers map[string]string, claudeDir, copilotDir, aiRoot string) {
	q36 := answers["Q36"]

	// Parse multi-select: "claude-code,copilot-cli" → set
	tools := map[string]bool{}
	for _, t := range strings.Split(q36, ",") {
		tools[strings.TrimSpace(t)] = true
	}

	// Default: wire Claude Code when Q36 is absent.
	if q36 == "" || len(tools) == 0 {
		tools["claude-code"] = true
	}

	if tools["claude-code"] {
		if err := writeClaudeMD(claudeDir, aiRoot); err != nil {
			fmt.Fprintf(os.Stderr, "setup: warning: Claude Code wiring failed: %v\n", err)
		} else {
			fmt.Println("setup: [✓] Claude Code wired (CLAUDE.md → Constitution.compact.md)")
		}
	}

	if tools["copilot-cli"] {
		if err := installCopilotSymlink(copilotDir, aiRoot); err != nil {
			fmt.Fprintf(os.Stderr, "setup: warning: Copilot wiring failed: %v\n", err)
		} else {
			fmt.Println("setup: [✓] GitHub Copilot CLI wired")
		}
	}

	if tools["cursor"] {
		fmt.Println("setup: [i] Cursor is per-repo — run in each project:")
		fmt.Println("         ai init-integrate --cursor")
	}

	if tools["codex"] {
		fmt.Println("setup: [i] Codex/AGENTS.md is per-repo — run in each project:")
		fmt.Println("         ai init-integrate --codex")
	}

	if !tools["claude-code"] && !tools["copilot-cli"] && !tools["cursor"] && !tools["codex"] {
		// Fallback: Q36 had a value we don't recognise (e.g. "other") — wire Claude Code.
		if err := writeClaudeMD(claudeDir, aiRoot); err != nil {
			fmt.Fprintf(os.Stderr, "setup: warning: Claude Code wiring failed: %v\n", err)
		}
	}
}

// installClaudePlugins installs Claude Code plugins based on Q36b/Q36c answers.
// Requires the `claude` CLI on PATH. Non-fatal: prints warnings on failure.
//
// Plugin mapping (Q36c values → marketplace slugs):
//   superpowers       → superpowers@claude-plugins-official
//   amendment-author  → amendment-author@claude-plugins-official
//   hook-author       → hook-author@claude-plugins-official
//   atom-publisher    → atom-publisher@claude-plugins-official
//   review-panel      → review-panel@claude-plugins-official
//   memory-curator    → memory-curator@claude-plugins-official
//
// When Q36b is "pick" and Q36c lists specific plugins, only those are installed.
// When Q36b is absent/skipped and we're on Claude Code, install security-guidance
// (the always-on governance plugin) only.
func installClaudePlugins(answers map[string]string) {
	claudePath, err := exec.LookPath("claude")
	if err != nil {
		return // claude CLI not on PATH — skip silently
	}

	const marketplace = "claude-plugins-official"

	// Always ensure the official marketplace is registered.
	ensureMarketplace := func() bool {
		check := exec.Command(claudePath, "plugin", "marketplace", "list") //nolint:gosec
		out, _ := check.Output()
		if strings.Contains(string(out), marketplace) {
			return true
		}
		add := exec.Command(claudePath, "plugin", "marketplace", "add", "anthropics/"+marketplace) //nolint:gosec
		add.Stdout = os.Stderr // progress to stderr
		add.Stderr = os.Stderr
		if err := add.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "setup: warning: could not add Claude plugin marketplace: %v\n", err)
			return false
		}
		return true
	}

	install := func(slug string) {
		full := slug + "@" + marketplace
		c := exec.Command(claudePath, "plugin", "install", full) //nolint:gosec
		if out, err := c.CombinedOutput(); err != nil {
			// Ignore "already installed" errors.
			if !strings.Contains(string(out), "already installed") {
				fmt.Fprintf(os.Stderr, "setup: warning: could not install plugin %s: %v\n", full, err)
			}
		} else {
			fmt.Printf("setup: [✓] Installed Claude plugin: %s\n", slug)
		}
	}

	q36b := strings.ToLower(answers["Q36b"])
	q36c := strings.ToLower(answers["Q36c"])

	// security-guidance is always installed for Claude Code users.
	if !ensureMarketplace() {
		return
	}
	install("security-guidance")

	if q36b != "pick" {
		return // user opted out of additional plugins
	}

	// Install plugins the user selected in Q36c.
	pluginMap := map[string]string{
		"superpowers":      "superpowers",
		"amendment-author": "amendment-author",
		"hook-author":      "hook-author",
		"atom-publisher":   "atom-publisher",
		"review-panel":     "review-panel",
		"memory-curator":   "memory-curator",
	}
	for key, slug := range pluginMap {
		if strings.Contains(q36c, key) {
			install(slug)
		}
	}
}
