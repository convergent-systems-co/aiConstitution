# Plan — Issue #398: ai hooks available fetches ai-hook atoms from skill-atoms.com

## Objective
Extend `ai hooks available` to also list `type: "ai-hook"` atoms from skill-atoms.com in addition to the embedded hooks, following the exact same fetch pattern already used by `ai skills available`.

## Rationale

### Alternatives Table
| Option | Notes |
|---|---|
| **Extend `ai hooks available` (chosen)** | Mirrors `ai skills available`; minimal new surface area; degrades gracefully on network failure |
| Add a separate `ai hooks registry` subcommand | More surface area; inconsistent UX with skills |
| Extend `ai hooks install` only (no listing) | Discoverability is the whole point of `available` |

## Scope
- **Modify:** `src/cmd/ai/cmd/hooks.go` — update the `available` subcommand
- **Modify:** `src/cmd/ai/cmd/skills.go` — add `Events []string` field to `skillAtom`; add `fetchHookAtomsDirectory()` and `fetchHookAtoms()` functions
- **Create:** `src/cmd/ai/cmd/hooks_available_test.go` — new test file with 3 required tests

## Approach
1. Add `Events []string json:"events,omitempty"` field to the `skillAtom` struct in `skills.go`.
2. Add `fetchHookAtomsDirectory()` — fetches `/skills/ai-hook` directory listing (reuses `skillAtomDirEntry`).
3. Add `hookAtomEntry` struct and `fetchHookAtoms()` function — fetches each `.json` file and hydrates a `hookAtomEntry`, filtering deprecated/retired.
4. Update the `available` subcommand RunE in `hooks.go` to:
   a. Print embedded hooks under `Embedded hooks (built-in):` header.
   b. Call `fetchHookAtoms()`.
   c. On success with results: print `Registry hooks from skill-atoms.com:` table.
   d. On failure: print `(could not reach skill-atoms.com: <err>)` warning.
   e. On empty: no registry section.

## Testing Strategy
Three tests in `hooks_available_test.go`:
- `TestHooksAvailable_RegistryFetched` — mock server with valid ai-hook atoms; assert registry section appears.
- `TestHooksAvailable_RegistryUnreachable` — mock server returning 500; assert embedded hooks still show, warning appears.
- `TestHooksAvailable_RegistryEmpty` — mock server returning empty array; assert embedded hooks show, no registry section.

## Risk Assessment
- **Network in prod:** fetch is best-effort; failure path is non-fatal (warning only). Low risk.
- **Linter:** must use `//nolint:noctx` on raw http.NewRequest calls (consistent with existing pattern).
- **Struct extension:** adding `Events []string` to `skillAtom` is backward-compatible (omitempty).

## Dependencies
None — no prerequisite PRs.

## Backward Compatibility
The `available` command output format changes: a new section is added when the registry has results. Existing callers that only check for the embedded-hooks header still work.
