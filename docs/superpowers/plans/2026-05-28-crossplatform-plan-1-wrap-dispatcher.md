# Cross-Platform Wrappers — Plan 1 of 4: `ai wrap` Go Dispatcher + Tri-Form Stubs

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Close the Windows safety gap where `git`/`gh` command interception silently doesn't run; move all wrapper logic from bash into `ai wrap <tool> -- <args>` (Go), ship tri-form stubs (bash/`.cmd`/`.ps1`), and platform-filter the wrapper install.

**Architecture:** Implement `newWrapCmd()` in a new `wrap.go` that loads `command-wrappers.toml`, finds the real binary by searching PATH while skipping `~/.ai/bin`, runs pre-hooks via the existing `discoverPythonArgs()` + `exec.Command`, execs the real binary, and runs post-hooks with WRAPPED_* env vars. Add `git.cmd`, `git.ps1`, `gh.cmd`, `gh.ps1` stub files (4 lines each). Reduce the existing bash `git`/`gh` wrappers to 3-line shims. Add platform filtering to `ExtractWrappers` in embed.go.

**Tech Stack:** Go 1.26, `github.com/BurntSushi/toml v1.4.0` (already a dep), `os/exec`, `runtime.GOOS`, `io/fs`, `time`, cobra.

---

## Scope of this plan

This is **Plan 1 of 4** for the cross-platform portability spec:

| Plan | Scope |
|---|---|
| **Plan 1 (this)** | `ai wrap` Go dispatcher + tri-form stubs (§2.2 + §5) |
| Plan 2 | Constitution template portability rewrite (§3.1–3.8) |
| Plan 3 | Skills symlink → copy/junction fallback on Windows (§6.3) |
| Plan 4 | Tri-OS CI matrix + portability lint (§7) |

---

## Context: what already exists

Read these files before starting — they establish the patterns this plan follows:

- `src/cmd/ai/cmd/hooks.go` — `runHooksRun()` (line ≈1187), `discoverPythonArgs()` (line ≈1253)
- `src/cmd/ai/embed/embed.go` — `ExtractWrappers()` (line 131), `writeFile()` (line 168)
- `src/cmd/ai/embed/wrappers/git` — the full 77-line bash fallback this plan replaces with a 3-line shim
- `src/cmd/ai/embed/hooks/command-wrappers.toml` — the TOML schema this plan parses
- `src/cmd/ai/cmd/brand.go` — `github.com/BurntSushi/toml` import + `toml.Unmarshal` usage pattern

Key facts:
- `discoverPythonArgs()` in hooks.go (same package, unexported) handles `python3 → python → py -3` on Windows — `wrap.go` can call it directly.
- `paths.HooksDir()` and `paths.BinDir()` are safe cross-platform (use `os.UserHomeDir()` + `filepath.Join`, never literal `~`).
- `embed.HooksFS()` returns an `fs.FS` of the embedded hooks — used to fall back to the embedded `command-wrappers.toml`.

---

## Files

**Create:**
- `src/cmd/ai/cmd/wrap.go` — `newWrapCmd()` + full dispatch logic
- `src/cmd/ai/cmd/wrap_test.go` — unit tests
- `src/cmd/ai/embed/wrappers/git.cmd` — Windows CMD stub
- `src/cmd/ai/embed/wrappers/git.ps1` — Windows PowerShell stub
- `src/cmd/ai/embed/wrappers/gh.cmd` — Windows CMD stub
- `src/cmd/ai/embed/wrappers/gh.ps1` — Windows PowerShell stub

**Modify:**
- `src/cmd/ai/embed/wrappers/git` — reduce from 77 lines to 3-line POSIX shim
- `src/cmd/ai/embed/wrappers/gh` — reduce to 3-line POSIX shim
- `src/cmd/ai/embed/embed.go` — add `wrapperAppliesOnOS()` + platform filtering in `ExtractWrappers`
- `src/cmd/ai/cmd/root.go` — register `newWrapCmd()`
- `src/cmd/ai/cmd/export_test.go` — export `FindRealBinaryForTest`, `HookSlugForTest`

---

## Task 1: TOML config types + `loadCommandWrappers()`

These types match the schema in `command-wrappers.toml`. Read that file before writing the struct — confirm field names match.

**Files:**
- Create: `src/cmd/ai/cmd/wrap.go`

- [ ] **Step 1.1: Create the file with package declaration + imports**

  ```go
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
  ```

