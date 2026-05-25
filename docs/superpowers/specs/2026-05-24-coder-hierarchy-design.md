# Coder.md — Hierarchy Workflow Design

**Date:** 2026-05-24
**Status:** Approved
**Audience:** AI agents (Claude Code, Copilot) loading this as operational context
**Scope:** Task-list workaround for agile issue hierarchy + migration steps to native sub-issues

---

## Design

### Format

Instruction block: imperative, no prose narrative, code blocks for every command.
Target ≤ 80 lines. Loads fast as AI context.

### Three sections

**Section 1 — Hierarchy rules (always active)**
- Label vocabulary: `agile/epic` → `agile/feature` → `agile/story` → `agile/task`
- Parent-child links live in `## Children` task list in each parent's body
- `scripts/create-issues.sh` is the single source of truth for creating and re-linking

**Section 2 — Task-list workaround (active until sub-issues enabled)**
Numbered commands in order:
1. Test if native API is live (`gh api … /sub_issues` — `[]` = live, `404` = not yet)
2. If `404`: run `create-issues.sh` to create missing issues and rebuild all `## Children` lists
3. When adding a single child manually: fetch parent body → strip old children section → append `- [ ] #N [Label] Title` → `gh issue edit --body-file`

**Section 3 — Migration trigger**
Two-command test (GET + POST). If POST returns the sub-issue object:
- Delete `link_all_children()` from `create-issues.sh`
- Replace `record_child` call sites with direct `gh api … -F sub_issue_id=N` calls
- Re-run `create-issues.sh` once to wire all existing issues natively
- `## Children` task lists become redundant; leave or strip

### What is NOT in scope

- Branching conventions, PR process, commit format — separate docs
- Any content useful only to human contributors (narrative, context, rationale)
