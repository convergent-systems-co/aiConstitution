# Amendment Lifecycle Plan

**Objective:** Implement `ai amend` subcommands (draft, apply, list, show, publish) and `ai update --migrate` to make the amendment lifecycle functional end-to-end.

**Issues:** #184, #185, #186, #187, #188, #189, #190, #191, #199
**Branch:** feature/amend-lifecycle
**Worktree:** .worktrees/feature/amend-lifecycle

---

## Rationale

| Alternative | Pros | Cons | Verdict |
|---|---|---|---|
| Single amend.go with all logic | Simple file structure | Functions become large; harder to test | Chosen (scope is bounded) |
| Split into amend/ sub-package | Cleaner separation | Over-engineering for this scope | Rejected |
| Keep stubs | Zero risk | Features never delivered; issues remain open | Rejected |
| Shell scripts | Fast to write | Not testable, not portable, not idiomatic | Rejected |

---

## Scope

**Files to create:**
- `src/cmd/ai/cmd/amend_test.go` — test suite (TDD Writer)

**Files to modify:**
- `src/cmd/ai/cmd/amend.go` — replace stub with subcommands: draft, apply, list, show, publish
- `src/cmd/ai/cmd/update.go` — add --migrate functional implementation

**Files explicitly NOT touched:**
- `src/cmd/ai/internal/wizard/`
- `src/cmd/ai/embed/hooks/`
- `src/internal/constitution/template.go`

---

## Approach

### Phase 1 — TDD (failing tests RED)
Write `amend_test.go` covering:
1. `draft` creates file at `$AI_ROOT/governance/plans/<UTC>-<slug>.md` with correct content parsed from violation stub
2. `apply` patches the target section in Constitution.md, bumps version, appends changelog
3. `list` shows files newest-first from plans dir
4. `show` finds file by slug prefix

All tests use `t.Setenv("AI_ROOT", tmpDir)`.

### Phase 2 — Coder A: amend draft (#184, #185)
Implementation in `amend.go`:
- New `newAmendCmd()` returns a cobra command with subcommands
- `draft` subcommand:
  - Accepts `<violation-file-path>` positional arg OR `--from-violation=<path>` flag
  - Parses violation file for `File / Rule violated:`, `What happened:`, `Proposed amendment:` fields
  - Derives slug: kebab-case from rule field, max 32 chars
  - Writes to `paths.AIRoot()/governance/plans/<UTC>-<slug>.md`
  - Format:
    ```
    # Amendment Draft — <slug>

    ## Target
    <§-ref>

    ## Proposed Change
    <ProposedAmendment>

    ## Rationale
    <WhatHappened>
    ```
  - When `$EDITOR` set: exec `$EDITOR <path>`; when unset: print path to stdout

### Phase 3 — Coder B: amend apply (#186, #187)
Implementation in `amend.go`:
- `apply` subcommand:
  - Args: `<slug-or-path>` — resolve to plan file
  - Parse stub for `## Target` (§-ref) and `## Proposed Change` body
  - Locate section in `~/.ai/Constitution.md` by matching `## §<ref>` heading
  - Replace section body (text between this heading and next `##`) with Proposed Change
  - Parse version from Constitution.md: `**Version:** x.y` in first 20 lines
  - Bump minor: `"1.0"` → `"1.1"`
  - Append to Changelog section: `- **1.1** — <slug>: <first-line-of-proposed-change>`
  - After apply: no runtime regeneration needed (constitution.ExtractRuntime is out of scope for v0.8)

### Phase 4 — Coder C: amend list/show/publish + update --migrate (#188–191, #199)
Implementation in `amend.go` and `update.go`:
- `list` subcommand:
  - Read `paths.AIRoot()/governance/plans/` directory
  - Sort filenames newest-first (UTC prefix → lexicographic descending = newest-first)
  - Print: `<slug>  <first-line-of-content>`
- `show <slug>` subcommand:
  - Find file with prefix match on slug in plans dir
  - Print full content to stdout
- `publish --dry-run` subcommand:
  - Parse plan stub, find §-ref section in Constitution.md
  - Verify section body matches `## Proposed Change` body
  - Print `Would run: gh release create v<version>`
  - `--dry-run` flag: validate only (both modes are dry-run for now per spec notes)
- `update --migrate` flag in `update.go`:
  - Detect if v2 layout via heuristic (presence of `~/.ai/Constitution.md` + no legacy single-file)
  - If v2: print "Already v2 — no migration needed"
  - If v1: print diff summary, prompt "Migrate to unified v2? (yes/no)"
  - `--non-interactive` flag: skip prompt, proceed
  - On yes: print migration steps (stub — actual migration pipeline is out of scope for v0.8)

---

## Testing Strategy

Each test function is isolated via `t.Setenv("AI_ROOT", t.TempDir())`.
- `TestAmendDraftCreatesFile` — happy path
- `TestAmendDraftParsesViolationFields` — correct content extraction
- `TestAmendApplyPatchesSection` — section replacement correct
- `TestAmendApplyBumpsVersion` — version increment correct
- `TestAmendListNewestFirst` — sort order
- `TestAmendShowBySlugPrefix` — prefix matching

---

## Risk Assessment

| Risk | Mitigation |
|---|---|
| Constitution.md section parsing fragile | Use well-tested regex; test with real fixture content |
| Version bump with non-standard format | Parse only `**Version:** x.y` pattern; fail loudly if not found |
| `exec $EDITOR` breaks tests | Tests set `$EDITOR=""` to force print-path path |
| File sorting newest-first | UTC prefix guarantees lexicographic = chronological; test with known timestamps |

---

## Dependencies

- `paths.AIRoot()` — already implemented, no changes needed
- `paths.GovernanceDir()` — exists at `filepath.Join(AIRoot(), "governance")`
- No new external dependencies; stdlib only

---

## Backward Compatibility

- Existing `ai amend` stub is replaced entirely; current stub always errored, so no breaking change for callers
- `ai update --migrate` adds a flag to existing `update` command; existing flags unchanged
