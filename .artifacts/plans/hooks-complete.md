# Plan: hooks-complete batch (#200–#207)

**Date:** 2026-05-24
**TL:** TL3 — hooks-complete domain
**Issues:** #200, #201, #202, #203, #204, #205, #206, #207
**Branch:** feature/hooks-complete
**Worktree:** /Users/itsfwcp/workspace/convergent-system-co/aiConstitution/.worktrees/feature/hooks-complete

---

## Objective

Ship eight related governance-hook features: `ai hooks validate` (lint/report), `ai hooks evaluate` (smoke-test), branch-guard drift/violation audit writing, git/gh command-wrapper shell scripts, and `ai hooks install command-wrappers`.

---

## Alternatives Table

| Alternative | Pros | Cons | Verdict |
|---|---|---|---|
| Implement validate/evaluate as a subprocess call to a Python helper | Easy to write, single language | Extra process, hard to test in Go unit tests | Rejected — Go subprocess calling Python is fine for integration but Go unit-testable validate logic is cleaner |
| Implement validate/evaluate fully in Go (exec python3 -m py_compile) | Testable in Go, consistent with CLI surface | Some complexity in parsing py_compile output | **Chosen** |
| Implement branch-guard violation writes via _lib audit function | Consistent with lib design | _lib has no audit writer yet; adds coupling | Add minimal write_audit_record() directly in branch-guard.py |
| Wrapper scripts: generate dynamically in Go | More flexible | Over-engineered for static scripts | Rejected — embed static shell scripts, already done |

---

## Scope

### Files to modify
- `src/cmd/ai/cmd/hooks.go` — add `validate` subcommand (#200/#201), implement `evaluate` (#202), extend `install command-wrappers` (#206)
- `src/cmd/ai/embed/hooks/branch-guard.py` — add drift+violation audit writing (#203/#204)
- `src/cmd/ai/embed/wrappers/git` — already complete, minimal spec compliance review (#205)
- `src/cmd/ai/embed/wrappers/gh` — already complete, minimal spec compliance review (#207)

### Files to create
- `src/cmd/ai/cmd/hooks_validate_test.go` — Go tests for validate/evaluate
- `src/cmd/ai/embed/hooks/test_branch_guard.py` — pytest for branch-guard audit writes

---

## Approach

### Step 1 — TDD Writer produces failing tests (RED)

**Go tests** (`hooks_validate_test.go`):
1. `TestValidateExitOneOnSyntaxError` — create a temp dir with a .py file containing bad syntax, run validate, expect exit 1
2. `TestValidatePassOnValidHook` — create a temp dir with a .py file containing valid shebang+syntax, expect [✓] in output
3. `TestEvaluateRunsEmbeddedHooks` — run evaluate command, expect at least one [✓] or [✗] in output (not a stub error)

**Python tests** (`test_branch_guard.py`):
4. `test_deny_operation_writes_violation_file` — monkeypatch current_branch() to return "main", call check_invocation(["commit"]) with AI_ROOT pointed at tmp dir, assert violation file created in `<AI_ROOT>/audit/violations/`

### Step 2 — Coder A: hooks.go validate + evaluate (#200, #201, #202)

Add `validate` subcommand to `newHooksCmd()`:
- Enumerate all `.py` files in the embedded HooksFS
- For each: check shebang (first line starts with `#!`), run `python3 -m py_compile <extracted-to-tmp-file>`, scan for bare `except:` or `except :`
- For each `.sh`: run `bash -n <extracted-to-tmp-file>`  
- Print `[✓]`, `[⚠]`, or `[✗]` per file
- Exit 1 if any `[✗]`; exit 0 otherwise

Implement `evaluate` subcommand (replace stub):
- For each embedded .py hook: extract to temp, construct minimal synthetic JSON event, pipe it via `echo '<json>' | python3 <path>`, assert exit 0
- Print `[✓]` or `[✗]` per hook
- Exit 1 if any `[✗]`

### Step 3 — Coder B: branch-guard.py (#203, #204)

Add `write_audit_record(kind, subcmd, branch, ai_root)` function to branch-guard.py:
- kind: "violation" or "drift"
- Writes to `<ai_root>/audit/violations/<UTC>-branch-guard.md` (violation) or `<ai_root>/audit/drift/<UTC>-branch-guard.md` (drift)
- Content: markdown with timestamp, rule, subcmd, branch, evidence
- Called from `violation()` function (existing) for deny events
- Drift record fires when a near-miss check reaches >= 90% of threshold (quantitative checks don't exist yet in this hook; add a TODO near-miss placeholder with the write path wired)

### Step 4 — Coder C: wrappers (#205, #207) + command-wrappers install (#206)

- Review `embed/wrappers/git` — already implemented; verify it matches spec (branch-guard call, audit-command postHook)
- Review `embed/wrappers/gh` — already implemented; verify it calls destructive-gh-guard
- `hooks.go` install command-wrappers: already implemented via `installWrappers()`; verify it prints PATH note per task instructions; confirm it calls `embed.ExtractWrappers`

### Step 5 — Adversarial Tester review

### Step 6 — PR open

---

## Testing Strategy

- Go `hooks_validate_test.go`: exercises validate and evaluate subcommands via direct function call or `exec.Command("go run .")` against temp hooks dir
- Python `test_branch_guard.py`: pytest, monkeypatches subprocess and env vars, asserts file written

---

## Risk Assessment

| Risk | Mitigation |
|---|---|
| `python3 -m py_compile` not available | Bail gracefully with [⚠] warning, not [✗] error |
| Wrapper scripts already substantially complete | Read carefully before touching — don't regress |
| branch-guard.py writes to AI_ROOT in production tests | Always use env var `AI_ROOT` pointing to temp dir in tests |

---

## Dependencies

- embed.HooksFS() already works — no changes needed to embed.go
- embed.ExtractWrappers() already works — no changes needed

---

## Backward Compatibility

- `ai hooks evaluate` was a stub; now implements real behavior — not a breaking change
- `ai hooks validate` is a new subcommand — additive only
- branch-guard.py only adds audit file writes — no change to exit codes or blocking logic
- wrapper scripts: read-only review, no functional changes

---

## Out of Scope

- src/cmd/ai/cmd/amend.go
- src/cmd/ai/internal/wizard/
- src/internal/constitution/
