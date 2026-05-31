package cmd

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	cbterm "github.com/charmbracelet/x/term"

	"github.com/convergent-systems-co/aiConstitution/src/cmd/ai/embed"
)

// hookDescriptions maps embedded hook filenames to one-line descriptions.
// The map keys are filenames; values are human-readable descriptions, not credentials.
var hookDescriptions = map[string]string{ //nolint:gosec // G101: false positive — no credentials here
	"audit.py":                       "Log every tool use to ~/.ai/audit/ (required for governance)",
	"audit-command.py":               "Log Bash commands and arguments before execution",
	"branch-guard.py":                "Block direct commits/pushes to protected branches (main/master)",
	"checkpoint-tick.py":             "Legacy HANDOFF.md checkpoint hook (manual wiring only; disabled by default)",
	"destructive-gh-guard.py":        "Block destructive gh CLI operations (delete, force-push)",
	"destructive-kubectl-guard.py":   "Block destructive kubectl operations (delete, drain, cordon)",
	"destructive-terraform-guard.py": "Block destructive terraform operations (destroy, force-replace)",
	"no-verify-strip.py":             "Strip --no-verify flags from git commands",
	"op-redact.py":                   "Redact 1Password secrets from tool output",
	"patterns.json":                  "Secret scan pattern library (required by secret-block)",
	"secret-block.py":                "Block commands that would print secret values (env, cat .env)",
	"secret-precommit.py":            "Git pre-commit hook: scan staged files for secrets",
	"worktree-guard.py":              "Enforce §U17 worktree placement — worktrees must be in canonical paths",
}

// hookInstallFn is the function signature for installing a single hook by name.
type hookInstallFn func(name, hooksDir string, force bool) error

// hookWireFn is the function signature for wiring installed hooks into the
// Claude settings.json at settingsPath. The function is expected to be
// idempotent.
type hookWireFn func(settingsPath, hooksDir string) error

// runHookSelectionPrompt shows a numbered hook list and installs the selected hooks.
//
// isTTY=false is a no-op (non-interactive path). installHook and wireHooks are
// injectable so that tests can use mocks without touching the filesystem.
func runHookSelectionPrompt(
	w io.Writer,
	r io.Reader,
	isTTY bool,
	hooksDir string,
	installHook hookInstallFn,
	wireHooks hookWireFn,
	home string,
) error {
	if !isTTY {
		return nil
	}

	// Build ordered list of installable hooks from the embedded FS.
	names, err := embed.HookNames()
	if err != nil {
		fmt.Fprintf(w, "\nNote: could not list embedded hooks (%v). Skipping.\n", err)
		return nil
	}

	var rows []string
	for _, n := range names {
		if isHookFile(n) || n == "patterns.json" {
			rows = append(rows, n)
		}
	}
	if len(rows) == 0 {
		return nil
	}

	// Clear screen so the list is always visible from the top.
	fmt.Fprint(w, "\033[2J\033[H")
	fmt.Fprintln(w, "╔══════════════════════════════════════════════════════════════╗")
	fmt.Fprintln(w, "║  Install governance hooks                                    ║")
	fmt.Fprintln(w, "║  Hooks enforce constitution rules on every AI tool use.      ║")
	fmt.Fprintln(w, "╚══════════════════════════════════════════════════════════════╝")
	fmt.Fprintln(w)

	for i, name := range rows {
		desc := hookDescriptions[name]
		if desc == "" {
			desc = name
		}
		fmt.Fprintf(w, "  %2d. %-35s\n      %s\n", i+1, name, desc)
	}

	fmt.Fprintln(w)
	fmt.Fprintf(w, `Install which? (e.g. 1,3,5 or "all" or Enter to skip): `)

	scanner := bufio.NewScanner(r)
	scanner.Scan()
	line := strings.TrimSpace(scanner.Text())

	if line == "" {
		fmt.Fprintln(w, "\nSkipping hook installation.")
		return nil
	}

	var toInstall []string
	if strings.EqualFold(line, "all") {
		toInstall = append(toInstall, rows...)
	} else {
		for _, token := range strings.Split(line, ",") {
			token = strings.TrimSpace(token)
			if token == "" {
				continue
			}
			n, parseErr := strconv.Atoi(token)
			if parseErr != nil || n < 1 || n > len(rows) {
				fmt.Fprintf(w, "Warning: %q is not a valid selection — skipping.\n", token)
				continue
			}
			toInstall = append(toInstall, rows[n-1])
		}
	}

	if len(toInstall) == 0 {
		return nil
	}

	if err := os.MkdirAll(hooksDir, 0o750); err != nil {
		return fmt.Errorf("setup hooks: mkdir: %w", err)
	}

	fmt.Fprintln(w)
	for _, name := range toInstall {
		fmt.Fprintf(w, "  Installing %-35s ", name+"...")
		if instErr := installHook(name, hooksDir, false); instErr != nil {
			fmt.Fprintf(w, "warning: %v\n", instErr)
		} else {
			fmt.Fprintln(w, "done")
		}
	}

	// Wire installed hooks into Claude settings (~/.claude/settings.json).
	settingsPath := filepath.Join(home, ".claude", "settings.json")
	if addErr := wireHooks(settingsPath, hooksDir); addErr != nil {
		fmt.Fprintf(w, "\nNote: could not wire hooks into settings.json: %v\n", addErr)
	}

	return nil
}

// runHookSelectionPromptReal binds the real embed/install functions and runs
// the hook selection prompt against the user's actual ~/.ai/hooks/ directory.
func runHookSelectionPromptReal() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	aiRoot := os.Getenv("AI_ROOT")
	if aiRoot == "" {
		aiRoot = filepath.Join(home, ".ai")
	}
	hooksDir := filepath.Join(aiRoot, "hooks")
	isTTY := cbterm.IsTerminal(os.Stdout.Fd())
	return runHookSelectionPrompt(
		os.Stdout,
		os.Stdin,
		isTTY,
		hooksDir,
		func(name, dir string, force bool) error {
			_, extractErr := embed.ExtractHook(name, dir, force)
			return extractErr
		},
		updateSettingsJSON,
		home,
	)
}
