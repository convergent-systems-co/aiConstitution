# Plan: memory-curator Plugin Artifact (#327)

**Objective:** Create the `memory-curator` plugin artifact under `plugins/memory-curator/` — a guided workflow for reviewing, curating, and archiving AI memory entries, with optional governance amendment proposal.

## Rationale

| Alternative | Pros | Cons | Verdict |
|---|---|---|---|
| Plugin artifact (SKILL.md + manifest.yaml) | Matches established plugin pattern; installable; no runtime code required | None | Chosen |
| Standalone script | Executable, automatable | Diverges from plugin convention; no integration with `ai plugins install` | Rejected |
| Do nothing | Zero effort | Pattern for memory hygiene absent from toolchain | Rejected |

## Scope

Files to create:
- `plugins/memory-curator/manifest.yaml`
- `plugins/memory-curator/SKILL.md`

No files modified. No existing files deleted.

## Approach

1. Create `plugins/memory-curator/` directory structure in the worktree.
2. Write `manifest.yaml` with required fields: `name`, `version`, `description`, `skills`.
3. Write `SKILL.md` with frontmatter triggers and 5-step workflow (review → identify stale → identify patterns → propose amendments → archive/retire).
4. Validate YAML with `python3 -c "import yaml, sys; yaml.safe_load(...)"`.
5. Adversarial check: `name`+`version` present, SKILL frontmatter valid, only real CLI subcommands referenced, ≤200 lines.

## Testing Strategy

TDD skip: plugin artifact — no executable code. Validation is YAML parse correctness + adversarial checklist.

## Risk Assessment

- Risk: SKILL references a non-existent CLI subcommand → Mitigation: cross-check each subcommand against the verified list in the task spec.
- Risk: YAML syntax error → Mitigation: validate with python3 yaml.safe_load before commit.

## Dependencies

None. `plugins/` directory does not exist yet; it will be created as part of this task.

## Backward Compatibility

No existing code modified. Additive only.
