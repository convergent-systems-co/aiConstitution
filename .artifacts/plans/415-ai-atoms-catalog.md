# Plan: #415 — Switch to ai-atoms.com catalog

## Objective
Replace the GitHub Contents API catalog lookups in `ai skills available` and
`ai hooks available` with a single fetch from `https://ai-atoms.com/exports/catalog.json`.

## Approach

1. Create `src/cmd/ai/cmd/ai_atoms.go` — `AiAtomsCatalogURL` var, `aiAtomEntry`/`aiAtomsCatalog` structs, `fetchAiAtomsCatalog()` helper.
2. Update `runSkillsAvailable` in `skills.go` — replace GitHub directory + per-atom fetches with catalog filter on `type == "skill"`.
3. Update `runHooksAvailable` in `hooks.go` — replace `fetchHookAtoms()` (GitHub-based) with catalog filter on `type == "hook"`. Remove `fetchHookAtomsDirectory`, `fetchHookAtoms`, `hookAtomEntry` now defined in skills.go.
4. Add `AiAtomsCatalogURLForTest` export in `export_test.go`.
5. Write `ai_atoms_test.go` (unit tests for catalog fetch/parse/error).
6. Update `hooks_available_test.go` and `skills_test.go` to use the catalog format mock instead of GitHub directory mock.

## Key decisions
- Keep `SkillAtomsBaseURL` and `fetchSkillAtomFromURL` / `fetchSkillAtomJSON` — still used by `ai skills install/upgrade`.
- Keep `fetchSkillsDirectory` — still used by `ai skills install --all`.
- Remove `fetchHookAtomsDirectory`, `fetchHookAtoms`, `hookAtomEntry` from `skills.go` — replaced by catalog.
- Warn string stays `"could not reach skill-atoms.com"` in hooks available (existing test expects this).
- New catalog error in skills available returns the error (breaking, but consistent with existing behavior).
