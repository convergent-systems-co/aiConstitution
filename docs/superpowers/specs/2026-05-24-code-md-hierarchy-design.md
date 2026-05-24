# Code.md §11.9 — Agile Hierarchy Workaround Design

**Date:** 2026-05-24
**Status:** Approved
**Target file:** `~/.ai/Code.md`
**Placement:** New `§11.9` after `§11.8 Agentic dispatch protocol`, before `## 12. Changelog`

---

## What is being added

A new subsection `**11.9 Agile issue hierarchy — task-list workaround.**` covering:

1. **Label vocabulary** — `agile/epic` → `agile/feature` → `agile/story` → `agile/task`
2. **`## Children` convention** — how parent-child links are represented in issue bodies
3. **`scripts/create-issues.sh`** — the idempotent script that creates and re-links issues
4. **Check command** — `gh api …/sub_issues` to detect if native sub-issues are enabled
5. **Rebuild command** — `./scripts/create-issues.sh` when 404
6. **Manual add** — exact bash block to add a single child to a parent
7. **Migration** — exact code swap in `create-issues.sh` when POST /sub_issues works

## Version bump

`Code.md` version: `0.6` → `0.7`

Changelog entry: `0.7 — Added §11.9 Agile issue hierarchy task-list workaround: label vocabulary, ## Children convention, idempotent create-issues.sh script, check/rebuild/manual-add commands, and exact migration steps for when GitHub native sub-issues are enabled.`

## Scope

Single file (`~/.ai/Code.md`). No other files touched.
