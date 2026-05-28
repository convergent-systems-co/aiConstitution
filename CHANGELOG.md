# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

Spec version (separate cadence from binary): see
[`SPEC.md` Changelog §19](./SPEC.md#19-changelog). The spec and the
binary version independently — the binary tracks `SemVer` over the
**implemented** surface; the spec tracks decisions and architecture.

## [1.3.0] — 2026-05-28

### Added

- `ai clone` identity routing: reads `~/.config/aiConstitution/metadata/projects.json` and applies `git config user.name/email` per URL-pattern match; `--identity <name>` forces a named entry (#394)
- `ai hooks available`: lists hooks from both the embedded library and the `ai-hook` atom registry on skill-atoms.com (#397, #399)
- `ai hooks list`: per-client wiring columns — INSTALLED / CLAUDE (global|project|-) / COPILOT (#400)
- `ai hooks install --copilot`: wires `Constitution.runtime.md` into `~/.copilot/instructions/` (#400)
- `ai hooks propose`: scaffolds `.py` or `.sh` hook files from a description or `--from-violation` audit log (#389)
- `ai audit override` / `ai audit violation`: write structured markdown records to `~/.ai/audit/` (#386)
- `ai memory retire`: archives a named memory entry and removes it from MEMORY.md (#386)
- `ai issue file`: creates GitHub issues from description, body, or `--from-audit` log files (#387)
- `ai hooks share`, `ai mode share`, `ai persona share`, `ai profile share`, `ai skills share`: file upstream contribution issues on the atom registries (#387)
- `ai skills available`: deduplication hides sub-skills listed in `depends_on`; shows parent with `(+N)` count (#375, #379)
- `ai skills install`: resolves `depends_on` and installs all sub-skills automatically (#375)
- `ai skills link`: symlinks installed skills into `~/.claude/skills/` and `~/.copilot/instructions/` (#373)
- `ai setup`: skill selection step after wizard; idempotent `~/.claude/CLAUDE.md` rewrite (#368, #370, #376)
- `ai doctor`: hook wiring completeness check; `checkPersonasBlock` only warns when persona sections exist (#391, #404)
- `ai status`: unified vs 4-file constitution detection; correct wired hook count (#390, #402)
- skill-atoms: `type: "ai-hook"` added to schema; 13 governance hooks published as atoms (#55 in skill-atoms)

### Fixed

- `ai update --migrate` now calls the real migration pipeline (`runMigrateFlatten`, `runMigrateAddBehavioral`, `runMigrateGenerateRuntime`) instead of printing placeholder text (#396)
- `ai hooks install --all`: hook wiring now covers all 11 event hooks, not just 5; `readWiredHookNames` handles both group and flat settings.json formats (#401)
- `ai setup`: no longer produces misleading migration warning on fresh TTY install; creates `audit/`, `memory/`, `governance/` directories on first run (#361, #368)
- `ai doctor` and `ai status` no longer false-positive on missing `Common.md`/`Code.md`/`Writing.md` for unified-model installs (#390)
- `ai hooks list`: `__init__.py`, `_lib.py`, `test_*.py`, `*.example`, `*.toml` filtered from display (#381, #400)

### Changed

- `ai atoms` group removed; atom management delegated to the `atoms` binary from `convergent-systems/atoms` (#363)
- `ai brand`, `ai sync status`, `ai plan list/new/show` implemented (#354, #355)
- **`ai skills available` + `ai hooks available`**: now fetch from `https://ai-atoms.com/exports/catalog.json` — single CDN fetch replaces GitHub API calls (#416)
- **`ai skills install`**: fetches skill content from ai-atoms.com catalog (`system_prompt_fragment` field) instead of GitHub API (#418)
- **`ai hooks install`**: fetches hook scripts from ai-atoms.com catalog (`script` field) with embed fallback for infrastructure files; 13 hooks shipped by catalog (#419)
- **`ai hooks run <slug>`**: new portable cross-platform hook runner — settings.json entries now use `ai hooks run audit` instead of absolute `python3` paths (#411)
- **`ai constitution setup`**: new subcommand bootstrapping a personal constitution via the guided TUI wizard (#408)
- **`ai constitution restore --url <git-url>`**: restore `~/.ai/` from a personal git repo URL (#408)
- **`ai setup` TUI**: hook selection step added before skill selection; users pick individual hooks interactively (#410)
- **ai-atoms.com**: 14 governance hook atoms published (13 with Python scripts + `hook/lib`); `hook-v1.json` schema extended with `script` and `depends_on` fields

### Fixed (post-initial entry)

- `ai hooks install --claude`: purges old absolute-path entries before re-wiring so hooks don't fire twice after upgrading (#413)
- `readWiredHookNames`: recognizes portable `ai hooks run <slug>` format alongside old `python3 /abs/path` format (#414)
- `ai hooks list`: `lib.py` filtered (transition artifact); `audit-logger.py` wired to `PreToolUse` (#421)
- `ai status`: wired hook count now reflects both group and flat settings.json formats (#402)

## [Unreleased]

### Spec — v0.10: GitHub Actions trinity

- `workflow-atoms.com` (introduced v0.9) splits into three sibling
  registries (`action-atoms.com`, `workflow-atoms.com`,
  `pipeline-atoms.com`) matching how GitHub layers actions / workflows
  / pipelines. Seven atom registries total; eight Convergent Systems
  Astro sites (SPEC §14.1).
- **Canonical-identity vs consumption-form** (SPEC §7.11.1): atoms.com
  URLs are the canonical identity humans/CLI/docs use; GHA's `uses:`
  accepts only `owner/repo@ref`; the CLI translates atoms.com URLs
  to the GH-grammar form at file-write time. Each atom version
  corresponds to exactly one git tag on the backing repo;
  `ai doctor` surfaces drift.
- New CLI verbs: `ai action` and `ai pipeline` (same shape as
  `ai workflow`). All three accept the canonical atoms.com URL form
  as the install argument.
- Wizard adds Q36d/e/f/g in Phase 8 (one per layer plus the
  install-mode prompt).
- `settings.toml` v0.4: adds `actionRegistry`, `pipelineRegistry`,
  `[action]`, `[pipeline]`, `[atoms.cache.action]`,
  `[atoms.cache.pipeline]`; the v0.9 `[workflow] install` moves to
  `[atoms] install` (cross-layer setting).
- The v0.9 `atom-action` data-fetcher idea is dropped — it
  conflated runtime data-atom fetching (still useful, but a separate
  concern) with workflow-atom consumption (which can't be
  runtime-fetched because GHA parses YAML at workflow-init time).

### Changed — refactor: single-binary distribution

- **Hook library is now embedded into the `ai` binary** via `//go:embed`
  (see `src/cmd/ai/embed/`). Repo-root `hooks/` and `bin/` directories
  removed. The 15 hook files + wrapper templates + patterns.json
  + command-wrappers.toml live at `src/cmd/ai/embed/{hooks,wrappers}/`.
  Extracted onto disk at install time via `ai setup` or
  `ai hooks install --all` / `ai hooks install command-wrappers`.
- **`ai clone <url>`** subcommand replaces `bin/clone` shell script.
  Identity-routing against `metadata/projects.json` is stubbed for
  v0.8; the v0.8 implementation runs `git clone` and installs the
  pre-commit secret hook into the clone.
- **`ai audit rotate`** subcommand replaces `bin/audit-rotate.sh`.
  Same behavior (gzip prior-month JSONLs); `--dry-run` flag honored.
- **`ai hooks install`** is now the canonical surface for materializing
  the embedded library: `--all`, `<name>`, or `command-wrappers`.
  Idempotent; `--force` to overwrite.
- **`bin/ai` PATH-shim removed.** A misconfigured PATH already yields
  a clear "command not found"; the stub provided no signal beyond that.
- **`.goreleaser.yaml` ldflags fixed.** Previously targeted
  `main.version` (no such var); now stamps
  `.../internal/buildinfo.{version,commit,date}` correctly.
  `ai version` output:
  `v0.8.0  (commit abc1234, built 2026-05-23T08:00:00Z)`.

### Added

- `SPEC.md` v0.8 — authoritative implementation specification.
- `questions.yaml` v0.8 — wizard question taxonomy.
- `settings.toml.example` — canonical defaults for
  `~/.config/aiConstitution/settings.toml`.
- `GOALS.md` — G1-G7 goals, non-goals, anti-goals.
- `ARCHITECTURE.md` — navigational architecture overview, indexed to
  `SPEC.md` sections.
- `docs/adr/` — ADR-0001 through ADR-0004 backfilled from spec
  decisions:
  - **ADR-0001** Atoms architecture (versioned immutable units).
  - **ADR-0002** Command Wrapper Facade for cross-tool enforcement.
  - **ADR-0003** No trufflehog; `patterns.json` + optional gitleaks.
  - **ADR-0004** Markdown issue templates in v0.8; YAML Issue Forms
    deferred.
- `.github/ISSUE_TEMPLATE/` — six Markdown templates per
  `SPEC.md §9.5` and `§14.3`: `epic`, `feature`, `story`, `task`,
  `hook`, `finding`.
- `hooks/` — Python stdlib-only hook library:
  `patterns.json`, `audit.py`, `secret-block.py`,
  `secret-precommit.py`, `branch-guard.py`, `worktree-guard.py`,
  `no-verify-strip.py`, `destructive-gh-guard.py`,
  `destructive-terraform-guard.py`, `destructive-kubectl-guard.py`,
  `audit-command.py`, `checkpoint-tick.py`, `command-wrappers.toml`.
- `bin/` — helper scripts: `clone`, `audit-rotate.sh`,
  `git.template`, `gh.template`, `ai` (PATH-shim error stub).
- `governance/policy/branch-guard.json` — canonical protected branch
  set.
- `governance/wizard/` — pointer to `questions.yaml`.
- `governance/seed/answers.example.yaml` — wizard answer template.
- `src/cmd/ai/` — cobra-based CLI scaffold; every verb from
  `SPEC.md §3` is registered (stubs for v0.8).
- `src/internal/` — packages: `config`, `paths`, `audit`, `state`,
  `hooks`, `atoms` (all skeletal).
- `src/pkg/patterns/` — Go bindings for `patterns.json` (consumes
  the same canonical pattern set as the Python hooks).
- `src/pkg/version/` — build-time version stamping
  (`-ldflags "-X .../version.Version=…"`).
- `web/ai-constitution/` — Astro scaffold for the methodology site.
  No live brand-atoms fetch (deferred); inline brand tokens from
  `[email protected]` per `SPEC.md §14.4`.

### Changed

- `.github/workflows/secret-scan.yml` — replaced `trufflehog` with a
  diff scan against `hooks/patterns.json`. `SPEC.md §10.4` forbids
  trufflehog; the canonical CI net is gitleaks (opt-in) and the
  `patterns.json` set is authoritative for what gets blocked.

### Spec corrections (v0.8 → see `SPEC.md §19`)

- Fixed typo: `itsx own` → `its own` (header status line).
- Fixed section numbering: `§13.1–13.4` (Settings.toml Schema) and
  `§14.1–14.4` (Brand Integration) had subsections labeled with the
  previous section's number; now consistent with their parent.
