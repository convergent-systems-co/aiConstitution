# Plan: amendment-author Plugin Artifact (#321)

## Objective
Create an installable `amendment-author` plugin under `plugins/amendment-author/` that guides users through the full amendment lifecycle using existing `ai amend` CLI subcommands.

## Rationale

| Alternative | Pros | Cons | Verdict |
|---|---|---|---|
| Inline skill in `~/.ai/skills/` | No new directory convention | Not installable/distributable; not co-located with repo governance tooling | Rejected |
| Plugin artifact in `plugins/` | Installable via `ai plugins install`; versioned; co-located with repo | New `plugins/` directory (no precedent in repo yet) | Chosen |
| Do nothing | Zero work | Lifecycle guidance lives only in docs, not as an executable workflow | Rejected |

## Scope
Files to create (both in worktree, under `plugins/amendment-author/`):
- `manifest.yaml` — plugin metadata: name, version, description, skills list
- `SKILL.md` — complete skill definition with YAML frontmatter and 5-step workflow

No executable code. TDD phase skipped per `Code.md §11.8` narrow skip rule (plugin artifact files only).

## Approach
1. Create `plugins/amendment-author/manifest.yaml` with required fields
2. Create `plugins/amendment-author/SKILL.md` with YAML frontmatter + workflow
3. Validate YAML with `python3 -c "import yaml, sys; yaml.safe_load(...)"` for both files
4. Adversarial pass: verify all `ai amend` subcommands referenced exist in `src/cmd/ai/cmd/amend.go`
5. Commit, push, open PR

## Testing Strategy
- YAML parse validation for both files (python3 pyyaml)
- Manual review that all `ai amend` subcommand names match those registered in `amend.go`
- No unit tests (docs-only artifact)

## Risk Assessment
- Risk: YAML frontmatter in SKILL.md is malformed → mitigation: parse-validate before commit
- Risk: referenced `ai amend` subcommands don't exist → mitigation: verified against `amend.go` (draft, apply, list, show, publish all confirmed present)

## Dependencies
- Worktree on `feat/321-amendment-author-plugin` off `main`
- `src/cmd/ai/cmd/amend.go` already implements all five subcommands

## Backward Compatibility
Additive only. New directory `plugins/` created. No existing files modified.
