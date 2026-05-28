# Plan: Fix checkPersonasBlock false positive (#403)

## Objective
`checkPersonasBlock` should only warn about a missing personas block in CLAUDE.md when
`Constitution.md` actually has persona sections to wire. If there are no persona sections,
the warning is a false positive and must be silenced.

## Alternatives Table

| Approach | Notes |
|---|---|
| **Check Constitution.md first (chosen)** | Read Constitution.md, parse sections; only warn if sections exist and block is missing. Consistent with `checkDerivativeFiles` pattern. |
| Always suppress the warning | Would hide a real misconfiguration when sections do exist. Rejected. |
| Move check to a post-compress hook | Adds complexity for no gain. Rejected. |

## Scope

Files to modify:
- `src/cmd/ai/cmd/doctor.go` — replace `checkPersonasBlock` body
- `src/cmd/ai/cmd/doctor_test.go` — add three new test cases
- `src/cmd/ai/cmd/export_test.go` — export `CheckPersonasBlockForTest`

## Approach

1. Read `Constitution.md` from `paths.AIRoot()`.
2. If unreadable, return silently (can't evaluate).
3. Call `constitution.ParseSections()`.
4. If no sections, return silently (nothing to wire → not a problem).
5. Otherwise, check CLAUDE.md for `<!-- ai:personas`.
6. Emit `[⚠]` or `[✓]` accordingly.

## Testing Strategy

- `TestCheckPersonasBlock_NoSections` — no persona sections → no warning
- `TestCheckPersonasBlock_SectionsNoBlock` — sections exist, block missing → warning
- `TestCheckPersonasBlock_SectionsWithBlock` — sections exist, block present → OK

Use `AI_ROOT` env var (honored by `paths.AIRoot()`) and `HOME` env var for CLAUDE.md path.

## Risk Assessment

- Low risk: change is additive (adds a guard condition). Existing behavior for the
  "sections present + block missing/present" cases is preserved.
- The only behavioral change: no warning when Constitution.md has no persona sections.

## Dependencies

None — `constitution.ParseSections` is already imported in doctor.go.

## Backward Compatibility

Users whose Constitution.md has no persona sections will no longer see a spurious warning.
Users with persona sections see the same behavior as before.
