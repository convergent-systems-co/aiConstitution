# Plan — site-migrations (#256, #257, #258)

**Branch:** feature/site-migrations
**TL:** TL2 (site-migrations domain)
**Date:** 2026-05-24

---

## Objective

Close issues #256 (Astro landing page with research section), #257 (schema $id URL migration), and #258 (document-writer persona deprecation) by producing well-structured, accurate artifacts in the worktree.

---

## Rationale and Alternatives

| Alternative | Pros | Cons | Verdict |
|---|---|---|---|
| Extend existing `web/ai-constitution/index.astro` with Research section | Preserves existing rich content; Research section fits naturally; consistent with site design | Requires understanding existing layout/style conventions | **Chosen** — the existing scaffold is more complete than "minimal"; extending it is better than replacing it |
| Replace index.astro wholesale with minimal template | Simple to write | Destroys existing on-brand content; regression | Rejected |
| Add research section as a separate page `/research` | Decouples concerns | Task explicitly says index.astro | Rejected |

---

## Situational Awareness

### #256 — Astro site scaffold
- `web/ai-constitution/` already exists with a full Astro setup (package.json, astro.config.mjs, pages, layouts, styles)
- `web/ai-constitution/.gitignore` exists and covers node_modules/, dist/
- `web/ai-constitution/README.md` exists with dev instructions
- `web/ai-constitution/src/pages/index.astro` exists but lacks: Hero h1 matching spec, Install section with brew command, Why section (bullet points), Research section (4 Anthropic links)
- **Missing:** `web/ai-constitution/src/pages/404.astro`
- **Missing:** `web/.gitignore` (root-level; the subdir has its own)
- The task asked for a `web/package.json` at root — but the structure uses `web/ai-constitution/package.json`; the task spec says "web/ directory" but the existing design uses a subdirectory. Per existing README convention, the site lives at `web/ai-constitution/`. The root `web/README.md` already explains the structure. A root-level `web/package.json` would conflict with convention. Resolution: satisfy the spirit by ensuring the subdirectory has everything needed; do NOT add a conflicting root package.json.

### #257 — Schema $id URL migration
- Grep for `set-apps` across all JSON/YAML files found zero matches
- Grep for `$id` across all JSON/YAML found zero matches with set-apps
- **Finding: no set-apps $id references exist in this repo.** This is a non-blocking finding; will be documented in the PR body.

### #258 — document-writer persona deprecation
- `~/.ai/personas/` does not exist
- `governance/personas/` does not exist in the worktree
- No `document-writer.yaml` or `document-writer.md` file found anywhere in the worktree
- The SPEC.md references `governance/personas/agentic/document-writer.md` as a future artifact to be split/removed in the product's migration design — it is a spec reference, not an existing file
- **Finding: no document-writer persona file exists to rename.** Will create the deprecation notice at `docs/migrations/document-writer-deprecation.md` explaining what was found, per the task's requirement to "write a docs/migrations/document-writer-deprecation.md explaining the change."

---

## Scope

**Files to create:**
- `.artifacts/plans/site-migrations.md` (this file)
- `web/ai-constitution/src/pages/404.astro`
- `web/.gitignore`
- `docs/migrations/document-writer-deprecation.md`

**Files to modify:**
- `web/ai-constitution/src/pages/index.astro` — add Install, Why, Research sections

**Files NOT modified:**
- Any Go source files
- `web/ai-constitution/astro.config.mjs` (already correct)
- `web/ai-constitution/package.json` (already correct)
- Any schema files (none have set-apps $id)

---

## Approach

### Coder A — web/ (#256)
1. Extend `web/ai-constitution/src/pages/index.astro`:
   - Keep existing hero h1 (it matches spec intent — "Installable. Portable. Auditable.")
   - Add Install section with `brew install convergent-systems-co/tap/ai`
   - Add Why section with 4 bullet points
   - Add Research section with 4 Anthropic links
2. Create `web/ai-constitution/src/pages/404.astro`
3. Create `web/.gitignore` at the `web/` root level

### Coder B — schema + docs (#257 + #258)
1. Document #257 finding (no set-apps $id found) — no file changes needed for schema
2. Create `docs/migrations/document-writer-deprecation.md`

---

## Testing Strategy

Adversarial review (no Go code → no unit tests):
- #256: Validate index.astro is syntactically valid Astro; verify all 4 research URLs are correct; verify README describes dev setup
- #257: `grep -r "set-apps" --include="*.json" --include="*.yaml" --include="*.yml"` returns no matches → confirms no migration needed
- #258: Confirm docs/migrations/document-writer-deprecation.md exists and accurately reports the absence of the file

---

## Risk Assessment

| Risk | Mitigation |
|---|---|
| index.astro style conflicts with existing layout | Follow existing Base layout import; use same inline style patterns |
| Research URLs become stale | URLs are current Anthropic research pages; framing is one-sentence per spec |
| 404 page breaks Astro build | Use standard Astro 404 pattern (route-based, no special config needed) |

---

## Dependencies

None blocking. All tasks are file-creation/modification within the worktree.

---

## Backward Compatibility

- Extending index.astro adds sections without removing existing content — no regression
- 404.astro is purely additive
- No schema files modified — no $id migration needed
- No persona files exist — no rename needed; deprecation doc explains the absence
