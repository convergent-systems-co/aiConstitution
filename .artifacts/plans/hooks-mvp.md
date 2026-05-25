# hooks-mvp Implementation Plan
**Date:** 2026-05-24
**Branch:** feature/hooks-mvp
**Stories:** #84, #91, #92, #93, #94, #96, #180

---

## Objective

Ship a complete, tested enforcement-hook suite for the `ai` CLI: install wiring, all five hook scripts synced/implemented to spec, and an adversarial validation test suite that proves bypass paths are blocked.

---

## Alternatives Table

| Alternative | Pros | Cons | Verdict |
|---|---|---|---|
| Sync only — make embedded == live | Minimal diff, fast | Misses #84/#96 (install wiring), misses #180 (tests) | Rejected |
| Rewrite all hooks from scratch | Clean slate | High risk of regression vs. battle-tested live versions | Rejected |
| Sync live→embedded + add install wiring + tests | Preserves battle-tested logic, adds missing pieces, covers all 7 stories | Larger surface | **Chosen** |

---

## Discovery Summary

### branch-guard.py — embedded vs live delta
The **live** version (`~/.ai/hooks/branch-guard.py`) has these features **missing from the embedded** version:
1. `split_invocations()` — recursively descends into `bash -c '...'` to catch nested git calls (bash-c bypass fix)
2. `--no-verify` denial on protected branches (pre-empt hook bypass)
3. `parse_git_call()` — properly handles `git -C <path>` global flags
4. `push_targets_protected()` — strips `refs/heads/` prefix and handles `+<refspec>` forced-push syntax
5. Proper `deny()` output schema (JSON stdout with `permissionDecision`) instead of `_lib.log()` + exit 1
6. `pull --ff-only` carve-out (safe fast-forward sync, not a mutation)
7. Uses event `cwd` field from Claude payload (not `subprocess.run` for current branch)

