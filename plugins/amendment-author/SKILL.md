---
name: amendment-author
description: >
  Guided workflow for authoring and publishing governance amendments against
  the four canonical files (Constitution, Common, Code, Writing). Walks the
  full lifecycle: identify the violation, draft the amendment stub, review it,
  apply it to the governance file, and optionally publish it as a versioned
  amendment atom. Triggers on: /amendment-author, "author an amendment",
  "create an amendment", "write a governance amendment",
  "propose a governance change".
---

# Amendment Author — Guided Workflow

Guide the user through every stage from violation to published amendment.
Use PM discipline: each step ends with Did / Next / Blocked / Artifact.

---

## Step 1 — Identify the Violation or Gap

Ask the user:

> "What rule was violated, or what gap needs addressing?"

Accept one of three inputs:

1. **Violation file path** — produced by `ai audit` or a hook, located under
   `~/.ai/audit/violations/`. Example: `~/.ai/audit/violations/2026-05-22T173810Z.md`
2. **Free-text description** — the user describes the behavior; the assistant
   creates a minimal violation stub at `~/.ai/audit/violations/<UTC>.md` with the
   required schema fields (`File / Rule violated`, `What happened`, `How noticed`,
   `Remediation`, `Proposed amendment`).
3. **Reference to a prior audit entry** — user provides the UTC slug or path;
   confirm the file exists before proceeding.

**If creating a stub from free text:** write
`~/.ai/audit/violations/<UTC>.md` with the standard violation schema
(`File / Rule violated`, `What happened`, `How noticed`, `Remediation`,
`Proposed amendment`). Populate from the user's description and use the
resulting path as `<violation-path>` in Step 2.

**PM checkpoint:**

- Did: Identified or created violation file at `<path>`
- Next: Draft the amendment stub
- Blocked: —
- Artifact: `~/.ai/audit/violations/<UTC>.md`

---

## Step 2 — Draft the Amendment

Run:

```bash
ai amend draft <violation-path>
```

This reads the violation file, derives a slug from the rule reference, and
writes a stub at `~/.ai/governance/plans/<UTC>-<slug>.md`.

When `$EDITOR` is set, the stub opens for editing automatically. When it is
not set, the stub path is printed to stdout — capture it.

Show the draft to the user immediately:

```bash
ai amend show <slug>
```

Use the slug from the plan filename (everything after the 17-char UTC prefix).

**Error — `ai amend draft` exits non-zero** (e.g. `"violation file missing
'File / Rule violated:' field"`): return to Step 1, ensure the violation file
contains `- **File / Rule violated:** <value>`, and retry.

**PM checkpoint:**

- Did: Ran `ai amend draft`; stub written to `~/.ai/governance/plans/<UTC>-<slug>.md`
- Next: User reviews the draft
- Blocked: —
- Artifact: `~/.ai/governance/plans/<UTC>-<slug>.md`

---

## Step 3 — Review the Draft

Show the stub via `ai amend show <slug>`. The stub contains three sections:

- `## Target` — the governance section to patch (e.g. `"2. Autonomy Gates"`)
- `## Proposed Change` — the replacement body text
- `## Rationale` — why the change is warranted

The user chooses one of three paths:

**a) Accept as-is** — proceed to Step 4.

**b) Edit directly** — the user opens the file and edits it. Re-show after edits:

```bash
ai amend show <slug>
```

Repeat until the user is satisfied.

**c) Reject** — ask what to do differently. Options: restart Step 2 with a
revised violation description, or abandon the workflow entirely.

**PM checkpoint:**

- Did: Reviewed draft with user; draft accepted / edited / rejected
- Next: Apply the amendment (if accepted or edited) or restart/abandon
- Blocked: —
- Artifact: `~/.ai/governance/plans/<UTC>-<slug>.md` (final reviewed version)

---

## Step 4 — Apply the Amendment

Run:

```bash
ai amend apply <slug>
```

This patches the target section in the referenced canonical governance file,
bumps the file's minor version, and appends a Changelog entry.

Verify the amendment was recorded:

```bash
ai amend list
```

Confirm the slug appears in the output. A successful apply also prints:

```
Applied: bumped version to <new-version>
```

**Error — apply fails:**

- `"section not found"`: the `## Target` value in the stub does not match any
  section heading in the canonical file. Open the stub, correct the `## Target`
  value to match the exact heading text, and retry.
- `"read Constitution.md: no such file"` or similar: the `AI_ROOT` env var may
  point to the wrong location. Verify with `echo $AI_ROOT` and correct if needed.
- Already applied (content already present): inform the user and skip to Step 5.

**PM checkpoint:**

- Did: Applied amendment; version bumped to `<new-version>`; Changelog entry added
- Next: Optionally publish as an amendment atom
- Blocked: —
- Artifact: Patched canonical governance file at `~/.ai/<FileName>.md`

---

## Step 5 — Publish (Optional)

Ask the user:

> "Do you want to publish this amendment as a versioned release atom?"

**If yes:**

```bash
ai amend publish <slug>
```

This validates that the stub's proposed change is present in the canonical
file, then prints the `gh release create` command. The `gh` invocation is
printed but not executed — confirm the command with the user before running
it manually.

**If no:** mark the amendment as applied-only. The workflow ends here. The
Changelog entry in the governance file is the durable record.

**PM checkpoint:**

- Did: Published amendment atom (or skipped publish by user choice)
- Next: —
- Blocked: —
- Artifact: `gh release create v<version> ...` (printed command) or applied-only record

---

## Error Reference

| Error | Cause | Resolution |
|---|---|---|
| `violation file missing 'File / Rule violated:' field` | Stub malformed | Add the required field; retry `ai amend draft` |
| `no plan matching prefix "<slug>"` | Slug typo or wrong directory | Run `ai amend list` to see available slugs |
| `section "<target>" not found in Constitution.md` | `## Target` value doesn't match file heading | Edit stub's `## Target`; retry `ai amend apply` |
| `stub not yet applied` (on publish) | `apply` not run first | Run `ai amend apply <slug>` then retry |
| `read Constitution.md: no such file` | `AI_ROOT` misconfigured | Verify `echo $AI_ROOT`; default is `~/.ai/` |
