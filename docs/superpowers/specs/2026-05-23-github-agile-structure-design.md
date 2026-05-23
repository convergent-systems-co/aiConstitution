# GitHub Agile Issue Hierarchy — Design

**Date:** 2026-05-23
**Status:** Approved
**Scope:** Create a 4-level agile issue hierarchy (epic → feature → story → task) from all specs in `specs/*`, using GitHub native sub-issues. Then implement starting with the aiConstitution spec.

---

## Architecture

### Label hierarchy

All items are GitHub issues. Labels describe their position in the tree:

| Label | Children | Spec source |
|---|---|---|
| `agile/epic` | features | Domain grouping |
| `agile/feature` | stories | Major spec section |
| `agile/story` | tasks | Sub-section / user narrative |
| `agile/task` | none (leaf) | Concrete implementation item |

Sub-issue relationships use GitHub's native sub-issues API (`POST /repos/:owner/:repo/issues/:n/sub_issues`), not issue body references.

### Existing open issues

Issues #10–#19 are classified and linked to the appropriate feature/story after the hierarchy is built (pass 4).

No label cleanup required: `kind/{epic,feature,story,issue}` labels do not exist in this repo. `kind/finding` and `kind/hook` remain unchanged.

---

## The 10 epics (domain-driven, priority-ordered)

| # | Epic | Spec coverage | Priority |
|---|---|---|---|
| E1 | Constitution Layer & Wizard | §3, §13 | 1 |
| E2 | Hook System | §8, §14 | 2 |
| E3 | CLI Surface & Commands | §12, §10 | 3 |
| E4 | Persona & Panel System | §4, §5 | 4 |
| E5 | Skill & Prompt System | §6, §7 | 5 |
| E6 | Plugin System | §11, plugin-spec | 6 |
| E7 | op Plugin | plugin_op-spec | 7 |
| E8 | Notification System | notification-spec | 8 |
| E9 | Memory → Amendment Lifecycle | §9 | 9 |
| E10 | Public Sites & Distribution | §16, §17, §18 | 10 |

---

## Feature breakdown

### E1 — Constitution Layer & Wizard

| Feature | Stories |
|---|---|
| F1.1 Four-file management | Load+validate `~/.ai/{Constitution,Common,Code,Writing}.md`; inject into AI tool sessions; `Constitution.local.md` override support; validate file integrity |
| F1.2 Constitution atoms | Publish/consume constitution atoms from `constitution-atoms.com`; fork/adopt canonical version; `ai atoms publish` |
| F1.3 Amendment lifecycle | `ai amend draft` (propose change); `ai amend apply` (write to file); `ai amend list/show`; amendment atom publication to `amendment-atoms.com` |
| F1.4 Wizard TUI | Question taxonomy from `questions.yaml`; Bubble Tea TUI; `settings.toml` generation; `~/.ai/` initialization; `ai update --migrate` migration wizard |

### E2 — Hook System

| Feature | Stories |
|---|---|
| F2.1 Hook registration & lifecycle | `ai hooks install/validate/list/evaluate`; hook audit logging; violation → `~/.ai/audit/violations/` |
| F2.2 Command wrapper facade | `~/.ai/bin/git` wrapper; `~/.ai/bin/gh` wrapper; `command-wrappers.toml` config; PATH wiring via `ai doctor` |
| F2.3 Core hook scripts | `secret-block.py`; `branch-guard.py`; `worktree-guard.py`; `audit.py`; `secret-precommit.py`; `no-verify-strip.py` |
| F2.4 AI tool integrations | Claude Code PreToolUse/PostToolUse/Stop hooks; Copilot wrapper facade; Cursor `.cursor/rules/`; Codex `AGENTS.md` |

### E3 — CLI Surface & Commands

| Feature | Stories |
|---|---|
| F3.1 Setup & health | `ai doctor` (full check); `ai status` (current state); `ai version` |
| F3.2 Mode & persona commands | `ai mode <name/current/clear/list>`; `ai profile <list/show/new/edit/remove>`; `ai persona <list/show>` |
| F3.3 Governance commands | `ai memory <codify/list/show/archive>`; `ai audit <list/show/rotate>`; `ai override <approve/list>` |
| F3.4 Sync & restore | `ai sync <push/pull>`; `ai backup`; `ai restore <snapshot>`; `~/.ai/` git remote management |
| F3.5 Project layer | `ai init` (project.yaml + integration files); `ai pm-mode`; `ai spawn <persona>`; `ai worktree <add/list/remove>`; `ai issue <file>` |

### E4 — Persona & Panel System