**New addition (proposed in #92):** Allow first push to a remote ref that doesn't yet exist (empty repo bootstrap).

### audit.py — embedded vs live delta
The live version has more event types and richer per-kind payload fields:
- `PostToolUseFailure` → `invocation-failure`
- `SubagentStart` → `subagent-trace-open`
- `Notification` → `signal`
- `ErrorOccurred` → `fault`
- `PermissionRequest` → `permission-prompt`
- Rich per-kind fields: `probe_result_marker`, `compaction_trigger`, `fault_*`, `permission_reason`, `signal_*`, `transcript_marker`

The embedded version handles only 8 core events and maps them to a simpler schema. **Plan:** upgrade embedded audit.py to match the live version's richer KIND_MAP and per-kind fields.

### secret-block.py — embedded vs live delta
The **live** version reads tool output via Claude's permission-decision JSON schema (stdout `permissionDecision` deny). The **embedded** version returns exit 1 + stderr. 

The live version also uses **`patterns.json`** via `_lib.load_patterns()` — the canonical pattern source — while the embedded version has hardcoded inline patterns.

**Plan:** upgrade embedded secret-block.py to use `_lib.load_patterns()` and emit proper permission-decision JSON (matching live).

### worktree-guard.py — embedded vs live delta
The **live** version (`~/.ai/hooks/worktree-guard.py`) is more feature-rich:
- Uses `split_invocations()` (from its own implementation) to handle compound commands
- Emits JSON `permissionDecision` deny (not exit 1)
- Uses event `cwd` from Claude payload for repo-root detection

The **embedded** version has simpler `from_claude_payload()` that uses `cmd.split()[1:]` (fragile, no shlex).

**Plan:** upgrade embedded worktree-guard.py to match live behavior.

### hooks.go — missing settings.json wiring
`installAllHooks()` extracts files to `~/.ai/hooks/` but does NOT update `~/.claude/settings.json`. The settings.json hook wiring must be added to `runHooksInstall()`.

---

## Scope

### Files to modify
- `src/cmd/ai/embed/hooks/branch-guard.py` — full sync from live (Coder B)
- `src/cmd/ai/embed/hooks/no-verify-strip.py` — minor: add --no-verify check for non-commit subcommands (Coder B)
- `src/cmd/ai/embed/hooks/audit.py` — upgrade KIND_MAP and per-kind fields (Coder A)
- `src/cmd/ai/embed/hooks/audit-command.py` — ensure it handles WRAPPED_CMD variants (Coder A)
- `src/cmd/ai/embed/hooks/secret-block.py` — migrate to _lib.load_patterns() + JSON deny output (Coder C)
- `src/cmd/ai/embed/hooks/secret-precommit.py` — verify uses _lib.load_patterns() (Coder C, verify only)
- `src/cmd/ai/embed/hooks/worktree-guard.py` — upgrade to match live (Coder D)
- `src/cmd/ai/cmd/hooks.go` — add settings.json wiring after extract (Coder E)

### Files to create (tests)
- `src/cmd/ai/embed/hooks/test_branch_guard.py`
- `src/cmd/ai/embed/hooks/test_audit.py`
- `src/cmd/ai/embed/hooks/test_secret_block.py`
- `src/cmd/ai/embed/hooks/test_worktree_guard.py`
- `src/cmd/ai/cmd/hooks_install_test.go`

---

## Acceptance Criteria

### #84 + #96 — hooks install + settings.json wiring
- AC1: `ai hooks install --all` extracts all hooks to `~/.ai/hooks/`
- AC2: after install, `~/.claude/settings.json` contains hook entries for all 8 event types
- AC3: `SessionStart` → `audit.py`; `UserPromptSubmit` → `audit.py`; `PreToolUse` → `branch-guard.py, secret-block.py, worktree-guard.py, audit.py`; `PostToolUse` → `audit.py`; `Stop` → `audit.py, checkpoint-tick.py`; `SessionEnd` → `audit.py`; `SubagentStop` → `audit.py`; `PreCompact` → `audit.py`
- AC4: install is idempotent (safe to run twice)
- AC5: existing unrelated hooks in settings.json are preserved

### #91 — secret-block.py
- AC1: `gho_<36+chars>` pattern in tool command → deny with permissionDecision JSON
- AC2: `ghp_<36+chars>` → deny
- AC3: `github_pat_<60+chars>` → deny
- AC4: `AKIA<16 uppercase alphanum>` → deny
- AC5: `Bearer <30+chars>` → deny
- AC6: `-----BEGIN RSA PRIVATE KEY-----` → deny
- AC7: clean command with no secrets → exit 0, no stdout
- AC8: deny output does NOT echo the full secret value (truncates/redacts)
- AC9: uses `_lib.load_patterns()` (not hardcoded inline patterns)

### #92 — branch-guard.py
- AC1: `git commit` on `main` → deny
- AC2: `git merge` on `main` → deny
- AC3: `bash -c 'git commit -m test'` → deny (bash-c recursion)
- AC4: `git commit --no-verify` on `main` → deny with specific --no-verify message
- AC5: `git push origin main` → deny
- AC6: `git push origin feature/foo` on `main` HEAD → allow (refspec not protected)
- AC7: `git pull --ff-only` on `main` → allow (carve-out)
- AC8: `git push origin main` where remote ref does NOT exist → allow (empty repo bootstrap)
- AC9: deny output is JSON `permissionDecision` (not exit 1)

### #93 — worktree-guard.py
- AC1: `git worktree add /tmp/foo` → deny
- AC2: `git worktree add ../sibling` → deny
- AC3: `git worktree add .git/worktrees/name` → deny
- AC4: `git worktree add <repo>/.worktrees/name` → allow
- AC5: `git worktree add ~/.ai/worktrees/name` → allow
- AC6: deny output is JSON `permissionDecision`

### #94 — audit.py
- AC1: `SessionStart` event → kind=`trace-open`, actor=`system`
- AC2: `UserPromptSubmit` event → kind=`request`, actor=`human`, `stimulus` field present (truncated at 2000)
- AC3: `PreToolUse` event → kind=`invocation-attempt`, actor=`tool`, `probe` and `probe_payload` fields
- AC4: `PostToolUse` event → kind=`invocation-result`, actor=`tool`
- AC5: `Stop` event → kind=`emission`, actor=`assistant`, `emission_marker` field
- AC6: `SessionEnd` event → kind=`trace-close`, actor=`system`
- AC7: `SubagentStop` event → kind=`subagent-emission`, actor=`assistant`
- AC8: `PreCompact` event → kind=`compaction-attempt`, actor=`system`
- AC9: output written to `~/.ai/audit/interactions/<YYYY-MM>.jsonl`
- AC10: audit failure never causes non-zero exit (exit always 0)
- AC11: stimulus and probe_payload are redacted via `_lib.redact()`

### #180 — adversarial validation suite
- AC1: test verifies `bash -c 'git commit -m test'` on `main` is denied by branch-guard
- AC2: test verifies `git commit --no-verify` on `main` is denied by branch-guard
- AC3: test verifies first push to nonexistent remote ref is ALLOWED (bootstrap)
- AC4: test verifies `git worktree add /tmp/foo` is denied by worktree-guard
- AC5: test verifies `git worktree add <canonical>/.worktrees/name` is allowed
- AC6: test verifies `gho_<36chars>` in command is denied by secret-block
- AC7: test verifies clean command passes secret-block

---

## Approach (numbered steps, per-coder)

### Seed step (TL pre-work — before coders spawn)
None needed. The `_lib.py` shared helper is already present and correct.

### Coder A — audit.py, audit-command.py
1. Upgrade `CLAUDE_KIND_MAP` in `audit.py` to match live version's `KIND_MAP` (add PostToolUseFailure, SubagentStart, Notification, ErrorOccurred, PermissionRequest)
2. Add per-kind payload fields in `normalize_event()`: `probe_result_marker`, `compaction_trigger`, `fault_*`, `permission_reason`, `signal_*`, `transcript_marker`
3. Fix field name in `normalize_event`: use `hook_event_name` OR `hookEventName` (live uses `hookEventName` as primary)
4. Verify `audit-command.py` handles `WRAPPED_CMD` variants correctly — no changes needed if tests pass

### Coder B — branch-guard.py, no-verify-strip.py
1. Replace embedded `branch-guard.py` entirely with the logic from the live version
2. Add remote-ref existence check: before denying `git push` to a protected branch, run `git ls-remote --exit-code --heads <remote> <branch>` — if returns non-zero (ref doesn't exist), allow (bootstrap)
3. Verify `no-verify-strip.py` handles `--no-verify` advisory correctly

### Coder C — secret-block.py, secret-precommit.py
1. Replace hardcoded inline patterns with `_lib.load_patterns()` call
2. Change output from stderr+exit1 to JSON `permissionDecision` deny on stdout
3. Verify `secret-precommit.py` already uses `_lib` — no change needed if correct

### Coder D — worktree-guard.py
1. Replace `from_claude_payload()` with proper JSON event parsing (match live)
2. Change output from `_lib.log()+exit1` to JSON `permissionDecision` deny
3. Add `split_invocations`-equivalent: use `shlex.split()` not `cmd.split()`
4. Use `event.get("cwd")` for repo root detection

### Coder E — hooks.go
1. Add `updateSettingsJSON(settingsPath, hooksDir string) error` function
2. Call it from `installAllHooks()` after `ExtractAllHooks()`
3. Settings JSON schema: `{"hooks": {"EventName": [{"hooks": [{"type":"command","command":"<path>"}]}]}}`
4. Merge/upsert — don't clobber existing entries; add missing hooks under each event
5. Matcher: `branch-guard.py` goes under `PreToolUse` with `"matcher": "Bash"` entry

---

## Test Strategy

**Python:** pytest, invoked as `pytest src/cmd/ai/embed/hooks/ -v`
- Tests use `subprocess.run(["python3", "<hook>.py"], input=json.dumps(event), ...)` to invoke hooks as processes
- This tests the full hook process including stdin parsing, not just internal functions
- `conftest.py` or test fixtures provide `mock_git_repo` that sets up a temp dir with `.git/` and a branch

**Go:** `go test ./src/cmd/ai/cmd/...`
- `hooks_install_test.go` tests `runHooksInstall` with temp AI_ROOT
- Verifies settings.json is created/updated with correct structure

**Red criterion (TDD Writer):** Tests must fail when run against the CURRENT (pre-fix) embedded code.

---

## Risk Assessment

| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| Remote-ref existence check for push bootstrap adds `git ls-remote` subprocess latency | Medium | Low | Timeout 5s, fail open (allow) on subprocess error |
| settings.json update breaks existing Claude Code session | Low | High | Read-merge-write (not overwrite); preserve all non-hooks keys |
| Python tests that invoke hooks as subprocesses need patterns.json in path | Medium | Medium | Tests set PYTHONPATH or copy patterns.json to temp dir |
| branch-guard live version uses `deny()` JSON stdout — Claude Code may not pick this up via exit code | Low | Medium | Already used in production live version; tested path |

---

## Dependencies

- `_lib.py` is already correct — no changes needed
- `patterns.json` must exist alongside hook scripts (already in embed/hooks/)
- `embed.go` is correct — no changes needed (ExtractAllHooks already works)

---

## Backward Compatibility

- `branch-guard.py` changes are behavior-preserving for the wrapper mode (argv-based). The Claude mode output schema changes from exit-1+stderr to JSON-stdout, which is the correct Claude Code hook protocol.
- `secret-block.py` output schema change (exit-1 → JSON deny) is required for correct Claude Code hook behavior. This is a bug fix, not a breaking change.
- `hooks.go` settings.json update is additive (merge, not overwrite).

---

## Out of Scope

- `ai doctor` hook tamper detection (#TL2 domain)
- `checkpoint-tick.py` changes
- `destructive-*.py` changes
- `command-wrappers.toml` changes
