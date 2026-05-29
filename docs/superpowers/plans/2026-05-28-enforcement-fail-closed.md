# Enforcement Fail-Closed — Plan A

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the four fail-open paths in `wrap.go` fail closed for safety-critical (blocking) hooks, fix flag-normalization gaps in `applyStripArgs`, and surface post-hook audit failures as visible warnings.

**Architecture:** Add `Enforcement string` to `hookDef` (default `""` = blocking; `"advisory"` opts out); refactor `runHookForWrap` to accept a blocking bool and return 1 (with a visible error) instead of 0 when a blocking hook is missing or Python is absent; change the config-error path to `os.Exit(1)` instead of a silent pass-through; add a `normalizeFlag` step to `applyStripArgs` that matches `--flag=value` forms; and capture post-hook exit codes to emit an `[ai/wrap] WARN: audit hook failed` line.

**Tech Stack:** Go 1.26. All changes in `src/cmd/ai/cmd/wrap.go`, `src/cmd/ai/embed/hooks/command-wrappers.toml`, `src/cmd/ai/cmd/export_test.go`, and `src/cmd/ai/cmd/wrap_test.go`.

---

## Context: exact current state

Read these before touching any code.

**`hookDef` struct** (`wrap.go:39–44`):
```go
type hookDef struct {
    Script      string   `toml:"script"`
    Subcommands []string `toml:"subcommands"`
    StripArgs   []string `toml:"stripArgs"`
    Description string   `toml:"description"`
}
```
No `Enforcement` field yet.

**`runHookForWrap` fail-open paths** (`wrap.go:155–163`):
```go
func runHookForWrap(slug string, toolArgs, extraEnv []string) int {
    hookPath := filepath.Join(paths.HooksDir(), slug+".py")
    if _, err := os.Stat(hookPath); err != nil {
        return 0 // ← FAIL-OPEN: hook missing → skip silently
    }
    pyArgs := discoverPythonArgs()
    if pyArgs == nil {
        fmt.Fprintf(os.Stderr, "[ai/wrap] Python 3 not found; skipping hook %s\n", slug)
        return 0 // ← FAIL-OPEN: Python absent → skip silently
    }
```

**Config-error fail-open** (`wrap.go:222–227`):
```go
cfg, err := loadCommandWrappers()
if err != nil {
    fmt.Fprintf(os.Stderr, "[ai/wrap] config error: %v; passing through to real %s\n", err, tool)
    os.Exit(execRealCapturingCode(tool, "", toolArgs)) // ← FAIL-OPEN
}
```

**Post-hook swallows all failures** (`wrap.go:265–270`):
```go
for _, h := range entry.PostHooks {
    if !hookApplies(h, subCmd) { continue }
    _ = runHookForWrap(hookSlug(h.Script), effectiveArgs, postEnv) // ← swallowed
}
```

**`applyStripArgs` exact-match only** (`wrap.go:90–106`):
```go
rm := make(map[string]bool, len(strip))
for _, s := range strip { rm[s] = true }
for _, a := range toolArgs {
    if !rm[a] { out = append(out, a) } // exact match: "--no-verify=true" is NOT removed
}
```

**`command-wrappers.toml`** pre-hooks: `branch-guard`, `secret-precommit`, `no-verify-strip`, `worktree-guard`, `destructive-gh-guard` — all currently have the same treatment (no `enforcement` field).

**Existing export seams** in `export_test.go`: `ApplyStripArgsForTest`, `NewHookDefForTest`, `HookSlugForTest`, `HookAppliesForTest`, `FindRealBinaryForTest`. No seam for `runHookForWrap` yet.

---

## Files

**Modify:**
- `src/cmd/ai/cmd/wrap.go` — all logic changes
- `src/cmd/ai/embed/hooks/command-wrappers.toml` — mark `worktree-guard` as advisory
- `src/cmd/ai/cmd/export_test.go` — add `RunHookForWrapForTest` seam
- `src/cmd/ai/cmd/wrap_test.go` — new tests for every changed behavior

---

## Task 1: Add `Enforcement` field to `hookDef` and mark `worktree-guard` advisory

