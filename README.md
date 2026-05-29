# aiConstitution

> A personal AI Constitution as a product — install it, answer a guided
> interview, and be governed by a personalized constitution inside
> thirty minutes.

`ai` is the Go CLI that operationalizes the unified AI Constitution
governance system (a single `Constitution.md` in `~/.ai/`). It ships
a TUI wizard, a memory-to-amendment review loop, a sync/restore flow,
a self-repairing doctor, atom-based persona/profile/skill distribution,
and a cross-tool command-wrapper facade that enforces governance
regardless of which AI tool (Claude Code, Copilot CLI, Cursor, Codex)
issued the command.

**Status:** in development. Spec at v1.0.0-draft — see [`SPEC.md`](./SPEC.md).
The binary surface is being built out from the spec.

> **Migrating from the legacy four-file layout?** If your `~/.ai/` contains
> separate `Common.md`, `Code.md`, and `Writing.md` files, run `ai migrate`
> to fold them into a single `Constitution.md`. The `ai migrate` command
> detects the legacy layout automatically.

## What it does

| Command | Purpose |
|---|---|
| `ai amend` | Amendment lifecycle: draft, apply, list, show, publish |
| `ai audit` | Record overrides and violations into ~/.ai/audit/ |
| `ai backup` | Snapshot the canonical tree to a local archive (used by migrations) |
| `ai brand` | Fetch or list brand atoms from brand-atoms.com |
| `ai clone` | Clone a repo with identity routing + post-clone hook install |
| `ai compress` | Generate compact constitution or per-persona YAML derivatives |
| `ai constitution` | Backup, restore, and bootstrap the entire ~/.ai/ directory and tool wiring |
| `ai doctor` | Detect and repair structural damage to the ~/.ai/ tree |
| `ai focus` | Alias of `ai mode` |
| `ai generate` | Generate derived artifacts from Constitution.md |
| `ai hooks` | Hook lifecycle: list, evaluate, propose, share upstream, install |
| `ai init` | Scaffold project.yaml and AI-tool integration files in the current directory |
| `ai init-integrate` | Wire AI tool integrations (Cursor, Codex/AGENTS.md) |
| `ai issue` | File hook / finding issues upstream |
| `ai memory` | Inspect and curate ~/.ai/memory/ |
| `ai migrate` | Migrate from four-file constitution to unified v2 format |
| `ai mode` | Activate a persona or profile (additive; not exclusive) |
| `ai op` | 1Password CLI integration (env, signin, signout, whoami, clip) |
| `ai persona` | Inspect persona atoms (agentic + reviewer) |
| `ai plan` | Manage work-product plans under ~/.ai/governance/plans/ |
| `ai plugins` | Manage Claude plugins that extend the agent's workflow surface |
| `ai pm-mode` | Activate PM mode (plan-first discipline) — shortcut for `ai mode pm` |
| `ai profile` | Manage profiles (compositions of atomic personas) |
| `ai restore` | Restore ~/.ai/ from a local snapshot (.tar.gz) or a remote URL |
| `ai review` | Memory-to-amendment review loop (default cadence: 30 days) |
| `ai settings` | Read or write user preferences at ~/.config/aiConstitution/settings.toml |
| `ai setup` | Run the guided constitution-setup wizard (TUI by default) |
| `ai skills` | Manage skill atoms (tarball bundles: SKILL.md + templates + assets) |
| `ai spawn` | Spawn a persona agent |
| `ai status` | Print a short status report (sync state, review cadence, doctor warnings) |
| `ai sync` | Push or pull the canonical tree to a user-owned remote |
| `ai update` | Update the binary + reconcile new hooks/skills/personas/questions |
| `ai version` | Print the binary version, Code.md version, and questions.yaml version |
| `ai worktree` | Create worktrees in the canonical locations (~/.ai/Common.md §U17) |
| `ai wrap` | Cross-platform tool wrapper (invoked by git/gh shims) |

See [`SPEC.md §3`](./SPEC.md#3-cli-surface) for the authoritative surface definition.

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

After setup, `Constitution.md` is at `~/.ai/`, the hook library is wired
into your AI tool of choice, and a `~/.config/aiConstitution/` directory
holds your per-machine mutable state (settings, mode, cache).

## Build from source

```bash
go work sync
make build           # produces dist/ai
./dist/ai version
```

Requirements:

- Go 1.26 or later (CI builds and tests on 1.26).
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
SPEC.md              authoritative implementation specification (v1.0.0-draft)
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

`ai` is itself governed by the unified constitution it operationalizes.
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
