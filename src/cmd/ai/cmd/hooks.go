package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

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

	// list
	c.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "All installed hooks + wiring status",
		RunE: func(cmd *cobra.Command, _ []string) error {
			names, err := embed.HookNames()
			if err != nil {
				return err
			}
			sort.Strings(names)
			fmt.Println("Embedded hooks available for `ai hooks install`:")
			for _, n := range names {
				fmt.Println("  " + n)
			}
			fmt.Println()
			fmt.Println("Wrappers available for `ai hooks install command-wrappers`:")
			fmt.Println("  git, gh")
			return nil
		},
	})

	// evaluate
	c.AddCommand(&cobra.Command{
		Use:   "evaluate",
		Short: "Run each hook's --self-check; emit findings",
		RunE: func(cmd *cobra.Command, _ []string) error {
			notice("hooks evaluate:", "would run --self-check on every installed hook")
			return stub("hooks evaluate", "§9 + §3.10")
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
			notice("hooks propose:", args[0], "from:", fromViolation, "lang:", lang)
			return stub("hooks propose", "§9.2")
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
			notice("hooks share:", args[0])
			return stub("hooks share", "§9.3")
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

  --force                overwrite existing files
  --repo=<path>          (with no positional) install a pre-commit
                         hook into the specified repo's .git/hooks/
                         that defers to ~/.ai/hooks/secret-precommit.py
  --claude               wire ~/.ai/hooks/*.py into .claude/settings.json
  --claude-root=<path>   directory containing .claude/ (default ".")

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
			return runHooksInstall(installRepo, target, installAllHooks || installAll, installForce)
		},
	}
	install.Flags().StringVar(&installRepo, "repo", "", "install a pre-commit shim into the specified repo")
	install.Flags().BoolVar(&installAll, "all-future-clones", false, "(reserved; wires into `ai clone` per SPEC §10.2)")
	install.Flags().BoolVar(&installAllHooks, "all", false, "extract every embedded hook to ~/.ai/hooks/")
	install.Flags().BoolVar(&installForce, "force", false, "overwrite existing files")
	install.Flags().BoolVar(&installClaude, "claude", false, "wire ~/.ai/hooks/*.py into .claude/settings.json")
	install.Flags().StringVar(&installClaudeRoot, "claude-root", ".", "directory containing .claude/ (default: current dir)")

	c.AddCommand(propose, install)
	return c
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
		// PreToolUse: all-tools group
		{event: "PreToolUse", matcher: "", hooks: h("audit.py", "secret-block.py", "worktree-guard.py")},
		// PreToolUse: Bash-only group for branch-guard
		{event: "PreToolUse", matcher: "Bash", hooks: h("branch-guard.py")},
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