| Feature | Stories |
|---|---|
| F4.1 Agentic personas | Load/validate 12 personas from `governance/personas/agentic/`; `ai mode` activation; spawn DAG enforcement; containment (denied_paths, denied_operations) |
| F4.2 Reviewer personas | Load/validate 7 reviewer YAML; `domain` → `domains[]` schema migration (§18.2); per-review-pass invocation |
| F4.3 Panel system | Load 19 panels from `panels.defaults.json`; scoring + confidence aggregation; policy profile selection; `ai review` integration |
| F4.4 Policy atoms migration | Migrate 14 profiles to `policy-atoms.com`; migrate `panels.defaults.json` to policy atom |

### E5 — Skill & Prompt System

| Feature | Stories |
|---|---|
| F5.1 Skill management | `ai skills <list/show>`; fill `project-workspace` SKILL.md gap; validate skill ↔ prompt pairing |
| F5.2 Prompt management | Load/render 29 prompt templates; validate pairings; `ai prompts <list/show>` |
| F5.3 Atomization | `ai atoms publish` workflow for skills; `ai atoms publish` workflow for prompts; skill + prompt atom TOML shape |

### E6 — Plugin System

| Feature | Stories |
|---|---|
| F6.1 Plugin install & lifecycle | `ai plugins install <name>`; atom resolution from `plugin-atoms.com`; on-disk tarball layout; `enable/disable/status/update` |
| F6.2 Amendment Author plugin | Violation → finding → draft flow; review → apply → publish flow |
| F6.3 Hook Author plugin | Describe → write → validate → install → test flow |
| F6.4 Remaining plugins | `atom-publisher`; `review-panel`; `memory-curator` |

### E7 — op Plugin

| Feature | Stories |
|---|---|
| F7.1 Core op verbs | `ai op env <vault> <item>`; `ai op signin/signout/whoami`; `ai op ref check`; `ai op field present/clip/items by-tag` |
| F7.2 Governance integration | `op-redact.py` PreToolUse hook; op doctor check; op SKILL.md |

### E8 — Notification System

| Feature | Stories |
|---|---|
| F8.1 macOS notifications | `notify-me` wrapper (terminal-notifier); sound levels info/warn/urgent; doctor + permission check |
| F8.2 Windows notifications | BurntToast PS module; `notify-me.ps1` + `notify-me.cmd` shim; Focus Assist documentation |
| F8.3 Push fallback | ntfy integration for `--level urgent`; 1Password `op://` reference for ntfy token |
| F8.4 Agent hook integration | Claude Code Stop hook → `notify-me`; doctor verification |

### E9 — Memory → Amendment Lifecycle

| Feature | Stories |
|---|---|
| F9.1 Memory management | `ai memory codify` (finding from violation); `ai memory list/show/archive`; per-project memory storage layout |
| F9.2 Audit infrastructure | Interaction audit JSONL logging; violation file format; `ai audit list/show/rotate`; 30-day GC for interaction logs |

### E10 — Public Sites & Distribution

| Feature | Stories |
|---|---|
| F10.1 Methodology site | `aiConstitution.convergent-systems.co` Astro build; brand token integration `[email protected]` |
| F10.2 Schema migrations | Schema `$id` migration set-apps → convergent-systems (§18.1); `document-writer` deprecation → `prose-writer`/`tech-writer` (§18.3) |
| F10.3 Schema atomization | Migrate `governance/schemas/*.json` to `schema-atoms.com`; atomization workflow |

---

## Issue creation strategy

### Batched passes (avoids secondary rate limits)

```
Pass 1 — 10 epics created in sequence, numbers captured
Pass 2 — ~40 features created in sequence, linked to parent epics
Pass 3 — ~100 stories created in sequence, linked to parent features  
Pass 4 — Existing open issues #10–#19 classified and linked
```

Sub-issue wiring after each pass:
```bash
gh api repos/:owner/:repo/issues/:parent/sub_issues \
  -f sub_issue_id=<child>
```

A small `sleep 1` between API calls prevents secondary rate limit hits.

### Script approach

One idempotent bash script: `scripts/create-issues.sh`. It checks if an issue with a matching title already exists before creating (idempotent re-runs). State is tracked in a local JSON file `scripts/.issue-ids.json` that maps `slug → issue_number`.

---

## Implementation approach

After issue creation, implement in priority order against the existing Go source in `src/cmd/ai/cmd/`:

1. **§3 Constitution layer** — `src/cmd/ai/cmd/setup.go`, `src/internal/config/`
2. **§13 Wizard** — New `src/cmd/ai/internal/wizard/` package (Bubble Tea)
3. **§8 Hook system** — `src/cmd/ai/cmd/hooks.go`, `src/internal/hooks/`
4. **§12 CLI surface** — Remaining commands in `src/cmd/ai/cmd/`

Each implementation step follows the TDD protocol: failing tests first, then implementation.

---

## Out of scope

- Atom substrate implementation (Atom Spec v1.1.0, atom registries) — separate repos
- Olympus integration — peer application, separate repo
- Phase 2 multi-organization features

---

## Changelog

- 2026-05-23: Initial design, approved by Thomas Polliard
