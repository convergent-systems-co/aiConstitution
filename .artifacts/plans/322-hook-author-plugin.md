# Plan: hook-author Plugin Artifact (#322)

**Objective:** Create the `hook-author` plugin under `plugins/hook-author/` — a guided workflow skill that walks users through the full lifecycle of authoring, validating, installing, and testing a governance hook.

## Rationale / Alternatives Table

| Alternative | Pros | Cons | Verdict |
|---|---|---|---|
| Inline skill in existing plugin | Less files | Mixes concerns, harder to install independently | Rejected |
| Standalone plugin under plugins/ | Clean install surface, composable, matches plugin convention | Requires two files | Chosen |
| README-only docs | Simple | Not machine-invocable as a skill | Rejected |

## Scope

Files to create:
- `plugins/hook-author/manifest.yaml` — plugin metadata
- `plugins/hook-author/SKILL.md` — guided 5-step workflow skill

Files to NOT create or modify — no changes to `src/`, no Go code.

## Approach

1. Confirm actual `ai hooks` subcommands from `hooks.go` — only reference confirmed ones.
2. Create `plugins/hook-author/manifest.yaml` with required `name`, `version`, `description`, `skills` fields.
3. Create `plugins/hook-author/SKILL.md` with YAML frontmatter and the 5-step workflow.
4. YAML-lint both files.
5. Adversarial pass: verify all referenced subcommands exist, no placeholder values, frontmatter is valid.
6. Commit in worktree, push, open PR.

## Confirmed `ai hooks` subcommands

From `hooks.go` grep:
- `list`
- `validate`
- `evaluate`
- `propose`
- `share`
- `install`

The workflow references `validate`, `install`, and `list` — all confirmed present.

## Testing Strategy

TDD skip applies — plugin artifact files contain no executable code. Validation is:
- `python3 -c "import yaml; yaml.safe_load(open(...))"` for both files
- Manual adversarial review of subcommand references

## Risk Assessment

| Risk | Mitigation |
|---|---|
| Referencing a non-existent subcommand | Cross-checked against hooks.go before writing |
| Invalid YAML frontmatter | python3 yaml.safe_load validation before commit |
| Skill file exceeds 200-line limit | Count lines before commit |

## Dependencies

None — standalone artifact, no code changes.

## Backward Compatibility

New files only — no existing behavior affected.
