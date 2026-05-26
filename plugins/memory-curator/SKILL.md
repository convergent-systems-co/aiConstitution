---
name: memory-curator
triggers:
  - /memory-curator
  - curate memory
  - review memory
  - clean up memory
  - audit my memories
  - memory review
description: Guided workflow for reviewing, curating, and archiving AI memory entries. Identifies patterns and optionally proposes governance amendments.
---

# memory-curator

Guided workflow: **review → identify stale → identify patterns → propose amendments → archive**.

---

## Step 1 — Review all memories

Run the full inventory:

```bash
ai memory list
```

Group results by type: `user`, `feedback`, `project`, `reference`.

Report the summary:
- Total count by type
- Oldest entry (by created date)
- Most recently updated entry

If the list is empty, inform the user and suggest building memories first:

```bash
ai memory codify <path-to-conversation-note>
```

---

## Step 2 — Identify stale or redundant entries

For each group, scan for:

- **Outdated entries** — superseded by newer information (e.g., a feedback memory about a deprecated tool, a project memory for a repo that no longer exists).
- **Duplicates or near-duplicates** — two or more entries that capture the same rule or preference with minor wording differences.
- **Contradictions** — entries that directly conflict with each other (e.g., "always use X" vs. "never use X").

Show the candidate stale list to the user. Ask: "Which of these should we retire or archive?"

Wait for confirmation before proceeding to any archival action.

---

## Step 3 — Identify patterns

Scan across all memory entries (not just the stale candidates) for recurring themes:

- Three or more `feedback` memories that express the same preference in different contexts → candidate for a governance rule.
- Multiple `project` memories that share a structural convention → candidate for a project-level standard.
- A `reference` memory that is cited repeatedly in other entries → candidate for promotion to a standing rule.

For each identified pattern, summarize:

```
Pattern: <one-sentence description>
Evidence: <list of memory slugs that exhibit it>
Proposed action: codify in <Common.md | Code.md | Writing.md | project CLAUDE.md>
```

Show the pattern summary to the user. Ask: "Which patterns should we escalate to governance amendments?"

---

## Step 4 — Propose amendments

For each pattern the user approves for escalation, draft a governance amendment:

```bash
ai amend draft "<pattern description>"
```

If `ai amend draft` fails, show the error and offer to create a minimal violation stub manually at
`~/.ai/audit/violations/<UTC-ISO-8601>.md` using the schema from `Constitution.md §5.2`.

Review the draft with the user. They choose:
- **Escalate** — submit the amendment PR.
- **Keep as memory** — no amendment; the memory stays as-is.
- **Discard** — the pattern is not significant enough to retain.

---

## Step 5 — Archive and retire

Execute the confirmed archival actions from Step 2:

**Archive** (preserve but deprioritize — entry is no longer active but may be useful for reference):

```bash
ai memory archive <name>
```

**Retire** (soft-delete — entry is wrong, irrelevant, or superseded with no archival value):

```bash
ai memory retire <slug>
```

Confirm each action with the user before running it (per `Common.md §2.2` — destructive action gate).

After all actions are complete, run `ai memory list` again and report the delta:
- Entries before / entries after
- Archives created
- Retirements performed
- Amendments drafted
