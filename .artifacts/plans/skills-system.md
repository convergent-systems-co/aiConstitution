# Plan: ai skills list/show/validate/templates + project-workspace SKILL.md

**Issues:** #228, #229, #230, #231, #232, #233 (Epic #27 Skill & Prompt System)
**Branch:** feature/skills-system
**Worktree:** /Users/itsfwcp/workspace/convergent-system-co/aiConstitution/.worktrees/feature/skills-system
**Date:** 2026-05-24

---

## Objective

Implement the `ai skills` subcommands (list, show, validate, templates list, templates show) that operate on the local `~/.ai/skills/` directory, and write a `SKILL.md` for the existing project-workspace skill.

---

## Alternatives Table

| Alternative | Pros | Cons | Verdict |
|---|---|---|---|
| Implement all in skills.go, replacing stubs | Clean, self-contained, follows existing persona.go pattern | All tasks in one file | Chosen |
| New internal package for skill loading | Better separation | Overkill for this scope — no other consumers anticipated | Rejected |
| Parse SKILL.md frontmatter with a YAML lib | Full YAML support | Adds dependency; frontmatter is simple key: value | Rejected — parse manually |
| Use gopkg.in/yaml.v3 for frontmatter | Correct parsing | go.mod only has cobra; adding yaml.v3 for simple k:v is overkill | Rejected — regex/string scan sufficient |

---

## Scope

**Files to create:**
- `src/cmd/ai/cmd/skills_test.go` — tests for all skills subcommands (TDD Writer step)
- `~/.ai/skills/project-workspace/SKILL.md` — written as atomic step

**Files to modify:**
- `src/cmd/ai/cmd/skills.go` — replace stubs with working implementations

**Files NOT to touch:**
- src/cmd/ai/cmd/persona.go, profile.go, mode.go, restore.go, init.go, root.go, or any other cmd file

---

## Approach

### Phase A: TDD Writer (RED)
1. Write `skills_test.go` covering:
   - `skills list` reads SKILL.md files from temp dir, prints name + description aligned table
   - `skills list` prints "(no skills installed)" for empty dir
   - `skills show <name>` finds skill by exact match and prints SKILL.md content
   - `skills show <name>` prefix match (partial name)
   - `skills show <name>` error for unknown skill
   - `skills validate` reports [✓] for valid skills, [⚠] for missing SKILL.md, [⚠] for missing frontmatter field
   - `skills templates list <skill>` prints filenames from templates/ dir
   - `skills templates list <skill>` error when skill not found or no templates/
   - `skills templates show <skill> <template>` substitutes $VAR from --var flags
   - `skills templates show <skill> <template>` unresolved vars left as-is
2. All tests RED (skills.go still has stubs)

### Phase B: Atomic step — write SKILL.md (#230)
- Write `~/.ai/skills/project-workspace/SKILL.md` with specified content
- Commit in worktree

### Phase C: Coder A — list + show (#228, #229)
- Replace install/uninstall/upgrade/upgrade-all/share stubs with:
  - `list` subcommand: walks skills dir, reads SKILL.md frontmatter, prints aligned table
  - `show <name>` subcommand: finds skill by exact then prefix, cats SKILL.md
- Tests for list and show go GREEN

### Phase D: Coder B — validate + templates (#231, #232, #233)
- Add to skills.go:
  - `validate` subcommand: walks skills dir, checks SKILL.md + frontmatter validity
  - `templates` subcommand group with `list` and `show` sub-subcommands
- Tests for validate and templates go GREEN

---

## Testing Strategy

All tests use `AI_ROOT` env var to point at a temp dir with known fixture structure:
```
$AI_ROOT/skills/
  valid-skill/SKILL.md       (has name: and description: frontmatter)
  no-md-skill/               (no SKILL.md)
  templates-skill/
    SKILL.md
    templates/
      greeting.txt           (contains "Hello $NAME")
```

Tests invoke subcommands via `NewRootCmd().SetArgs(...)` pattern (same as other cmd tests would use), capturing stdout/stderr.

---

## Risk Assessment

| Risk | Mitigation |
|---|---|
| SKILL.md frontmatter varies (tabs, spaces, extra fields) | Parse conservatively: scan for `key: value` pattern, trim whitespace |
| skills dir doesn't exist | Return "(no skills installed)" gracefully, no panic |
| templates dir doesn't exist | Return clear error per spec |
| $VAR substitution in template show conflicts with shell | Perform in Go, not shell; strings.Replacer or regexp |

---

## Dependencies

- None external; only stdlib + cobra (already in go.mod)

## Backward Compatibility

- The existing install/uninstall/upgrade/upgrade-all/share stubs remain as stubs (not removed, not implemented — out of scope for this batch)
- `newSkillsCmd()` gains new subcommands added via `c.AddCommand()`