- [ ] **Step 1.2: Add TOML config types**

  ```go
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
  ```

- [ ] **Step 1.3: Add `loadCommandWrappers()`**

  ```go
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
  ```

- [ ] **Step 1.4: Verify it compiles**

  ```bash
  go build ./src/cmd/ai/...
  ```
  Expected: no output (clean build). If you see an import error for `embed`, check that `embed.HooksFS()` is exported — it is, at line 63 of `src/cmd/ai/embed/embed.go`.

---

## Task 2: Helper functions — `findRealBinary`, `hookSlug`, `hookApplies`, `applyStripArgs`

**Files:**
- Modify: `src/cmd/ai/cmd/wrap.go` (append)

- [ ] **Step 2.1: Add `hookSlug()`**

  ```go
  // hookSlug extracts the slug from a TOML script path.
  // "~/.ai/hooks/branch-guard.py" → "branch-guard"
  // The slug is used to look up the hook in paths.HooksDir().
  func hookSlug(scriptPath string) string {
  	base := filepath.Base(scriptPath)
  	return strings.TrimSuffix(base, filepath.Ext(base))
  }
  ```

- [ ] **Step 2.2: Add `hookApplies()` and `applyStripArgs()`**

  ```go
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
  ```

- [ ] **Step 2.3: Add `findRealBinary()`**

  ```go
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
  ```

- [ ] **Step 2.4: Verify build**

  ```bash
  go build ./src/cmd/ai/...
  ```
  Expected: clean build.

---

## Task 3: `runHookForWrap()` + `runWrap()` core dispatch

**Files:**
- Modify: `src/cmd/ai/cmd/wrap.go` (append)

- [ ] **Step 3.1: Add `runHookForWrap()`**

  ```go
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
  	// Build: python <path> --mode=wrapper <toolArgs...>
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
  ```

- [ ] **Step 3.2: Add `execRealCapturingCode()`**

  ```go
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
  ```

- [ ] **Step 3.3: Add `runWrap()` — the main dispatch**

  ```go
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
  		// Config missing — pass through to real binary (fail-open on config error).
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
  ```

- [ ] **Step 3.4: Verify build**

  ```bash
  go build ./src/cmd/ai/...
  ```
  Expected: clean. If you see "undefined: discoverPythonArgs", confirm wrap.go is `package cmd` (same package as hooks.go).

---

## Task 4: `newWrapCmd()` + register in root.go

**Files:**
- Modify: `src/cmd/ai/cmd/wrap.go` (append)
- Modify: `src/cmd/ai/cmd/root.go`

- [ ] **Step 4.1: Add `newWrapCmd()` to wrap.go**

  ```go
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
  ```

- [ ] **Step 4.2: Register `newWrapCmd()` in root.go**

  In `src/cmd/ai/cmd/root.go`, inside `root.AddCommand(...)`, add `newWrapCmd()` on a new line after `newMigrateCmd()`:

  ```go
  		newMigrateCmd(),
  		newWrapCmd(),    // ← add this line
  		newInitIntegrateCmd(),
  ```

- [ ] **Step 4.3: Verify the command appears in help**

  ```bash
  go run ./src/cmd/ai --help 2>&1 | grep wrap
  ```
  Expected:
  ```
    wrap        Cross-platform tool wrapper (invoked by git/gh shims)
  ```

- [ ] **Step 4.4: Commit**

  ```bash
  git add src/cmd/ai/cmd/wrap.go src/cmd/ai/cmd/root.go
  git commit -m "feat(wrap): add ai wrap Go dispatcher with cross-platform hook execution

  Implements §5.1 of the cross-platform portability spec.
  Reads command-wrappers.toml, finds the real binary by skipping
  ~/.ai/bin, runs pre/post hooks via discoverPythonArgs().
  Fail-open: passes through to real binary on config error.

  Co-Authored-By: Claude Sonnet 4.6 (1M context) <noreply@anthropic.com>"
  ```

---

## Task 5: Write unit tests for wrap.go helpers

**Files:**
- Create: `src/cmd/ai/cmd/wrap_test.go`

