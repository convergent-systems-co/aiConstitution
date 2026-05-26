# Plan: Add kind/* Label Set to aiConstitution and Atom Repos

**Issue:** #308
**Branch:** feat/308-kind-labels
**Date:** 2026-05-25

## Objective

Add four new `kind/*` GitHub labels (`kind/epic`, `kind/feature`, `kind/story`, `kind/task`) to `convergent-systems-co/aiConstitution` and all sibling atom repos. Deliver an idempotent script for re-use when new repos are added.

## Rationale

| Alternative | Pros | Cons | Verdict |
|---|---|---|---|
| Manual label creation via GitHub UI | Simple, no code | Not repeatable, error-prone across 26+ repos | Rejected |
| GitHub Actions workflow to sync labels | Fully automated on trigger | Adds CI complexity, harder to run ad-hoc | Rejected |
| Idempotent shell script using `gh` CLI | Repeatable, simple, auditable, runs ad-hoc or in CI | Requires `gh` CLI and auth | **Chosen** |

## Scope

- Create: `.artifacts/plans/308-kind-labels.md` (this file)
- Create: `scripts/sync-labels.sh`
- Side effects (not committed): GitHub labels created in 27 repos

## Labels

All four new labels use color `#d4c5f9` (matching `kind/finding`):

| Name | Description |
|---|---|
| `kind/epic` | Top-level organizational container for a body of work |
| `kind/feature` | User-facing capability addition |
| `kind/story` | User-level description of a desired behavior |
| `kind/task` | Atomic unit of implementation work |

## Approach

1. Write `scripts/sync-labels.sh` with:
   - Hardcoded label definitions (name, color, description)
   - Idempotent `ensure_label` function using `gh label create --force`
   - Target: `aiConstitution` first, then all `*atom*` repos in the org
2. Run the script to create labels in all repos
3. Verify via `gh label list` on `aiConstitution` and spot-check one atom repo
4. Verify idempotency by running the script a second time
5. Commit `scripts/sync-labels.sh` and the plan

## Testing Strategy

- Idempotency: run script twice; second run must exit 0 without creating duplicates
- Verification: `gh label list --repo convergent-systems-co/aiConstitution --json name --jq '[.[].name] | sort'` must include all four new labels
- Spot-check: verify one atom repo (e.g., `skill-atoms`) shows the same labels

## Risk Assessment

- `gh label create --force` upserts; no risk of partial-create failures leaving inconsistent state
- Existing labels (`kind/finding`, `kind/hook`) are untouched — script only creates, does not delete
- API rate limiting: 27 repos × 4 labels = 108 calls, well within GitHub's 5000/hour limit

## Dependencies

- `gh` CLI authenticated as `polliard` (org member with write access)
- All atom repos must be accessible under `convergent-systems-co` org

## Backward Compatibility

- Additive only; no existing labels are modified or removed
