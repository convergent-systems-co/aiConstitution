# Plan: 418 — hooks install from ai-atoms.com catalog with embed fallback

**Status:** In Progress

## Objective

Implement `ai hooks install <slug>` and `ai hooks install --all` to fetch hook scripts from the ai-atoms.com catalog when available, falling back to the embedded copy when not.

## Rationale

The ai-atoms.com catalog will soon carry a `script` field on hook atoms (parallel PR). This change makes the CLI catalog-aware for hooks the same way it already is for skills, while preserving the embedded fallback for backward compatibility during the transition period.

### Alternatives Table

| Option | Notes |
|---|---|
| Catalog-first with fallback (chosen) | Best of both worlds: fresh catalog scripts when available, embedded copy always works |
| Catalog only | Breaks install when offline or during catalog transition |
| Embed only | Status quo — no catalog integration |

## Scope

Files to modify:
- `src/cmd/ai/cmd/ai_atoms.go` — add `Script` field to `aiAtomEntry`
- `src/cmd/ai/cmd/hooks.go` — update `installOneHook` and `installAllHooksAndWire`

Files to create:
- `src/cmd/ai/cmd/hooks_catalog.go` — `installHookFromCatalog` + `ErrHookNotInCatalog`
- `src/cmd/ai/cmd/hooks_catalog_test.go` — 4 targeted tests

## Approach

1. Add `Script string` field to `aiAtomEntry` in `ai_atoms.go`
2. Create `hooks_catalog.go` with `ErrHookNotInCatalog` and `installHookFromCatalog(slug, hooksDir)`
3. Update `installOneHook` in `hooks.go` to try catalog first, fall back to embed
4. Update `installAllHooksAndWire` in `hooks.go` to try catalog for all hook atoms, then always run `embed.ExtractAllHooks` for infrastructure files
5. Write tests first (TDD)

## Testing Strategy

- `TestInstallHookFromCatalog_Success` — mock server, hook atom with `script` → file written at correct path with 0755 mode
- `TestInstallHookFromCatalog_NotFound` — hook absent from catalog → `ErrHookNotInCatalog`
- `TestInstallHookFromCatalog_NoScript` — atom present but `script` empty → `ErrHookNotInCatalog`
- `TestInstallHookFromCatalog_NetworkError` — 500 response → wrapped error, not `ErrHookNotInCatalog`

## Risk Assessment

- Catalog fetch adds network I/O to `hooks install`. Non-fatal on failure — falls back to embed.
- `DependsOn` recursion: bounded by catalog size; non-fatal on dep failure matches skill behavior.

## Backward Compatibility

- Existing `hooks install --all` output format changes when catalog installs succeed (new message with counts). Tests updated accordingly.
- All existing hooks_install_test.go tests must remain green (catalog URL is empty by default in those tests → falls through to embed).
