# Plan: 390-391 — ai status unified model + ai doctor hook wiring

## Objective
Eliminate false-positive `✗ Common.md missing` lines for unified-constitution
users (#390), and add hook wiring completeness check to `ai doctor` (#391).

## Rationale

### Alternatives

| Option | #390 | #391 |
|---|---|---|
| A (chosen) | Helper `isUnifiedConstitution()` selects file list | New `checkHookWiring()` with `readWiredHookNames()` |
| B | Suppress output for missing files silently | Reuse `countWiredHooks` and cross-reference | 
| C | Add config flag to select model | Defer to shell script |

Option A is transparent, verifiable, and requires no user configuration.

## Scope

Files modified:
- `src/cmd/ai/cmd/status.go` — add helper, update proseFiles loop, update criticalDoctorChecks
- `src/cmd/ai/cmd/status_test.go` — 4 new tests
- `src/cmd/ai/cmd/doctor.go` — add checkHookWiring, readWiredHookNames, fileExists, homeDir
- `src/cmd/ai/cmd/doctor_test.go` — 4 new tests

## Approach

1. Write failing tests for #390 in status_test.go
2. Implement isUnifiedConstitution + loop changes in status.go
3. Write failing tests for #391 in doctor_test.go
4. Implement checkHookWiring + helpers in doctor.go
5. Run tests, lint, build

## Testing strategy
- Unit tests for all helpers in isolation
- Integration test: unified-model temp dir produces no `✗ Common.md` in status output
- Integration test: checkHookWiring with full settings.json fixture

## Risk assessment
- Low: purely additive changes, no destructive ops
- The existing `TestStatus_ShowsPresentVsMissing` test creates only Constitution.md
  without the siblings — it will now enter unified model, so the ✗ assertions in
  that test need to be verified they still pass under new semantics

## Dependencies
None — self-contained changes in cmd package.

## Backward compatibility
Four-file model users unaffected: all four files present → `isUnifiedConstitution`
returns false → behaviour unchanged.
