# Plan: CLI Integration — Copilot / Cursor / Codex / Doctor Checks

**Plan date:** 2026-05-24
**Issues:** #221, #222, #223, #224
**Branch:** feature/cli-integration
**Worktree:** .worktrees/feature/cli-integration

---

## Objective

Extend the `ai` CLI with three new integration surfaces (Copilot symlink, Cursor symlink, Codex AGENTS.md) and extend `ai doctor` to report health of all three.

---

## Alternatives Table

| Alternative | Pros | Cons | Verdict |
|---|---|---|---|
| Add all logic to hooks.go monolith | One file | Violates SRP; hooks.go already complex | Rejected |
| New integrate.go file for init-style subcommands | Clean separation; each issue in its own command | Adds a new cobra registration in root.go | Chosen |
| Extend init.go (TL1 file) | Fewer files | TL1 owns init.go; cross-TL file mutation violates OWNS | Rejected |
| Add integrate as a sub-command of hooks | Hooks is about ~/.ai/hooks/; Cursor/Codex are tool-integration | Wrong abstraction level | Rejected |

---

## Scope

**Files to create:**
- `src/cmd/ai/cmd/integrate.go` — `ai init --cursor` and `ai init --codex` commands (#222, #223)
- `src/cmd/ai/cmd/integrate_test.go` — tests for integrate commands
- `src/cmd/ai/cmd/hooks_copilot_test.go` — tests for hooks install --copilot

**Files to modify:**
- `src/cmd/ai/cmd/hooks.go` — add `--copilot` flag to `install` subcommand (#221)
- `src/cmd/ai/cmd/doctor.go` — add Copilot/Cursor/AGENTS.md checks (#224)
- `src/cmd/ai/cmd/root.go` — register `newInitIntegrateCmd()` from integrate.go

**Files NOT to touch:**
- `src/cmd/ai/cmd/init.go` (TL1)
- `src/cmd/ai/cmd/profile.go`
- `src/cmd/ai/cmd/persona.go`
- `src/cmd/ai/cmd/restore.go`

---

## Approach

### Step 1: TDD Writer — failing tests (RED)
- `hooks_copilot_test.go`: test that `hooks install --copilot` creates `~/.copilot/instructions/constitution.md` symlink pointing at `~/.ai/Constitution.runtime.md`; tests for idempotency; tests for stale symlink replacement; test for missing `Constitution.runtime.md` warning
- `integrate_test.go`: test `ai init --cursor` creates `.cursor/rules/constitution.md` symlink; idempotency; test `ai init --codex` writes AGENTS.md; idempotency (no-dup); append case

### Step 2: Coder A — hooks.go + hooks_copilot_test.go
- Add `--copilot bool` flag to existing `install` cobra subcommand
- Wire to `runHooksCopilotInstall(aiRoot, home string) error`
- Logic: mkdir `~/.copilot/instructions/`; check symlink target; create/replace as needed; warn if runtime missing

### Step 3: Coder B — integrate.go + integrate_test.go
- `newInitIntegrateCmd()` returns a cobra command `init-integrate` registered in root.go
- `--cursor` flag: mkdir `.cursor/rules/`; symlink `.cursor/rules/constitution.md` → `~/.ai/Constitution.runtime.md`
- `--codex` flag: write or append AGENTS.md with @-include block; idempotent check

### Step 4: Coder C — doctor.go
- Extend `runDoctor` from stub → actual checks
- Check 1 (Copilot): if `~/.copilot/instructions/` exists, verify symlink `constitution.md` is valid → `[✓]` / `[⚠]`
- Check 2 (Cursor): if `.cursor/` exists in cwd, verify `.cursor/rules/constitution.md` symlink → `[✓]` / `[⚠]`
- Check 3 (AGENTS.md): if `codex` in settings tools list, check AGENTS.md contains `@~/.ai/Constitution.md` → `[✓]` / `[⚠]`
- All three are warnings only; exit 0

### Step 5: Adversarial Tester review

### Step 6: PR creation

---

## Testing Strategy

- Unit tests using `t.TempDir()` for all filesystem operations
- `AI_ROOT` env var used to override AIRoot in tests
- Tests are hermetic: no writes to real `~/.copilot`, `~/.ai`, or cwd
- All tests RED before implementation, GREEN after

---

## Risk Assessment

| Risk | Mitigation |
|---|---|
| Symlink creation fails on Windows | Acceptable — target OS is macOS/Linux; skip Windows symlink support in v1 |
| `Constitution.runtime.md` path doesn't exist in test env | Test explicitly creates the temp file before testing "happy path"; separate test for missing-file warning |
| doctor.go still a stub (returns early) | Replace stub behavior; tests cover actual check logic |
| root.go register of newInitIntegrateCmd may conflict with TL1's init command | Use a different cobra Use name (`init-integrate`) to avoid collision; reassess after TL1 merges |

---

## Dependencies

- No new external dependencies
- `paths.AIRoot()` already available for `AI_ROOT` override
- `config.Load()` available for codex detection (though Settings lacks a `tools` field — use stub/simplified logic)

---

## Backward Compatibility

- `ai hooks install` without `--copilot` behaves identically to before
- `ai init-integrate` is a new subcommand; no existing callers
- `ai doctor` previously returned stub; now returns real checks — behavior change is intentional and documented

---

## Out of Scope

- Windows symlink support
- `ai generate runtime` (referenced in warning message, implemented elsewhere)
- `config.Load()` tools field (Settings.Tools does not yet exist — use a simplified heuristic for codex detection in doctor)