- [ ] **Step 5.1: Add export seams to export_test.go**

  Append to `src/cmd/ai/cmd/export_test.go`:

  ```go
  // FindRealBinaryForTest exposes findRealBinary to external tests.
  func FindRealBinaryForTest(tool, override string) (string, error) {
  	return findRealBinary(tool, override)
  }

  // HookSlugForTest exposes hookSlug to external tests.
  func HookSlugForTest(scriptPath string) string { return hookSlug(scriptPath) }

  // HookAppliesForTest exposes hookApplies to external tests.
  func HookAppliesForTest(h hookDef, subCmd string) bool { return hookApplies(h, subCmd) }

  // ApplyStripArgsForTest exposes applyStripArgs to external tests.
  func ApplyStripArgsForTest(args, strip []string) []string { return applyStripArgs(args, strip) }

  // NewHookDefForTest constructs a hookDef for tests.
  func NewHookDefForTest(script string, subcommands, stripArgs []string) hookDef {
  	return hookDef{Script: script, Subcommands: subcommands, StripArgs: stripArgs}
  }
  ```

- [ ] **Step 5.2: Write failing tests**

  ```go
  package cmd_test

  import (
  	"testing"

  	cmd "github.com/convergent-systems-co/aiConstitution/src/cmd/ai/cmd"
  )

  func TestHookSlug(t *testing.T) {
  	cases := []struct{ in, want string }{
  		{"~/.ai/hooks/branch-guard.py", "branch-guard"},
  		{"~/.ai/hooks/audit-command.py", "audit-command"},
  		{"/abs/path/secret-precommit.py", "secret-precommit"},
  		{"no-verify-strip.py", "no-verify-strip"},
  	}
  	for _, c := range cases {
  		if got := cmd.HookSlugForTest(c.in); got != c.want {
  			t.Errorf("hookSlug(%q) = %q, want %q", c.in, got, c.want)
  		}
  	}
  }

  func TestHookApplies_EmptySubcmds(t *testing.T) {
  	h := cmd.NewHookDefForTest("audit.py", nil, nil)
  	for _, subCmd := range []string{"commit", "push", "merge", ""} {
  		if !cmd.HookAppliesForTest(h, subCmd) {
  			t.Errorf("hook with empty subcommands should apply to %q", subCmd)
  		}
  	}
  }

  func TestHookApplies_Matching(t *testing.T) {
  	h := cmd.NewHookDefForTest("branch-guard.py", []string{"commit", "merge", "push"}, nil)
  	for _, subCmd := range []string{"commit", "merge", "push"} {
  		if !cmd.HookAppliesForTest(h, subCmd) {
  			t.Errorf("hook should apply to %q", subCmd)
  		}
  	}
  	for _, subCmd := range []string{"status", "log", "diff", ""} {
  		if cmd.HookAppliesForTest(h, subCmd) {
  			t.Errorf("hook should NOT apply to %q", subCmd)
  		}
  	}
  }

  func TestApplyStripArgs(t *testing.T) {
  	args := []string{"commit", "--no-verify", "-m", "msg", "-n"}
  	strip := []string{"--no-verify", "-n"}
  	got := cmd.ApplyStripArgsForTest(args, strip)
  	want := []string{"commit", "-m", "msg"}
  	if len(got) != len(want) {
  		t.Fatalf("applyStripArgs = %v, want %v", got, want)
  	}
  	for i := range want {
  		if got[i] != want[i] {
  			t.Errorf("applyStripArgs[%d] = %q, want %q", i, got[i], want[i])
  		}
  	}
  }

  func TestApplyStripArgs_NoStrip(t *testing.T) {
  	args := []string{"push", "origin", "main"}
  	got := cmd.ApplyStripArgsForTest(args, nil)
  	if len(got) != len(args) {
  		t.Fatalf("applyStripArgs with nil strip should return input unchanged, got %v", got)
  	}
  }

  func TestFindRealBinary_OverrideUsed(t *testing.T) {
  	// When realCommandOverride is non-empty, findRealBinary returns it directly.
  	got, err := cmd.FindRealBinaryForTest("git", "/usr/bin/git")
  	if err != nil {
  		t.Fatalf("unexpected error: %v", err)
  	}
  	if got != "/usr/bin/git" {
  		t.Errorf("got %q, want /usr/bin/git", got)
  	}
  }
  ```

- [ ] **Step 5.3: Run the failing tests**

  ```bash
  go test -run 'TestHookSlug|TestHookApplies|TestApplyStripArgs|TestFindRealBinary' -v ./src/cmd/ai/cmd/
  ```
  Expected: all PASS (the implementation was written before the tests in this plan; tests should pass immediately).

