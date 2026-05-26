---
name: atom-publisher
triggers:
  - /atom-publisher
  - publish an atom
  - create an atom
  - distribute an atom
  - draft an atom
  - contribute an atom
---

# atom-publisher

Guided 5-step workflow: draft → canonicalize → verify → PR → publish.
Produces an atom in the appropriate catalog (brand-atoms.com, persona-atoms.com, profile-atoms.com, or skill-atoms.com).

---

## Step 1 — Identify the atom type

Ask the user:

1. **Kind:** What type of atom is this?
   - `skill` — a skill bundle (`skill-atoms.com`)
   - `persona` — an agentic or reviewer persona (`persona-atoms.com`)
   - `profile` — a profile composition (`profile-atoms.com`)
   - `plugin` — a plugin artifact (`plugin-atoms.com`)
   - `amendment` — a governance amendment
   - `hook` — a lifecycle hook
   - `action` — a CLI action
   - `workflow` — a multi-step workflow
   - `pipeline` — an agent pipeline definition

2. **Catalog:** Which registry does it belong to? (Infer from kind above; confirm with the user.)

3. **Name:** What is the atom's name? Must be kebab-case.

State the kind, catalog, and name before proceeding.

---

## Step 2 — Draft

Two paths:

**A. Fork an existing atom (recommended when a close template exists):**

```bash
ai atoms fork <base-atom-name>
```

This copies the installed atom to `~/.ai/atoms/<base-atom-name>-local/` and patches `atom.toml` with an `upstream_ref`. Edit the fork to match your intent.

**B. Start from scratch:**

Create a directory under `~/.ai/atoms/<name>/` with:
- `atom.toml` — manifest (required fields: `name`, `version`)
- Payload files (SKILL.md, persona YAML, hook script, etc.)

Show the draft to the user for review before proceeding.

---

## Step 3 — Canonicalize and verify

1. **Check structure.** Confirm `atom.toml` has valid TOML, `name` and `version` are present, and all referenced payload files exist.

2. **Run the verifier:**

```bash
ai atoms verify
```

`ai atoms verify` re-hashes every cached atom and compares to the stored SHA256. A passing result means the on-disk content matches the recorded hash.

**If `ai atoms verify` fails (malformed TOML or missing fields):**
- Show the specific error output.
- Offer to fix the draft: identify the exact field or file that is malformed and apply the correction.
- Re-run `ai atoms verify` after the fix.

---

## Step 4 — PR and review

Open a pull request to the appropriate catalog repo.

Before creating the PR, confirm:
- The atom name and version are correct.
- The catalog repo URL is known (e.g., `convergent-systems-co/skill-atoms`).

PR body template:

```
## What this atom does
<One paragraph: purpose, behavior, and governed surfaces.>

## Why it belongs in this catalog
<One paragraph: how it fits the catalog's scope; any prior art it supersedes or extends.>

## Verification
- [ ] `ai atoms verify` passes
- [ ] atom.toml has `name` and `version`
- [ ] Payload files are present and valid
```

Create the PR with:

```bash
gh pr create \
  --repo convergent-systems-co/<catalog-repo> \
  --title "<kind>(<name>): <short description>" \
  --body "$(cat <<'EOF'
<filled PR body>
EOF
)"
```

---

## Step 5 — Publish

After the PR is merged, finalize:

```bash
ai atoms publish --name <atom-name> --version <version>
```

Then confirm the atom is visible:

```bash
ai atoms list
```

The atom should appear with the correct name, version, and path.

**If publish fails because the atom already exists:**

Offer to bump the version with a `@<new-version>` suffix:

```bash
ai atoms publish --name <atom-name> --version <version-bump>
```

Use SemVer conventions: patch bump for corrections, minor bump for new content, major bump for breaking changes to the atom's contract.

---

## Quick reference: verified `ai atoms` subcommands

| Command | Purpose |
|---|---|
| `ai atoms fork <name>` | Copy an installed atom as a local draft |
| `ai atoms verify` | Check SHA256 hashes of all cached atoms |
| `ai atoms publish --name <n> --version <v>` | Package and publish the atom |
| `ai atoms list` | Show all installed atoms |
| `ai atoms fetch <id-or-url>` | Download and install an atom |
| `ai atoms gc` | Garbage-collect unreferenced atom cache entries |
