# Plan: Wire executeMigrationSteps to real migrate functions (Issue #395)

## Objective
Replace the stub body of `executeMigrationSteps` in `update.go` with real calls to
`runMigrateFlatten`, `runMigrateAddBehavioral`, and `runMigrateGenerateRuntime` from
`migrate.go` (same package).

## Alternatives Table

| Option | Notes |
|--------|-------|
| Keep stub, expose flags | Already done via `ai migrate --flatten` etc. — the update path needs end-to-end wiring. |
| **Wire to migrate functions (chosen)** | One-function change; all three migrate functions are already tested in migrate_test.go. |
| Inline migrate logic in update.go | Code duplication; migrate.go is the source of truth. |

## Scope

Files to modify:
- `src/cmd/ai/cmd/update.go` — replace stub body of `executeMigrationSteps`

Files to create:
- Tests appended to `src/cmd/ai/cmd/update_test.go`

## Approach

1. Read migrate.go to understand what fixtures each function needs.
2. Write failing tests (`TestExecuteMigrationSteps_V1Layout`, `TestExecuteMigrationSteps_Step1Fails`).
3. Replace `executeMigrationSteps` stub body with real calls + progress prints.
4. Run `go test ./src/cmd/ai/...` — green.
5. Run `golangci-lint run ./src/cmd/ai/...` — 0 issues.
6. Run `go build -o /dev/null ./src/cmd/ai/.` — success.
7. Commit, push, open PR.

## Testing Strategy

- `TestExecuteMigrationSteps_V1Layout`: temp dir with all 4 source files; assert "Migration complete." in output and `Constitution.md` still exists.
- `TestExecuteMigrationSteps_Step1Fails`: temp dir with NO files; assert error wraps "migrate step 1 (flatten)"; assert no "[2/3]" or "[3/3]" in output.

## Risk Assessment

- `runMigrateFlatten` reads all 4 files and will error if any are missing — test fixture must include all 4.
- `runMigrateAddBehavioral` expects `Constitution.md` with `## §3` marker after flatten.
- `runMigrateGenerateRuntime` is lenient on parse failures; it writes a (possibly partial) runtime file regardless.

## Dependencies

- `migrate.go` functions already implemented and tested — no changes needed there.

## Backward Compatibility

The stub previously always returned nil. The new implementation may return errors on real failures (v1 layout issues). This is the intended behavior.
