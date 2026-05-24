# Coder Guide

Practical steps for working in this repo. Start here before opening a PR.

---

## Agile Issue Hierarchy

The backlog is organized as a four-level tree:

```
agile/epic
  └── agile/feature
        └── agile/story
              └── agile/task   ← created manually when you start work
```

All four levels are regular GitHub issues. Hierarchy is represented by a
`## Children` task list in the parent issue's body (e.g. `- [ ] #33 [Feature] …`).
GitHub renders this as a progress indicator on the parent.

### Why task lists instead of native sub-issues

GitHub's native sub-issues feature (the `POST /issues/:n/sub_issues` API) is a
beta that must be enabled per-org. Until it is enabled for
`convergent-systems-co`, task lists in the issue body are the workaround.
Once the feature is enabled, see [Migrating to native sub-issues](#migrating-to-native-sub-issues) below.

---

## Working with issues

### Finding your work

- Epics: `gh issue list --label agile/epic`
- Features under an epic: open the epic, read the `## Children` task list
- Stories under a feature: open the feature, read the `## Children` task list

### Starting a story

1. Find the story you are picking up (it will have label `agile/story`).
2. Create a task-level issue manually:
   ```
   gh issue create \
     --title "[Task] <what you are doing>" \
     --body "Part of #<story number>." \
     --label "agile/task,status/in-progress"
   ```
3. Add it to the story's body by editing the story issue and appending to its
   `## Children` section:
   ```
   - [ ] #<task number> [Task] <what you are doing>
   ```
4. Create a branch: `git switch -c <story-slug>` (or use `ai worktree add <name>`
   for an isolated workspace).
5. When done: open a PR, link it to the story with `Closes #<story>` in the PR
   description.

### Closing an issue in the hierarchy

- When a task PR merges, GitHub auto-closes the task issue if you used
  `Closes #N` in the PR body.
- Mark the matching task-list checkbox in the parent story by editing the
  story issue body: change `- [ ] #N` to `- [x] #N`.
- When all tasks in a story are checked, close the story manually or via
  a final PR.
- Stories roll up to features, features roll up to epics — keep checkboxes
  current so the progress indicators stay accurate.

---

## Rebuilding or extending the hierarchy

All hierarchy management goes through `scripts/create-issues.sh`. The script is
**idempotent** — re-running it skips existing issues and rebuilds the
`## Children` sections cleanly.

### Prerequisites

```bash
brew install jq          # JSON processing
gh --version             # GitHub CLI ≥ 2.30
gh auth status           # must be authenticated
/opt/homebrew/bin/bash --version  # bash ≥ 5 (macOS ships bash 3.2)
```

### Dry-run first

```bash
./scripts/create-issues.sh --dry-run 2>&1 | head -30
```

### Run for real

```bash
./scripts/create-issues.sh 2>&1 | tee /tmp/create-issues.log
```

What happens:
- **Pass 1** — creates any missing epics (`agile/epic`)
- **Pass 2** — creates any missing features (`agile/feature`)
- **Pass 3** — creates any missing stories (`agile/story`)
- **Pass 4** — rebuilds `## Children` task lists on all parents

Pass 4 strips any existing `## Features`, `## Children`, or bare `- [ ] #N`
lines before writing, so re-runs always produce a clean result.

### Adding a new story to the spec

1. Open `scripts/create-issues.sh` and find the `STORY_DEFS` array.
2. Append a new entry in `"TITLE|||PARENT_FEATURE_TITLE|||BODY"` format:
   ```bash
   "[Story] My new story|[Feature] The parent feature title|One-sentence description. Spec §N."
   ```
   The `|` delimiter is safe in body text; only the first two `|` characters
   are used as field separators (the `cut -d'|' -f3-` idiom captures the rest).
3. Re-run the script. It will create the new story and add it to its parent's
   `## Children` list.

### Adding a new feature or epic

Same pattern — add to `FEATURE_DEFS` or `EPIC_TITLES`/`EPIC_BODIES` arrays and
re-run. The script resolves parent relationships automatically.

---

## Migrating to native sub-issues

When GitHub enables the sub-issues feature for `convergent-systems-co`:

1. Verify the API works:
   ```bash
   gh api repos/convergent-systems-co/aiConstitution/issues/23/sub_issues
   # should return [] not 404
   gh api repos/convergent-systems-co/aiConstitution/issues/23/sub_issues \
     -X POST -F sub_issue_id=33 2>&1
   # should return the sub-issue object, not an error
   ```

2. Replace `record_child` in the script with the native API call. Change this
   function in `scripts/create-issues.sh`:
   ```bash
   # BEFORE (task-list workaround)
   record_child() {
     local parent="$1" child="$2" child_title="$3"
     [[ "$parent" == "0" || "$child" == "0" ]] && return
     echo "${child}|${child_title}" >> "${CHILDREN_DIR}/parent_${parent}"
   }
   ```
   to:
   ```bash
   # AFTER (native sub-issues)
   add_sub_issue() {
     local parent="$1" child="$2"
     [[ "$DRY_RUN" == "--dry-run" ]] && { echo "  DRY-RUN: would link #$child → #$parent" >&2; return; }
     [[ "$parent" == "0" || "$child" == "0" ]] && return
     sleep 0.5
     gh api "repos/$REPO/issues/$parent/sub_issues" \
       -X POST -F sub_issue_id="$child" --silent 2>/dev/null || true
   }
   ```
   Also rename `record_child` call sites to `add_sub_issue` and remove the
   `link_all_children` call at the end.

3. Re-run `./scripts/create-issues.sh` once to wire native sub-issue
   relationships for all existing issues.

4. The `## Children` task lists in issue bodies will become redundant once
   native sub-issues are wired. You can leave them (they do no harm) or strip
   them with a one-off edit pass.
