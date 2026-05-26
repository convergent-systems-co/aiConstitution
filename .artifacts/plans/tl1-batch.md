# TL-1 Plan: Tasks #152-#157

**Owner:** Tech Lead 1 — `domain:cli`, `domain:internal`
**Branch base:** `main`
**Workflow:** TDD — failing tests first, then implementation. All work happens in worktrees under `.worktrees/`.

---

## 1. Acceptance Criteria per task

### #152 — Map wizard answers to Settings; save settings.toml
- OWNS: `src/cmd/ai/cmd/setup.go`, `src/internal/config/config.go`
- `runSetupNonInteractive` MUST pass `seeds` to `RunNonInteractive` and apply the resulting answers to a `Settings` value before `config.Save`.
- New helper `config.ApplyAnswers(s *Settings, answers map[string]string)` (in `src/internal/config/config.go`) implements the mapping:
  - `defaultMode` → `s.Focus.DefaultMode` (string passthrough)
  - `shareNewHooks` → `s.Upstream.ShareNewHooks` (parse bool: "true"/"false"/"yes"/"no"/"1"/"0", case-insensitive)
  - `reviewCadenceDays` → `s.Review.CadenceDays` (parse int via `strconv.Atoi`)
  - `syncIncludeSettings` → `s.Sync.IncludeSettingsFile` (parse bool)
- Unknown answer keys are silently ignored (no error).
- Parse-failure on a known key is silently ignored (treat as "keep default").
- Hidden seed plumbing: add a `--seed key=value` repeatable flag to `ai setup` (or accept an `AICONST_SEEDS=key=val,key=val` env var) so tests can drive it. Choose env var to avoid expanding the cobra surface contract for this v0.8 step.
- Acceptance test: run `ai setup --non-interactive` with `AICONST_SEEDS="defaultMode=writer,shareNewHooks=false,reviewCadenceDays=14,syncIncludeSettings=false"` in a temp `AICONST_CONFIG_DIR` — assert settings.toml decodes into a Settings with those exact values.

### #153 — `ai setup` writes ~/.ai/ tree + AI tool integration files
- OWNS: `src/cmd/ai/cmd/setup.go`, `src/cmd/ai/internal/init/` (NEW package)
- After saving `settings.toml`, `runSetupNonInteractive` MUST:
  1. `os.MkdirAll(paths.AIRoot(), 0o750)`.
  2. Write `~/.ai/CLAUDE.md` if absent — template body:
     ```
     # Claude Instructions

     Load and follow ~/.ai/{Constitution,Common,Code,Writing}.md strictly.
     ```
  3. Write `~/.ai/.github/copilot-instructions.md` if absent — template body adapted for Copilot ("Load and follow ~/.ai/{Constitution,Common,Code,Writing}.md strictly. These four files are authoritative for every task.").
  4. Write `~/.ai/AGENTS.md` if absent — template body adapted for Codex.
- Existing files are NEVER overwritten (use a `writeIfAbsent` helper that returns `(written bool, err error)`).
- New package `github.com/convergent-systems-co/aiConstitution/src/cmd/ai/internal/init` exposes `EnsureToolFiles(aiRoot string) ([]string, error)` returning the list of paths actually written.
- Acceptance test: run setup in a temp AI_ROOT — assert the four files exist with the expected content; pre-create CLAUDE.md with custom content, re-run, assert content unchanged.

### #154 — `ai amend draft <file>/<section>`
- OWNS: `src/cmd/ai/cmd/amend.go`, `src/internal/amend/` (NEW package)
- New package `github.com/convergent-systems-co/aiConstitution/src/internal/amend` with:
  - `type Draft struct { File, Section, ProposedChange, AuditRef, Slug string; Created time.Time }`
  - `ParseRef(ref string) (file, section string, err error)` — splits "Common.md/U17" → ("Common.md", "U17").
  - `LocateSection(content, section string) (start, end int, found bool)` — finds the section header in a constitution file (matches lines like `## 17. ...`, `## U17`, `### U17. ...`).
  - `WriteDraft(d Draft, plansDir string) (path string, err error)` — writes `<plansDir>/<UTC>-<slug>.md`. UTC timestamp format `2006-01-02T150405Z`. Slug derived from file+section (e.g. `Common-md-U17`).
- `ai amend draft <file>/<section> [--from-violation=<path>]`:
  1. Parse the ref.
  2. Read `~/.ai/<file>` and locate the section (record warning to stderr if not found; still proceed with empty proposed_change).
  3. Build the draft frontmatter + body (YAML-flavored frontmatter — `file:`, `section:`, `audit_ref:`, then a `## Proposed change` placeholder).
  4. Write to `paths.GovernanceDir() + "/plans/<UTC>-<slug>.md"`.
  5. If `$EDITOR` set: `exec.Command($EDITOR, draftPath).Run()`; else print path to stdout.
