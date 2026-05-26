# Plan: review-panel plugin artifact (#326)

**Objective:** Create the `review-panel` installable plugin under `plugins/review-panel/` that guides users through a multi-panel review workflow using `ai review` and `ai persona` commands.

## Alternatives Table

| Alternative | Pros | Cons | Verdict |
|---|---|---|---|
| Create plugin with generic review instructions | Fast | Does not reference real CLI surface; breaks on `ai review run` which does not exist | Rejected |
| Create plugin using only verified `ai review` flags and `ai persona` subcommands | Accurate, maintainable, grounded in live CLI | Requires reading source before writing | Chosen |
| Embed review logic as a Go command | Correct behavior at runtime | Out of scope for plugin artifact; plugins are SKILL.md + manifest | Rejected |

## Scope

Files to create:
- `plugins/review-panel/manifest.yaml`
- `plugins/review-panel/SKILL.md`

Files to modify: none.

## Approach

1. Confirm actual `ai review` flags: `--check`, `--since`, `--apply`, `--dry-run`, `--pr=<n>` (confirmed from `src/cmd/ai/cmd/review.go`).
2. Confirm actual `ai persona` subcommands: `list`, `show`, `share`, `new` (confirmed from `src/cmd/ai/cmd/persona.go`).
3. Write `manifest.yaml` (name, version, description, skills list).
4. Write `SKILL.md` with frontmatter triggers and 5-step workflow referencing only verified commands.
5. Validate YAML is parseable.

## Testing Strategy

TDD skip applies: plugin artifact — no executable code, no behavior to test with unit/integration tests.

Adversarial checks:
- manifest.yaml is valid YAML
- SKILL.md frontmatter is valid YAML
- No invented commands (no `ai review run`, no `ai review panel`, no `ai persona score`)
- File count ≤ 200 lines total

## Risk Assessment

- **Risk:** SKILL references a command that doesn't exist → mitigated by reading source before writing.
- **Risk:** manifest YAML parse error → mitigated by validating with `python3 -c "import yaml; yaml.safe_load(...)"`.

## Dependencies

None — plugins directory is new, no existing structure to conflict with.

## Backward Compatibility

New directory; nothing broken.
