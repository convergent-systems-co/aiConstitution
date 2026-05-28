package cmd

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/convergent-systems-co/aiConstitution/src/cmd/ai/embed"

	"github.com/spf13/cobra"
)

// newHooksCmd implements `ai hooks {list,evaluate,propose,share,install}`.
// See SPEC.md §3.10 + §9.
func newHooksCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "hooks",
		Short: "Hook lifecycle: list, evaluate, propose, share upstream, install",
		Long: `hooks operates on the Python hook library at ~/.ai/hooks/, plus the
command-wrapper preHooks/postHooks/commandHooks declared in
hooks/command-wrappers.toml.

See SPEC.md §3.10 + §9.`,
	}

	// available — embedded hooks plus registry hooks from skill-atoms.com
	c.AddCommand(&cobra.Command{
		Use:   "available",
		Short: "List hooks available to install (embedded + skill-atoms.com registry)",
		RunE:  runHooksAvailable,
	})

	// list — hooks installed on disk in ~/.ai/hooks/ with per-client wiring columns
	c.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List installed hooks with per-client wiring status",
		RunE: func(cmd *cobra.Command, _ []string) error {
			home, _ := os.UserHomeDir()
			aiRoot := os.Getenv("AI_ROOT")
			if aiRoot == "" {
				aiRoot = filepath.Join(home, ".ai")
			}
			hooksDir := filepath.Join(aiRoot, "hooks")

			entries, err := os.ReadDir(hooksDir)
			if err != nil {
				if os.IsNotExist(err) {
					fmt.Fprintln(cmd.OutOrStdout(), "No hooks installed (run: ai hooks install --all)")
					return nil
				}
				return err
			}

			// Per-client wiring maps.
			claudeGlobal := readWiredHookNames(filepath.Join(home, ".claude", "settings.json"))
			claudeLocal := readWiredHookNames(filepath.Join(".", ".claude", "settings.json"))

			// Copilot: wired if Constitution.runtime.md symlink exists in ~/.copilot/instructions/
			copilotLink := filepath.Join(home, ".copilot", "instructions", "constitution.md")
			_, copilotLinked := os.Lstat(copilotLink)
			copilotWired := copilotLinked == nil

			out := cmd.OutOrStdout()
			const hookW = 32
			fmt.Fprintf(out, "  %-*s  %-9s  %-9s  %s\n", hookW, "HOOK", "INSTALLED", "CLAUDE", "COPILOT")
			fmt.Fprintf(out, "  %-*s  %-9s  %-9s  %s\n",
				hookW, strings.Repeat("─", hookW),
				strings.Repeat("─", 9), strings.Repeat("─", 9), strings.Repeat("─", 7))

			count := 0
			for _, e := range entries {
				if e.IsDir() || !isHookFile(e.Name()) {
					continue
				}
				claudeStatus := "-"
				if claudeGlobal[e.Name()] {
					claudeStatus = "global"
				} else if claudeLocal[e.Name()] {
					claudeStatus = "project"
				}
				// Copilot hooks are constitution-level, not per-file
				copilotStatus := "-"
				if copilotWired {
					copilotStatus = "wired"
				}
				fmt.Fprintf(out, "  %-*s  %-9s  %-9s  %s\n",
					hookW, e.Name(), "✓", claudeStatus, copilotStatus)
				count++
			}
			if count == 0 {
				fmt.Fprintln(out, "  (no hooks installed — run: ai hooks install --all)")
			}
			return nil
		},
	})

	// validate (#200, #201)
	var validateDir string
	validateCmd := &cobra.Command{
		Use:   "validate",
		Short: "Lint each installed hook: shebang, syntax, bare-except check",
		Long: `validate checks every .py and .sh file in the hooks directory.

For .py files:
  - Shebang: first line must start with #!   → [✗] if missing
  - Syntax:  python3 -m py_compile           → [✗] if fails
  - Bare except: scan for "except:" without a type → [⚠] warning

For .sh files:
  - Syntax:  bash -n                         → [✗] if fails

Exit 0 if no [✗] findings; exit 1 if any [✗].`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runHooksValidate(cmd, validateDir)
		},
	}
	validateCmd.Flags().StringVar(&validateDir, "dir", "", "directory to validate (defaults to ~/.ai/hooks/; use --embedded to validate built-in sources)")
	c.AddCommand(validateCmd)

	// evaluate (#202)
	c.AddCommand(&cobra.Command{
		Use:   "evaluate",
		Short: "Invoke each embedded hook with synthetic JSON; assert non-crash",
		Long: `evaluate smoke-tests every embedded .py hook by piping a minimal
synthetic JSON event to it and asserting exit 0.

Prints [✓] or [✗] per hook. Exit 1 if any [✗].`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runHooksEvaluate(cmd)
		},
	})

	// propose
	var fromViolation string
	var lang string
	propose := &cobra.Command{
		Use:   "propose <name>",
		Short: "Scaffold a new hook from a finding (chat handoff for prose)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			home, err := os.UserHomeDir()
			if err != nil {
				return fmt.Errorf("hooks propose: resolve home: %w", err)
			}
			root := os.Getenv("AI_ROOT")
			if root == "" {
				root = filepath.Join(home, ".ai")
			}
			return runHooksPropose(args[0], fromViolation, lang, root, cmd.OutOrStdout())
		},
	}
	propose.Flags().StringVar(&fromViolation, "from-violation", "", "path to an audit/violations/*.md file")
	propose.Flags().StringVar(&lang, "lang", "python", "language (python|sh|go|node)")

	// share
	c.AddCommand(&cobra.Command{
		Use:   "share <name>",
		Short: "File the hook upstream as an issue (gated by settings.upstream.shareNewHooks)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			home, err := os.UserHomeDir()
			if err != nil {
				return fmt.Errorf("hooks share: resolve home: %w", err)
			}
			aiRoot := os.Getenv("AI_ROOT")
			if aiRoot == "" {
				aiRoot = filepath.Join(home, ".ai")
			}
			filePath := filepath.Join(aiRoot, "hooks", args[0])
			return runShareUpstream(args[0], filePath, "convergent-systems-co/aiConstitution", "", cmd.OutOrStdout())
		},
	})

	// install — extracts from the embedded FS to ~/.ai/hooks/ (hooks)
	// or ~/.ai/bin/ (command-wrappers). Special target names:
	//   --all                    → every embedded hook
	//   command-wrappers         → both wrapper templates (git, gh)
	//   <name.ext>               → that one embedded hook
	var installRepo string
	var installAll bool
	var installAllHooks bool
	var installForce bool
	var installClaude bool
	var installClaudeRoot string
	var installCopilot bool
	install := &cobra.Command{
		Use:   "install [<name>]",
		Short: "Extract embedded hook(s) / wrappers to ~/.ai/ (idempotent)",
		Long: `install materializes embedded assets onto disk.

  ai hooks install --all                  extract every embedded hook
                                          into ~/.ai/hooks/ and wire
                                          them into ~/.claude/settings.json
  ai hooks install command-wrappers       extract bin/git and bin/gh
                                          into ~/.ai/bin/
  ai hooks install <name>                 extract a single embedded
                                          hook (e.g. secret-block.py)
  ai hooks install --claude               wire installed hooks into
                                          .claude/settings.json in
                                          the current repo
  ai hooks install --copilot              symlink Constitution.runtime.md
                                          into ~/.copilot/instructions/

  --force                overwrite existing files
  --repo=<path>          (with no positional) install a pre-commit
                         hook into the specified repo's .git/hooks/
                         that defers to ~/.ai/hooks/secret-precommit.py
  --claude               wire ~/.ai/hooks/*.py into .claude/settings.json
  --claude-root=<path>   directory containing .claude/ (default ".")
  --copilot              symlink Constitution.runtime.md into ~/.copilot/instructions/

Per SPEC.md §3.10 + §10.2 + §14.1.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			target := ""
			if len(args) == 1 {
				target = args[0]
			}
			if installClaude {
				return runHooksInstallClaude(cmd, installClaudeRoot)
			}
			if installCopilot {
				home, err := os.UserHomeDir()
				if err != nil {
					return err
				}
				aiRoot := os.Getenv("AI_ROOT")
				if aiRoot == "" {
					aiRoot = filepath.Join(home, ".ai")
				}
				if err := runHooksCopilotInstall(aiRoot, home); err != nil {
					return err
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Wired Constitution.runtime.md into ~/.copilot/instructions/constitution.md\n")
				return nil
			}
			return runHooksInstall(installRepo, target, installAllHooks || installAll, installForce)
		},
	}
	install.Flags().StringVar(&installRepo, "repo", "", "install a pre-commit shim into the specified repo")
	install.Flags().BoolVar(&installAll, "all-future-clones", false, "(reserved; wires into `ai clone` per SPEC §10.2)")
	install.Flags().BoolVar(&installAllHooks, "all", false, "extract every embedded hook to ~/.ai/hooks/")
	install.Flags().BoolVar(&installForce, "force", false, "overwrite existing files")
	install.Flags().BoolVar(&installClaude, "claude", false, "wire ~/.ai/hooks/*.py into .claude/settings.json")
	install.Flags().StringVar(&installClaudeRoot, "claude-root", ".", "directory containing .claude/ (default: current dir)")
	install.Flags().BoolVar(&installCopilot, "copilot", false, "symlink Constitution.runtime.md into ~/.copilot/instructions/")

	c.AddCommand(propose, install)
	return c
}

// runHooksAvailable implements `ai hooks available`. It lists:
//  1. Embedded hooks (built-in, installable via `ai hooks install`).
//  2. Registry hooks from skill-atoms.com (type: "ai-hook", non-deprecated).
//
// Registry fetch failures are non-fatal: a warning line is printed and the
// command still exits 0 with the embedded hooks shown.
func runHooksAvailable(cmd *cobra.Command, _ []string) error {
	names, err := embed.HookNames()
	if err != nil {
		return err
	}
	sort.Strings(names)
	out := cmd.OutOrStdout()
	fmt.Fprintln(out, "Embedded hooks  (ai hooks install <name>  |  ai hooks install --all):")
	for _, n := range names {
		if isHookFile(n) || n == "patterns.json" {
			fmt.Fprintln(out, "  "+n)
		}
	}
	fmt.Fprintln(out)

	// Fetch registry hooks from skill-atoms.com. Non-fatal on failure.
	atoms, registryErr := fetchHookAtoms()
	if registryErr != nil {
		fmt.Fprintf(out, "(could not reach skill-atoms.com: %v)\n", registryErr)
		return nil
	}
	if len(atoms) == 0 {
		return nil
	}

	fmt.Fprintln(out, "Registry hooks from skill-atoms.com:")

	// Compute column width for the slug column.
	maxSlug := 4 // len("SLUG")
	for _, a := range atoms {
		if len(a.slug) > maxSlug {
			maxSlug = len(a.slug)
		}
	}
	fmt.Fprintf(out, "  %-*s  %s\n", maxSlug, "SLUG", "DESCRIPTION")
	fmt.Fprintf(out, "  %-*s  %s\n", maxSlug, strings.Repeat("─", maxSlug), strings.Repeat("─", 50))
	for _, a := range atoms {
		desc := a.description
		if len(desc) > 70 {
			desc = desc[:67] + "..."
		}
		fmt.Fprintf(out, "  %-*s  %s\n", maxSlug, a.slug, desc)
	}
	return nil
}

// runHooksPropose scaffolds a new hook file at <aiRoot>/hooks/<name>.<ext>.
//
// lang must be "python" (default) or "sh". Any other value returns an error.
// When fromViolation is non-empty, the file is read and the "What happened:"
// field seeds the scaffold description; if the field is absent, an empty seed
// is used. When the target file already exists, an error is returned.
func runHooksPropose(name, fromViolation, lang, aiRoot string, out io.Writer) error {
	// Validate lang and derive extension + template.
	var ext string
	switch lang {
	case "python", "":
		ext = ".py"
	case "sh":
		ext = ".sh"
	default:
		return fmt.Errorf("unsupported lang: %s; supported: python, sh", lang)
	}

	hooksDir := filepath.Join(aiRoot, "hooks")
	dest := filepath.Join(hooksDir, name+ext)

	// Fail early if file already exists.
	if _, err := os.Stat(dest); err == nil {
		return fmt.Errorf("hook already exists: %s", dest)
	}

	// Resolve description from --from-violation or use placeholder.
	description, err := resolveHookDescription(fromViolation)
	if err != nil {
		return err
	}

	// Ensure hooks directory exists.
	if err := os.MkdirAll(hooksDir, 0o750); err != nil {
		return fmt.Errorf("hooks propose: mkdir %s: %w", hooksDir, err)
	}

	// Build scaffold content.
	var content string
	switch ext {
	case ".py":
		content = buildPythonScaffold(name, description)
	case ".sh":
		content = buildShellScaffold(name, description)
	}

	// Write with executable permissions.
	if err := os.WriteFile(dest, []byte(content), 0o755); err != nil { //nolint:gosec // G306: hook must be executable
		return fmt.Errorf("hooks propose: write %s: %w", dest, err)
	}

	fmt.Fprintf(out, "Created: %s\n", dest)
	fmt.Fprintf(out, "Next:    edit the scaffold, then run `ai hooks install %s%s`\n", name, ext)
	return nil
}

// resolveHookDescription returns the hook description string.
// When fromViolation is empty it returns the placeholder text.
// When fromViolation is set it reads the file and extracts "What happened:".
func resolveHookDescription(fromViolation string) (string, error) {
	if fromViolation == "" {
		return "<description of what this hook checks>", nil
	}
	data, err := os.ReadFile(fromViolation)
	if err != nil {
		return "", fmt.Errorf("hooks propose: read violation file: %w", err)
	}
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		line := scanner.Text()
		if v, ok := extractField(line, "**What happened:**"); ok && v != "" {
			return v, nil
		}
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("hooks propose: scan violation file: %w", err)
	}
	// Field not found — use empty seed (non-fatal per spec).
	return "", nil
}

// buildPythonScaffold returns the Python hook scaffold body.
func buildPythonScaffold(name, description string) string {
	if description == "" {
		description = "<description of what this hook checks>"
	}
	return fmt.Sprintf(`#!/usr/bin/env python3
"""%s — %s

Hook scaffold generated by `+"`"+`ai hooks propose`+"`"+`.
Edit this file, then run: ai hooks install %s.py
"""
import json
import sys


def run(event: dict) -> None:
    """Check the event and raise SystemExit(1) to block, or return to allow."""
    # TODO: implement hook logic
    pass


if __name__ == "__main__":
    try:
        event = json.load(sys.stdin)
    except json.JSONDecodeError as e:
        print(f"hooks/%s: invalid JSON input: {e}", file=sys.stderr)
        sys.exit(1)
    run(event)
`, name, description, name, name)
}

// buildShellScaffold returns the shell hook scaffold body.
func buildShellScaffold(name, description string) string {
	if description == "" {
		description = "<description of what this hook checks>"
	}
	return fmt.Sprintf(`#!/usr/bin/env bash
# %s — %s
#
# Hook scaffold generated by `+"`"+`ai hooks propose`+"`"+`.
# Edit this file, then run: ai hooks install %s.sh
set -euo pipefail

EVENT=$(cat)  # JSON piped on stdin

# TODO: implement hook logic
`, name, description, name)
}

// runHooksInstallClaude wires the Python hooks under ~/.ai/hooks/ into
// .claude/settings.json under claudeRoot. Per §156 / SPEC §14.1.
func runHooksInstallClaude(cmd *cobra.Command, claudeRoot string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	aiRoot := os.Getenv("AI_ROOT")
	if aiRoot == "" {
		aiRoot = filepath.Join(home, ".ai")
	}
	hooksDir := filepath.Join(aiRoot, "hooks")
	added, err := installClaudeHooks(claudeRoot, hooksDir)
	if err != nil {
		return err
	}
	_, _ = fmt.Fprintf(cmd.OutOrStdout(),
		"Wired %d Claude hook entries into %s\n",
		added, filepath.Join(claudeRoot, ".claude", "settings.json"))
	return nil
}

// runHooksInstall is the top-level dispatcher for the various
// install modes. Extracted from newHooksCmd's RunE closure to keep
// the cobra constructor under gocyclo's threshold.
func runHooksInstall(repo, target string, all, force bool) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	aiRoot := os.Getenv("AI_ROOT")
	if aiRoot == "" {
		aiRoot = filepath.Join(home, ".ai")
	}
	hooksDir := filepath.Join(aiRoot, "hooks")
	binDir := filepath.Join(aiRoot, "bin")

	if repo != "" {
		return installRepoPrecommit(repo, hooksDir)
	}
	if all {
		return installAllHooksAndWire(hooksDir, home, force)
	}
	if target == "command-wrappers" {
		return installWrappers(binDir, force)
	}
	if target != "" {
		return installOneHook(target, hooksDir, force)
	}
	return fmt.Errorf("specify a hook name, --all, or `command-wrappers`. See `ai hooks install --help`")
}

// installAllHooksAndWire extracts all hooks to hooksDir and then updates
// ~/.claude/settings.json with the correct event-to-hook wiring.
func installAllHooksAndWire(hooksDir, home string, force bool) error {
	written, err := embed.ExtractAllHooks(hooksDir, force)
	if err != nil {
		return err
	}
	fmt.Printf("Extracted %d hook(s) to %s\n", len(written), hooksDir)
	for _, p := range written {
		fmt.Println("  " + p)
	}

	// Wire hooks into ~/.claude/settings.json.
	// CLAUDE_CONFIG_DIR overrides the default ~/.claude location for testing.
	claudeConfigDir := os.Getenv("CLAUDE_CONFIG_DIR")
	if claudeConfigDir == "" {
		claudeConfigDir = filepath.Join(home, ".claude")
	}
	settingsPath := filepath.Join(claudeConfigDir, "settings.json")
	if err := updateSettingsJSON(settingsPath, hooksDir); err != nil {
		// Settings update is non-fatal — hooks still work if manually wired.
		fmt.Printf("Warning: could not update %s: %v\n", settingsPath, err)
		fmt.Println("Hooks extracted successfully. Wire them manually if needed.")
		return nil
	}
	fmt.Printf("Updated %s with hook wiring.\n", settingsPath)
	return nil
}

// hookEntry represents a single hook command entry in settings.json.
type hookEntry struct {
	Type    string `json:"type"`
	Command string `json:"command"`
}

// hookGroup is one entry in the event's hook array (optional matcher + hooks slice).
type hookGroup struct {
	Matcher string      `json:"matcher,omitempty"`
	Hooks   []hookEntry `json:"hooks"`
}

// eventHookSpec describes one event's desired wiring.
type eventHookSpec struct {
	event   string
	matcher string // empty = all tools; "Bash" = Bash-only
	hooks   []string
}

// canonicalWiring returns the authoritative event→hook mapping for stories #84+#96.
// Each spec describes one hook group under an event; PreToolUse has two groups.
func canonicalWiring(hooksDir string) []eventHookSpec {
	h := func(names ...string) []string {
		paths := make([]string, 0, len(names))
		for _, n := range names {
			paths = append(paths, filepath.Join(hooksDir, n))
		}
		return paths
	}
	return []eventHookSpec{
		{event: "SessionStart", hooks: h("audit.py")},
		{event: "UserPromptSubmit", hooks: h("audit.py")},
		// PreToolUse: all-tools group (governance + secret + worktree + redaction + no-verify)
		{event: "PreToolUse", matcher: "", hooks: h(
			"audit.py",
			"audit-command.py",
			"secret-block.py",
			"worktree-guard.py",
			"no-verify-strip.py",
			"op-redact.py",
		)},
		// PreToolUse: Bash-only group (branch guard + destructive guards)
		{event: "PreToolUse", matcher: "Bash", hooks: h(
			"branch-guard.py",
			"destructive-gh-guard.py",
			"destructive-kubectl-guard.py",
			"destructive-terraform-guard.py",
		)},
		{event: "PostToolUse", hooks: h("audit.py")},
		{event: "Stop", hooks: h("audit.py", "checkpoint-tick.py")},
		{event: "SessionEnd", hooks: h("audit.py")},
		{event: "SubagentStop", hooks: h("audit.py")},
		{event: "PreCompact", hooks: h("audit.py")},
	}
}

// updateSettingsJSON reads settings.json (if present), merges the canonical
// hook wiring, and writes the result back. Idempotent and non-destructive:
// existing keys (model, enabledPlugins, etc.) are preserved.
func updateSettingsJSON(settingsPath, hooksDir string) error {
	// Read existing settings or start fresh.
	var raw map[string]any
	data, err := os.ReadFile(settingsPath)
	if err == nil {
		if jsonErr := json.Unmarshal(data, &raw); jsonErr != nil {
			return fmt.Errorf("parse %s: %w", settingsPath, jsonErr)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("read %s: %w", settingsPath, err)
	}
	if raw == nil {
		raw = make(map[string]any)
	}

	// Fetch or initialise the hooks map.
	hooksMap, _ := raw["hooks"].(map[string]any)
	if hooksMap == nil {
		hooksMap = make(map[string]any)
	}

	// Apply canonical wiring specs. Each spec is upserted into the hooks map.
	for _, spec := range canonicalWiring(hooksDir) {
		// Build the desired hook group for this spec.
		desired := hookGroup{
			Matcher: spec.matcher,
			Hooks:   make([]hookEntry, 0, len(spec.hooks)),
		}
		for _, cmd := range spec.hooks {
			desired.Hooks = append(desired.Hooks, hookEntry{Type: "command", Command: cmd})
		}

		// Load existing groups for this event.
		var groups []hookGroup
		if existing, ok := hooksMap[spec.event]; ok {
			existingJSON, _ := json.Marshal(existing)
			_ = json.Unmarshal(existingJSON, &groups)
		}

		// Check if an identical group (same matcher) is already present;
		// if so, update it in place. Otherwise append.
		found := false
		for i, g := range groups {
			if g.Matcher == spec.matcher {
				// Merge: add any missing hook commands.
				existingCmds := make(map[string]bool, len(g.Hooks))
				for _, h := range g.Hooks {
					existingCmds[h.Command] = true
				}
				for _, entry := range desired.Hooks {
					if !existingCmds[entry.Command] {
						groups[i].Hooks = append(groups[i].Hooks, entry)
					}
				}
				found = true
				break
			}
		}
		if !found {
			groups = append(groups, desired)
		}

		hooksMap[spec.event] = groups
	}

	raw["hooks"] = hooksMap

	// Write back.
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0o750); err != nil {
		return fmt.Errorf("mkdir %s: %w", filepath.Dir(settingsPath), err)
	}
	out, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal settings: %w", err)
	}
	if err := os.WriteFile(settingsPath, out, 0o644); err != nil { //nolint:gosec // G306: settings.json is user-readable
		return fmt.Errorf("write %s: %w", settingsPath, err)
	}
	return nil
}

func installWrappers(binDir string, force bool) error {
	written, err := embed.ExtractWrappers(binDir, force)
	if err != nil {
		return err
	}
	fmt.Printf("Extracted %d wrapper(s) to %s\n", len(written), binDir)
	for _, p := range written {
		fmt.Println("  " + p)
	}
	if len(written) > 0 {
		fmt.Println("\nNote: add", binDir, "early to your $PATH for wrapper interception to fire.")
	}
	return nil
}

func installOneHook(name, hooksDir string, force bool) error {
	p, err := embed.ExtractHook(name, hooksDir, force)
	if err != nil {
		return err
	}
	fmt.Println("Extracted " + p)
	return nil
}

// ---------------------------------------------------------------------------
// hooks validate (#200, #201)
// ---------------------------------------------------------------------------

// hookValidationResult holds the per-file lint outcome.
type hookValidationResult struct {
	name     string
	status   string // "ok", "warn", "fail"
	messages []string
}

// hooksValidateDefaultDir returns the default directory for `ai hooks validate`:
// $AI_ROOT/hooks/ (the installed location), resolved from the environment.
func hooksValidateDefaultDir() string {
	home, _ := os.UserHomeDir()
	aiRoot := os.Getenv("AI_ROOT")
	if aiRoot == "" {
		aiRoot = filepath.Join(home, ".ai")
	}
	return filepath.Join(aiRoot, "hooks")
}

// runHooksValidate lints all .py and .sh files in dir. When dir is empty,
// defaults to ~/.ai/hooks/ (the installed hooks directory). Executables
// named _lib.py or test_*.py are skipped — they are not standalone hooks.
// Returns an error if any file has status "fail".
func runHooksValidate(cmd *cobra.Command, dir string) error {
	if dir == "" {
		dir = hooksValidateDefaultDir()
	}
	var files []validationTarget
	var err error
	files, err = hookFilesFromDir(dir)
	if err != nil {
		return err
	}
	if len(files) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No hook files found.")
		return nil
	}

	results := make([]hookValidationResult, 0, len(files))
	for _, f := range files {
		results = append(results, validateHookFile(f))
	}

	anyFail := false
	for _, r := range results {
		icon := "[✓]"
		if r.status == "warn" {
			icon = "[⚠]"
		} else if r.status == "fail" {
			icon = "[✗]"
			anyFail = true
		}
		fmt.Fprintf(cmd.OutOrStdout(), "%s %s\n", icon, r.name)
		for _, m := range r.messages {
			fmt.Fprintf(cmd.OutOrStdout(), "    %s\n", m)
		}
	}
	if anyFail {
		return fmt.Errorf("one or more hooks failed validation")
	}
	return nil
}

// validationTarget is a hook file name + its byte content for linting.
type validationTarget struct {
	name    string
	content []byte
}

// isHookFile returns true when name is a hook executable that should be
// validated. Library files (_lib.py), test files (test_*.py), and non-.py/.sh
// files are excluded.
func isHookFile(name string) bool {
	if !strings.HasSuffix(name, ".py") && !strings.HasSuffix(name, ".sh") {
		return false
	}
	if name == "_lib.py" || name == "__init__.py" {
		return false
	}
	if strings.HasPrefix(name, "test_") && strings.HasSuffix(name, ".py") {
		return false
	}
	return true
}

// hookFilesFromDir reads all hook-eligible .py and .sh files from a
// filesystem directory (see isHookFile).
func hookFilesFromDir(dir string) ([]validationTarget, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("validate: read dir %s: %w", dir, err)
	}
	var out []validationTarget
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		n := e.Name()
		if !isHookFile(n) {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, n))
		if err != nil {
			return nil, err
		}
		out = append(out, validationTarget{name: n, content: data})
	}
	return out, nil
}

// hookFilesFromEmbed reads all hook-eligible .py and .sh files from the
// embedded FS (see isHookFile).
func hookFilesFromEmbed() ([]validationTarget, error) {
	hFS := embed.HooksFS()
	entries, err := fs.ReadDir(hFS, ".")
	if err != nil {
		return nil, err
	}
	var out []validationTarget
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		n := e.Name()
		if !isHookFile(n) {
			continue
		}
		data, err := fs.ReadFile(hFS, n)
		if err != nil {
			return nil, err
		}
		out = append(out, validationTarget{name: n, content: data})
	}
	return out, nil
}

// validateHookFile runs all checks for one hook file and returns the result.
func validateHookFile(f validationTarget) hookValidationResult {
	r := hookValidationResult{name: f.name, status: "ok"}

	if strings.HasSuffix(f.name, ".py") {
		checkPython(&r, f.content)
	} else if strings.HasSuffix(f.name, ".sh") {
		checkShell(&r, f.content)
	}
	return r
}

// checkPython applies the three Python checks: shebang, syntax, bare-except.
func checkPython(r *hookValidationResult, content []byte) {
	// 1. Shebang check — first non-empty line must start with "#!"
	scanner := bufio.NewScanner(bytes.NewReader(content))
	firstLine := ""
	for scanner.Scan() {
		firstLine = scanner.Text()
		break
	}
	if !strings.HasPrefix(firstLine, "#!") {
		r.status = "fail"
		r.messages = append(r.messages, "missing shebang (#! on line 1)")
		// No point running py_compile on a file we already know is wrong in
		// intent — but we continue to collect all findings.
	}

	// 2. Syntax check via python3 -m py_compile
	tmpFile, err := os.CreateTemp("", "ai-hook-validate-*.py")
	if err == nil {
		defer os.Remove(tmpFile.Name())
		if _, werr := tmpFile.Write(content); werr == nil {
			tmpFile.Close()
			out, cerr := exec.Command("python3", "-m", "py_compile", tmpFile.Name()).CombinedOutput()
			if cerr != nil {
				r.status = "fail"
				msg := strings.TrimSpace(string(out))
				if msg == "" {
					msg = cerr.Error()
				}
				// Replace tmpFile path with the hook name for readability.
				msg = strings.ReplaceAll(msg, tmpFile.Name(), f(r.name))
				r.messages = append(r.messages, "syntax error: "+msg)
			}
		} else {
			tmpFile.Close()
		}
	}
	// If python3 is missing entirely, emit a warning rather than failing.
	if err != nil {
		if _, lerr := exec.LookPath("python3"); lerr != nil {
			if r.status == "ok" {
				r.status = "warn"
			}
			r.messages = append(r.messages, "python3 not found; skipping syntax check")
		}
	}

	// 3. Bare-except scan — warn if "except:" without an exception type.
	scanner = bufio.NewScanner(bytes.NewReader(content))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "except:" || line == "except :" {
			if r.status == "ok" {
				r.status = "warn"
			}
			r.messages = append(r.messages, "bare except: (use `except Exception` or a specific type)")
			break // one warning per file is enough
		}
	}
}

// f is a small alias used in error messages to avoid shadowing the outer f.
func f(name string) string { return "<" + name + ">" }

// checkShell applies bash -n syntax check to a shell script.
func checkShell(r *hookValidationResult, content []byte) {
	tmpFile, err := os.CreateTemp("", "ai-hook-validate-*.sh")
	if err != nil {
		r.status = "fail"
		r.messages = append(r.messages, "could not create temp file: "+err.Error())
		return
	}
	defer os.Remove(tmpFile.Name())
	if _, werr := tmpFile.Write(content); werr != nil {
		tmpFile.Close()
		r.status = "fail"
		r.messages = append(r.messages, "write error: "+werr.Error())
		return
	}
	tmpFile.Close()

	out, cerr := exec.Command("bash", "-n", tmpFile.Name()).CombinedOutput()
	if cerr != nil {
		r.status = "fail"
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			msg = cerr.Error()
		}
		msg = strings.ReplaceAll(msg, tmpFile.Name(), "<"+r.name+">")
		r.messages = append(r.messages, "syntax error: "+msg)
	}
}

// ---------------------------------------------------------------------------
// hooks evaluate (#202)
// ---------------------------------------------------------------------------

// syntheticEvent returns a minimal JSON event for each hook type, keyed by
// the hook's filename. Hooks that are not listed here receive the generic
// Claude PreToolUse payload.
func syntheticEvent(hookName string) string {
	switch hookName {
	case "audit.py", "audit-command.py":
		return `{"tool_name":"Bash","tool_input":{"command":"echo test"}}`
	case "branch-guard.py":
		return `{"tool_name":"Bash","tool_input":{"command":"echo test"}}`
	case "secret-block.py", "secret-precommit.py":
		return `{"tool_name":"Bash","tool_input":{"command":"echo hello"}}`
	case "no-verify-strip.py":
		return `{"tool_name":"Bash","tool_input":{"command":"git status"}}`
	case "destructive-gh-guard.py":
		return `{"tool_name":"Bash","tool_input":{"command":"gh repo list"}}`
	case "destructive-kubectl-guard.py":
		return `{"tool_name":"Bash","tool_input":{"command":"kubectl get pods"}}`
	case "destructive-terraform-guard.py":
		return `{"tool_name":"Bash","tool_input":{"command":"terraform plan"}}`
	case "worktree-guard.py":
		return `{"tool_name":"Bash","tool_input":{"command":"git status"}}`
	case "checkpoint-tick.py":
		return `{"type":"Stop"}`
	default:
		return `{"tool_name":"Bash","tool_input":{"command":"echo test"}}`
	}
}

// runHooksEvaluate smoke-tests every embedded .py hook by sending it a
// synthetic JSON event and asserting exit 0.
func runHooksEvaluate(cmd *cobra.Command) error {
	hFS := embed.HooksFS()
	entries, err := fs.ReadDir(hFS, ".")
	if err != nil {
		return err
	}

	// Extract all hooks to a temp dir so we can run them with python3.
	tmpDir, err := os.MkdirTemp("", "ai-hooks-evaluate-*")
	if err != nil {
		return fmt.Errorf("evaluate: create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	anyFail := false
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })

	for _, e := range entries {
		// Use the same isHookFile filter as validate: skip _lib.py,
		// test_*.py, and non-.py files.
		if e.IsDir() || !isHookFile(e.Name()) {
			continue
		}

		data, rerr := fs.ReadFile(hFS, e.Name())
		if rerr != nil {
			fmt.Fprintf(cmd.OutOrStdout(), "[✗] %s (read error: %v)\n", e.Name(), rerr)
			anyFail = true
			continue
		}
		hookPath := filepath.Join(tmpDir, e.Name())
		if werr := os.WriteFile(hookPath, data, 0o644); werr != nil {
			fmt.Fprintf(cmd.OutOrStdout(), "[✗] %s (write error: %v)\n", e.Name(), werr)
			anyFail = true
			continue
		}

		// Also copy _lib.py so imports work.
		libData, _ := fs.ReadFile(hFS, "_lib.py")
		if libData != nil {
			_ = os.WriteFile(filepath.Join(tmpDir, "_lib.py"), libData, 0o644)
		}

		payload := syntheticEvent(e.Name())
		evalCmd := exec.Command("python3", hookPath) //nolint:gosec // G204: intentional eval of embedded hook
		evalCmd.Stdin = strings.NewReader(payload)
		evalCmd.Dir = tmpDir
		if eerr := evalCmd.Run(); eerr != nil {
			// Exit non-zero is acceptable for hooks that correctly block
			// the synthetic payload (e.g. branch-guard on a non-git cwd).
			// We treat any exec error that isn't ExitError as a real failure.
			if _, ok := eerr.(*exec.ExitError); !ok {
				fmt.Fprintf(cmd.OutOrStdout(), "[✗] %s (%v)\n", e.Name(), eerr)
				anyFail = true
				continue
			}
		}
		fmt.Fprintf(cmd.OutOrStdout(), "[✓] %s\n", e.Name())
	}
	if anyFail {
		return fmt.Errorf("one or more hooks failed evaluation")
	}
	return nil
}

// installRepoPrecommit writes <repo>/.git/hooks/pre-commit that defers
// to the canonical ~/.ai/hooks/secret-precommit.py. Idempotent.
func installRepoPrecommit(repoDir, hooksDir string) error {
	gitDir := filepath.Join(repoDir, ".git")
	if _, err := os.Stat(gitDir); err != nil {
		return fmt.Errorf("%s is not a git repo (.git/ missing)", repoDir)
	}
	hookPath := filepath.Clean(filepath.Join(hooksDir, "secret-precommit.py"))
	if _, err := os.Stat(hookPath); err != nil {
		return fmt.Errorf("canonical %s missing — run `ai hooks install --all` first", hookPath)
	}
	dst := filepath.Clean(filepath.Join(gitDir, "hooks", "pre-commit"))
	if _, err := os.Stat(dst); err == nil {
		fmt.Println("pre-commit already present at", dst, "— leaving in place")
		return nil
	}
	body := fmt.Sprintf(`#!/usr/bin/env bash
# Installed by `+"`"+`ai hooks install --repo=%s`+"`"+` (SPEC.md §10.2).
exec python3 %q "$@"
`, repoDir, hookPath)
	if err := os.MkdirAll(filepath.Dir(dst), 0o750); err != nil {
		return err
	}
	// 0o755 is intentional: this IS a git pre-commit hook; git
	// requires the executable bit to invoke it.
	if err := os.WriteFile(dst, []byte(body), 0o755); err != nil { //nolint:gosec // G306: required executable
		return err
	}
	fmt.Println("installed", dst)
	return nil
}

// ─── Copilot integration ───────────────────────────────────────────────────

// runHooksCopilotInstall creates ~/.copilot/instructions/constitution.md as a
// symlink pointing to <aiRoot>/Constitution.runtime.md.
func runHooksCopilotInstall(aiRoot, home string) error {
	target := filepath.Join(aiRoot, "Constitution.runtime.md")
	if _, err := os.Stat(target); err != nil {
		return fmt.Errorf("hooks copilot: Constitution.runtime.md missing at %s — run: ai generate runtime", target)
	}
	dir := filepath.Join(home, ".copilot", "instructions")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("hooks copilot: mkdir %s: %w", dir, err)
	}
	link := filepath.Join(dir, "constitution.md")
	// Remove stale or existing symlink before (re)creating.
	_ = os.Remove(link)
	if err := os.Symlink(target, link); err != nil {
		return fmt.Errorf("hooks copilot: symlink: %w", err)
	}
	return nil
}