- Acceptance test: `ai amend draft Common.md/U17` in a temp AI_ROOT that has a stub Common.md — assert the draft file exists, has the expected frontmatter keys, and (with `EDITOR=true` no-op) returns no error.

### #155 — `ai amend apply <draft-path>`
- OWNS: `src/cmd/ai/cmd/amend.go`, `src/internal/amend/`
- Extends `src/internal/amend/` with:
  - `LoadDraft(path string) (Draft, error)` — parses the draft file written by #154.
  - `BumpVersion(currentLine, newVersion string) string` — minor bump on a `**Version:** X.Y` line.
  - `AppendChangelog(content, entry string) string` — appends a new bullet under the `## Changelog` (or `## <N>. Changelog`) heading.
  - `Apply(d Draft, aiRoot string) (ApplyResult, error)` — orchestrates: read target file, append proposed_change to the section, bump version, append changelog entry, write back atomically.
- `ai amend apply <draft-path>`:
  1. `LoadDraft(path)`.
  2. `Apply(draft, paths.AIRoot())`.
  3. Write `audit/overrides/<UTC>-<slug>.md` containing the apply record (per `Common.md §5`).
- Acceptance test: write a known-shape stub `Common.md` with version line + changelog section, write a draft with a known proposed_change, run apply — assert (a) the section grew by the proposed_change; (b) version bumped 0.17 → 0.18; (c) changelog has the new entry; (d) audit/overrides entry was written.

### #156 — `ai hooks install --claude`
- OWNS: `src/cmd/ai/cmd/hooks.go`
- New `--claude` flag on `ai hooks install`.
- New helper `installClaudeHooks(repoRoot string)`:
  1. Determine the target: `.claude/settings.json` under `repoRoot` (default `.`).
  2. Read the JSON if present; else start from `{}`.
  3. Walk `~/.ai/hooks/*.py`. For each file, detect Claude event from filename via a lookup table:
     - `audit.py` → registers as `SessionStart`, `UserPromptSubmit`, `PreToolUse`, `PostToolUse`, `Stop`, `SessionEnd`, `SubagentStop`, `PreCompact` (per `Common.md §5.5`).
     - `branch-guard.py` → `PreToolUse`.
     - `worktree-guard.py` → `PreToolUse`.
     - `secret-block.py` → `PreToolUse`.
     - `audit-command.py` → `PostToolUse`.
     - `checkpoint-tick.py` → `Stop`.
     - `no-verify-strip.py` → `PreToolUse`.
     - `destructive-*-guard.py` → `PreToolUse`.
   - Hooks without a known event are skipped.
  4. For each (hook, event) pair add `{"type": event, "command": "python3 <abspath>/<hook>"}` under `settings.hooks[event]` if not already present (idempotent — compare on `command` field).
  5. Write back atomically (temp file in same dir + `os.Rename`).
- Acceptance test: in a temp dir with a fake `~/.ai/hooks/` containing `audit.py` and `branch-guard.py`, run install — assert `.claude/settings.json` exists and has the expected entries; re-run — assert no duplicates.

### #157 — `ai hooks install command-wrappers` + `ai doctor` PATH check
- OWNS: `src/cmd/ai/cmd/hooks.go`, `src/cmd/ai/cmd/doctor.go`
- (1) Verify the existing `ai hooks install command-wrappers` path works — add a regression test that runs it against a temp AI_ROOT and asserts `~/.ai/bin/git`, `~/.ai/bin/gh` are present and executable.
- (2) Add a `checkBinPath()` helper to `doctor.go` that:
  - Splits `PATH` on `os.PathListSeparator`.
  - Returns OK if `~/.ai/bin` appears in PATH AND appears before `/usr/local/bin` and `/opt/homebrew/bin` (when those paths are present in PATH).
  - Returns WARN with an actionable message otherwise (e.g. "~/.ai/bin not on PATH" or "~/.ai/bin is shadowed by /usr/local/bin — move it earlier").
- (3) Integrate `checkBinPath()` into the `ai doctor` output. Print `[✓] ~/.ai/bin on PATH before system bins` or `[!] <message>`. A warn-tier finding does NOT fail doctor (only the constitution-file check returns a hard error).
- Acceptance test: set PATH to a known string, run doctor, assert the expected line in output.

---

## 2. Seed commits (shared interfaces that must land first)

Before any of the parallelizable groups can land, the shared `amend` package interface must exist so #154 and #155 don't fight over the package shape. Approach: the TDD writer creates a single package-skeleton commit on a branch named `task/seed-amend-pkg` containing:

- `src/internal/amend/amend.go` — type declarations only (Draft, ParseRef sig, LocateSection sig, WriteDraft sig, LoadDraft sig, Apply sig, ApplyResult sig). All functions return `nil` / zero / `errors.New("not yet implemented")`.

