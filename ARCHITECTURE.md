# Architecture

This document is a navigational summary of the system architecture.
The authoritative source is [`SPEC.md`](./SPEC.md); this file links into
it rather than duplicating prose.

## At a glance

- **Deterministic core in Go** (`src/cmd/ai/`) — the `ai` CLI.
- **Bubble Tea TUI** for guided setup (`ai setup`, `ai --tui`).
- **Python hook library** (`hooks/`) — audit, secret scanning,
  branch guard, worktree guard, command-wrapper pre/post hooks.
- **Four atom registries** consumed by the CLI:
  `brand-atoms.com`, `persona-atoms.com`, `profile-atoms.com`,
  `skill-atoms.com`. Versioned, immutable, content-addressable.
- **Five Astro web properties** under the same brand, of which one
  (`aiConstitution.convergent-systems.co` — canonical
  `ai-constitution.convergent-systems.co`) lives in this repo at
  `web/ai-constitution/`.
- **Mutable-state rule** — all per-machine mutable state lives in
  `~/.config/aiConstitution/`. `~/.ai/` holds canonical governance
  content, audit records, memory, hooks, and atom references only.

## System diagram

See [`SPEC.md §2.1`](./SPEC.md#21-where-this-slots-in) for the full
mermaid topology. The summary form:

```
┌────────────────────────┐    ┌───────────────────────────────────┐
│   User-facing surfaces │    │   Convergent Systems atoms (CDN)  │
│                        │    │                                   │
│  ai <cmd>  (CLI)       │    │  brand-atoms.com    (design)      │
│  ai --tui  (Bubble Tea)│    │  persona-atoms.com  (agentic +    │
│  ai-constitution.…site │    │                     reviewer)     │
└─────────┬──────────────┘    │  profile-atoms.com  (composition) │
          │                   │  skill-atoms.com    (bundles)     │
          v                   └───────────────┬───────────────────┘
┌────────────────────────┐                    │ fetch + cache
│  ~/.ai/  (canonical;   │<───────────────────┘
│  synced via git;       │
│  ZERO mutable state)   │    ┌───────────────────────────────────┐
│                        │    │  ~/.config/aiConstitution/        │
│  Constitution.md       │    │  (machine-local; NOT synced;      │
│  Common.md / Code.md / │    │   ALL mutable state)              │
│  Writing.md            │    │                                   │
│  memory/  audit/  hooks/    │  settings.toml                    │
│  governance/profiles/  │    │  mode.json   state.json           │
│  metadata/projects.json│    │  *-drafts/   *-cache/             │
└──────────┬─────────────┘    └───────────────────────────────────┘
           │ symlinks
           v
┌──────────────────────────────────────────────────────────────────┐
│  Per-tool consumers: ~/.claude/, ~/.copilot/, .cursor/,         │
│  AGENTS.md (Codex). All read the four-file constitution via      │
│  symlinks; all wire the hooks into their native event surfaces.  │
└──────────────────────────────────────────────────────────────────┘
```

## Deterministic vs model-judgment seam

Per `GOALS.md` principle #1, every behavior is deterministic and lives
in `bin/ai` (a hook, or a config table) **except** for these three
chat-handoff windows:

| Behavior | Surface |
|---|---|
| Render TUI, capture answers, validate, persist | `bin/ai` (Bubble Tea) |
| Template-fill the four canonical files from answers | `bin/ai` (Go templates) |
| Diff memory against canonical files, classify candidates | `bin/ai` |
| **Propose amendment prose** | model (chat handoff) |
| Apply approved amendment | `bin/ai` |
| Sync push/pull, Restore, Doctor, Migration | `bin/ai` |
| Mode activation, focus-change audit event | `bin/ai` |
| **Hook proposal scaffolding (prose)** | model (chat handoff) |
| Hook upstream issue filing | `bin/ai` via `gh` |
| Pre-commit secret scan | `hooks/secret-precommit.py` |

See [`SPEC.md §2.2`](./SPEC.md#22-deterministic-vs-model-judgment-seam)
for the full table and rationale.

## Repository layout

```
.
├── src/                    Go source — all of it
│   ├── cmd/ai/             ai CLI entry point (single binary)
│   ├── internal/           internal packages
│   │   ├── config/         settings.toml load/save (TBD)
│   │   ├── paths/          ~/.ai vs ~/.config/aiConstitution (TBD)
│   │   ├── audit/          JSONL interaction log writer (TBD)
│   │   ├── state/          state.json + mode.json (TBD)
│   │   ├── hooks/          hook registration helpers (TBD)
│   │   └── atoms/          atom resolver (TBD; *-atoms.com stubs)
│   ├── pkg/                public packages
│   │   ├── patterns/       patterns.json matcher
│   │   └── version/        build-time version (ldflags)
│   └── plugins/            Go-loadable plugins (future)
├── hooks/                  Python hook library (stdlib-only)
│   ├── patterns.json       canonical secret pattern set
│   ├── audit.py            interaction logger
│   ├── secret-block.py     PreToolUse secret denier
│   ├── secret-precommit.py git pre-commit secret scanner
│   ├── branch-guard.py     protected-branch enforcer
│   ├── worktree-guard.py   §U17 placement enforcer
│   ├── no-verify-strip.py  wrapper preHook (strips --no-verify)
│   ├── destructive-*.py    gh / terraform / kubectl destructive guards
│   ├── audit-command.py    wrapper postHook (records each invocation)
│   ├── checkpoint-tick.py  30-min background HANDOFF.md tick
│   └── command-wrappers.toml  ~/.ai/bin/<cmd> wrapper config
├── bin/                    helper scripts (NO ai binary; ai lives on PATH)
│   ├── clone               identity-routing wrapper for git clone
│   ├── audit-rotate.sh     monthly JSONL rotation
│   ├── git.template        ~/.ai/bin/git wrapper template
│   └── gh.template         ~/.ai/bin/gh  wrapper template
├── governance/             repo-shipped governance content
│   ├── policy/             branch-guard.json + other policy json
│   ├── wizard/             pointer to ../../questions.yaml
│   └── seed/               wizard goldens, answers.example.yaml
├── web/                    Astro front-ends (one subdir per site)
│   └── ai-constitution/    methodology + spec site (in scope)
├── docs/
│   └── adr/                MADR-format architecture decisions
├── scripts/                project tooling (release.sh, etc.)
├── .github/                GitHub integration
│   ├── ISSUE_TEMPLATE/     epic/feature/story/task/hook/finding (MD)
│   └── workflows/          CI: build, lint, test, secret-scan
├── SPEC.md                 authoritative spec (draft v0.8)
├── questions.yaml          wizard question taxonomy (v0.8)
├── settings.toml.example   default settings.toml (v0.2)
├── GOALS.md                this project's goal layer
├── ARCHITECTURE.md         this file
├── CHANGELOG.md            Keep-a-Changelog + SemVer
└── README.md               install + getting-started
```

## Authoritative section references

When in doubt about how a subsystem should behave, the spec is the
source. The most-referenced sections:

| Topic | `SPEC.md` section |
|---|---|
| Goals (G1–G7) | §1.1 |
| CLI surface (every verb) | §3 |
| TUI wizard | §4 + `questions.yaml` |
| Memory → amendment lifecycle | §6 |
| Modes, personas, focus, profiles | §7 |
| Atoms architecture (four registries) | §7.9 + §7.10 |
| Update reconciliation | §8 |
| Hook authorship + upstream | §9 |
| Pre-commit secret scanning | §10 |
| Command Wrapper Facade | §10.5 |
| Plugins | §11 |
| Sync & Restore | §12 |
| `settings.toml` schema | §13 |
| Brand + public sites | §14 |
| File layout | §15 |
| Implementation phases | §16 |

## Architecture decisions

Individual MADR-format decisions live in [`docs/adr/`](./docs/adr/).
The decisions backfilled into this repo from `SPEC.md`:

- **ADR-0001** — Atoms architecture (versioned immutable units).
- **ADR-0002** — Command Wrapper Facade for cross-tool enforcement.
- **ADR-0003** — No trufflehog; `patterns.json` + optional gitleaks.
- **ADR-0004** — Markdown issue templates in v0.8; YAML Issue Forms
  deferred.

Add new ADRs as decisions are made. Do not edit accepted ADRs;
supersede them.

## Diagram source

Mermaid diagrams live inline in `SPEC.md` (`§2.1`). Per
[`~/.ai/Code.md §9.2`](https://github.com/convergent-systems-co/ai) all
diagrams ship as source (`.mmd` or fenced mermaid), never as opaque
binaries.