- [ ] **Step 5.4: Commit**

  ```bash
  git add src/cmd/ai/cmd/wrap_test.go src/cmd/ai/cmd/export_test.go
  git commit -m "test(wrap): unit tests for hookSlug, hookApplies, applyStripArgs, findRealBinary

  Co-Authored-By: Claude Sonnet 4.6 (1M context) <noreply@anthropic.com>"
  ```

---

## Task 6: Reduce bash git/gh wrappers to 3-line POSIX shims

The existing 77-line bash `git` and 60-line bash `gh` wrappers had all their logic. That logic now lives in `ai wrap`. Replace them with shims.

**Files:**
- Modify: `src/cmd/ai/embed/wrappers/git`
- Modify: `src/cmd/ai/embed/wrappers/gh`

- [ ] **Step 6.1: Read the current `git` wrapper**

  ```bash
  wc -l src/cmd/ai/embed/wrappers/git
  ```
  Expected: 77 lines. If it's already a 3-line shim, skip this task.

- [ ] **Step 6.2: Replace `git` wrapper with 3-line shim**

  Overwrite `src/cmd/ai/embed/wrappers/git` with exactly:

  ```bash
  #!/usr/bin/env bash
  # git — POSIX shim: delegates all governance logic to `ai wrap`.
  exec ai wrap git -- "$@"
  ```

- [ ] **Step 6.3: Replace `gh` wrapper**

  Read `src/cmd/ai/embed/wrappers/gh` to confirm its current content, then replace with:

  ```bash
  #!/usr/bin/env bash
  # gh — POSIX shim: delegates all governance logic to `ai wrap`.
  exec ai wrap gh -- "$@"
  ```

- [ ] **Step 6.4: Verify the embed package still compiles**

  ```bash
  go build ./src/cmd/ai/...
  ```
  Expected: clean build. The `//go:embed all:wrappers` directive picks up the updated files.

- [ ] **Step 6.5: Commit**

  ```bash
  git add src/cmd/ai/embed/wrappers/git src/cmd/ai/embed/wrappers/gh
  git commit -m "refactor(wrappers): reduce bash git/gh to 3-line POSIX shims

  All logic moved to ai wrap (Go). Shim: exec ai wrap <tool> -- \"\$@\".
  Exit-code fidelity, signal handling, and Windows compatibility are
  handled in Go, not bash.

  Co-Authored-By: Claude Sonnet 4.6 (1M context) <noreply@anthropic.com>"
  ```

---

## Task 7: Add Windows stub files (`.cmd` and `.ps1`)

**Files:**
- Create: `src/cmd/ai/embed/wrappers/git.cmd`
- Create: `src/cmd/ai/embed/wrappers/git.ps1`
- Create: `src/cmd/ai/embed/wrappers/gh.cmd`
- Create: `src/cmd/ai/embed/wrappers/gh.ps1`

- [ ] **Step 7.1: Create `git.cmd`**

  ```
  @echo off
  :: git.cmd — Windows CMD shim: delegates governance logic to ai wrap.
  ai wrap git -- %*
  ```

- [ ] **Step 7.2: Create `git.ps1`**

  ```powershell
  # git.ps1 — Windows PowerShell shim: delegates governance logic to ai wrap.
  & ai wrap git -- @args
  ```

- [ ] **Step 7.3: Create `gh.cmd`**

  ```
  @echo off
  :: gh.cmd — Windows CMD shim: delegates governance logic to ai wrap.
  ai wrap gh -- %*
  ```

- [ ] **Step 7.4: Create `gh.ps1`**

  ```powershell
  # gh.ps1 — Windows PowerShell shim: delegates governance logic to ai wrap.
  & ai wrap gh -- @args
  ```

- [ ] **Step 7.5: Verify embed still compiles**

  ```bash
  go build ./src/cmd/ai/...
  ```
  Expected: clean. The `//go:embed all:wrappers` picks up all four new files.

- [ ] **Step 7.6: Commit**

  ```bash
  git add src/cmd/ai/embed/wrappers/git.cmd src/cmd/ai/embed/wrappers/git.ps1 \
          src/cmd/ai/embed/wrappers/gh.cmd src/cmd/ai/embed/wrappers/gh.ps1
  git commit -m "feat(wrappers): add Windows CMD and PowerShell shims for git and gh

  Mirrors the notify-me tri-form pattern. Stubs delegate to ai wrap.

  Co-Authored-By: Claude Sonnet 4.6 (1M context) <noreply@anthropic.com>"
  ```

