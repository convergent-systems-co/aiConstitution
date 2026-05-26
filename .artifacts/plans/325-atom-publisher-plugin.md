# Plan: atom-publisher Plugin Artifact

**Issue:** #325
**Branch:** feat/325-atom-publisher-plugin
**Date:** 2026-05-26

## Objective

Create the `atom-publisher` plugin artifact under `plugins/atom-publisher/`, providing a guided workflow for drafting, verifying, and publishing governance atoms to their respective registries.

## Rationale

| Alternative | Pros | Cons | Verdict |
|---|---|---|---|
| Document the workflow as a README only | Simple | Not discoverable or executable as a skill | Rejected |
| Embed workflow in a persona YAML | Reuses existing persona machinery | Personas are identity; workflows are skills — wrong abstraction | Rejected |
| Plugin with manifest.yaml + SKILL.md | Installable, skill-discoverable, follows plugin spec §3 | Requires creating the top-level `plugins/` directory | **Chosen** |

## Scope

Files to create:
- `.artifacts/plans/325-atom-publisher-plugin.md` (this file)
- `plugins/atom-publisher/manifest.yaml`
- `plugins/atom-publisher/SKILL.md`

No executable code; TDD Writer step is skipped per `Code.md §11.8` (TDD skip: docs/plugin artifact only).

## Approach

1. Verify real `ai atoms` subcommands against `src/cmd/ai/cmd/atoms.go` — confirmed: `fetch`, `fork`, `publish`, `list`, `gc`, `verify`.
2. Write `manifest.yaml` matching the `PluginManifest` schema in `src/internal/plugins/manifest.go` (required: `name`, `version`; optional: `description`, `skills`).
3. Write `SKILL.md` with frontmatter triggers and a 5-step guided workflow referencing only verified CLI commands.
4. Validate YAML: `python3 -c "import yaml, sys; yaml.safe_load(open('plugins/atom-publisher/manifest.yaml'))"`.
5. Adversarial pass: manifest has name+version; SKILL frontmatter valid; only real subcommands referenced; ≤200 lines total.
6. Commit, push, open PR against main.

## Testing Strategy

- YAML parse validation via Python `yaml.safe_load`.
- Adversarial check: grep SKILL.md for any `ai atoms` subcommand not in the verified set {fetch, fork, publish, list, gc, verify}.

## Risk Assessment

- No executable code introduced; blast radius is documentation only.
- `plugins/` directory is new at repo root — consistent with plugin spec §3 which places artifacts under `~/.ai/plugins/<name>/` at runtime; the repo-level `plugins/<name>/` is the source of truth before installation.

## Dependencies

- None upstream.

## Backward Compatibility

- Additive only; no existing files modified.
