# Plan: cli-identity — profile, persona, pm-mode, spawn

**Date:** 2026-05-24
**Branch:** feature/cli-identity
**Issues:** #215, #216, #217, #218, #219, #220
**Tech Lead:** TL2 (domain: cli-identity)

---

## Objective

Implement six CLI features (profile list/show, profile new/edit/remove, persona list, persona show, pm-mode, spawn) so they produce real behavior instead of stub errors.

---

## Rationale

| Alternative | Pros | Cons | Verdict |
|---|---|---|---|
| Implement in existing stub files (profile.go, persona.go, mode.go) | Keeps cobra tree registrations intact; no root.go changes | None significant | Chosen |
| Create new files per feature | Cleaner isolation | Breaks existing cmd registration, more merge surface | Rejected |
| Rewrite state package first | Cleaner abstraction | Blocks all six issues; over-engineered for this wave | Rejected — implement inline, state package TBD |

---

## Scope

### Files to modify
- `src/cmd/ai/cmd/profile.go` — replace stubs with real list/show/new/edit/remove logic
- `src/cmd/ai/cmd/persona.go` — replace stubs with real list/show logic and `--type` flag
- `src/cmd/ai/cmd/mode.go` — add `pm-mode` subcommand (writes mode.json)
- `src/cmd/ai/cmd/root.go` — register `newPmModeCmd()` and `newSpawnCmd()` (if separate file)

### Files to create
- `src/cmd/ai/cmd/profile_test.go` — TDD tests for profile commands
- `src/cmd/ai/cmd/persona_test.go` — TDD tests for persona commands
- `src/cmd/ai/cmd/mode_test.go` — TDD tests for mode/pm-mode commands
- `src/cmd/ai/cmd/spawn_cmd.go` — spawn command implementation
- `src/cmd/ai/cmd/spawn_cmd_test.go` — TDD tests for spawn command

---

## Approach

### #215 — profile list/show

1. `profile list`:
   - Reads `~/.config/aiConstitution/profiles/` via `paths.ConfigDir()` + `/profiles/`
   - Parses YAML frontmatter for `description:` field from each `*.yaml`
   - Prints `name | description` table
   - Prints `(no profiles)` on empty dir or dir-not-exist
2. `profile show <name>`:
   - Finds `<name>.yaml` in profiles dir (exact match, then slug match)
   - Cats file content to stdout
   - Returns error when not found

### #216 — profile new/edit/remove

3. `profile new <name>`:
   - Target path: `~/.config/aiConstitution/profiles/<name>.yaml`
   - Errors if file already exists
   - Writes frontmatter: `name: <name>\ndescription: ""\ndomains: []\n`
4. `profile edit <name>`:
   - Opens `$EDITOR <path>` via `exec.Command`
   - When `$EDITOR` unset: prints path to stdout
5. `profile remove <name>`:
   - Deletes file
   - Returns error if not found

### #217 — persona list

6. `persona list`:
   - Reads `~/.ai/personas/*.yaml` via `paths.AIRoot()` + `/personas/`
   - Parses YAML frontmatter for `name:`, `type:`, `description:`
   - Prints `name | type | description` table
   - `--type agentic|reviewer` flag filters by type field
   - Prints `(no personas installed)` if empty

### #218 — persona show

7. `persona show <name>`:
   - Looks up `<name>.yaml` in `~/.ai/personas/` (exact/slug match)
   - Cats content to stdout
   - Returns error when not found

### #219 — pm-mode

8. `pm-mode` subcommand on `mode`:
   - Writes `~/.config/aiConstitution/mode.json`:
     `{"mode":"pm","activatedAt":"<UTC RFC3339>","discipline":"plan-first"}`
   - Prints: `PM mode activated. Plan-first discipline is active.`
   - Also registered at root as `ai pm-mode` shortcut

### #220 — spawn

9. `spawn <name>` in `spawn_cmd.go`:
   - Resolves persona: `~/.ai/personas/<name>.yaml` first, error if not found
   - Reads current mode from `~/.config/aiConstitution/mode.json` (empty string if absent)
   - Writes `~/.config/aiConstitution/state/spawn.json`:
     `{"persona":"<name>","spawnedAt":"<UTC RFC3339>","parentMode":"<current mode>"}`
   - Prints a markdown activation block with persona file content as context
   - Returns error when persona not found

---

## Testing Strategy

All tests use real `os.MkdirTemp` directories. Paths are injected via
`AICONST_CONFIG_DIR` and `AI_ROOT` environment variables (already honored
by `paths.ConfigDir()` and `paths.AIRoot()`).

**TDD Writer produces RED tests first.** Coders make them GREEN.

Key test cases:
- profile list: scans temp dir, parses YAML description, empty dir
- profile new: creates file, errors on duplicate
- profile show: finds file, errors on missing
- profile edit: prints path when EDITOR unset
- profile remove: deletes file, errors on missing
- persona list: reads personas dir, applies --type filter, empty dir
- persona show: finds file, errors on missing
- pm-mode: writes correct JSON to mode.json
- spawn: writes spawn.json, prints activation block, errors on missing persona

---

## Risk Assessment

| Risk | Mitigation |
|---|---|
| YAML parsing — go.mod has no yaml dep | Parse frontmatter with simple line scanner (no external dep needed for key: value fields) |
| `$EDITOR` invocation in tests | Test only path-printing path (EDITOR unset); exec path skipped in unit tests |
| spawn.json state dir | Use `os.MkdirAll` before write |
| Concurrent persona dir reads | Tests are sequential; no race condition in this batch |

---

## Dependencies

- `paths.ConfigDir()` and `paths.AIRoot()` already exist and honor env overrides — use directly
- No new go.mod deps required (frontmatter parsed manually)
- `state` package is NOT used — mode.json is written directly (state package is all TBD stubs)

---

## Backward Compatibility

- All existing stub commands are replaced — no behavior was shipped, so no migration needed
- `root.go` gets two new registrations (`newPmModeCmd()`, `newSpawnCmd()`) — additive only
- No existing command signatures are changed

---

## Out of Scope

- `profile share` (spec §7.9.3) — remains stub
- `persona share` — remains stub
- `mode list / mode show / mode clear / mode share` — remain stubs
- `ai mode <name>` (generic activation) — remains stub
- Network fetch from persona-atoms.com — not in this batch
- State package implementation — deferred

---

## Coder Partition

- **Coder A OWNS:** `profile.go`, `profile_test.go` (issues #215, #216)
- **Coder B OWNS:** `persona.go`, `persona_test.go` (issues #217, #218)
- **Coder C OWNS:** `mode.go`, `spawn_cmd.go`, `spawn_cmd_test.go`, `mode_test.go` (issues #219, #220); root.go registration for spawn