---

## Task 8: Platform-filter `ExtractWrappers` in embed.go

Currently `ExtractWrappers` extracts all files from the wrappers FS regardless of OS. We need to skip `.cmd`/`.ps1` on POSIX and skip the bare `git`/`gh` bash scripts on Windows (which would be useless there).

**Files:**
- Modify: `src/cmd/ai/embed/embed.go`

- [ ] **Step 8.1: Read `ExtractWrappers` to find the insertion point**

  ```bash
  grep -n 'func ExtractWrappers\|for _, e :=' src/cmd/ai/embed/embed.go
  ```
  Expected: two lines — the function definition and the `for _, e := range entries` loop at ≈ line 140.

- [ ] **Step 8.2: Add `wrapperAppliesOnOS()` near the top of the file (after imports)**

  Find the `WrappersFS()` function (≈ line 63) and insert after it:

  ```go
  // windowsShimBasenames is the set of wrapper basenames that have Windows
  // .cmd/.ps1 counterparts. On Windows the bare bash form is skipped;
  // on POSIX the .cmd/.ps1 forms are skipped.
  var windowsShimBasenames = map[string]bool{"git": true, "gh": true}

  // wrapperAppliesOnOS reports whether a wrapper filename should be installed
  // on the current OS.
  //
  //   - *.cmd / *.ps1          → Windows only
  //   - bare names in the set  → POSIX only (they have Windows siblings)
  //   - everything else        → all platforms (notify-me, tests/, etc.)
  func wrapperAppliesOnOS(name string) bool {
  	isWindows := runtime.GOOS == "windows"
  	ext := strings.ToLower(filepath.Ext(name))
  	if ext == ".cmd" || ext == ".ps1" {
  		return isWindows
  	}
  	// Bare names that have Windows .cmd/.ps1 siblings are POSIX-only.
  	if windowsShimBasenames[name] {
  		return !isWindows
  	}
  	return true
  }
  ```

  Then add the necessary imports. Check that `embed.go` already imports `"runtime"` and `"strings"`:

  ```bash
  grep '"runtime"\|"strings"' src/cmd/ai/embed/embed.go
  ```
  If either is missing, add it to the import block.

- [ ] **Step 8.3: Add the `wrapperAppliesOnOS` call in `ExtractWrappers`**

  In the `for _, e := range entries` loop (line ≈140), after `if e.IsDir() { continue }`, add:

  ```go
  		if !wrapperAppliesOnOS(e.Name()) {
  			continue
  		}
  ```

  The loop body should now look like:

  ```go
  	for _, e := range entries {
  		if e.IsDir() {
  			continue
  		}
  		if !wrapperAppliesOnOS(e.Name()) {
  			continue
  		}
  		data, err := fs.ReadFile(WrappersFS(), e.Name())
  		// ... rest unchanged
  	}
  ```

- [ ] **Step 8.4: Add an embed test for the platform filter**

  In `src/cmd/ai/embed/` there should be a `*_test.go` file. If not, create `embed_test.go`:

  ```go
  package embed_test

  import (
  	"runtime"
  	"strings"
  	"testing"

  	"github.com/convergent-systems-co/aiConstitution/src/cmd/ai/embed"
  )

  func TestExtractWrappers_PlatformFilter(t *testing.T) {
  	t.Parallel()
  	tmp := t.TempDir()
  	written, err := embed.ExtractWrappers(tmp, false)
  	if err != nil {
  		t.Fatalf("ExtractWrappers: %v", err)
  	}

  	names := make(map[string]bool, len(written))
  	for _, p := range written {
  		base := p[strings.LastIndexByte(p, '/')+1:]
  		if i := strings.LastIndexByte(base, '\\'); i >= 0 {
  			base = base[i+1:]
  		}
  		names[base] = true
  	}

  	if runtime.GOOS == "windows" {
  		// Windows must get .cmd and .ps1, not bare bash script.
  		for _, want := range []string{"git.cmd", "git.ps1", "gh.cmd", "gh.ps1"} {
  			if !names[want] {
  				t.Errorf("Windows: expected %q to be extracted, got %v", want, names)
  			}
  		}
  		for _, notwant := range []string{"git", "gh"} {
  			if names[notwant] {
  				t.Errorf("Windows: bare %q should NOT be extracted (no bash on Windows)", notwant)
  			}
  		}
  	} else {
  		// POSIX must get bare scripts, not .cmd/.ps1.
  		for _, want := range []string{"git", "gh"} {
  			if !names[want] {
  				t.Errorf("POSIX: expected %q to be extracted, got %v", want, names)
  			}
  		}
  		for _, notwant := range []string{"git.cmd", "git.ps1", "gh.cmd", "gh.ps1"} {
  			if names[notwant] {
  				t.Errorf("POSIX: %q should NOT be extracted on non-Windows", notwant)
  			}
  		}
  	}
  }
  ```

