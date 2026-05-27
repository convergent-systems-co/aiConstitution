# Plan: 347 — ai skills install/uninstall/upgrade/upgrade-all

**Issue:** #347 (V1-blocking)
**Branch:** feat/347-skills-mgmt
**Objective:** Replace the four `stub(...)` returns in `skills.go` with working implementations for `install`, `uninstall`, `upgrade`, and `upgrade-all`.

---

## Rationale

| Alternative | Pros | Cons | Verdict |
|---|---|---|---|
| Fetch SKILL.md tar.gz from skill-atoms.com CDN | Consistent with plugins pattern | CDN not yet live; tar.gz format not defined for skills | Rejected |
| Fetch atom JSON from GitHub API (skill-atoms repo) | GitHub API is live; raw JSON has `system_prompt_fragment` | Adds GitHub API dependency; requires Accept header | Chosen for v1 |
| Bundle skill atoms inside the binary | Zero network dependency | Stale on every release; not scalable | Rejected |

Do nothing is not acceptable — issue #347 is V1-blocking.

---

## Scope

**Files modified:**
- `src/cmd/ai/cmd/skills.go` — replace 4 stubs with working subcommands
- `src/cmd/ai/cmd/skills_test.go` — add 6 new tests covering install/uninstall/upgrade/upgrade-all

**Files NOT modified:** `plugins.go`, `atoms.go`, `go.mod`, `go.sum`

---

## Approach

### Data model

Skill atom JSON (from `https://api.github.com/repos/convergent-systems-co/skill-atoms/contents/skills/skill/<slug>.json` with `Accept: application/vnd.github.raw+json`) has at minimum:
- `id` (string, e.g. `"skill/commit"`)
- `version` (string, e.g. `"1.0.0"`)
- `name` (string)
- `description` (string)
- `system_prompt_fragment` (string)

### SKILL.md format written on install

```
---
name: <slug>
description: <description>
version: <version>
user-invocable: true
allowed-tools:
  - Bash
  - Read
---
# <name>

<system_prompt_fragment>
```

The `version` field in frontmatter is key — it enables upgrade to detect the currently installed version.

### HTTP helper

Extract a `fetchSkillAtomJSON(slug string) (*skillAtom, error)` function that:
1. Builds the GitHub API URL.
2. Issues a GET with `Accept: application/vnd.github.raw+json`.
3. Decodes the JSON body into `skillAtom`.
4. Returns a clear error on non-200 (including 404 → "skill not found").

The HTTP client is injected via a package-level `var skillHTTPClient *http.Client = http.DefaultClient` to allow test replacement without a real server.

### `skills install <name>[@version]`

1. Split `name@version` if `@` present (version ignored for v1 — always fetches latest from API).
2. Call `fetchSkillAtomJSON(slug)`.
3. `os.MkdirAll(~/.ai/skills/<slug>/, 0o750)`.
4. Write SKILL.md.
5. Symlink `~/.claude/skills/<slug>` → `~/.ai/skills/<slug>` if `~/.claude/skills/` exists.
6. Print `Installed <slug> v<version>`.

### `skills uninstall <name>`

1. Locate skill dir (`findSkillDir`); error if not found.
2. `os.RemoveAll` the skill dir.
3. Remove `~/.claude/skills/<name>` symlink if it exists.
4. Print `Uninstalled <name>`.

### `skills upgrade <name> [<version>]`

1. Locate skill dir; error if not installed.
2. Read current version from `parseFrontmatter` on SKILL.md.
3. Call `fetchSkillAtomJSON(slug)`.
4. Overwrite SKILL.md.
5. Re-create symlink if it exists.
6. Print `Upgraded <name> from v<old> to v<new>` (or `already up-to-date` if same).

### `skills upgrade-all`

1. `listSkillDirs(skillsDir)` — return nil on empty.
2. For each dir: call `runSkillsUpgrade(cmd, skillsDir, claudeSkillsDir, slug, "")`.
3. Print `Upgraded N skill(s)` or `All skills up-to-date`.

---

## Testing strategy

The HTTP client is package-level `var skillHTTPClient`, replaced in tests with an `httptest.NewServer` that serves fake JSON. Tests use `AI_ROOT` env var (existing pattern) to redirect the skills dir.

Tests:
- `TestSkillsInstall_Success` — mock server returns valid atom JSON → SKILL.md written, symlink NOT created (no ~/.claude/skills/ in temp dir)
- `TestSkillsInstall_NotFound` — mock server returns 404 → clear error mentioning skill name
- `TestSkillsUninstall_Success` — pre-populated skill dir and symlink → both removed
- `TestSkillsUninstall_NotInstalled` — missing dir → error
- `TestSkillsUpgrade_Success` — pre-installed skill + mock server with new version → SKILL.md updated, output mentions old and new version
- `TestSkillsUpgradeAll_Empty` — empty skills dir → exits 0 with "no skills installed" or summary

---

## Risk assessment

| Risk | Mitigation |
|---|---|
| GitHub API rate limits (60 req/hr unauthenticated) | Acceptable for CLI v1; future work can add GITHUB_TOKEN support |
| skill-atoms repo slug not matching user input | Return clear "skill not found" on 404 |
| Symlink races or permission issues on ~/.claude/skills/ | Wrap in error check; log warning if symlink fails, do not abort |
| SKILL.md version field format variations | Normalize to string; unknown = "(unknown)" |

---

## Dependencies

None. Reuses existing `http.DefaultClient`, `parseFrontmatter`, `findSkillDir`, and `listSkillDirs` already in `skills.go`.

## Backward compatibility

Existing `skills list/show/validate/templates` commands unchanged. The four stubs are replaced — no callers depend on the stub error return.
