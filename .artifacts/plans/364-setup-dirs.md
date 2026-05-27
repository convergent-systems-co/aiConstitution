# Plan: 364 — Create audit/memory/governance dirs on install; fix hook +x

## Objective
Ensure all directories the system writes to on first use exist after `ai setup`, and that Python hook files are extracted with executable permissions.

## Alternatives Table

| Alternative | Pros | Cons | Verdict |
|---|---|---|---|
| Create dirs in `runSetupPostWizard` (chosen) | Single place, runs on every setup path | — | Chosen |
| Create dirs lazily at first write | No setup change needed | Fragile — first write from hook/audit code races ahead of any dir guarantee | Rejected |
| Create dirs via Makefile install target | Works for dev workflow | Doesn't help end users who install via `go install` | Rejected |

## Scope

Files to modify (in worktree):
- `src/cmd/ai/cmd/setup.go` — add `MkdirAll` loop after `os.MkdirAll(aiRoot, 0o750)`
- `src/cmd/ai/embed/embed.go` — add `"strings"` import (already has `executableForName` fix; import is missing, causing build failure)

Files to add/modify (tests):
- `src/cmd/ai/cmd/setup_test.go` — add `TestSetupCreatesDirectories`
- `src/cmd/ai/embed/embed_test.go` — add `TestHookPermissions`

## Approach

1. Fix `embed.go`: add `"strings"` to import block.
2. Modify `setup.go`: insert the 11-dir `MkdirAll` loop after the `aiRoot` mkdir, before writing Constitution.md.
3. Add `TestSetupCreatesDirectories` to `setup_test.go`: run `runSetupPostWizard` with temp dirs, verify all 11 subdirs exist.
4. Add `TestHookPermissions` to embed test file: verify `executableForName` returns `0o755` for `.py` and `0o644` for others.
5. Run tests; verify green.

## Testing Strategy
- `TestSetupCreatesDirectories`: calls `runSetupPostWizard` (noHooks=true) against temp dir; checks `os.Stat` on each of the 11 paths.
- `TestHookPermissions`: calls `executableForName` directly; asserts mode values.

## Risk Assessment
- Low: additive changes only. MkdirAll is idempotent.
- Missing `"strings"` import in embed.go currently breaks the build — fixing it is a prerequisite for all tests.

## Dependencies
None — self-contained.

## Backward Compatibility
Fully additive. Directories that already exist are unaffected (MkdirAll is a no-op on existing dirs).