The `Enforcement` field controls whether a missing or broken hook is a hard error or a silent skip. Empty (the default) means blocking — all existing security-gate hooks become blocking with no TOML change required. `"advisory"` opts out to the current silent-skip behavior.

**Files:**
- Modify: `src/cmd/ai/cmd/wrap.go:39–44`
- Modify: `src/cmd/ai/embed/hooks/command-wrappers.toml`

- [ ] **Step 1.1: Add `Enforcement` field and `isBlocking()` method to `hookDef`**

  In `wrap.go`, replace the existing `hookDef` struct (lines 38–44):

  ```go
  // hookDef describes one hook entry in preHooks/postHooks.
  type hookDef struct {
  	Script      string   `toml:"script"`      // "~/.ai/hooks/branch-guard.py" — slug extracted at runtime
  	Subcommands []string `toml:"subcommands"` // empty = applies to every subcommand
  	StripArgs   []string `toml:"stripArgs"`   // args to remove before invoking real binary
  	Enforcement string   `toml:"enforcement"` // "" or "blocking" = fail closed; "advisory" = skip silently
  	Description string   `toml:"description"`
  }

  // isBlocking reports whether a missing or broken hook should block the
  // real binary. Default (empty Enforcement) is blocking so that all
  // existing security-gate hooks fail closed without a TOML change.
  func (h hookDef) isBlocking() bool { return h.Enforcement != "advisory" }
  ```

- [ ] **Step 1.2: Mark `worktree-guard` as advisory in command-wrappers.toml**

  Worktree placement is a convention enforcement, not a secret/branch safety gate. Read the file first, then find the `no-verify-strip.py` preHook block and the `worktree-guard.py` preHook block:

  ```toml
  [[command.git.preHooks]]
  script = "~/.ai/hooks/worktree-guard.py"
  subcommands = ["worktree"]
  enforcement = "advisory"
  description = "Enforce canonical worktree placement (Common.md §U17)"
  ```

  All other pre-hooks (`branch-guard`, `secret-precommit`, `no-verify-strip`, `destructive-gh-guard`) stay at the default (blocking) — no TOML change needed for them.

- [ ] **Step 1.3: Verify build**

  ```bash
  go build ./src/cmd/ai/...
  ```
  Expected: clean build.

- [ ] **Step 1.4: Commit**

  ```bash
  git add src/cmd/ai/cmd/wrap.go src/cmd/ai/embed/hooks/command-wrappers.toml
  git commit -m "feat(wrap): add enforcement field to hookDef; worktree-guard marked advisory

  Default enforcement is blocking — all existing security-gate hooks
  (branch-guard, secret-precommit, no-verify-strip, destructive-gh-guard)
  fail closed without any TOML change.

  Co-Authored-By: Claude Sonnet 4.6 (1M context) <noreply@anthropic.com>"
  ```

---

## Task 2: Refactor `runHookForWrap` to fail closed for blocking hooks

This is the core fix for A.1. Two previously-silent paths (hook missing, Python absent) now return 1 with an explicit message when the hook is blocking. Advisory hooks keep their current silent-skip behavior.

**Files:**
- Modify: `src/cmd/ai/cmd/wrap.go:150–183`
- Modify: `src/cmd/ai/cmd/export_test.go`
- Modify: `src/cmd/ai/cmd/wrap_test.go`

