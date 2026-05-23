# aiConstitution

> A personal AI Constitution as a product — install it, answer a guided
> interview, and be governed by a personalized constitution inside
> thirty minutes.

`ai` is the Go CLI that operationalizes the four-file AI governance
system (`Constitution.md`, `Common.md`, `Code.md`, `Writing.md`). It
ships a TUI wizard, a memory-to-amendment review loop, a sync/restore
flow, a self-repairing doctor, atom-based persona/profile/skill
distribution, and a cross-tool command-wrapper facade that enforces
governance regardless of which AI tool (Claude Code, Copilot CLI,
Cursor, Codex) issued the command.

**Status:** in development. Spec at draft v0.8 — see [`SPEC.md`](./SPEC.md).
The binary surface is being built out from the spec.

## What it does

| Command | Purpose |
|---|---|
| `ai setup` / `ai --tui` | Guided interview → personalized four-file constitution |
| `ai review` | Memory → amendment loop; default 30-day cadence |
| `ai doctor` | Detect + repair broken symlinks, missing hooks, stale binary |
| `ai sync push` / `ai sync pull` | Push/pull canonical tree to user-owned remote |
| `ai restore <url>` | Reproduce the system on a fresh machine |
| `ai mode <name>` | Activate a persona or profile (additive, not exclusive) |
| `ai profile new` | Compose a profile from atomic personas |
| `ai persona share` | File a draft as an upstream atom PR |
| `ai skills install <name>[@<ver>]` | Resolve from `skill-atoms.com`, cache, symlink |
| `ai hooks propose <name>` | Scaffold a new hook from a finding |
| `ai update --migrate` | Reconcile new hooks/personas/questions after upgrade |
| `ai settings get/set/edit` | Manage `~/.config/aiConstitution/settings.toml` |
| `ai clone <url>` | Identity-aware git clone + post-clone pre-commit secret hook install |
| `ai audit rotate` | Gzip prior-month audit JSONLs (suitable for cron) |
| `ai hooks install --all` | Extract all embedded hooks to `~/.ai/hooks/` |
| `ai hooks install command-wrappers` | Extract embedded git/gh wrappers to `~/.ai/bin/` |

See [`SPEC.md §3`](./SPEC.md#3-cli-surface) for the complete surface.

## Install

The `ai` binary is distributed through the system package manager
(per `SPEC.md §15`, `~/.ai/bin/` does NOT contain `ai`):

```bash
# macOS / Linux
brew install convergent-systems-co/tap/ai

# Windows
scoop bucket add convergent-systems-co https://github.com/convergent-systems-co/scoop-bucket
scoop install ai

# winget (Windows)
winget install convergent-systems-co.ai
```

Then run:

```bash
ai setup            # guided wizard
ai --tui            # same, explicit
```

After setup, the four canonical files are at `~/.ai/`, the hook library
is wired into your AI tool of choice, and a `~/.config/aiConstitution/`
directory holds your per-machine mutable state (settings, mode, cache).

## Build from source

```bash
go work sync
make build           # produces dist/ai
./dist/ai version
```

Requirements:

- Go 1.22 or later (toolchain matrix in CI covers 1.22 and 1.23).
- `python3` on PATH (the hook library is stdlib-only Python).
- `gh` CLI for upstream-contribution flows.

## Test / lint

```bash
make test            # go test ./... -race across workspace modules
make lint            # golangci-lint
```

## Repository layout

```
src/                 Go source
  cmd/ai/            CLI entry point (single binary: ai)
    cmd/             cobra subcommands (one per SPEC §3 verb)
    embed/           embedded assets — the canonical hook library
      hooks/         Python hook source (extracted to ~/.ai/hooks/ at install)
      wrappers/      git / gh wrapper templates (→ ~/.ai/bin/ at install)
    internal/        binary-internal packages
  internal/          workspace-internal packages
  pkg/               public packages
  plugins/           Go-loadable plugins (future)
governance/          policy json + wizard pointers + seed answers
web/ai-constitution/ Astro site (methodology + spec)
docs/adr/            MADR-format architecture decisions
SPEC.md              authoritative implementation specification (draft v0.8)
GOALS.md             G1-G7 goals, non-goals, anti-goals
ARCHITECTURE.md      navigational architecture overview
```

**One distribution unit.** Hooks, wrapper templates, and the canonical
secret-pattern set are embedded into the `ai` binary at build time
via `//go:embed` (see `src/cmd/ai/embed/`). They land on disk at
install time via `ai setup` or `ai hooks install --all`. No separate
shell scripts ship.

See [`ARCHITECTURE.md`](./ARCHITECTURE.md) for the layout-with-context
view and [`SPEC.md §15`](./SPEC.md#15-file-layout-v08) for the full
file-layout specification.

## Atoms ecosystem

`ai` resolves personas, profiles, skills, and brand identity from four
versioned, immutable atom registries:

| Registry | Hosts |
|---|---|
| `brand-atoms.com` | W3C design tokens (palettes, fonts, brand compositions) |
| `persona-atoms.com` | Agentic personas (`/agentic/`) + reviewer personas (`/reviewer/`) |
| `profile-atoms.com` | Profile compositions (TOML recipes pinning persona atom versions) |
| `skill-atoms.com` | Skill bundles (tarballs: SKILL.md + templates + assets) |

All four follow the same pattern: versioned, immutable, content-addressable,
cached locally, mutation-impossible at the published version. See
[`SPEC.md §7.9`](./SPEC.md#79-persona-atoms-and-profile-atoms--the-mutation-barrier).

## Governance

`ai` is itself governed by the four-file constitution it operationalizes.
Override and violation audit logs live at `~/.ai/audit/overrides/` and
`~/.ai/audit/violations/` in canonical UTC ISO-8601 filenames.

## Contributing

- File `hook` and `finding` issues against this repo when AI behavior
  recurs in a way that warrants enforcement.
- Persona atoms: PR against `convergent-systems-co/persona-atoms`.
- Profile atoms: PR against `convergent-systems-co/profile-atoms`.
- Skill atoms: PR against `convergent-systems-co/skill-atoms`.
- Brand atoms: PR against `convergent-systems-co/branding-library`.

The full contribution flow is documented at
`aiConstitution.convergent-systems.co/community/`.

## License

AGPL-3.0. See [`LICENSE`](./LICENSE) and [`COPYRIGHT`](./COPYRIGHT).
