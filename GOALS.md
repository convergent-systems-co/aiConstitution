# GOALS.md

## Project Name

aiConstitution

## Mission

Turn the four-file personal AI Constitution prose stack (`Constitution.md`,
`Common.md`, `Code.md`, `Writing.md`) into a portable, onboardable,
self-repairing product — a Go CLI plus a Bubble Tea TUI plus a public
methodology site — so that anyone, not only the original author, can install
it, run a guided interview, and be governed by a personalized constitution
within thirty minutes.

The system is designed to be **deterministic** where it can be (file
operations, schema validation, sync, restore, doctor), **model-mediated**
only where judgment is required (drafting amendment prose, proposing hook
shapes), and **always auditable** — every override, every violation, every
focus change leaves a permanent record the user can read.

See [`SPEC.md`](./SPEC.md) for the authoritative implementation
specification (currently draft v0.8). This file is the goals layer that
governs which decisions are in scope.

## Principles

- **Deterministic** — same input produces the same output. The TUI, the
  resolver, the sync, the restore, the doctor: all reproducible, all
  scriptable. The model is used for prose, never for state.
- **Auditable** — every override, violation, focus change, and upstream
  contribution leaves a typed record. The audit trail is local-only and
  is never synced raw.
- **Reproducible** — atom registries (`brand-atoms.com`,
  `persona-atoms.com`, `profile-atoms.com`, `skill-atoms.com`) are
  versioned and content-hashed; a profile pinning `coder@1.2.0` produces
  identical context across machines, sessions, and time.
- **Onboardable** — a literate adult who has never seen the prose stack
  can complete `ai setup` in under thirty minutes and end with a working
  personalized constitution.
- **Strengthen, never weaken** — every new surface inherits from the four
  canonical files. Lower-tier files MAY strengthen rules; they MUST NOT
  relax them. No new surface bypasses an existing hook or audit gate.
- **Honest about defense-in-depth** — the wrapper facade (`~/.ai/bin/git`,
  `gh`, etc.) is a defense-in-depth measure, not a security control. The
  spec is explicit about this; the wrapper's first-run message says so.

## Core Goals (G1–G7)

| ID | Goal | Definition |
|---|---|---|
| **G1** | **Onboardable** | A literate adult who has never seen this system can install it, answer a guided interview, and be running with a personalized constitution inside thirty minutes — without writing prose, without reading the existing 1500-line canon, and without the assistant fabricating rules on their behalf. |
| **G2** | **Maintainable in the loop** | The system observes its own memory layer and proposes amendments back into the constitution on a regular cadence (`ai review`, default 30 days), with the user as approver, never as scribe. |
| **G3** | **Portable** | A new laptop, a borrowed dev container, a recovered backup — `ai restore <url>` brings the full constitution and memory back, including the symlink and hook topology, with one command. |
| **G4** | **Self-repairing** | `ai doctor` detects and fixes the predictable failure modes — broken symlinks, missing hooks, dirty repo, hook misregistered, stale binary — without conversation. |
| **G5** | **Brand-coherent** | The website, the TUI chrome, and the installer all read from `brand-atoms.com` rather than re-inventing visual identity. All five Convergent Systems web properties consume `[email protected]`. |
| **G6** | **Strengthen, never weaken** | This system extends `~/.ai/Constitution.md §2.2` (strengthening only). No new surface relaxes an existing rule. No new surface bypasses an existing hook. |
| **G7** | **Hygienic across updates** | When the binary, hooks, skills, personas, or wizard taxonomy change upstream, the user is prompted to reconcile on next `ai` invocation — never silently mutated, never silently skipped. Existing user-authored hooks are re-evaluated alongside. |

## Non-Goals

- **Not a multi-tenant SaaS.** Each user's `~/.ai/` is theirs. There is no
  shared identity service, no central registry of constitutions, no
  cross-user features. Sync targets are user-owned (private GitHub, GitLab,
  S3-compatible bucket, etc.).
