# Code.md §11.9 — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` (recommended) or `superpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `§11.9 Agile issue hierarchy — task-list workaround` to `~/.ai/Code.md` after §11.8, bump the file version to 0.7, and add a changelog entry.

**Architecture:** Single file edit. No code changes. TDD skip applies (governance prose only — per §11.8 TDD skip rule). Edit → verify → commit.

**Tech Stack:** Markdown, bash.

---

## File structure

| Path | Action |
|---|---|
| `~/.ai/Code.md` | Modify — insert §11.9, bump version, add changelog entry |

---

## Task 1: Insert §11.9 into Code.md

**Files:**
- Modify: `~/.ai/Code.md`

- [ ] **Step 1: Confirm exact insertion point**

```bash
grep -n "^---" ~/.ai/Code.md | tail -5
grep -n "^## 12" ~/.ai/Code.md
```

Expected: one line matching `## 12. Changelog` — note its line number. The `---` immediately before it is the insertion point.

- [ ] **Step 2: Insert §11.9 before the final `---` separator**

Use the Edit tool to insert after the last line of §11.8 ("Stale TODO resolution..." paragraph ends with `...without wire-up.`) and before the `---` separator:

Find this exact string in `~/.ai/Code.md`:

```
...the deferred behavior, updating the comment, and updating any test that was written to validate the deferred state.

---

## 12. Changelog
```

Replace it with:

```
...the deferred behavior, updating the comment, and updating any test that was written to validate the deferred state.

**11.9 Agile issue hierarchy — task-list workaround.**

Labels: `agile/epic` → `agile/feature` → `agile/story` → `agile/task`. Parent-child links live in a `## Children` task list in each parent's body. `scripts/create-issues.sh` in the `convergent-systems-co/aiConstitution` repo creates and re-links all issues; it is idempotent and safe to re-run.

**Check if native sub-issues are enabled:**

```bash
gh api repos/convergent-systems-co/aiConstitution/issues/23/sub_issues
```

Returns `[]` → live, skip to migration. Returns `404` → not yet enabled, use steps below.

**If 404 — rebuild all links:**

```bash
./scripts/create-issues.sh
```

**Add a single child manually:**

```bash
PARENT=<number>  CHILD=<number>  CHILD_TITLE="[Label] Title"
REPO="convergent-systems-co/aiConstitution"
current=$(gh issue view "$PARENT" --repo "$REPO" --json body --jq '.body // ""')
base=$(printf '%s' "$current" | awk '
  /^## (Features|Children|Stories|Tasks)/{exit}
  /^- \[.?\] #[0-9]/{exit}
  {print}
' | sed 's/[[:space:]]*$//')
tmpfile=$(mktemp)
printf '%s\n\n## Children\n- [ ] #%s %s\n' "$base" "$CHILD" "$CHILD_TITLE" > "$tmpfile"
gh issue edit "$PARENT" --repo "$REPO" --body-file "$tmpfile" && rm "$tmpfile"
```

**Migration — when native sub-issues are enabled:**

Confirm: `gh api repos/convergent-systems-co/aiConstitution/issues/23/sub_issues -X POST -F sub_issue_id=33 2>&1` — expect JSON with `id`, `number`, `title` (not `404`).

Then in `scripts/create-issues.sh`: replace the `record_child` function with:

```bash
add_sub_issue() {
  local parent="$1" child="$2"
  [[ "$DRY_RUN" == "--dry-run" ]] && { echo "  DRY-RUN: #$child → #$parent" >&2; return; }
  [[ "$parent" == "0" || "$child" == "0" ]] && return
  sleep 0.5
  gh api "repos/$REPO/issues/$parent/sub_issues" \
    -X POST -F sub_issue_id="$child" --silent 2>/dev/null || true
}
```

Rename every `record_child "$p" "$n" "$t"` call to `add_sub_issue "$p" "$n"`. Delete `link_all_children`, its call, `CHILDREN_DIR`, `trap`, and `record_child`. Re-run `./scripts/create-issues.sh` once.

---

## 12. Changelog
```

- [ ] **Step 3: Verify §11.9 appears in the file**

```bash
grep -n "11.9" ~/.ai/Code.md
```

Expected: one line containing `**11.9 Agile issue hierarchy`.

---

## Task 2: Bump version and add changelog entry

**Files:**
- Modify: `~/.ai/Code.md` (version line and changelog section)

- [ ] **Step 1: Bump version from 0.6 to 0.7**

In `~/.ai/Code.md`, find:

```
**Version:** 0.6
```

Replace with:

```
**Version:** 0.7
```

- [ ] **Step 2: Add 0.7 changelog entry**

In `~/.ai/Code.md`, find the start of the changelog:

```
## 12. Changelog

- **0.6** —
```

Replace with:

```
## 12. Changelog

- **0.7** — Added §11.9 **Agile issue hierarchy task-list workaround**: label vocabulary (`agile/epic` → `agile/feature` → `agile/story` → `agile/task`), `## Children` task list convention, idempotent `scripts/create-issues.sh` script, check/rebuild/manual-add bash commands, and exact migration steps for when GitHub native sub-issues (POST `/issues/:n/sub_issues`) are enabled. Active workaround until the `convergent-systems-co` org enables the GitHub sub-issues beta.

- **0.6** —
```

- [ ] **Step 3: Verify version and changelog**

```bash
grep -n "Version:" ~/.ai/Code.md | head -2
grep -n "0.7" ~/.ai/Code.md
```

Expected: `Version: 0.7` on line ~5, and the new 0.7 changelog entry.

---

## Task 3: Commit

**Files:**
- `~/.ai/Code.md`

- [ ] **Step 1: Stage and commit**

```bash
cd ~/.ai
git add Code.md
git commit -m "docs(code): add §11.9 agile hierarchy task-list workaround

New subsection in §11 Change Management:
- Label vocabulary (epic→feature→story→task)
- ## Children task list convention
- create-issues.sh idempotent script reference
- Check/rebuild/manual-add bash commands
- Exact migration steps for when GitHub sub-issues are enabled

Bumps Code.md version 0.6 → 0.7."
```

- [ ] **Step 2: Verify commit**

```bash
cd ~/.ai && git log --oneline -2
```

Expected: most recent commit contains `docs(code): add §11.9`.

---

## Self-review

**Spec coverage:**
- Label vocabulary ✅ — Task 1 Step 2
- `## Children` convention ✅ — Task 1 Step 2
- `create-issues.sh` note ✅ — Task 1 Step 2
- Check command ✅ — Task 1 Step 2
- Rebuild command ✅ — Task 1 Step 2
- Manual add bash block ✅ — Task 1 Step 2
- Migration code swap ✅ — Task 1 Step 2
- Version bump ✅ — Task 2 Step 1
- Changelog entry ✅ — Task 2 Step 2

**Placeholder scan:** None. All bash blocks are complete and exact.

**Scope:** Single file (`~/.ai/Code.md`). Nothing else touched.
