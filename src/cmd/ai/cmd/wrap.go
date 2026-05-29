package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/convergent-systems-co/aiConstitution/src/cmd/ai/embed"
	"github.com/convergent-systems-co/aiConstitution/src/internal/paths"
	"github.com/spf13/cobra"
)

// commandWrappersConfig is the top-level schema of command-wrappers.toml.
type commandWrappersConfig struct {
	SchemaVersion string                    `toml:"schemaVersion"`
	Command       map[string]wrappedCommand `toml:"command"`
}

// wrappedCommand describes one intercepted tool (git, gh, etc.).
type wrappedCommand struct {
	RealCommand string    `toml:"realCommand"` // override binary path; "" = auto-discover
	Enabled     *bool     `toml:"enabled"`     // nil means true (default on)
	PreHooks    []hookDef `toml:"preHooks"`
	PostHooks   []hookDef `toml:"postHooks"`
}

// isEnabled reports whether this command entry is active.
func (w wrappedCommand) isEnabled() bool { return w.Enabled == nil || *w.Enabled }

// hookDef describes one hook entry in preHooks/postHooks.
type hookDef struct {
	Script      string   `toml:"script"`      // "~/.ai/hooks/branch-guard.py" — slug extracted at runtime
	Subcommands []string `toml:"subcommands"` // empty = applies to every subcommand
	StripArgs   []string `toml:"stripArgs"`   // args to remove before invoking real binary
	Description string   `toml:"description"`
}

// loadCommandWrappers reads command-wrappers.toml from AI_ROOT/hooks/,
// falling back to the embedded copy when not found on disk.
func loadCommandWrappers() (*commandWrappersConfig, error) {
	diskPath := filepath.Join(paths.HooksDir(), "command-wrappers.toml")
	data, err := os.ReadFile(diskPath) //nolint:gosec // path is always within AI_ROOT
	if err != nil {
		// Disk copy absent — fall back to the embedded version.
		data, err = fs.ReadFile(embed.HooksFS(), "command-wrappers.toml")
		if err != nil {
			return nil, fmt.Errorf("wrap: load command-wrappers.toml: %w", err)
		}
	}
	var cfg commandWrappersConfig
	if _, err := toml.Decode(string(data), &cfg); err != nil {
		return nil, fmt.Errorf("wrap: parse command-wrappers.toml: %w", err)
	}
	return &cfg, nil
}

// hookSlug extracts the slug from a TOML script path.
// "~/.ai/hooks/branch-guard.py" → "branch-guard"
// The slug is used to look up the hook in paths.HooksDir().
func hookSlug(scriptPath string) string {
	base := filepath.Base(scriptPath)
	return strings.TrimSuffix(base, filepath.Ext(base))
}

// hookApplies reports whether a hook should run for the given subcommand.
// If hook.Subcommands is empty, the hook applies to every invocation.
func hookApplies(hook hookDef, subCmd string) bool {
	if len(hook.Subcommands) == 0 {
		return true
	}
	subCmd = strings.ToLower(subCmd)
	for _, sc := range hook.Subcommands {
		// Multi-word subcommands like "repo delete" match on the first word.
		first := strings.Fields(strings.ToLower(sc))[0]
		if subCmd == first {
			return true
		}
	}
	return false
}

// applyStripArgs removes StripArgs entries from toolArgs.
func applyStripArgs(toolArgs, strip []string) []string {
	if len(strip) == 0 {
		return toolArgs
	}
	rm := make(map[string]bool, len(strip))
	for _, s := range strip {
		rm[s] = true
	}
	out := make([]string, 0, len(toolArgs))
	for _, a := range toolArgs {
		if !rm[a] {
			out = append(out, a)
		}
	}
	return out
}

// firstPositional returns the first non-flag argument.
// Used to identify the git/gh subcommand from the full arg list.
func firstPositional(args []string) string {
	for _, a := range args {
		if !strings.HasPrefix(a, "-") {
			return a
		}
	}
	return ""
}

// findRealBinary searches PATH for tool, skipping paths.BinDir() to avoid
// invoking the shim recursively. If realCommandOverride is non-empty, it is
// returned directly without a PATH search.
func findRealBinary(tool, realCommandOverride string) (string, error) {
	if realCommandOverride != "" {
		return realCommandOverride, nil
	}
	binDir := filepath.Clean(paths.BinDir())
	for _, d := range strings.Split(os.Getenv("PATH"), string(os.PathListSeparator)) {
		if filepath.Clean(d) == binDir {
			continue // skip ourselves
		}
		for _, cand := range realBinaryCandidates(d, tool) {
			if info, err := os.Stat(cand); err == nil && !info.IsDir() {
				return cand, nil
			}
		}
	}
	return "", fmt.Errorf("%s not found on PATH (excluding %s)", tool, binDir)
}

// realBinaryCandidates returns the path variants to check in one directory.
// On Windows, also tries .exe when the tool has no extension.
func realBinaryCandidates(dir, tool string) []string {
	base := filepath.Join(dir, tool)
	if runtime.GOOS != "windows" || filepath.Ext(tool) != "" {
		return []string{base}
	}
	return []string{base, base + ".exe"}
}

