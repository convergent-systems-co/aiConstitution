# TL-2 Plan: Tasks #150, #151, #158, #159

**Tech Lead:** TL-2 (`domain:wizard`, `domain:hooks`)
**Repo:** `convergent-systems-co/aiConstitution`
**Base branch:** `main` (current tip: `6d82c32` fetched from origin)

---

## Acceptance Criteria per task

### #150 — Scaffold bubbletea.Model for wizard (`src/cmd/ai/internal/wizard/tui.go`, NEW)
- Define `Model` struct with fields: current question index, `answers map[string]string`, `done bool`, plus a handle to the `wizard.Taxonomy`.
- Define messages: `nextMsg`, `prevMsg`, `answerMsg(value string)`.
- `Init()` returns a no-op (`nil` `tea.Cmd`).
- `Update()`:
  - `nextMsg`: advance index (clamped to len(active)); when past last active question and all required have answers, set `done=true` and return `tea.Quit`.
  - `prevMsg`: decrement (clamped to 0).
  - `answerMsg`: store value under current question's `ID`.
- `View()` renders the current question's `Prompt` plus a status line. Does NOT branch by question type — a single text line per question is sufficient at the scaffold tier.
- New constructor `NewModel(tax wizard.Taxonomy) Model`.
- `Answers() map[string]string` accessor (used by tests + Task #151).
- Tests: programmatic `Update()` exercise of forward/back navigation; answer accumulation; `done` flips at end.
- Must merge **before** #151.

### #151 — Type-specific renderers (`src/cmd/ai/internal/wizard/tui.go`)
- Extend `Update()` and `View()` to branch on `Question.Type`:
  - `text`: cursor + character input via `tea.KeyMsg` (printable runes append, backspace removes). Enter commits.
  - `confirm`: yes/no toggle on left/right arrow; enter commits as `"yes"`/`"no"`.
  - `select`: arrow-up / arrow-down to move highlight across `Options`; enter commits selected option's `Value`.
  - `multi-select`: arrow keys to move; spacebar toggles membership; enter commits as a `,`-joined `Value` list.
- Plain bubbletea only — no `bubbles/`, no `lipgloss`-heavy components. `lipgloss` for trivial styling is acceptable (already a transitive dep of bubbletea).
- Tests: per type, drive a sequence of `tea.KeyMsg` through `Update` and assert the recorded answer matches the expected output.

### #158 — `audit.py` JSONL event logging (`src/cmd/ai/embed/hooks/audit.py`)
- **Current state:** the hook already exists and is largely correct. The work is verification + a unit test, plus filling the gap that `SessionEnd` is mapped (`trace-close`) but not part of the issue's listed event types. Confirm coverage of all 7 issue-listed events: SessionStart, UserPromptSubmit, PreToolUse, PostToolUse, Stop, SubagentStop, PreCompact. (`SessionEnd` is also handled; it's an extra and that's fine.)
- Schema check against `Common.md §5.2`: `chronon`, `trace`, `cwd`, `actor`, `kind`, `engine`, `stimulus` (truncate 2000), `probe`, `probe_payload` (truncate 1000), `emission_marker`. All present in `normalize_event`.
- Output path: `~/.ai/audit/interactions/<YYYY-MM>.jsonl`. Already implemented via `audit_dir()` + `month_file()`. `AI_ROOT` override honored.
- **Add:** `src/cmd/ai/embed/hooks/tests/test_audit.py` — pytest, no third-party deps beyond `pytest` itself. Mock Claude payloads for each event type; pipe via subprocess; assert JSONL line on disk matches the schema and field rules.

### #159 — Harden `branch-guard.py` (`src/cmd/ai/embed/hooks/branch-guard.py`)
- **Current state:** already implements blocking of `commit/merge/rebase/cherry-pick/revert/am/pull` on protected branches and refspec-aware `push` blocking. Policy override file is read from `~/.ai/governance/policy/branch-guard.json` (note: the issue body says `~/.ai/branch-guard.json` — the existing implementation uses the namespaced path, which matches `Common.md §5.5`'s "policy" location convention. Document in PR description; do NOT regress to the bare-home location.)
- **Add:** `src/cmd/ai/embed/hooks/tests/test_branch_guard.py` — 5+ tests, one per guarded subcommand on a protected branch, plus push-refspec coverage, plus policy override coverage. Use `git init` in a tmp dir to create a real `main` branch HEAD and then drive `check_invocation()` directly.
- BLOCK message already cites `~/.ai/Common.md §2.2`. Verify wording in test.

---

## Seed commits (shared code that must land first)

- **#150 is the seed for #151** — the scaffold must merge before renderers extend it. No code is shared *between* the hook tasks (#158/#159), so they parallelize freely.
- Adding bubbletea to `go.mod`/`go.sum` happens **in #150** via `go get github.com/charmbracelet/bubbletea@latest` inside `src/cmd/ai`. #151 inherits.

## Sub-task execution order

### Wave 1 (parallel): #150, #158, #159
- #150 worktree: `.worktrees/task-150-wizard-scaffold` on `task/150-wizard-scaffold` from `main`.
- #158 worktree: `.worktrees/task-158-audit-hooks` on `task/158-audit-hooks` from `main`.
- #159 worktree: `.worktrees/task-159-branch-guard` on `task/159-branch-guard` from `main`.

### Wave 2 (after #150 merges): #151
- Branch from updated `main` (which now has tui.go scaffold).
- `.worktrees/task-151-wizard-renderers` on `task/151-wizard-renderers`.

## Test strategy

### Go (tasks #150, #151)
- `go test ./internal/wizard/...` from `src/cmd/ai/` once tui.go lives under `src/cmd/ai/internal/wizard/` (note: the OWNS path is `src/cmd/ai/internal/wizard/tui.go`, which is a **new** sub-package distinct from `src/internal/wizard/`). The new package will import the existing `src/internal/wizard` for `Taxonomy` and `Question` types.
- Tests instantiate `Model`, call `Update(msg)` directly (no `tea.Program` event loop), and assert state transitions on the returned model.
- Coverage target: navigation forward/back; answer storage; `done` transition; per-type input handling in #151.

### Python (tasks #158, #159)
- Tests live in `src/cmd/ai/embed/hooks/tests/`.
- `audit.py`: invoke as a subprocess, feed JSON on stdin, point `AI_ROOT` at a tmp dir, read the resulting JSONL and assert per-field correctness for each of the 7 event kinds.
- `branch-guard.py`: drive `check_invocation()` directly (import the module) inside a `git init`'d tmp repo with HEAD on `main`. Assert exit code 1 for each guarded subcommand and exit code 0 for the same subcommand on a non-protected branch. Cover the push refspec parser with `git push origin main`, `git push origin work:main`, `git push --all`, and bare `git push`.

## Out of scope

- Persisting wizard answers to `settings.toml` (a downstream task — `runner.go` already returns the map).
- Wiring the TUI into `cmd/setup.go` (the `--tui` branch in `setup.go` currently falls back to non-interactive; rewiring is a follow-up task).
- Reorganizing the hook policy directory layout.
