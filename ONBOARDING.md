# Onboarding — aiConstitution

## What is this?

`aiConstitution` is the **tool** half of a two-repo AI governance system. It builds the `ai` Go CLI — a binary that operationalizes the unified AI Constitution governance system (a single `Constitution.md` in `~/.ai/`). The CLI ships a TUI setup wizard, a memory-to-amendment review loop, a self-repairing doctor command, sync/restore for cross-machine portability, and an atom-based persona/profile/skill distribution layer.

The binary reads governance data from `~/.ai/` on the user's machine. It does not own or modify that data.

## The two-repo contract

| Repo | What it is | Modified by |
|---|---|---|
| `convergent-systems-co/aiConstitution` (this repo) | Go CLI binary, embedded hooks, skill templates, CI | Engineers via PRs |
| `convergent-systems-co/ai` (personal governance) | Constitution.md, memory files, audit logs | AI assistant + user |

**The contract in one sentence:** this repo builds the tool; `convergent-systems-co/ai` is the data the tool reads. Nothing in this repo should ever write to `~/.ai/` or commit governance prose.

## Prerequisites

| Dependency | Minimum version | Notes |
|---|---|---|
| Go | 1.26 | Workspace uses `go 1.26.3` in `go.work`; CI targets `1.26` |
| `gh` CLI | any recent | Required for upstream contribution flows (`ai persona share`, etc.) |
| `make` | any | Thin wrapper around `go` commands |
| `golangci-lint` | v2.12.2 | Required for `make lint`; install via `brew install golangci-lint` or the official binary |
| Python 3 | any | The embedded hook library is stdlib-only Python; must be on PATH |

## Clone and build

```bash
# Clone
git clone https://github.com/convergent-systems-co/aiConstitution.git
cd aiConstitution

# Sync workspace modules (required before first build)
go work sync

# Build — produces dist/ai
make build

# Verify
./dist/ai version
```

## Run tests

```bash
# All tests across workspace modules, with race detector
make test

# Lint
make lint

# Format
make fmt

# Tidy all workspace module go.sum files
make tidy
```

CI runs lint, test (with `-race -cover`), and build on every push and PR.

## CLI overview

The `ai` binary is structured as a Cobra command tree. Every top-level verb has its own file under `src/cmd/ai/cmd/`. The full registered surface (from `root.go`):

| Command | Purpose |
|---|---|
| `ai setup` / `ai --tui` | Guided TUI interview that generates a personalized `Constitution.md` |
| `ai review` | Memory-to-amendment loop; default 30-day cadence |
| `ai doctor` | Detect and repair broken symlinks, missing hooks, stale binary |
| `ai compress` | Compress/compact context |
| `ai sync` | Push/pull canonical `~/.ai/` tree to a user-owned remote |
| `ai restore` | Reproduce the governance system on a fresh machine |
| `ai amend` | Draft, list, show, publish, and apply governance amendments |
| `ai constitution` | Backup and restore the unified governance prose |
| `ai memory` | List, show, codify, archive, and retire memory entries |
| `ai brand` | Brand token management from `brand-atoms.com` |
| `ai mode` | Activate a persona or profile (additive, not exclusive) |
| `ai focus` | Set the active cognitive focus mode |
| `ai profile` | Compose, list, show, edit, and remove profiles |
| `ai persona` | List, show, create, and share persona atoms |
| `ai skills` | Install, list, show, validate, and manage skill bundles |
| `ai plugins` | Install, enable, disable, update, and check plugin status |
| `ai update` | Reconcile new hooks/personas/questions after an upgrade |
| `ai hooks` | Manage the embedded hook library |
| `ai settings` | Get, set, and edit `~/.config/aiConstitution/settings.toml` |
| `ai issue` | File governance-related GitHub issues |
| `ai status` | Sprint and system status |
| `ai audit` | List and show override/violation audit entries |
| `ai plan` | Scaffold a work plan document |
| `ai backup` | Backup the governance tree |
| `ai worktree` | Create and manage git worktrees at canonical locations |
| `ai clone` | Identity-aware git clone with post-clone hook install |
| `ai pm` | Activate PM discipline mode |
| `ai spawn` | Dispatch agentic sub-tasks |
| `ai init` | Initialize a repo for governance integration |
| `ai version` | Print the binary version |
| `ai generate` | Generate runtime artifacts |
| `ai migrate` | Reconcile schema changes after upgrade |
| `ai integrate` | Wire governance into an existing project |
| `ai op` | 1Password integration (clip, env, signin, signout, whoami) |

