# Plan: Implement `ai mode list/current/show` (#345)

## Objective
Replace the three stubs in `src/cmd/ai/cmd/mode.go` with working implementations for `mode current`, `mode list`, and `mode show`.

## Alternatives Table

| Alternative | Pros | Cons | Verdict |
|---|---|---|---|
| Reuse persona/profile `.yaml` scan (matching existing `ai persona list` and `ai profile list` patterns) | Consistent with existing code, no new dependencies, tests easy to write | Does not include atom cache | **Chosen** |
| Walk atom cache subdirs for `atom.json` | Covers cached remote atoms | No atoms installed in typical dev env, complex fixture setup, spec says `atom.json` but no existing code creates it | Rejected for V1 — can extend later |
| Parse CLAUDE.md block for `mode current` | Reuse existing `readActivePersonas` | mode.json is the authoritative state written by `ai mode pm` and `ai mode <name>` | Rejected — wrong source |

## Scope
- **Modify:** `src/cmd/ai/cmd/mode.go` (worktree copy)
- **Add:** `src/cmd/ai/cmd/mode_list_test.go` (new tests — 6 cases)

## Approach

### mode current
1. Read `paths.ModeJSON()` → parse as `pmModeJSON` (already defined in mode.go).
2. If file absent or `.Mode == ""` → print `(none)`.
3. Otherwise print the `Mode` field.

### mode list
1. Walk `paths.AIRoot()/personas/` for `.yaml` files → TYPE = persona, SOURCE = local.
2. Walk `paths.ConfigDir()/profiles/` for `.yaml` files → TYPE = profile, SOURCE = local.
3. Print tabwriter table: `NAME | TYPE | SOURCE`.
4. If both dirs empty/absent → print `(no modes available)`.

### mode show <name>
1. Search `paths.AIRoot()/personas/<name>.yaml`.
2. If not found, search `paths.ConfigDir()/profiles/<name>.yaml`.
3. If found, print file content.
4. If not found, return `fmt.Errorf("mode %q not found", name)`.

## Testing Strategy
6 tests in `mode_list_test.go`, all using `modeTestEnv` + `t.Setenv("AI_ROOT", ...)` for isolation:
- `TestModeCurrent_NoFile` — returns `(none)` when mode.json missing
- `TestModeCurrent_WithMode` — reads and prints the `mode` field
- `TestModeList_EmptyDirs` — prints `(no modes available)` when dirs empty
- `TestModeList_WithPersonas` — lists persona .yaml from AI_ROOT/personas/
- `TestModeShow_Found` — prints atom.yaml content
- `TestModeShow_NotFound` — returns error

## Risk Assessment
- **Race**: `modeTestEnv` sets `AICONST_CONFIG_DIR`; persona tests set `AI_ROOT`. Both must be set for mode list tests. Low risk — both env vars are set per-test with `t.Setenv`.
- **Stub function signature**: `stub()` returns `error` with "not yet implemented". Removing the stub calls removes the error path; must ensure RunE returns nil on success.

## Dependencies
- No new imports beyond what mode.go already uses (`encoding/json`, `os`, `path/filepath`, `text/tabwriter`, `errors`).

## Backward Compatibility
- `mode pm`, `mode clear` unchanged.
- `mode current`/`list`/`show` currently return stub errors; replacing with working code is forward-only.
