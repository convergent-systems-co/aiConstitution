# Plan: #417 — Migrate ai skills install to ai-atoms.com catalog

## Objective
Replace the GitHub API call in `runSkillsInstall` with a fetch from the ai-atoms.com catalog, so installs are catalog-driven and consistent with `ai skills available`.

## Rationale

| Alternative | Verdict |
|---|---|
| Keep GitHub API for install | Old approach; catalog is the canonical source of truth |
| Fetch catalog once and reuse | Chosen — catalog already fetched by `available` via `fetchAiAtomsCatalog` |
| New dedicated per-skill endpoint on ai-atoms.com | Not yet implemented; catalog is the available surface |

## Scope
- **Modify:** `src/cmd/ai/cmd/ai_atoms.go` — add `SystemPromptFragment` field, `aiAtomEntryToSkillAtom`, `fetchSkillAtomFromCatalog`
- **Modify:** `src/cmd/ai/cmd/skills.go` — swap `fetchSkillAtomJSON` → `fetchSkillAtomFromCatalog` in `runSkillsInstall`; deprecation comment on `fetchSkillAtomJSON`
- **Add:** `src/cmd/ai/cmd/skills_install_catalog_test.go` — new catalog-based install tests

## Approach
1. Add `SystemPromptFragment` field to `aiAtomEntry` in `ai_atoms.go`
2. Add `aiAtomEntryToSkillAtom` converter function
3. Add `fetchSkillAtomFromCatalog` that calls `fetchAiAtomsCatalog` and looks up by slug
4. Add `"strings"` import to `ai_atoms.go`
5. In `skills.go`, swap `fetchSkillAtomJSON` to `fetchSkillAtomFromCatalog` in `runSkillsInstall`
6. Add deprecation comment above `fetchSkillAtomJSON`
7. Write failing tests first, then implement

## Testing strategy
- `TestSkillsInstall_WritesSkillMD` — install "commit" from catalog mock, verify SKILL.md content
- `TestSkillsInstall_NotFoundError` — slug not in catalog → error contains "not found"
- `TestSkillsInstall_InstallsDependencies_Catalog` — skill with `depends_on` installs deps from catalog

## Risk assessment
- Existing `TestSkillsInstall_*` tests using `setSkillAtomBaseURL` will continue to pass for `upgrade` (which still calls `fetchSkillAtomJSON`); install tests need updating
- The `--all` flag in install uses `fetchSkillsDirectory()` (GitHub API) — not in scope for this issue; leave as-is

## Dependencies
None.

## Backward compatibility
- `fetchSkillAtomJSON` remains in the codebase (marked deprecated) for `upgrade`/`show` commands
- Behavior change: install now fetches from catalog rather than GitHub API per-file endpoint