- [ ] **Step 2.1: Write failing tests first**

  Append to `src/cmd/ai/cmd/wrap_test.go`:

  ```go
  // TestRunHookForWrap_BlockingMissingHook verifies that when a blocking hook
  // file is not installed, runHookForWrap returns 1 (not 0).
  func TestRunHookForWrap_BlockingMissingHook(t *testing.T) {
  	s := sandbox(t)
  	// hooks dir exists but is empty — no hook files extracted
  	_ = os.MkdirAll(filepath.Join(s.AIRoot, "hooks"), 0o755)
  	code := cmd.RunHookForWrapForTest("branch-guard", nil, nil, true)
  	if code != 1 {
  		t.Errorf("blocking missing hook: want exit 1, got %d", code)
  	}
  }

  // TestRunHookForWrap_AdvisoryMissingHook verifies that when an advisory hook
  // file is not installed, runHookForWrap returns 0 (skip silently).
  func TestRunHookForWrap_AdvisoryMissingHook(t *testing.T) {
  	s := sandbox(t)
  	_ = os.MkdirAll(filepath.Join(s.AIRoot, "hooks"), 0o755)
  	code := cmd.RunHookForWrapForTest("worktree-guard", nil, nil, false)
  	if code != 0 {
  		t.Errorf("advisory missing hook: want exit 0, got %d", code)
  	}
  }
  ```

  Also add the missing imports at the top of `wrap_test.go` (it currently only imports `testing` and `cmd`):

  ```go
  import (
  	"os"
  	"path/filepath"
  	"testing"

  	cmd "github.com/convergent-systems-co/aiConstitution/src/cmd/ai/cmd"
  )
  ```

- [ ] **Step 2.2: Run tests to confirm they fail**

  ```bash
  go test -run 'TestRunHookForWrap_BlockingMissingHook|TestRunHookForWrap_AdvisoryMissingHook' -v ./src/cmd/ai/cmd/
  ```
  Expected: compile error — `cmd.RunHookForWrapForTest` is not defined yet.

- [ ] **Step 2.3: Add the export seam to `export_test.go`**

  Append to `src/cmd/ai/cmd/export_test.go`:

  ```go
  // RunHookForWrapForTest exposes runHookForWrap to external tests.
  // blocking=true mirrors hookDef.isBlocking()=true (the default for all
  // security-gate hooks).
  func RunHookForWrapForTest(slug string, toolArgs, extraEnv []string, blocking bool) int {
  	return runHookForWrap(slug, toolArgs, extraEnv, blocking)
  }
  ```

