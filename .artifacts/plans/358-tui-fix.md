# Plan: fix(setup) — TTY detection + suppress misleading migration warning (#358)

**Objective:** Fix two UX bugs in `ai setup` / `ai --tui` so that (1) the runtime-extraction warning no longer alarming users when migration can't fully parse their pre-existing constitution, and (2) `ai setup` fails gracefully rather than silently in non-TTY environments.

## Alternatives Table

| Alternative | Pros | Cons | Verdict |
|---|---|---|---|
| Bug 1: Make warning verbose-only (use `cmd.Printf` behind a `--verbose` flag) | Standard CLI pattern | Requires flag wiring | Rejected — adds complexity for a zero-user-value message |
| Bug 1: Silently continue when ExtractRuntime fails | Simple; user sees no alarm; wizard still runs and produces correct file | No signal at all | Chosen — runtime file is a bonus optimization |
| Bug 2: Use `mattn/go-isatty` (already indirect dep) | No new deps; tiny surface | Need to promote to direct dep in go.mod | Considered — but `charmbracelet/x/term` is the better fit since bubbletea already uses it |
| Bug 2: Use `charmbracelet/x/term` (already indirect dep) | Already in go.mod (indirect); same package bubbletea uses internally | Indirect dep | Chosen — promote to direct import; no new module fetch needed |
| Bug 2: Wrap prog.Run() error and fall back without TTY check | Still runs TUI startup, only recovers on failure | Garbled output before fallback | Partial — combine with upfront TTY check |

## Scope

**Files to modify:**
- `src/cmd/ai/cmd/migrate.go` — suppress warning in `runMigrateGenerateRuntime` when extraction fails
- `src/cmd/ai/cmd/setup.go` — add TTY detection before `tea.NewProgram`, wrap `prog.Run()` error with fallback
- `src/cmd/ai/go.mod` + `go.sum` — promote `github.com/charmbracelet/x/term` to direct dependency

**Files to add:**
- `src/cmd/ai/cmd/migrate_test.go` — new test `TestRunMigrateGenerateRuntime_PartialExtractionSilent`
- `src/cmd/ai/cmd/setup_test.go` — new test `TestRunSetupTUI_NonTTY_FallsBack`

## Approach

1. **migrate.go:** In `runMigrateGenerateRuntime`, when `ExtractRuntime` returns an error, continue silently (don't call `fmt.Fprintf` with a warning). Still write whatever `FormatRuntime` produces.
2. **setup.go:** 
   - Import `github.com/charmbracelet/x/term`
   - Before `tea.NewProgram(m)`, call `term.IsTerminal(int(os.Stdout.Fd()))` — if false, print fallback message to stderr and call `runSetupNonInteractive("")`
   - Wrap `prog.Run()` error: on failure fall back to `runSetupNonInteractive("")` rather than returning an error
3. **go.mod:** Promote `github.com/charmbracelet/x/term` from indirect to direct

## Testing Strategy

- `TestRunMigrateGenerateRuntime_PartialExtractionSilent`: run `--generate-runtime` on a Constitution.md that lacks `§3.1 Prime Directives` and verify no "warning:" line appears in stdout/stderr
- `TestRunSetupTUI_NonTTY_FallsBack`: call `runSetupTUI` in a test environment where stdout is not a TTY (always true in test), verify it calls the non-interactive path and does not panic or return TUI startup errors

## Risk Assessment

- `charmbracelet/x/term` is an indirect dep so no network fetch needed; `go mod tidy` will stabilize it as direct
- The TTY fallback calls `runSetupNonInteractive("")` which uses AICONST_SEEDS env var — safe in tests via `t.Setenv`
- Silencing the ExtractRuntime warning is a strict narrowing of output — no behavior change

## Backward Compatibility

No breaking changes. Warning removal is a pure UX improvement. TTY fallback is a new code path (previously: crash or garbled output).

## Dependencies

None external — all within the `src/cmd/ai` module.