- **Not a replacement for the prose files.** `Constitution.md` and the
  three companion files remain authoritative. The TUI generates them and
  the review loop amends them — neither replaces them.
- **Not a model wrapper.** This is the configuration plane around whatever
  agent the user already runs (Claude Code, Copilot CLI, Cursor, Codex).
- **Not a secrets store.** Per `Common.md §4`, secrets live in OS
  keychains, vaults, or env files. The system MUST NOT add a new place
  where they could land.
- **Not exclusive mode-switching.** `ai mode --switch` is honored as a
  CLI surface, but the underlying architecture activates personas
  **additively** rather than swapping domain files. See `SPEC.md §7`.

## Anti-Goals (explicit)

- A wizard that fabricates rules.
- A sync that ships raw interaction logs
  (`audit/interactions/*.jsonl` is local-only per `Common.md §5.2`).
- A "doctor" that silently rewrites the user's prose.
- An auto-update that silently re-wires hooks. Every migration step is
  shown and approved per item, except where the user has explicitly
  enabled `update.autoMigrateApprove` in `settings.toml`.

## Out-of-Scope (this repo only)

This repository ships the CLI, the TUI, the hook library, the spec, and
the public methodology site (`aiConstitution.convergent-systems.co`). The
following are **sibling projects** that this repo references but does not
contain:

- `brand-atoms.com` — W3C design tokens. Repo:
  `convergent-systems-co/branding-library` (existing).
- `persona-atoms.com` — agentic + reviewer persona atoms. Repo:
  `convergent-systems-co/persona-atoms` (in-progress).
- `profile-atoms.com` — profile composition atoms. Repo:
  `convergent-systems-co/profile-atoms` (in-progress).
- `skill-atoms.com` — skill bundle atoms. Repo:
  `convergent-systems-co/skill-atoms` (in-progress).

The atom registries are consumed by `bin/ai` via a single resolver
implementation (only the URL template and content type differ between
them). The contracts (atom metadata schema, content-hash verification,
SemVer pinning) are defined in `SPEC.md §7.9` and `§7.10`.

## Definition of Done — v0.8

The v0.8 milestone is complete when:

1. `SPEC.md` is consistent, signed off, and matches the implementation
   surface this repo ships.
2. The Go CLI (`ai`) builds cleanly and exposes all verbs from
   `SPEC.md §3` (stubs are acceptable for v0.8; the surface is the
   contract).
3. The hook library at `hooks/` ships every hook named in
   `SPEC.md §15`, each runnable on stdlib `python3` with a `--self-check`
   mode.
4. `hooks/patterns.json` is the single canonical secret pattern source,
   consumed by `secret-block.py`, `secret-precommit.py`, the pre-sync
   scan in `bin/ai`, and the redaction pass in `ai issue file`.
5. `hooks/command-wrappers.toml` ships the canonical wrapper config
   (`git`, `gh`, opt-in `terraform`/`kubectl`).
6. The `.github/ISSUE_TEMPLATE/` directory contains the six Markdown
   templates from `SPEC.md §9.5` and `§14.3` (`epic`, `feature`, `story`,
   `task`, `hook`, `finding`).
7. The Astro site scaffold at `web/ai-constitution/` builds and links to
   the spec.
8. The repo's own CI does not use `trufflehog`
   (`SPEC.md §10.4` forbids it).

Out of scope for v0.8 (morning work, deliberately deferred):

- Persona content (`persona-atoms.com/agentic/*`,
  `persona-atoms.com/reviewer/*`).
- Profile content (`profile-atoms.com/*`).
- JSON Schemas (`settings.schema.json`, `profile.schema.json`,
  `skill-manifest.schema.json`, `atom-metadata.schema.json`).
- Live atom-registry integration (HTTP fetch + cache).
- Live brand-atoms fetch in the Astro site.

These are tracked as TODO markers in code and in the relevant section of
this repo. `SPEC.md` is the authoritative source on what they should
become.
