# Plan: Implement `ai hooks propose` (#388)

## Objective

Replace the stub body in `src/cmd/ai/cmd/hooks.go` for `ai hooks propose <name>` with a working implementation that scaffolds `.py` or `.sh` hook files at `~/.ai/hooks/<name>.<ext>`.

## Scope

- **Modify:** `src/cmd/ai/cmd/hooks.go` — replace stub, add `runHooksPropose` function
- **Create:** `src/cmd/ai/cmd/hooks_propose_test.go` — 6 test cases

## Approach

1. Extract `runHooksPropose(name, fromViolation, lang, aiRoot string, out io.Writer) error`
2. Wire it into the cobra `RunE` closure (thin wrapper)
3. Language dispatch: `python` (default) → `.py`, `sh` → `.sh`, others → error
4. If file already exists → error
5. If `--from-violation` given → read file, extract `**What happened:**` paragraph, use as description
6. Create `~/.ai/hooks/` if missing (mode 0o750)
7. Write scaffold with 0o755 permissions
8. Print `Created:` and `Next:` lines to `out`

## Testing Strategy

- 6 tests calling `runHooksPropose` directly with temp dirs and `bytes.Buffer`
- No cobra wiring needed for tests; validates the pure function

## Risk Assessment

- Low: self-contained, no external dependencies, fully reversible
- Mitigation: temp dir per test, no real `~/.ai` touched

## Alternatives Considered

| Alternative | Rejected because |
|---|---|
| Cobra-level tests via `runHooksCmd` | Can't inject `aiRoot`; tests would hit real `~/.ai` |
| AI-mediated scaffold generation | Overkill for v1; issue says "scaffolding without AI mediation is sufficient" |

## Backward Compatibility

Replaces a stub that returns `errNotImplementedHint`. Any caller that tests for that error will now see a `nil` on success or a typed error on failure. No production users of this stub path.