- [ ] **Step 8.5: Run the test (on POSIX; Windows half validates in CI)**

  ```bash
  go test -run TestExtractWrappers_PlatformFilter -v ./src/cmd/ai/embed/
  ```
  Expected:
  ```
  === RUN   TestExtractWrappers_PlatformFilter
  --- PASS: TestExtractWrappers_PlatformFilter (0.00s)
  ```

- [ ] **Step 8.6: Run full suite**

  ```bash
  make test
  ```
  Expected: all packages pass, 0 failures.

- [ ] **Step 8.7: Commit**

  ```bash
  git add src/cmd/ai/embed/embed.go src/cmd/ai/embed/embed_test.go
  git commit -m "feat(embed): platform-filter ExtractWrappers; Windows gets .cmd/.ps1, POSIX gets bash

  Adds wrapperAppliesOnOS() that skips *.cmd/*.ps1 on POSIX and
  skips bare git/gh on Windows. Mirrors notify-me's existing tri-form
  pattern. TestExtractWrappers_PlatformFilter verifies on POSIX;
  Windows half will validate in the tri-OS CI matrix (Plan 4).

  Co-Authored-By: Claude Sonnet 4.6 (1M context) <noreply@anthropic.com>"
  ```

---

## Self-review

**§2.2 coverage:** `git`/`gh` wrappers now ship in three forms. On Windows, `git.cmd`/`git.ps1` are extracted; on POSIX, bash `git`/`gh`. `ExtractWrappers` platform-filters correctly. ✓

**§5.1 (single dispatcher):** All wrapper logic lives in Go (`runWrap`). The per-OS files are genuine 3-line shims. ✓

**§5.2 (install OS-appropriate form):** `wrapperAppliesOnOS` in embed.go handles this. ✓

**§5.3 (resolve hook paths from AI_ROOT at runtime):** `loadCommandWrappers` loads the TOML; `hookSlug` extracts the basename; `runHookForWrap` looks up `paths.HooksDir()/slug.py`. The `~` in the TOML `script` field is never used by Go — it's advisory text only. ✓

**§5.3 (PATH search that skips itself):** `findRealBinary` skips `paths.BinDir()` via `filepath.Clean` comparison. ✓

**§5.4 (PATH-precedence message):** Existing `installWrappers` in hooks.go already prints "Note: add `binDir` early to your $PATH". ✓

**Fail-open design:** If `command-wrappers.toml` can't be loaded, `runWrap` passes through to the real binary. The governance system never breaks `git`/`gh`. ✓

**Placeholder scan:** None found — all steps have complete code.

**Type consistency:** `hookDef` used consistently across Tasks 1, 2, 3, 5. `findRealBinary(tool, realCommandOverride string)` signature consistent across Task 2 definition and Task 5 test usage. ✓

**One known gap:** The `command-wrappers.toml` `script` field still uses `~/.ai/hooks/...`. This is a §2.4 fix that belongs in Plan 2 (constitution/config portability rewrite) — changing the TOML schema is a breaking change that needs a migration path.

---

## What's next

After this plan is merged:

- **Plan 2** (`2026-05-28-crossplatform-plan-2-constitution-rewrite.md`): Constitution template §3.4.1 rewrite (per-OS presence-test table), clipboard rule, `0600`/`0400` → portable intent, §3.5 path-resolution clause, §3.7 shell-neutral rule, §3.8 line-ending rule, portability lint.
- **Plan 3** (`2026-05-28-crossplatform-plan-3-skills-symlink.md`): `ensureSymlink` → copy/junction fallback on Windows; `APPDATA` PATH wiring message on Windows install.
- **Plan 4** (`2026-05-28-crossplatform-plan-4-ci-matrix.md`): `.github/workflows/ci.yml` matrix (`windows-latest`, `ubuntu-latest`, `macos-latest`), hook `--self-check` job, portability lint CI step.