> See `SPEC.md §3` for the full intended CLI surface.

## Project structure

```
src/
  cmd/ai/            CLI entry point — single binary: ai
    cmd/             Cobra subcommands, one file per verb
    embed/           Embedded assets (extracted at install)
      hooks/         Python hook source → ~/.ai/hooks/ at setup
      wrappers/      git/gh wrapper templates → ~/.ai/bin/ at setup
    internal/        Binary-internal packages (buildinfo, etc.)
  internal/          Workspace-internal packages shared across modules
  pkg/               Public packages
governance/          Policy JSON, wizard pointers, seed answers
  schemas/           JSON Schemas validating config files
plugins/             Plugin artifact directories (manifest.yaml + SKILL.md)
questions.yaml       Setup wizard question tree
settings.toml.example  Example user settings file
web/ai-constitution/ Astro documentation site
docs/adr/            MADR-format architecture decision records
SPEC.md              Authoritative implementation specification (v1.0.0-draft)
GOALS.md             G1–G7 goals, non-goals, anti-goals
ARCHITECTURE.md      Navigational architecture overview
Makefile             Build, test, lint, fmt, tidy, clean targets
go.work              Go workspace — three modules: src/cmd/ai, src/internal, src/pkg
```

## How to contribute

### Branch model

Work in a feature branch cut from `main`. Naming convention: `feat/<issue>-<slug>`, `fix/<issue>-<slug>`, etc.

Use `ai worktree add` (or `git worktree add .worktrees/<branch>`) for isolated feature work. See `Constitution.md §U17` for canonical worktree placement rules.

### Commit convention

[Conventional Commits](https://www.conventionalcommits.org/). Required prefixes: `feat:`, `fix:`, `refactor:`, `perf:`, `docs:`, `test:`, `build:`, `ci:`, `chore:`.

- Subject line: 72 characters max, imperative mood, no trailing period.
- Body: explains *why*, wraps at 72 characters.
- One logical change per commit — no bundling a refactor and a feature.

### PR process

1. Open a GitHub issue before starting work.
2. Cut a feature branch / worktree.
3. Write a failing test first for any behavioral change (TDD). Skip only for docs-only or pure governance-prose changes.
4. Make the tests green.
5. Open a PR using the PR template.
6. CI must pass (lint + test + build) before merge.
7. Merge with a merge commit or rebase — squash merging is forbidden on non-release branches.

### CI requirements

Every PR must pass three jobs (`.github/workflows/ci.yml`):

| Job | What it runs |
|---|---|
| `lint` | `golangci-lint` across all workspace modules |
| `test` | `go test ./... -race -cover` across all workspace modules |
| `build` | `make build` — verifies the binary compiles |

### Where contributions go

- **Hook changes / governance enforcement:** PR against this repo under `src/cmd/ai/embed/hooks/`.
- **Persona atoms:** PR against `convergent-systems-co/persona-atoms`.
- **Profile atoms:** PR against `convergent-systems-co/profile-atoms`.
- **Skill atoms:** PR against `convergent-systems-co/skill-atoms`.
- **Brand tokens:** PR against `convergent-systems-co/branding-library`.

## The governance contract

This repo is itself governed by the unified constitution it operationalizes:

- `~/.ai/Constitution.md` — all governance rules (meta-rules, autonomy gates, honesty,
  secret handling, code quality, testing, commit discipline, prose quality)

The AI assistant working in this repo loads that file via `~/.claude/CLAUDE.md`.

**What this repo never does:**

- Commits user governance data (Constitution.md, memory, audit logs) — those live in `~/.ai/`, owned by `convergent-systems-co/ai`.
- Commits secrets — API keys, tokens, `.env` files, private keys.
- Writes directly to `~/.ai/` from CI or build scripts.

Override and violation audit logs for work done in this repo live at `~/.ai/audit/overrides/` and `~/.ai/audit/violations/` in UTC ISO-8601 filenames, per `Constitution.md §5`.