This commit lands FIRST and becomes the base of the Group B worktree.

The other two seed concerns are already satisfied:
- `internal/init` package (#153) is OWNS-exclusive to that task — no seed needed.
- `hooks.go --claude` and `doctor.go` PATH check don't share types — no seed needed.

---

## 3. Sub-task groups (serialized where files overlap)

### Group A: setup.go tasks (#152, #153) — serialized in one worktree
- Worktree: `.worktrees/task-152-153-setup`
- Branch: `task/152-153-setup`
- Order: #152 first (touches Settings mapping), then #153 (writes the tool files).
- Two commits, two test files (`setup_test.go` extended). Tests run before each impl commit.

### Group B: amend.go tasks (#154, #155) — serialized in one worktree
- Worktree: `.worktrees/task-154-155-amend`
- Branch: `task/154-155-amend`
- Base: includes the seed commit for `src/internal/amend/amend.go` types.
- Order: #154 first (draft creation), then #155 (apply).
- Two commits, separate test files (`amend_draft_test.go`, `amend_apply_test.go`).

### Group C: hooks.go + doctor.go (#156, #157) — serialized in one worktree
- Worktree: `.worktrees/task-156-157-hooks`
- Branch: `task/156-157-hooks`
- Order: #156 first (`--claude` flag), then #157 (PATH check + wrappers regression test).
- Tests in `hooks_test.go` (new) and `doctor_test.go` (extend).

---

## 4. Test strategy

| Task | Test file | Key assertions |
|---|---|---|
| #152 | `src/cmd/ai/cmd/setup_test.go` (extend) | seeded answers → settings.toml fields match |
| #152 | `src/internal/config/config_test.go` (extend) | `ApplyAnswers` mapping behavior, unknown-key tolerance, parse-error tolerance |
| #153 | `src/cmd/ai/cmd/setup_test.go` (extend) | tool files written; pre-existing not overwritten |
| #153 | `src/cmd/ai/internal/init/init_test.go` (new) | `EnsureToolFiles` returns paths actually written |
| #154 | `src/internal/amend/amend_draft_test.go` (new) | `ParseRef`, `LocateSection`, `WriteDraft` |
| #154 | `src/cmd/ai/cmd/amend_test.go` (new) | `ai amend draft` command produces file when EDITOR is no-op |
| #155 | `src/internal/amend/amend_apply_test.go` (new) | `LoadDraft`, `BumpVersion`, `AppendChangelog`, `Apply` end-to-end |
| #155 | `src/cmd/ai/cmd/amend_test.go` (extend) | `ai amend apply` integration |
| #156 | `src/cmd/ai/cmd/hooks_test.go` (new) | `--claude` flag writes settings.json; idempotent re-run |
| #157 | `src/cmd/ai/cmd/hooks_test.go` (extend) | command-wrappers extracts files |
| #157 | `src/cmd/ai/cmd/doctor_test.go` (extend) | PATH check OK / shadowed / missing outputs |

All tests use `t.TempDir()`, `t.Setenv("AI_ROOT", ...)`, `t.Setenv("AICONST_CONFIG_DIR", ...)` per the existing pattern.

Run with `go test ./...` from each module dir (`src/cmd/ai`, `src/internal`).

---

## 5. Risk & out-of-scope

- **Risk:** `ai setup` currently doesn't accept seeds via CLI flag. Resolution: thread seeds through env var `AICONST_SEEDS` (`k=v,k=v`) parsed inside `runSetupNonInteractive`. A future PR adds a real `--seed` flag once the TUI lands.
- **Risk:** `LocateSection` heuristics may not match all section headers. Resolution: regex permissive — match `(?m)^#+\s*(?:§|U|P|\d+\.)?\s*<section>\b`. If not found, the draft is still written with `proposed_change: ""` placeholder.
- **Out of scope:** Actual section-aware textual surgery on `Common.md` (e.g. inserting before the Changelog). #155 uses naive append-to-end-of-section. A proper patch format is future work (gated by a real reviewer trip).
- **Out of scope:** Bumping major version, BREAKING tags, prompt-injection screening of draft content.
- **Out of scope:** `propose` and `evaluate` hook subcommands (still stubs).

---

## 6. PR strategy

One PR per group. Title format:

- Group A → `feat(setup): wire wizard answers + ~/.ai/ tool files (closes #152, #153)`
- Group B → `feat(amend): draft + apply amendment lifecycle (closes #154, #155)`
- Group C → `feat(hooks): --claude install + doctor PATH check (closes #156, #157)`

Body includes:
- Summary bullets.
- Test plan checklist with `go test ./...` output expectation.
- `Closes #N` for each issue.

Merge method: merge commit (not squash, per `Code.md §11.2`).
