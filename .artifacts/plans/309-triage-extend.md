# Plan: Extend triage.yml — Tasks #309, #310, #311, #312

**Status:** approved  
**Date:** 2026-05-26  
**Branch:** feat/309-triage-extend

---

## Objective

Add three jobs to `.github/workflows/triage.yml` that run after the existing `triage` job to: (1) ensure `status/triage` is applied to every triaged issue, (2) post a structured decomposition proposal comment for `kind/epic` and `kind/feature` issues with idempotency, and (3) create child issues when a maintainer replies `/triage approve-decomposition`.

---

## Rationale

| Alternative | Pros | Cons | Verdict |
|---|---|---|---|
| Separate workflow files per job | Cleaner per-job isolation | Harder to see the triage pipeline at a glance; `needs: triage` dependency harder to express across files | Rejected |
| Single triage.yml with additional jobs | One file = one pipeline; `needs:` wiring is explicit; single PR | Slightly longer file | Chosen |
| Composite action in repo-standards | Reusable across repos | Extra cross-repo PR; out of scope | Rejected |

---

## Scope

Files to modify:
- `.github/workflows/triage.yml` — add three jobs: `post-label`, `propose-decomposition`, `approve-decomposition`

Files to create:
- `.artifacts/plans/309-triage-extend.md` (this file)

---

## Approach

1. Add `post-label` job (task #309)
   - `needs: triage`
   - Only on `issues` events
   - Uses `gh api` to check for `status/triage` label; adds if absent
   - Idempotency: label add is naturally idempotent (adding existing label is a no-op)

2. Add `propose-decomposition` job (tasks #310 + #312)
   - `needs: triage`
   - Only on `issues` events
   - Checks for `<!-- triage:proposal-v1 -->` marker in existing comments (idempotency gate)
   - If issue has `kind/epic` or `kind/feature`, posts structured JSON proposal comment
   - JSON template placeholder titles use `[Feature]`/`[Story]` prefixes
   - Uses `${child_kind^}` for bash capitalisation (bash 4+ on ubuntu-latest)

3. Add `approve-decomposition` job (task #311 + #312)
   - No `needs:` — triggered independently by `issue_comment` event
   - Guard: `contains(github.event.comment.body, '/triage approve-decomposition')`
   - Idempotency: checks for `<!-- triage:decomposed-v1 -->` marker before acting
   - Finds most recent proposal comment, extracts JSON block, creates child issues
   - Registers sub-issue relationships via `gh api .../sub_issues` (falls back with `|| true`)
   - Posts `<!-- triage:decomposed-v1 -->` completion marker

---

## Testing strategy

GitHub Actions workflows can only be fully tested live. Static validation approach:
- YAML syntax: `python3 -c "import yaml, sys; yaml.safe_load(sys.stdin)" < triage.yml`
- `actionlint` if available
- Manual review against GitHub Actions schema for expression syntax and job conditionals

Live testing requires:
- Opening an issue with `kind/epic` or `kind/feature` labels
- Verifying `status/triage` is applied by `post-label`
- Verifying proposal comment is posted by `propose-decomposition`
- Commenting `/triage approve-decomposition` and verifying child issues are created

---

## Risk assessment

| Risk | Likelihood | Mitigation |
|---|---|---|
| `${child_kind^}` bash capitalisation fails | Low — ubuntu-latest uses bash 5.x | Verified: ubuntu-latest ships bash 5.1; `^` param expansion supported since bash 4.0 |
| Heredoc injection via issue title/body | Low | `$ISSUE` is a number only; title/body read server-side via `gh api`, not interpolated in comment template |
| Sub-issues API returns 404 | Medium — feature not universally enabled | `|| true` fallback; sub-issue relationship silently skipped, not fatal |
| Infinite loop: propose-decomposition triggers on issue edits | None | Job only runs on `issues` events; `post-label` and `propose-decomposition` don't edit the issue body, only post comments; comment events are not in the `issues` event type |
| Duplicate proposals on re-run | None | Idempotency marker `<!-- triage:proposal-v1 -->` checked before posting |
| Duplicate decomposition on re-run | None | Idempotency marker `<!-- triage:decomposed-v1 -->` checked before acting |

---

## Dependencies

- Existing `triage` reusable workflow at `convergent-systems-co/repo-standards/.github/workflows/triage.yml@v1` — must complete before `post-label` and `propose-decomposition` run
- `github.token` has `issues: write` permission (already declared in top-level `permissions`)
- `gh` CLI available on `ubuntu-latest` (always present since github-runner v2.x)

---

## Backward compatibility

No existing behavior is changed. The existing `triage` job is unmodified. The three new jobs are additive.

---

## Commits

Three commits, one logical unit each:
1. `feat(triage): ensure status/triage label applied post-triage (#309)`
2. `feat(triage): post decomposition proposal for epic/feature issues (#310 #312)`
3. `feat(triage): create sub-issues on /triage approve-decomposition (#311 #312)`