// runHookForWrap runs a hook by slug using the cross-platform Python
// dispatcher. It sets the caller-provided extraEnv (WRAPPED_CMD,
// WRAPPED_ARGV, etc.) in the hook process's environment.
// Returns the hook's exit code; returns 0 if the hook is not installed
// (non-fatal: allows fresh installs where hooks aren't yet extracted).
func runHookForWrap(slug string, toolArgs, extraEnv []string) int {
	hookPath := filepath.Join(paths.HooksDir(), slug+".py")
	if _, err := os.Stat(hookPath); err != nil {
		return 0 // not installed — skip silently
	}
	pyArgs := discoverPythonArgs() // defined in hooks.go, same package
	if pyArgs == nil {
		fmt.Fprintf(os.Stderr, "[ai/wrap] Python 3 not found; skipping hook %s\n", slug)
		return 0
	}
	// Build: python <hookPath> --mode=wrapper <toolArgs...>
	args := make([]string, 0, len(pyArgs)+2+len(toolArgs))
	args = append(args, pyArgs[1:]...)
	args = append(args, hookPath, "--mode=wrapper")
	args = append(args, toolArgs...)
	c := exec.Command(pyArgs[0], args...) //nolint:gosec // hookPath is within AI_ROOT
	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	c.Env = append(os.Environ(), extraEnv...)
	if err := c.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return exitErr.ExitCode()
		}
		return 1
	}
	return 0
}

// execRealCapturingCode runs the real binary with toolArgs and returns its
// exit code. Stdin/stdout/stderr are forwarded to the parent process.
func execRealCapturingCode(tool, realCommandOverride string, args []string) int {
	realPath, err := findRealBinary(tool, realCommandOverride)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[ai/wrap] %v\n", err)
		return 127
	}
	c := exec.Command(realPath, args...) //nolint:gosec
	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	if err := c.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return exitErr.ExitCode()
		}
		return 1
	}
	return 0
}

// runWrap implements `ai wrap <tool> [-- <args...>]`.
//
//  1. Parse command-wrappers.toml.
//  2. Strip leading "--" separator from rawArgs.
//  3. Apply StripArgs for matching pre-hooks.
//  4. Run pre-hooks that apply to this subcommand; abort on non-zero exit.
//  5. Exec the real binary (capturing exit code).
//  6. Run post-hooks (best-effort; always runs even if real binary failed).
//  7. os.Exit with the real binary's exit code.
func runWrap(tool string, rawArgs []string) error {
	toolArgs := rawArgs
	if len(toolArgs) > 0 && toolArgs[0] == "--" {
		toolArgs = toolArgs[1:]
	}

	cfg, err := loadCommandWrappers()
	if err != nil {
		// Config missing — fail-open: pass through to real binary.
		fmt.Fprintf(os.Stderr, "[ai/wrap] config error: %v; passing through to real %s\n", err, tool)
		os.Exit(execRealCapturingCode(tool, "", toolArgs))
	}

	entry, ok := cfg.Command[tool]
	if !ok || !entry.isEnabled() {
		// Not configured or disabled — pass straight through.
		os.Exit(execRealCapturingCode(tool, entry.RealCommand, toolArgs))
	}

	subCmd := firstPositional(toolArgs)
	wrappedArgJSON, _ := json.Marshal(toolArgs)
	baseEnv := []string{
		"WRAPPED_CMD=" + tool,
		"WRAPPED_ARGV=" + string(wrappedArgJSON),
	}

	// Pre-hooks: abort on non-zero exit.
	effectiveArgs := make([]string, len(toolArgs))
	copy(effectiveArgs, toolArgs)
	for _, h := range entry.PreHooks {
		if !hookApplies(h, subCmd) {
			continue
		}
		effectiveArgs = applyStripArgs(effectiveArgs, h.StripArgs)
		if code := runHookForWrap(hookSlug(h.Script), effectiveArgs, baseEnv); code != 0 {
			os.Exit(code)
		}
	}

	// Exec real binary.
	start := time.Now()
	exitCode := execRealCapturingCode(tool, entry.RealCommand, effectiveArgs)
	duration := int(time.Since(start).Seconds())

	// Post-hooks: always run; ignore failures.
	postEnv := append(append([]string{}, baseEnv...),
		fmt.Sprintf("WRAPPED_EXIT=%d", exitCode),
		fmt.Sprintf("WRAPPED_DURATION=%d", duration),
	)
	for _, h := range entry.PostHooks {
		if !hookApplies(h, subCmd) {
			continue
		}
		_ = runHookForWrap(hookSlug(h.Script), effectiveArgs, postEnv)
	}

	os.Exit(exitCode)
	return nil // unreachable; satisfies RunE signature
}

// newWrapCmd builds the cobra subcommand for `ai wrap`.
func newWrapCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "wrap <tool> [-- args...]",
		Short: "Cross-platform tool wrapper (invoked by git/gh shims)",
		Long: `wrap is the cross-platform dispatcher for command-wrapper interception.

It is not intended to be called directly by users. The tri-form shims
in ~/.ai/bin/ (git, git.cmd, git.ps1 etc.) delegate to it:

  POSIX:       exec ai wrap git -- "$@"
  Windows cmd: ai wrap git -- %*
  PowerShell:  & ai wrap git -- @args

wrap loads command-wrappers.toml, runs the configured pre-hooks for
the tool and subcommand, invokes the real binary, and runs post-hooks.
If command-wrappers.toml cannot be loaded, wrap passes through directly
(fail-open on config error so git/gh are never broken by a bad config).`,
		DisableFlagParsing: true, // forward all flags to the wrapped tool
		SilenceUsage:       true,
		RunE: func(_ *cobra.Command, args []string) error {
			if len(args) == 0 {
				return fmt.Errorf("wrap: tool name required\n  Usage: ai wrap <tool> [-- <args...>]")
			}
			return runWrap(args[0], args[1:])
		},
	}
}
