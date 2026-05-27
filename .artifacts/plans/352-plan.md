# Plan: Implement `ai plan list/new/show`

**Status:** in-progress
**Date:** 2026-05-27
**Issue:** #352

## Objective

Implement the three stub commands in `src/cmd/ai/cmd/plan.go`:
`plan new [--title <title>]`, `plan list`, and `plan show <slug>`.

## Rationale

| Alternative | Pros | Cons | Verdict |
|---|---|---|---|
| Implement in plan.go (current stub file) | Consistent with codebase pattern; no new files | None | Chosen |
| New internal package | Testable isolation | Overkill for simple file I/O | Rejected |

## Scope

Files to modify:
- `src/cmd/ai/cmd/plan.go` — replace stubs with real implementations

Files to create:
- `src/cmd/ai/cmd/plan_test.go` — test file (TDD first)

## Approach

1. Write `plan_test.go` with failing tests (RED).
2. Implement `plan.go` to make tests green.
3. Adversarial pass — no stubs remain for list/new/show.

### `plan new [--title <title>]`
- Plans dir: `filepath.Join(paths.AIRoot(), "governance", "plans")`
  (consistent with `amend.go` which uses `aiRoot()/governance/plans`)
- Filename: `<UTC-ISO8601>-<slug>.md` (format `20060102T150405Z`)
- Default title: "new-plan" when `--title` is absent
- Slug derived from title: lowercase, non-alphanumeric → hyphens, trim
- Write MADR template to file
- `os.MkdirAll` for parent dirs
- If `$EDITOR` set: open file; otherwise print path
- Print "Created plan: <path>"

### `plan list`
- Walk `paths.AIRoot()/governance/plans/*.md`
- Extract date from filename prefix (first 16 chars `YYYYMMDDTHHMMSSZ`)
- Read first non-empty line as title (strip leading `# `)
- Print aligned table: `DATE | SLUG | TITLE`
- Empty/missing dir: print "(no plans yet)"

### `plan show <slug>`
- Look for `<slug>.md` or `*-<slug>.md` in the plans dir
- Print file contents
- Clear error if not found

## Testing Strategy

- `TestPlanNew_CreatesFile` — file exists, has MADR template
- `TestPlanNew_WithTitle` — slug derived from title
- `TestPlanNew_DefaultTitle` — missing `--title` uses "new-plan"
- `TestPlanList_Empty` — "(no plans yet)" output
- `TestPlanList_WithPlans` — table with DATE | SLUG | TITLE columns
- `TestPlanShow_Found` — prints file content
- `TestPlanShow_NotFound` — clear error, non-nil

## Risk Assessment

- Low: all I/O is under user's `~/.ai/` dir
- `$EDITOR` launch is skipped in tests by unsetting the env var (same pattern as amend_test.go)

## Dependencies

- `paths.AIRoot()` from `src/internal/paths` (already imported via `amend.go`'s `aiRoot()`)
- `os`, `path/filepath`, `strings`, `time`, `fmt` — all stdlib

## Backward Compatibility

- Replaces stubs — no existing behavior to break
