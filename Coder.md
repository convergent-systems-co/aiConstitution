# Coder.md — Agile Issue Hierarchy

## Rules (always active)

Labels: `agile/epic` → `agile/feature` → `agile/story` → `agile/task`

Parent-child links live in a `## Children` task list in each parent's body (`- [ ] #N [Label] Title`).
`scripts/create-issues.sh` creates issues and rebuilds all links. Idempotent — safe to re-run.

---

## Task-list workaround (active until sub-issues are enabled)

**Step 1 — Check if native sub-issues are enabled**

```bash
gh api repos/convergent-systems-co/aiConstitution/issues/23/sub_issues
```

Returns `[]` → live, skip to **Migration**. Returns `404` → not yet, continue below.

**Step 2 — Create missing issues and rebuild all links**

```bash
./scripts/create-issues.sh
```

**Step 3 — Add a single child manually**

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

---

## Migration (when native sub-issues are enabled)

**Step 1 — Confirm POST works**

```bash
gh api repos/convergent-systems-co/aiConstitution/issues/23/sub_issues \
  -X POST -F sub_issue_id=33 2>&1
```

Expected: JSON with `id`, `number`, `title`. Still `404` → not yet enabled.

**Step 2 — Update scripts/create-issues.sh**

Replace the `record_child` function with:

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

Then: rename `record_child "$p" "$n" "$t"` → `add_sub_issue "$p" "$n"` everywhere;
delete `link_all_children`, its call, `CHILDREN_DIR`, `trap`, and `record_child`.

**Step 3 — Re-run once:** `./scripts/create-issues.sh`

All links are now native sub-issues. `## Children` task lists are redundant — leave or strip.
