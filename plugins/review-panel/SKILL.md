---
name: review-panel
triggers:
  - /review-panel
  - run a review panel
  - panel review
  - multi-panel review
  - orchestrate a review
description: Guided workflow — select panels → run passes → aggregate scores → produce report.
---

# review-panel

Guided multi-panel review workflow. Selects review perspectives, runs each pass using `ai review`, aggregates scores, and writes a structured report artifact.

## When to Use

Use when you need more than a single-pass review: governance files, PRs, prose drafts, or any artifact that benefits from multiple independent evaluation perspectives (architecture, correctness, documentation quality, adversarial edge cases, security).

## Workflow

### Step 1 — Select the target

Ask the user: what is being reviewed?

- A PR: `ai review --pr=<number>` fetches the diff automatically.
- A local file or directory: the user provides the path.
- A governance file (one of the four canonical `~/.ai/` files): note that these are high-sensitivity artifacts and flag this to the user.

Confirm the target exists before proceeding. If `ai review --pr=<number>` fails to fetch the diff, report the error and ask the user to verify the PR number and that `gh` is authenticated.

### Step 2 — Select panels

Ask which review perspectives should run. Suggest a default set based on the target type:

| Target type | Suggested panels |
|---|---|
| PR / code diff | tech-lead, adversarial, security |
| Prose / governance doc | documentation-reviewer, adversarial |
| Mixed (code + docs) | tech-lead, documentation-reviewer, adversarial |

Each panel maps to a persona atom. Confirm available personas with:

```bash
ai persona list
```

If the user requests a persona not in the list, suggest the closest match from `ai persona list` output and ask for confirmation before proceeding.

Show the selected panel set and wait for the user to confirm before running.

### Step 3 — Run passes

For each selected panel, run the review pass and present output as it completes.

**For PR targets:**

```bash
ai review --pr=<number>
```

The command loads configured panels from the project and prints a scored report per panel. Note: panel evaluation is currently stubbed in the CLI (confidence 0.75 placeholder per panel). Present the raw output and supplement with the panel persona's evaluation if a richer pass is needed.

**For file targets:** invoke the panel persona directly, passing the file content as context. Present each panel's findings in order.

Each pass produces:
- A score (1–5, where 1 = critical issues, 5 = no action needed)
- Structured findings (issue description, severity, location)

### Step 4 — Aggregate scores

Once all panels have completed:

1. Compute the overall score: average of all panel scores, rounded to one decimal.
2. Surface universal findings: any issue flagged by two or more panels is a universal finding — highlight these prominently.
3. Surface panel disagreements: where panels scored the same section differently, name the disagreement and its likely cause.

Present the aggregate summary before writing the report.

### Step 5 — Produce report

Write the final report to `.artifacts/reviews/<UTC-timestamp>-<slug>.md`.

Report structure:

```markdown
# Review Report — <target> — <UTC date>

## Overall Score: <N>/5

## Panel Breakdown

| Panel | Score | Key Finding |
|---|---|---|
| <name> | <score>/5 | <one-line summary> |

## Universal Findings (flagged by 2+ panels)

- <finding>

## Panel-Specific Findings

### <panel name>
- <finding with location>

## Recommended Next Actions

1. <highest-priority action>
2. <next action>
```

Confirm the report path with the user after writing.

## Error Handling

| Situation | Response |
|---|---|
| `ai review --pr=<n>` returns no output | Verify PR number exists: `gh pr view <n>` |
| `ai persona list` shows persona not found | Suggest closest available persona; do not proceed with an unknown persona |
| Target file path does not exist | Surface the error per `Common.md P3`; ask for the correct path |
| Score cannot be computed (no panels ran) | Do not write a report; surface the gap explicitly |

## Notes

- `ai review --check` runs a cheap governance dry-run scan (not a panel review). Do not use it as a substitute for `--pr=<n>` when reviewing code.
- `ai review --dry-run` prints proposed governance amendments without writing. Useful for the governance-doc review path.
- Panel configuration lives in the project's panels config (loaded by `ai review --pr`). Use `ai persona show <name>` to inspect a specific panel's persona definition.
