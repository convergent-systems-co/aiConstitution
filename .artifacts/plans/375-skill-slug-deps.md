# Plan: 375 — Skill slug column in `available` + `depends_on` bundle support

**Objective:** `ai skills available` shows SLUG as first column; skill atoms can declare `depends_on` to create install bundles.

## Rationale

| Alternative | Pros | Cons | Verdict |
|---|---|---|---|
| Add SLUG column to `available` | Makes install command obvious from the listing | Minor column-width cost | Chosen |
| Leave NAME-only header | Simpler table | Name ≠ slug can confuse install arg | Rejected |
| `depends_on` as runtime resolution | Enables bundle installs in one command | Slightly more interactive surface | Chosen |
| No dependency support | Simpler | Forces multiple manual installs for bundles | Rejected |

## Scope

Files to modify:
- `src/cmd/ai/cmd/skills.go` — struct, runSkillsAvailable, runSkillsInstall
- `src/cmd/ai/cmd/skills_test.go` — 3 new tests
- `specs/aiConstitution-spec-v1.0.0-draft.md` — add slug + depends_on documentation

## Approach

1. Add `DependsOn []string` to `skillAtom` struct.
2. Update `runSkillsAvailable` row struct to include `slug`; update header and row format.
3. Update `runSkillsInstall` to check `atom.DependsOn` after install and prompt/auto-install deps.
4. Update `fakeSkillAtom` helper in tests (add `depends_on` to payload builder).
5. Add 3 tests: `TestSkillsAvailable_ShowsSlugColumn`, `TestSkillsInstall_InstallsDependencies`, `TestSkillsInstall_NoDependencies`.
6. Update SPEC (§6 skill section) to document slug and depends_on.

## Testing strategy

- All existing tests must remain green (no behavioral change for existing code paths).
- New tests use the existing httptest pattern already established in skills_test.go.
- `TestSkillsInstall_InstallsDependencies`: non-TTY path; serve two atoms; verify both SKILL.md files written.
- `TestSkillsAvailable_ShowsSlugColumn`: verify "SLUG" appears as first word of header line.

## Risk assessment

- `runSkillsInstall` is called recursively for deps; the slug@version strip at the top handles it correctly.
- Non-TTY auto-install errors are non-fatal (warnings only) — consistent with existing install --all pattern.

## Dependencies

None — all dependencies are already in go.mod.

## Backward compatibility

No breaking changes. `DependsOn` is `omitempty`; atoms without the field decode cleanly.
