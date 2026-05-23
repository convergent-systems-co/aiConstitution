# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

Spec version (separate cadence from binary): see
[`SPEC.md` Changelog §19](./SPEC.md#19-changelog). The spec and the
binary version independently — the binary tracks `SemVer` over the
**implemented** surface; the spec tracks decisions and architecture.

## [Unreleased]

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
