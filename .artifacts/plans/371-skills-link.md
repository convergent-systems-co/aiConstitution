# Plan: 371 — ai skills link + Copilot wiring

## Objective
Add `ai skills link` subcommand and wire Copilot instructions symlinks on `ai skills install` / `ai skills uninstall`, plus a doctor check that warns when skills are installed but not linked.

## Scope

### Files to modify
- `src/cmd/ai/cmd/skills.go` — add `copilotInstructionsDir()`, update `runSkillsInstall`, update `runSkillsUninstall`, add `newSkillsLinkCmd()` + `runSkillsLink`, register in `newSkillsCmd()`
- `src/cmd/ai/cmd/doctor.go` — extend `checkInstalledSkills` with unlinked-skills warning
- `src/cmd/ai/cmd/skills_test.go` — TDD tests for new behavior
- `src/cmd/ai/cmd/doctor_test.go` — TDD test for doctor check

## Approach

1. Add `copilotInstructionsDir()` helper (env override → `~/.copilot/instructions/` if it exists)
2. Extend `runSkillsInstall` to also create `~/.copilot/instructions/<slug>.md → SKILL.md`
3. Extend `runSkillsUninstall` to also remove the Copilot symlink
4. Add `runSkillsLink` + `newSkillsLinkCmd()` — batch re-link all installed skills to both Claude and Copilot
5. Register `newSkillsLinkCmd()` in `newSkillsCmd()`
6. Extend `checkInstalledSkills` in doctor.go: if skills installed + Claude dir exists + any skill missing symlink → WARN

## Testing strategy
- `TestSkillsLink_LinkedToBoth` — 2 skills, both dirs present → 2 Claude + 2 Copilot links
- `TestSkillsLink_NoDirs` — neither dir exists → 0,0 output
- `TestSkillsLink_Idempotent` — second call over existing symlinks succeeds without error
- `TestCopilotWiringOnInstall` — install creates Copilot symlink
- `TestCopilotWiringOnUninstall` — uninstall removes Copilot symlink
- `TestDoctorDetectsUnlinkedSkills` — skills present, Claude dir present, no symlinks → WARN with ai skills link

## Risks
- `~/.copilot/instructions/` may not exist on all machines — mitigated by returning "" when absent
- Idempotency of `ensureSymlink` already handles re-linking (removes then re-creates)

## Backward compatibility
- All existing behavior preserved; Copilot wiring is additive and non-fatal (warns on error)
