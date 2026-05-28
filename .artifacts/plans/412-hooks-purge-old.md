# Plan: #412 — Purge old absolute-path hook entries before re-wiring

## Objective
Remove stale pre-v1.3 hook entries (absolute paths like `python3 ~/.ai/hooks/audit.py`) from
`.claude/settings.json` before writing new portable `ai hooks run <slug>` entries, so hooks
never fire twice after an upgrade + re-wire.

## Rationale

After PR #411 changed wiring to use `ai hooks run <slug>`, re-running `ai hooks install --claude`
on a repo that already had the old absolute-path entries adds new entries without removing the old
ones. Both formats fire, doubling every hook invocation.

### Alternatives Table

| Approach | Verdict |
|---|---|
| Purge old entries before wiring (chosen) | Clean and idempotent; simple filter over known patterns |
| One-time migration command | Requires users to run extra step; chosen approach is automatic |
| Dedup on command suffix | Fragile — can't distinguish intentional duplicates from staleness |

## Scope

- **Modify:** `src/cmd/ai/cmd/hooks_claude.go` — add helpers + wire into `installClaudeHooks`
- **Modify:** `src/cmd/ai/cmd/hooks_claude_test.go` — add 5 new test cases
- **Modify:** `src/cmd/ai/cmd/export_test.go` — export `PurgeOldHookEntriesForTest`

## Approach

1. Add `isAbsoluteHookCmd(cmd string) bool`
2. Add `isOldHookEntry(m map[string]any) bool`
3. Add `purgeOldHookEntries(settings map[string]any)`
4. Call `purgeOldHookEntries(settings)` in `installClaudeHooks` after `loadClaudeSettings`
5. Export `purgeOldHookEntries` via `export_test.go` for white-box unit tests
6. Add 5 test functions to `hooks_claude_test.go`

## Testing strategy

- Unit tests via exported `PurgeOldHookEntriesForTest`:
  - Removes flat absolute-path entries
  - Removes group-format absolute-path entries
  - Preserves portable `ai hooks run` entries
  - No panic on empty/missing hooks section
- Integration test: temp dir with old entries → `installClaudeHooks` → assert purged + rewired, no duplicates

## Risk assessment

- **Risk:** Overly broad detection could remove legitimate entries.
  - **Mitigation:** Detection requires `/.ai/hooks/` in the command, which is highly specific.
- **Risk:** Mutation of slice via `entries[:0]` if underlying array is still referenced.
  - **Mitigation:** Assign result back to `hooks[event]`.

## Dependencies

None beyond existing imports (`strings`).

## Backward compatibility

No external API change. Internal function, no callers outside this package.