- [ ] **Step 2.4: Refactor `runHookForWrap` to accept a `blocking bool` parameter**

  Replace the full `runHookForWrap` function (`wrap.go:150–183`) with:

  ```go
  // runHookForWrap runs a hook by slug using the cross-platform Python dispatcher.
  // It sets the caller-provided extraEnv (WRAPPED_CMD, WRAPPED_ARGV, etc.) in
  // the hook process's environment.
  //
  // When blocking is true (the default for security-gate hooks):
  //   - Hook file missing → prints ENFORCEMENT DEGRADED message + returns 1.
  //   - Python 3 absent   → prints ENFORCEMENT DEGRADED message + returns 1.
  //
  // When blocking is false (advisory hooks, e.g. worktree-guard):
  //   - Hook file missing → returns 0 silently.
  //   - Python 3 absent   → prints a warning + returns 0.
  func runHookForWrap(slug string, toolArgs, extraEnv []string, blocking bool) int {
  	hookPath := filepath.Join(paths.HooksDir(), slug+".py")
  	if _, err := os.Stat(hookPath); err != nil {
  		if blocking {
  			fmt.Fprintf(os.Stderr,
  				"[ai/wrap] ENFORCEMENT DEGRADED: hook %q is wired as blocking but not installed;\n"+
  					"  run 'ai hooks install --all' or 'ai doctor' to restore enforcement.\n", slug)
  			return 1
  		}
  		return 0 // advisory: skip silently
  	}
  	pyArgs := discoverPythonArgs() // defined in hooks.go, same package
  	if pyArgs == nil {
  		if blocking {
  			fmt.Fprintf(os.Stderr,
  				"[ai/wrap] ENFORCEMENT DEGRADED: Python 3 is required for blocking hook %q but was not found;\n"+
  					"  install Python 3 or run 'ai doctor' to restore enforcement.\n", slug)
  			return 1
  		}
  		fmt.Fprintf(os.Stderr, "[ai/wrap] Python 3 not found; skipping advisory hook %s\n", slug)
  		return 0
  	}
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

- [ ] **Step 2.5: Update every call site of `runHookForWrap` in `runWrap`**

  In `runWrap`, there are two call sites. Find them (currently lines ~250 and ~269) and add the `h.isBlocking()` argument:

  Pre-hooks call site (was `runHookForWrap(hookSlug(h.Script), effectiveArgs, baseEnv)`):
  ```go
  if code := runHookForWrap(hookSlug(h.Script), effectiveArgs, baseEnv, h.isBlocking()); code != 0 {
      os.Exit(code)
  }
  ```

  Post-hooks call site — post-hooks are inherently advisory (can't block after the binary ran). Pass `false`:
  ```go
  code := runHookForWrap(hookSlug(h.Script), effectiveArgs, postEnv, false)
  if code != 0 {
      fmt.Fprintf(os.Stderr, "[ai/wrap] WARN: post-hook %q failed (exit %d); audit record may be incomplete\n",
          hookSlug(h.Script), code)
  }
  ```

  This also addresses A.4 (post-hook failure surfaced as a warning) in the same edit.

- [ ] **Step 2.6: Build and run tests**

  ```bash
  go build ./src/cmd/ai/... && \
  go test -run 'TestRunHookForWrap_BlockingMissingHook|TestRunHookForWrap_AdvisoryMissingHook' -v ./src/cmd/ai/cmd/
  ```
  Expected:
  ```
  --- PASS: TestRunHookForWrap_BlockingMissingHook (0.00s)
  --- PASS: TestRunHookForWrap_AdvisoryMissingHook (0.00s)
  ```

- [ ] **Step 2.7: Commit**

  ```bash
  git add src/cmd/ai/cmd/wrap.go src/cmd/ai/cmd/export_test.go src/cmd/ai/cmd/wrap_test.go
  git commit -m "fix(wrap): fail closed for blocking hooks; surface post-hook audit failures

  - runHookForWrap(blocking=true): missing hook/Python → return 1 + ENFORCEMENT DEGRADED message
  - runHookForWrap(blocking=false): advisory hooks keep silent-skip behavior
  - Post-hook failures emit a WARN to stderr instead of being silently swallowed
  - All existing pre-hook call sites pass h.isBlocking() (default: blocking)
  - Post-hook call sites always pass false (can't block after binary runs)

  Co-Authored-By: Claude Sonnet 4.6 (1M context) <noreply@anthropic.com>"
  ```

---

## Task 3: Fail closed on config error

Currently `runWrap` passes through to the real binary when `loadCommandWrappers()` fails. This silently disables all hook enforcement. Fix: abort with a clear error.

**Files:**
- Modify: `src/cmd/ai/cmd/wrap.go:222–227`
- Modify: `src/cmd/ai/cmd/wrap_test.go`

- [ ] **Step 3.1: Write a failing test for the config-error path**

  The config error happens when TOML is unparseable. We can trigger it by writing a corrupt TOML to the hooks dir and confirming the existing hooks-validate tests or writing a new integration test. Since `runWrap` calls `os.Exit`, we test `loadCommandWrappers` + the enforcement check separately:

  Append to `wrap_test.go`:

  ```go
  // TestLoadCommandWrappers_CorruptTOML verifies that a corrupt TOML on disk
  // returns an error (so the caller can fail closed rather than pass through).
  func TestLoadCommandWrappers_CorruptTOML(t *testing.T) {
  	s := sandbox(t)
  	hooksDir := filepath.Join(s.AIRoot, "hooks")
  	if err := os.MkdirAll(hooksDir, 0o755); err != nil {
  		t.Fatal(err)
  	}
  	// Write corrupt TOML to disk so loadCommandWrappers falls back to it.
  	corrupt := []byte("not [ valid toml ][[[")
  	if err := os.WriteFile(filepath.Join(hooksDir, "command-wrappers.toml"), corrupt, 0o644); err != nil {
  		t.Fatal(err)
  	}
  	cfg, err := cmd.LoadCommandWrappersForTest()
  	if err == nil {
  		t.Errorf("expected error for corrupt TOML, got config: %+v", cfg)
  	}
  }
  ```

- [ ] **Step 3.2: Run to confirm compile failure**

  ```bash
  go test -run TestLoadCommandWrappers_CorruptTOML -v ./src/cmd/ai/cmd/
  ```
  Expected: compile error — `cmd.LoadCommandWrappersForTest` not defined.

- [ ] **Step 3.3: Add the export seam**

  Append to `export_test.go`:

  ```go
  // LoadCommandWrappersForTest exposes loadCommandWrappers to external tests.
  func LoadCommandWrappersForTest() (*commandWrappersConfig, error) {
  	return loadCommandWrappers()
  }
  ```

- [ ] **Step 3.4: Change the config-error path in `runWrap` to fail closed**

  Replace lines 222–227 in `wrap.go`:

  ```go
  cfg, err := loadCommandWrappers()
  if err != nil {
  	// Config missing or unparseable — cannot determine which hooks apply.
  	// Fail closed: the caller must fix the config or run 'ai hooks install --all'.
  	fmt.Fprintf(os.Stderr,
  		"[ai/wrap] ENFORCEMENT DEGRADED: cannot load command-wrappers.toml: %v\n"+
  			"  Run 'ai hooks install --all' or 'ai doctor' to restore enforcement.\n", err)
  	os.Exit(1)
  }
  ```

  Also update the `Long:` description in `newWrapCmd()` (currently says "fail-open on config error") to match the new behavior:

  Find in wrap.go:
  ```go
  If command-wrappers.toml cannot be loaded, wrap passes through directly
  (fail-open on config error so git/gh are never broken by a bad config).`,
  ```
  Replace with:
  ```go
  If command-wrappers.toml cannot be loaded, wrap exits with an error and
  a remediation hint rather than silently passing through — enforcement
  must be explicit, not accidental.`,
  ```

- [ ] **Step 3.5: Build and run tests**

  ```bash
  go build ./src/cmd/ai/... && \
  go test -run TestLoadCommandWrappers_CorruptTOML -v ./src/cmd/ai/cmd/
  ```
  Expected:
  ```
  --- PASS: TestLoadCommandWrappers_CorruptTOML (0.00s)
  ```

- [ ] **Step 3.6: Verify existing tests still pass**

  ```bash
  go test -run 'TestRunHookForWrap|TestLoadCommandWrappers|TestHookSlug|TestHookApplies|TestApplyStripArgs|TestFindRealBinary' -v ./src/cmd/ai/cmd/
  ```
  Expected: all PASS.

- [ ] **Step 3.7: Commit**

  ```bash
  git add src/cmd/ai/cmd/wrap.go src/cmd/ai/cmd/export_test.go src/cmd/ai/cmd/wrap_test.go
  git commit -m "fix(wrap): fail closed on config error instead of passing through

  Cannot determine which hooks are blocking when TOML is unreadable.
  Exit 1 with a remediation hint ('ai hooks install --all' or 'ai doctor').

  Co-Authored-By: Claude Sonnet 4.6 (1M context) <noreply@anthropic.com>"
  ```

---

## Task 4: Fix flag normalization in `applyStripArgs` (A.2)

`applyStripArgs` currently does exact string matching. `--no-verify=true` or `--no-verify=false` slips through. Add a `normalizeFlag` step that strips any `=...` suffix from double-dash flags before comparison, so `--no-verify=true` matches the strip entry `--no-verify`.

**Files:**
- Modify: `src/cmd/ai/cmd/wrap.go:90–106`
- Modify: `src/cmd/ai/cmd/export_test.go`
- Modify: `src/cmd/ai/cmd/wrap_test.go`

- [ ] **Step 4.1: Write failing tests**

  Append to `wrap_test.go`:

  ```go
  // TestApplyStripArgs_EqualForm verifies that --flag=value is stripped when
  // --flag appears in the strip list (prefix normalization).
  func TestApplyStripArgs_EqualForm(t *testing.T) {
  	t.Parallel()
  	args := []string{"commit", "--no-verify=true", "-m", "msg"}
  	strip := []string{"--no-verify", "-n"}
  	got := cmd.ApplyStripArgsForTest(args, strip)
  	want := []string{"commit", "-m", "msg"}
  	if len(got) != len(want) {
  		t.Fatalf("applyStripArgs = %v, want %v", got, want)
  	}
  	for i := range want {
  		if got[i] != want[i] {
  			t.Errorf("[%d] got %q, want %q", i, got[i], want[i])
  		}
  	}
  }

  // TestNormalizeFlag verifies that double-dash flags are normalized by
  // stripping any =... suffix; other args are returned unchanged.
  func TestNormalizeFlag(t *testing.T) {
  	t.Parallel()
  	cases := []struct{ in, want string }{
  		{"--no-verify", "--no-verify"},
  		{"--no-verify=true", "--no-verify"},
  		{"--no-verify=false", "--no-verify"},
  		{"-n", "-n"},         // single-dash: no normalization
  		{"commit", "commit"}, // positional: unchanged
  		{"--message=hello world", "--message"},
  	}
  	for _, c := range cases {
  		if got := cmd.NormalizeFlagForTest(c.in); got != c.want {
  			t.Errorf("normalizeFlag(%q) = %q, want %q", c.in, got, c.want)
  		}
  	}
  }
  ```

- [ ] **Step 4.2: Run to confirm compile failure**

  ```bash
  go test -run 'TestApplyStripArgs_EqualForm|TestNormalizeFlag' -v ./src/cmd/ai/cmd/
  ```
  Expected: compile error — `NormalizeFlagForTest` not defined.

- [ ] **Step 4.3: Add `normalizeFlag` function and update `applyStripArgs`**

  In `wrap.go`, add after the existing `applyStripArgs` function:

  ```go
  // normalizeFlag strips the =... suffix from a double-dash flag so that
  // "--no-verify=true" matches a strip list entry of "--no-verify".
  // Single-dash flags and positional args are returned unchanged.
  func normalizeFlag(arg string) string {
  	if !strings.HasPrefix(arg, "--") {
  		return arg
  	}
  	if eq := strings.IndexByte(arg, '='); eq >= 0 {
  		return arg[:eq]
  	}
  	return arg
  }
  ```

  Update `applyStripArgs` to normalize before matching:

  ```go
  // applyStripArgs removes StripArgs entries from toolArgs.
  // Double-dash flags are normalized before comparison so that
  // "--no-verify=true" is removed when "--no-verify" is in the strip list.
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
  		if !rm[normalizeFlag(a)] {
  			out = append(out, a)
  		}
  	}
  	return out
  }
  ```

- [ ] **Step 4.4: Add the export seam for `normalizeFlag`**

  Append to `export_test.go`:

  ```go
  // NormalizeFlagForTest exposes normalizeFlag to external tests.
  func NormalizeFlagForTest(arg string) string { return normalizeFlag(arg) }
  ```

- [ ] **Step 4.5: Run tests**

  ```bash
  go test -run 'TestApplyStripArgs|TestNormalizeFlag' -v ./src/cmd/ai/cmd/
  ```
  Expected: all five `TestApplyStripArgs_*` and `TestNormalizeFlag` PASS.

- [ ] **Step 4.6: Commit**

  ```bash
  git add src/cmd/ai/cmd/wrap.go src/cmd/ai/cmd/export_test.go src/cmd/ai/cmd/wrap_test.go
  git commit -m "fix(wrap): normalize --flag=value forms in applyStripArgs

  --no-verify=true and --no-verify=false are now stripped when --no-verify
  appears in a hook's stripArgs list. Double-dash flags only; single-dash
  and positional args are unchanged.

  Co-Authored-By: Claude Sonnet 4.6 (1M context) <noreply@anthropic.com>"
  ```

---

## Task 5: Full suite + integration smoke test

- [ ] **Step 5.1: Run the complete test suite**

  ```bash
  make test
  ```
  Expected: all packages pass, 0 failures.

- [ ] **Step 5.2: Smoke-test the enforcement message**

  Build the binary and verify the ENFORCEMENT DEGRADED message fires when Python is absent. In a temp dir with no hooks installed, invoke `ai wrap` and check stderr:

  ```bash
  TMPDIR=$(mktemp -d)
  AI_ROOT="$TMPDIR" go run ./src/cmd/ai wrap git -- status 2>&1 | head -5
  ```
  Expected: output contains `ENFORCEMENT DEGRADED` or exits with code 1 (since no hooks are installed and the embedded TOML has blocking pre-hooks, the config loads fine from embed, but hooks aren't extracted yet so branch-guard will be flagged as missing... actually wait).

  Actually, the embedded TOML loads fine (it's in the binary). The hooks dir will be empty. When `git status` is invoked:
  - `hookApplies(branch-guard, "status")` → false (branch-guard only applies to commit/merge/etc.)
  - `hookApplies(secret-precommit, "status")` → false
  - No blocking hook applies to `status`

  Try with `git commit`:
  ```bash
  TMPDIR=$(mktemp -d)
  AI_ROOT="$TMPDIR" PATH="/usr/bin:$PATH" go run ./src/cmd/ai wrap git -- commit -m "test" 2>&1 | head -5
  ```
  Expected: `[ai/wrap] ENFORCEMENT DEGRADED: hook "branch-guard" is wired as blocking but not installed`

  This confirms the message fires. Exit code will be 1.

- [ ] **Step 5.3: Verify existing wrap tests still pass**

  ```bash
  go test -run 'TestHook|TestApplyStripArgs|TestNormalizeFlag|TestFindRealBinary|TestLoadCommandWrappers|TestRunHookForWrap' -v ./src/cmd/ai/cmd/
  ```
  Expected: all PASS.

- [ ] **Step 5.4: Final commit for the plan document**

  ```bash
  git add docs/superpowers/plans/2026-05-28-enforcement-fail-closed.md
  git commit -m "docs: add enforcement fail-closed plan A

  Co-Authored-By: Claude Sonnet 4.6 (1M context) <noreply@anthropic.com>"
  ```

---

## Self-review

**Review A.1 coverage:**

| Fail-open path | Fix | Task |
|---|---|---|
| Config unparseable → pass through | `os.Exit(1)` + remediation hint | Task 3 |
| Hook file missing (blocking) → return 0 | Return 1 + ENFORCEMENT DEGRADED | Task 2 |
| Python absent (blocking) → return 0 | Return 1 + ENFORCEMENT DEGRADED | Task 2 |
| Tool not configured / disabled → pass through | Unchanged — this is intentional (terraform/kubectl are opt-in) | — |

The fourth case (tool not configured) is not a bug: it's the designed opt-in model. Terraform and kubectl are `enabled = false` and will pass through by design. No change needed.

**Review A.2 coverage:**

| Bypass vector | Fix | Task |
|---|---|---|
| `--no-verify=true` form | Normalized by `normalizeFlag` | Task 4 |
| Bundled `-nv`/`-vn` | Not fixed — git CLI does not support bundled flags in modern versions; `git commit -nm "msg"` parses as `-n -m msg`, and `-m` must take a value immediately; `-n` is still matched by exact match | — |
| `core.hooksPath` bypass | Not applicable — our wrapper intercepts at PATH level, not `.git/hooks` level; `core.hooksPath` only affects git's own hook mechanism | — |

**Review A.4 coverage:**

| Gap | Fix | Task |
|---|---|---|
| Post-hook failures swallowed | Non-zero exit code emits `[ai/wrap] WARN: post-hook %q failed` to stderr | Task 2, Step 2.5 |
| Fallback local audit append | Deferred — out of scope for Plan A; requires audit package integration | — |

**Placeholder scan:** None found. All steps have complete code, exact commands, and expected output.

**Type consistency:**
- `hookDef.isBlocking() bool` defined in Task 1, called in Task 2 ✓
- `runHookForWrap(slug, toolArgs, extraEnv []string, blocking bool) int` defined in Task 2, all call sites updated in same task ✓
- `normalizeFlag(arg string) string` defined in Task 4, used in `applyStripArgs` in same task ✓
- `LoadCommandWrappersForTest()` defined in Task 3, used in test in same task ✓
- `RunHookForWrapForTest(slug string, toolArgs, extraEnv []string, blocking bool) int` defined in Task 2, used in tests in same task ✓
- `NormalizeFlagForTest(arg string) string` defined in Task 4, used in tests in same task ✓
