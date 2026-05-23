# 0001. Atoms architecture — versioned, immutable, content-addressable units

- Status: accepted
- Date: 2026-05-23
- Source: `SPEC.md §7.9`, `§7.10`

## Context

The original design treated personas, profiles, skills, and brand
tokens as files inside `~/.ai/`. This left a mutation surface open:
every shipped persona file could be edited (intentionally or by
mistake); two machines could disagree on what `coder` meant; sharing
required a binary release.

The user's existing `brand-atoms.com` is the proof-of-concept for a
different model: a public, versioned, content-hashed registry of
W3C design tokens consumed by any site that needs them.

## Decision

Extend the `brand-atoms.com` pattern to **four sibling registries**:

| Registry | Hosts | Schema |
|---|---|---|
| `brand-atoms.com` | Palettes, fonts, brand compositions | W3C design tokens |
| `persona-atoms.com` | Agentic personas (`/agentic/`) + reviewer personas (`/reviewer/`) | Markdown w/ frontmatter; YAML structured fields |
| `profile-atoms.com` | Profile compositions (recipes pinning atoms) | TOML |
| `skill-atoms.com` | Skill bundles | tarballs w/ manifest |

All four follow the same contract:

1. URL shape: `https://<registry>/<kind>/<name>/<semver>/<content-file>` plus
   `<content-file>.meta.json` for provenance/license/dependency/hash.
2. **Immutable at version.** Mutation requires publishing a new SemVer.
3. **Content-hashed.** `meta.json` carries `contentSha256`; consumers
   verify on every cache load.
4. **Cached locally** under `~/.config/aiConstitution/.{persona,profile,skill,brand}-cache/`
   in a content-addressed layout (`<name>/<version>/`).

## Consequences

- A profile pinning `coder@1.2.0` produces identical context across
  machines, sessions, and time — drift becomes structurally impossible.
- Cross-machine reproducibility is a property of the resolver, not of
  the user's diligence.
- The binary ships **without** persona/profile/skill content. Updates
  flow through the registries, not through `git pull` on `~/.ai/`.
- Publication requires a PR against the relevant public repo —
  attribution and review are unambiguous.
- Single resolver in `bin/ai` handles all four registries; only the URL
  template and content type differ.

## Alternatives considered

- **Personas as local files (status quo before v0.5).** Rejected —
  mutation surface too open; reproducibility depends on each machine
  having the same `coder.md`.
- **Single combined registry.** Rejected — brand, persona, profile, and
  skill have different lifecycles, different schemas, different
  composition rules. Forcing one taxonomy hurts each.
- **Persona inheritance instead of profile composition.** Rejected
  (Q-O10 in spec) — inheritance adds `super()` chains; composition is
  simpler to reason about and diff.
