# GitHub Agile Issue Hierarchy — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` (recommended) or `superpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Create the full 10-epic agile hierarchy (epic → feature → story) in GitHub using native sub-issues, in three batched passes to avoid rate limits.

**Architecture:** A single idempotent bash script `scripts/create-issues.sh` reads all issue definitions from inline arrays, creates missing issues, and wires sub-issue parent-child relationships via the GitHub REST API. A local state file `scripts/.issue-map.json` tracks title → issue number for idempotency.

**Tech Stack:** Bash 5+, `gh` CLI ≥ 2.30, `jq` ≥ 1.6, GitHub REST API sub-issues endpoint.

---

## File structure

| Path | Action | Responsibility |
|---|---|---|
| `scripts/create-issues.sh` | **CREATE** | Idempotent issue creation + sub-issue wiring |
| `scripts/.issue-map.json` | **CREATE** (at runtime) | title → issue_number state; gitignored |
| `.gitignore` | **MODIFY** | Add `scripts/.issue-map.json` |

---

## Task 1: Bootstrap helpers and .gitignore

**Files:**
- Modify: `.gitignore`

- [ ] **Step 1: Add state file to .gitignore**

```bash
cd /Users/itsfwcp/workspace/convergent-system-co/aiConstitution
echo 'scripts/.issue-map.json' >> .gitignore
```

- [ ] **Step 2: Verify tooling**

```bash
gh --version         # must be ≥ 2.30
jq --version         # must be ≥ 1.6
gh api user --jq .login   # must succeed (proves auth)
```

Expected: three lines of output, no errors. If `gh api user` fails: run `gh auth login` first.

- [ ] **Step 3: Commit .gitignore**

```bash
git add .gitignore
git commit -m "chore: gitignore issue-map state file"
```

---

## Task 2: Write create-issues.sh — core helpers

**Files:**
- Create: `scripts/create-issues.sh`

- [ ] **Step 1: Create the file with shebang, safety flags, and helpers**

```bash
cat > /Users/itsfwcp/workspace/convergent-system-co/aiConstitution/scripts/create-issues.sh << 'SCRIPT'
#!/usr/bin/env bash
# create-issues.sh — idempotent agile issue hierarchy for aiConstitution
# Usage: ./scripts/create-issues.sh [--dry-run]
set -euo pipefail

REPO="convergent-systems-co/aiConstitution"
MAP_FILE="$(dirname "$0")/.issue-map.json"
DRY_RUN="${1:-}"

# Initialise state file
if [[ ! -f "$MAP_FILE" ]]; then
  echo '{}' > "$MAP_FILE"
fi

# Returns the issue number for a title, or empty string if not found.
# Checks the local map first; falls back to a gh search.
issue_number() {
  local title="$1"
  local cached
  cached=$(jq -r --arg t "$title" '.[$t] // empty' "$MAP_FILE")
  if [[ -n "$cached" ]]; then echo "$cached"; return; fi

  local found
  found=$(gh issue list --repo "$REPO" --search "\"$title\" in:title" \
    --json number,title --jq \
    --arg t "$title" '.[] | select(.title == $t) | .number' 2>/dev/null | head -1)
  if [[ -n "$found" ]]; then
    jq --arg t "$title" --argjson n "$found" '. + {($t): $n}' "$MAP_FILE" > "${MAP_FILE}.tmp"
    mv "${MAP_FILE}.tmp" "$MAP_FILE"
    echo "$found"
  fi
}

# Creates an issue (or skips if already exists). Prints the issue number.
create_issue() {
  local title="$1"
  local body="$2"
  local labels="$3"

  local existing
  existing=$(issue_number "$title")
  if [[ -n "$existing" ]]; then
    echo "  skip (exists #${existing}): $title" >&2
    echo "$existing"
    return
  fi

  if [[ "$DRY_RUN" == "--dry-run" ]]; then
    echo "  DRY-RUN: would create: $title" >&2
    echo "0"
    return
  fi

  sleep 0.8   # secondary rate-limit guard

  local num
  num=$(gh issue create --repo "$REPO" \
    --title "$title" \
    --body "$body" \
    --label "$labels" \
    --json number --jq '.number')

  jq --arg t "$title" --argjson n "$num" '. + {($t): $n}' "$MAP_FILE" > "${MAP_FILE}.tmp"
  mv "${MAP_FILE}.tmp" "$MAP_FILE"
  echo "  created #${num}: $title" >&2
  echo "$num"
}

# Wires a child issue as a sub-issue of a parent.
add_sub_issue() {
  local parent="$1"
  local child="$2"
  [[ "$DRY_RUN" == "--dry-run" ]] && return
  [[ "$parent" == "0" || "$child" == "0" ]] && return
  sleep 0.5
  gh api "repos/$REPO/issues/$parent/sub_issues" \
    -X POST -f sub_issue_id="$child" --silent 2>/dev/null || true
}
SCRIPT
chmod +x /Users/itsfwcp/workspace/convergent-system-co/aiConstitution/scripts/create-issues.sh
```

- [ ] **Step 2: Verify the helpers parse cleanly**

```bash
bash -n /Users/itsfwcp/workspace/convergent-system-co/aiConstitution/scripts/create-issues.sh
echo "syntax OK"
```

Expected: `syntax OK`.

---

## Task 3: Add epic definitions to the script

**Files:**
- Modify: `scripts/create-issues.sh`

- [ ] **Step 1: Append the epic array and creation loop**

Append to `scripts/create-issues.sh`:

```bash
cat >> /Users/itsfwcp/workspace/convergent-system-co/aiConstitution/scripts/create-issues.sh << 'EPICS'

# ─── PASS 1: EPICS ──────────────────────────────────────────────────────────
echo "=== Pass 1: Epics ==="

declare -a EPIC_TITLES=(
  "[Epic] Constitution Layer & Wizard"
  "[Epic] Hook System"
  "[Epic] CLI Surface & Commands"
  "[Epic] Persona & Panel System"
  "[Epic] Skill & Prompt System"
  "[Epic] Plugin System"
  "[Epic] op Plugin (1Password Integration)"
  "[Epic] Notification System"
  "[Epic] Memory → Amendment Lifecycle"
  "[Epic] Public Sites & Distribution"
)

declare -a EPIC_BODIES=(
  "Spec §3, §13. The four-file governance layer (~/.ai/{Constitution,Common,Code,Writing}.md), constitution atoms, amendment lifecycle, and Bubble Tea wizard (ai setup)."
  "Spec §8, §14. Deterministic hook enforcement: PreToolUse/PostToolUse/Stop hooks, command wrapper facade (~/.ai/bin/), core enforcement scripts, and AI tool integrations (Claude Code, Copilot, Cursor, Codex)."
  "Spec §12, §10. The full ai binary command surface: setup/health (doctor, status), mode/persona, governance (memory, audit, override), sync/restore, and project layer (init, pm-mode, spawn, worktree)."
  "Spec §4, §5. 12 agentic personas, 7 reviewer personas, 19 panels, 14 policy profiles, and the confidence-aggregation model. Governs how AI agents reason and are reviewed."
  "Spec §6, §7. 16 skills (slash-command bounded tasks), 29 prompt templates, skill↔prompt pairing, and the atomization workflow to skill-atoms.com and prompt-atoms.com."
  "Spec §11, plugin-spec. ai plugins install/enable/disable lifecycle, plugin atom distribution from plugin-atoms.com, and the five plugin candidates (amendment-author, hook-author, atom-publisher, review-panel, memory-curator)."
  "plugin_op-spec. 1Password CLI wrapper: ai op env/signin/signout/ref-check/clip verbs, Common.md §4 redaction hook (op-redact.py), doctor integration, and SKILL.md."
  "notification-spec. Uniform notify-me interface: macOS (terminal-notifier), Windows (BurntToast), ntfy push fallback for urgent level, and Claude Code Stop hook integration."
  "Spec §9. ai memory codify/list/show/archive, interaction audit JSONL logging, violation file format, ai audit list/show/rotate, 30-day GC for interaction logs."
  "Spec §16–§18. aiConstitution.convergent-systems.co Astro site, brand-atoms.com token integration, schema \$id migration (set-apps → convergent-systems), document-writer deprecation, schema atomization."
)

declare -A EPIC_NUMS

for i in "${!EPIC_TITLES[@]}"; do
  num=$(create_issue "${EPIC_TITLES[$i]}" "${EPIC_BODIES[$i]}" "agile/epic,status/triage")
  EPIC_NUMS["E$((i+1))"]="$num"
done

echo "Epic numbers:"
for k in "${!EPIC_NUMS[@]}"; do echo "  $k = ${EPIC_NUMS[$k]}"; done
EPICS
```

- [ ] **Step 2: Test dry-run for epics**

```bash
cd /Users/itsfwcp/workspace/convergent-system-co/aiConstitution
bash scripts/create-issues.sh --dry-run 2>&1 | head -20
```

Expected: 10 lines like `DRY-RUN: would create: [Epic] ...`.

---

## Task 4: Add feature definitions to the script

**Files:**
- Modify: `scripts/create-issues.sh`

- [ ] **Step 1: Append feature array and creation loop**

Append to `scripts/create-issues.sh`:

```bash
cat >> /Users/itsfwcp/workspace/convergent-system-co/aiConstitution/scripts/create-issues.sh << 'FEATURES'

# ─── PASS 2: FEATURES ───────────────────────────────────────────────────────
echo "=== Pass 2: Features ==="

# Format: "TITLE|||PARENT_KEY|||BODY"
declare -a FEATURE_DEFS=(
  # E1 — Constitution Layer & Wizard
  "[Feature] Four-file management (load, validate, inject ~/.ai/)|E1|Load and validate Constitution/Common/Code/Writing.md at CLI startup; inject into AI tool system prompts; Constitution.local.md override support. Spec §3.1–§3.3."
  "[Feature] Constitution atoms (publish/consume)|E1|Consume canonical constitution from constitution-atoms.com; fork/adopt; publish local files as constitution atoms. Spec §3.4."
  "[Feature] Amendment lifecycle (ai amend)|E1|ai amend draft (propose change with diff); ai amend apply (write to file); ai amend list/show (history); amendment atom publication to amendment-atoms.com. Spec §3.5."
  "[Feature] Wizard TUI (ai setup)|E1|Bubble Tea TUI driven by questions.yaml taxonomy; settings.toml generation; ~/.ai/ initialization; migration wizard (ai update --migrate). Spec §13."
  # E2 — Hook System
  "[Feature] Hook registration and lifecycle (ai hooks)|E2|ai hooks install/validate/list/evaluate; hook audit logging to ~/.ai/audit/violations/. Spec §8.3."
  "[Feature] Command wrapper facade (~/.ai/bin/)|E2|~/.ai/bin/git and ~/.ai/bin/gh wrappers; command-wrappers.toml config; PATH wiring so wrappers intercept commands regardless of AI tool. Spec §8.2."
  "[Feature] Core hook scripts (enforcement plane)|E2|secret-block.py, branch-guard.py, worktree-guard.py, audit.py, secret-precommit.py, no-verify-strip.py. Each is a standalone enforcement script. Spec §8.1."
  "[Feature] AI tool integrations (hooks)|E2|Claude Code PreToolUse/PostToolUse/Stop wiring; Copilot wrapper-facade-only path; Cursor .cursor/rules/; Codex AGENTS.md. Spec §14."
  # E3 — CLI Surface
  "[Feature] Setup and health commands|E3|ai doctor (10-point health check with --fix); ai status (current state summary); ai version. Spec §12.1."
  "[Feature] Mode and persona commands|E3|ai mode <name/current/clear/list>; ai profile <list/show/new/edit/remove>; ai persona <list/show>. Spec §12.1."
  "[Feature] Governance commands|E3|ai memory <codify/list/show/archive>; ai audit <list/show/rotate>; ai override <approve/list>. Spec §12.1."
  "[Feature] Sync and restore commands|E3|ai sync <push/pull> (git-backed); ai backup (dated ~/.ai/ snapshot); ai restore <snapshot>. Spec §12.1, §15."
  "[Feature] Project layer commands|E3|ai init (project.yaml + integration files); ai pm-mode; ai spawn <persona>; ai worktree <add/list/remove>; ai issue <file>. Spec §12.1, §10."
  # E4 — Persona & Panel
  "[Feature] Agentic persona loading and activation|E4|Load/validate 12 personas from governance/personas/agentic/; ai mode activation; spawn DAG enforcement; containment (denied_paths, denied_operations). Spec §4.1–§4.5."
  "[Feature] Reviewer persona system|E4|Load/validate 7 reviewer YAML files; domain → domains[] schema migration (§18.2); per-review-pass invocation. Spec §4.3."
  "[Feature] Panel system (19 panels)|E4|Load panels.defaults.json; scoring + confidence aggregation model; policy profile selection; ai review integration. Spec §5."
  "[Feature] Policy atoms migration|E4|Migrate 14 policy profiles and panels.defaults.json to policy-atoms.com as versioned atoms. Spec §5.5."
  # E5 — Skill & Prompt
  "[Feature] Skill management (ai skills)|E5|ai skills list/show; fill project-workspace SKILL.md gap; validate skill↔prompt pairings. Spec §6."
  "[Feature] Prompt management (ai prompts)|E5|Load/render 29 prompt templates; validate pairings; ai prompts list/show. Spec §7."
  "[Feature] Skill and prompt atomization|E5|ai atoms publish workflow for skills and prompts; skill + prompt atom TOML shape. Spec §6.4, §7.3."
  # E6 — Plugin System
  "[Feature] Plugin install and lifecycle|E6|ai plugins install <name> from plugin-atoms.com; tarball unpack and symlink; enable/disable/status/update. plugin-spec §3–§5."
  "[Feature] Amendment Author plugin|E6|Guided workflow: violation → finding → draft → review → apply → publish. plugin-spec §2."
  "[Feature] Hook Author plugin|E6|Guided workflow: describe behavior → write hook → validate → install → test. plugin-spec §2."
  "[Feature] Remaining plugins (atom-publisher, review-panel, memory-curator)|E6|Three remaining plugin candidates from Spec §11.2. Each is a multi-step guided workflow."
  # E7 — op Plugin
  "[Feature] Core op verbs (ai op)|E7|ai op env/signin/signout/whoami/ref-check/items-by-tag/field-present/clip. plugin_op-spec §3."
  "[Feature] op plugin governance (redaction, doctor, SKILL.md)|E7|op-redact.py PreToolUse hook; ai doctor op check; SKILL.md for /op invocation. plugin_op-spec §4–§6."
  # E8 — Notification
  "[Feature] macOS notifications (terminal-notifier)|E8|notify-me wrapper; info/warn/urgent sound levels; doctor + System Settings permission check. notification-spec §3.1."
  "[Feature] Windows notifications (BurntToast)|E8|BurntToast PowerShell module; notify-me.ps1 + notify-me.cmd shim; Focus Assist documentation. notification-spec §3.2."
  "[Feature] Push fallback (ntfy)|E8|ntfy integration for --level urgent; 1Password op:// reference for ntfy token; no plaintext token on disk. notification-spec §4."
  "[Feature] Agent hook integration (notify-me)|E8|Claude Code Stop hook calls notify-me; doctor verification that notify-me is functional. notification-spec §5."
  # E9 — Memory → Amendment
  "[Feature] Memory management (ai memory)|E9|ai memory codify (structure a finding from a violation); ai memory list/show/archive; per-project memory storage layout. Spec §9.3."
  "[Feature] Audit infrastructure|E9|Interaction audit JSONL logging per Common.md §5.2; violation file format; ai audit list/show/rotate; 30-day GC for interactions/. Spec §9.2."
  # E10 — Sites & Distribution
  "[Feature] Methodology site (aiConstitution.convergent-systems.co)|E10|Astro-based site build; brand-atoms.com token integration ([email protected]). Spec §16."
  "[Feature] Schema migrations (set-apps → convergent-systems, domain → domains[])|E10|Schema \$id URL migration; domain→domains[] reviewer YAML migration; document-writer deprecation. Spec §17, §18."
  "[Feature] Schema atomization (governance/schemas/ → schema-atoms.com)|E10|Migrate JSON schemas to schema-atoms.com as versioned atoms; atomization workflow. Spec §17.4."
)

declare -A FEATURE_NUMS

for def in "${FEATURE_DEFS[@]}"; do
  IFS='|' read -r title parent_key body <<< "$def"
  parent_num="${EPIC_NUMS[$parent_key]}"
  num=$(create_issue "$title" "$body" "agile/feature,status/triage")
  FEATURE_NUMS["$title"]="$num"
  add_sub_issue "$parent_num" "$num"
done

echo "Features created: ${#FEATURE_NUMS[@]}"
FEATURES
```

- [ ] **Step 2: Dry-run to verify feature definitions parse**

```bash
cd /Users/itsfwcp/workspace/convergent-system-co/aiConstitution
bash scripts/create-issues.sh --dry-run 2>&1 | grep -c "would create"
```

Expected: `45` (10 epics + 35 features).

---

## Task 5: Add story definitions to the script

**Files:**
- Modify: `scripts/create-issues.sh`

- [ ] **Step 1: Append story array and creation loop**

Append to `scripts/create-issues.sh`:

```bash
cat >> /Users/itsfwcp/workspace/convergent-system-co/aiConstitution/scripts/create-issues.sh << 'STORIES'

# ─── PASS 3: STORIES ────────────────────────────────────────────────────────
echo "=== Pass 3: Stories ==="

# Format: "TITLE|||PARENT_FEATURE_TITLE|||BODY"
declare -a STORY_DEFS=(
  # F: Four-file management
  "[Story] Load and validate four constitution files at CLI startup|[Feature] Four-file management (load, validate, inject ~/.ai/)|Read ~/.ai/{Constitution,Common,Code,Writing}.md; validate files exist and are non-empty; return structured error if any file missing."
  "[Story] Inject constitution content into AI tool sessions|[Feature] Four-file management (load, validate, inject ~/.ai/)|Write per-tool integration files (CLAUDE.md, .github/copilot-instructions.md, AGENTS.md) that reference the four files. Spec §10.2."
  "[Story] Constitution.local.md override support|[Feature] Four-file management (load, validate, inject ~/.ai/)|Load Constitution.local.md if present; merge into constitution layer (loaded last, highest override priority). Never committed."
  "[Story] Implement config.Load() and config.Save() with TOML parsing|[Feature] Four-file management (load, validate, inject ~/.ai/)|Replace stub Load()/Save() in src/internal/config/config.go with real BurntSushi/toml parsing and env-var overlay. Spec §13.3."
  # F: Constitution atoms
  "[Story] Consume canonical constitution atom from constitution-atoms.com|[Feature] Constitution atoms (publish/consume)|ai atoms fetch constitution-atoms/common/convergent-systems-core@<version>; verify content hash; place under ~/.ai/. Spec §3.4."
  "[Story] Fork and adopt a constitution atom|[Feature] Constitution atoms (publish/consume)|ai atoms fork <atom-id> --local creates a local working copy and records the upstream reference. Spec §3.4."
  "[Story] Publish local ~/.ai/ files as constitution atoms|[Feature] Constitution atoms (publish/consume)|ai atoms publish --catalog=constitution creates atom TOML, hashes the asset, and stages for publication. Spec §3.4."
  # F: Amendment lifecycle
  "[Story] ai amend draft — propose a change to a constitution file|[Feature] Amendment lifecycle (ai amend)|Given a violation file path, generate a unified diff against the affected file; open in $EDITOR for review; save to ~/.ai/governance/plans/ as a draft. Spec §3.5."
  "[Story] ai amend apply — write approved amendment to file|[Feature] Amendment lifecycle (ai amend)|Apply the staged diff; bump the file's Version field; append Changelog entry with UTC date; write audit/overrides/ entry. Spec §3.5."
  "[Story] ai amend list/show — browse amendment history|[Feature] Amendment lifecycle (ai amend)|List amendment files in ~/.ai/audit/; show full diff and rationale for a specific amendment. Spec §3.5."
  "[Story] Amendment atom publication|[Feature] Amendment lifecycle (ai amend)|ai amend publish creates an amendment atom TOML (diff_asset, rationale, audit_ref, approved_at) and a new constitution atom version. Spec §3.5."
  # F: Wizard TUI
  "[Story] Parse questions.yaml into typed question structs|[Feature] Wizard TUI (ai setup)|Load governance/wizard/questions.yaml (shipped in binary); decode into []Question with type, default, depends, required fields. Spec §13.2."
  "[Story] Bubble Tea TUI: render question sequence and collect answers|[Feature] Wizard TUI (ai setup)|Implement bubbletea.Model for wizard; navigate forward/back; handle select, multi-select, text, confirm types; persist answers in-memory. Spec §13."
  "[Story] Generate settings.toml from wizard answers|[Feature] Wizard TUI (ai setup)|Map wizard answer set to Settings struct; write to ~/.config/aiConstitution/settings.toml with MarshalTOML. Spec §13."
  "[Story] Initialize ~/.ai/ from wizard answers|[Feature] Wizard TUI (ai setup)|Create ~/.ai/ directory tree; write integration files (CLAUDE.md, copilot-instructions.md, AGENTS.md); install selected hooks. Spec §13."
  "[Story] Migration wizard (ai update --migrate)|[Feature] Wizard TUI (ai setup)|Detect version delta; present per-change migration prompts; apply changes; update lastSeenWizardVersion in settings.toml. Spec §13."
  # F: Hook registration
  "[Story] ai hooks install — register hooks with Claude Code settings.json|[Feature] Hook registration and lifecycle (ai hooks)|Write PreToolUse/PostToolUse/Stop entries to .claude/settings.json pointing to ~/.ai/hooks/*.py. Spec §8.3."
  "[Story] ai hooks validate — lint hook scripts|[Feature] Hook registration and lifecycle (ai hooks)|Run python3 -m py_compile on each *.py hook; check shebang and --self-check flag support; report findings. Spec §8.3."
  "[Story] ai hooks evaluate — run --self-check on installed hooks|[Feature] Hook registration and lifecycle (ai hooks)|Invoke each hook with --self-check; collect pass/fail; emit structured findings. Spec §8.3."
  "[Story] Hook violation audit logging|[Feature] Hook registration and lifecycle (ai hooks)|When a hook blocks an action, write ~/.ai/audit/violations/<UTC>-<slug>.md with schema from Constitution.md §5.2."
  # F: Command wrapper facade
  "[Story] ~/.ai/bin/git wrapper script|[Feature] Command wrapper facade (~/.ai/bin/)|Wrapper that fires preHooks from command-wrappers.toml before delegating to real git; supports AI_BYPASS_WRAPPERS=1 escape. Spec §8.2."
  "[Story] ~/.ai/bin/gh wrapper script|[Feature] Command wrapper facade (~/.ai/bin/)|Same pattern as git wrapper; adds destructive-gh-guard.py preHook for dangerous gh operations. Spec §8.2."
  "[Story] PATH wiring and doctor check|[Feature] Command wrapper facade (~/.ai/bin/)|ai doctor checks that ~/.ai/bin/ appears before /usr/local/bin/ and /opt/homebrew/bin/ on PATH; offers fix. Spec §8.2."
  # F: Core hook scripts
  "[Story] secret-block.py — block tool outputs that contain secrets|[Feature] Core hook scripts (enforcement plane)|PostToolUse hook: scan tool result for secret patterns (patterns.json); if matched, redact and halt with BLOCK. Spec §8.1."
  "[Story] branch-guard.py — prevent commits to protected branches|[Feature] Core hook scripts (enforcement plane)|PreToolUse hook: deny git commit/merge/push to main/master/release/* without explicit confirmation. Common.md §2.15."
  "[Story] worktree-guard.py — enforce canonical worktree paths|[Feature] Core hook scripts (enforcement plane)|PreToolUse hook: deny git worktree add to non-canonical paths; enforce <repo>/.worktrees/ or ~/.ai/worktrees/. Common.md §U17."
  "[Story] audit.py — interaction audit JSONL logging|[Feature] Core hook scripts (enforcement plane)|SessionStart/UserPromptSubmit/PreToolUse/PostToolUse/Stop/SubagentStop hooks: append one JSONL line per event to ~/.ai/audit/interactions/<YYYY-MM>.jsonl. Common.md §5.2."
  "[Story] secret-precommit.py — pre-commit secret scanning|[Feature] Core hook scripts (enforcement plane)|Pre-commit hook: scan staged diff for secret patterns; block commit if found. Spec §8.1."
  # F: AI tool integrations
  "[Story] Claude Code PreToolUse/PostToolUse/Stop hook wiring|[Feature] AI tool integrations (hooks)|ai hooks install writes to .claude/settings.json hooks array; verify with ai doctor. Spec §14.1."
  "[Story] Copilot command wrapper facade (no native hook API)|[Feature] AI tool integrations (hooks)|Document that Copilot lacks a native hook protocol; ~/.ai/bin/ wrappers cover git/gh operations regardless of invoker. Spec §14.2."
  "[Story] Cursor .cursor/rules/ integration file|[Feature] AI tool integrations (hooks)|Write .cursor/rules/ai-constitution.md pointing to the four files; include in ai init. Spec §14.3."
  "[Story] Codex AGENTS.md integration file|[Feature] AI tool integrations (hooks)|Write AGENTS.md that loads the four files and declares allowed/denied operations for Codex agents. Spec §14.4."
  # F: Setup and health
  "[Story] ai doctor — 10-point health check|[Feature] Setup and health commands|Check symlinks, hooks, dirty tree, HEAD divergence, stale binary, caches, audit log writability, settings parse, PATH wrappers. --fix auto-repairs. Spec §12.1."
  "[Story] ai status — current state summary|[Feature] Setup and health commands|Print active mode, last review timestamp, hook status, sync status, pending amendments count. Spec §12.1."
  "[Story] ai version — binary and constitution version|[Feature] Setup and health commands|Print binary semver, constitution atom version, wizard taxonomy version. Spec §12.1."
  # F: Mode and persona commands
  "[Story] ai mode <name/current/clear/list>|[Feature] Mode and persona commands|Activate an agentic persona for the session; read/write mode.json; list available personas from governance/personas/agentic/. Spec §12.1."
  "[Story] ai profile <list/show/new/edit/remove>|[Feature] Mode and persona commands|Manage composition profiles in ~/.config/aiConstitution/profile-drafts/. Spec §12.1."
  "[Story] ai persona <list/show>|[Feature] Mode and persona commands|List all personas (both agentic and reviewer); show full YAML/MD for a specific persona. Spec §12.1."
  # F: Governance commands
  "[Story] ai memory codify — structure a finding from a violation|[Feature] Governance commands|Read a violation file; produce a structured memory entry under ~/.ai/memory/; add to MEMORY.md index. Spec §9.3."
  "[Story] ai memory list/show/archive|[Feature] Governance commands|List all memory entries; show full content; archive stale entries. Spec §9.3."
  "[Story] ai audit list/show/rotate|[Feature] Governance commands|List violations and overrides; show full markdown for a specific entry; rotate old interaction logs. Spec §9.2."
  # F: Sync and restore
  "[Story] ai sync push/pull — git-backed ~/.ai/ sync|[Feature] Sync and restore commands|Push/pull ~/.ai/ to/from the configured git remote; exclude .gitignored files; require clean tree before push. Spec §15."
  "[Story] ai backup — dated ~/.ai/ snapshot|[Feature] Sync and restore commands|Create a dated tar.gz of ~/.ai/ in a configured backup location. Spec §15."
  "[Story] ai restore <snapshot> — restore from snapshot|[Feature] Sync and restore commands|Verify snapshot hash; unpack to ~/.ai/; run ai doctor after restore. Spec §15."
  # F: Project layer commands
  "[Story] ai init — initialize repo with project.yaml and integration files|[Feature] Project layer commands|Create project.yaml from template; write CLAUDE.md, .github/copilot-instructions.md, AGENTS.md, .cursor/rules/. Spec §10."
  "[Story] ai pm-mode — launch PM mode|[Feature] Project layer commands|Activate project-manager persona; enforce spawn DAG and containment policy. Spec §12.1."
  "[Story] ai spawn <persona> — spawn a sub-persona in PM mode|[Feature] Project layer commands|Create a sub-agent with the specified persona; enforce topology limits from agent-topology.yaml. Spec §4.4."
  "[Story] ai worktree add/list/remove — canonical placement|[Feature] Project layer commands|Compute canonical path (single-repo: .worktrees/; cross-repo: ~/.ai/worktrees/); delegate to git worktree. Common.md §U17."
  # F: Agentic personas
  "[Story] Load and validate 12 agentic persona files|[Feature] Agentic persona loading and activation|Read governance/personas/agentic/*.md; validate required frontmatter fields; cache in memory for the session. Spec §4.2."
  "[Story] Agent topology enforcement (spawn DAG)|[Feature] Agentic persona loading and activation|Load agent-topology.yaml; enforce max-concurrent limits; block out-of-DAG spawns in PM mode. Spec §4.4."
  "[Story] Agent containment (denied_paths, denied_operations)|[Feature] Agentic persona loading and activation|Load agent-containment.yaml; enforce per-persona path/operation denials via PreToolUse hook. Spec §4.5."
  # F: Reviewer personas
  "[Story] Load and validate 7 reviewer persona YAML files|[Feature] Reviewer persona system|Read governance/personas/domains/**/*.yaml; validate against persona.schema.json; expose via ai persona list. Spec §4.3."
  "[Story] domain → domains[] schema migration|[Feature] Reviewer persona system|Migrate singular domain: string to domains: [string] in all 7 reviewer YAML files; update persona.schema.json. Spec §18.2."
  # F: Panel system
  "[Story] Load 19 panels from panels.defaults.json|[Feature] Panel system (19 panels)|Parse governance/schemas/panels.defaults.json; validate against panel schema; expose panel config to review commands. Spec §5.2."
  "[Story] Panel scoring and confidence aggregation|[Feature] Panel system (19 panels)|Implement weighted_average confidence model from default.yaml; aggregate multi-panel results; apply risk escalation rules. Spec §5.3."
  "[Story] ai review — run panels against current PR|[Feature] Panel system (19 panels)|Select panels from policy profile; spawn reviewer personas; aggregate scores; write report. Spec §5."
  # F: Skill management
  "[Story] ai skills list/show|[Feature] Skill management (ai skills)|List all 16 skills from skills/*/SKILL.md; show full SKILL.md for a specific skill. Spec §6.3."
  "[Story] Add missing project-workspace SKILL.md|[Feature] Skill management (ai skills)|Write skills/project-workspace/SKILL.md with trigger, allowed_tools, and description. Spec §6.3 note."
  "[Story] Validate skill↔prompt pairings|[Feature] Skill management (ai skills)|For each skill in §6.3, verify a paired prompt exists in governance/prompts/; report gaps. Spec §6.2."
  # F: Prompt management
  "[Story] Load and render 29 prompt templates|[Feature] Prompt management (ai prompts)|Read governance/prompts/*.md; expose via ai prompts list/show; validate paired skill references. Spec §7.2."
  # F: Plugin install
  "[Story] ai plugins install <name> — fetch and unpack plugin atom|[Feature] Plugin install and lifecycle|Resolve plugin name to plugin-atoms.com/<name>/latest; fetch tarball; verify content hash; unpack to ~/.ai/plugins/<name>/. plugin-spec §4."
  "[Story] Plugin enable/disable/status/update|[Feature] Plugin install and lifecycle|Enable: symlink plugin artifacts into canonical locations. Disable: remove symlinks. Status: list installed + enabled state. Update: fetch newer atom version. plugin-spec §5."
  "[Story] Plugin on-disk layout (tarball, symlinks, manifest)|[Feature] Plugin install and lifecycle|Define ~/.ai/plugins/<name>/ layout with plugin.toml manifest; symlink skills/hooks/scripts to canonical locations. plugin-spec §3."
  # F: op plugin core
  "[Story] ai op env — generate env-file lines as op:// references|[Feature] Core op verbs (ai op)|Given vault and item name(s), output KEY=\"op://vault/item/field\" lines using op item get; detect duplicates; @uri-encode field names. plugin_op-spec §3.2."
  "[Story] ai op signin/signout/whoami|[Feature] Core op verbs (ai op)|Thin wrappers around op signin, op signout, op whoami that apply Common.md §4 redaction to all output. plugin_op-spec §3.3."
  "[Story] ai op clip <ref> — copy secret to clipboard (only allowed value surface)|[Feature] Core op verbs (ai op)|op read <ref> | pbcopy/xclip/wl-copy; confirm 'copied to clipboard' without echoing value. plugin_op-spec §3.7."
  "[Story] op-redact.py — PreToolUse hook scrubs raw secret values|[Feature] op plugin governance (redaction, doctor, SKILL.md)|Scan PostToolUse results for patterns matching known op:// resolved values; redact before output. plugin_op-spec §4."
  # F: macOS notifications
  "[Story] notify-me wrapper script (macOS)|[Feature] macOS notifications (terminal-notifier)|~/.ai/bin/notify-me: resolve --level to terminal-notifier -sound; pass --url as -open; group under -group claude-agent. notification-spec §3.1."
  "[Story] Doctor check for terminal-notifier and notification permission|[Feature] macOS notifications (terminal-notifier)|ai doctor checks brew list terminal-notifier; fires test ping; reports if notifications are blocked in System Settings. notification-spec §3.1."
  # F: Windows notifications
  "[Story] notify-me.ps1 and notify-me.cmd shim (Windows)|[Feature] Windows notifications (BurntToast)|PowerShell script using New-BurntToastNotification; .cmd shim calls powershell -File; sound mapping for info/warn/urgent. notification-spec §3.2."
  # F: Push fallback
  "[Story] ntfy push fallback for --level urgent|[Feature] Push fallback (ntfy)|When --level urgent OR local notification fails: curl to ntfy topic; token from op:// reference; never plaintext. notification-spec §4."
  # F: Memory management
  "[Story] ai memory codify — structured finding from a violation file|[Feature] Memory management (ai memory)|Read violation MD; extract rule, what happened, remediation; write typed memory file to ~/.ai/memory/ with MEMORY.md pointer. Spec §9.3."
  "[Story] ai memory list/show/archive|[Feature] Memory management (ai memory)|List MEMORY.md entries; cat a specific memory file; move to ~/.ai/memory/archived/ with date prefix. Spec §9.3."
  # F: Audit infrastructure
  "[Story] Interaction audit JSONL logging|[Feature] Audit infrastructure|Implement audit.AppendEvent(); wire into PostToolUse hook; write one JSONL line per event to ~/.ai/audit/interactions/<YYYY-MM>.jsonl. Common.md §5.2."
  "[Story] Violation file format and writer|[Feature] Audit infrastructure|Implement audit.WriteViolation(); write ~/.ai/audit/violations/<UTC>-<slug>.md with schema from Constitution.md §5.2. Spec §9.2."
  "[Story] ai audit list/show/rotate|[Feature] Audit infrastructure|List violation + override files with dates; cat a specific file; rotate interactions/ logs older than 30 days. Spec §9.2."
  # F: Methodology site
  "[Story] Astro site scaffold for aiConstitution.convergent-systems.co|[Feature] Methodology site (aiConstitution.convergent-systems.co)|Scaffold Astro project; configure Cloudflare Pages deployment; wire brand-atoms.com token import. Spec §16."
  # F: Schema migrations
  "[Story] Schema \$id URL migration (set-apps → convergent-systems)|[Feature] Schema migrations (set-apps → convergent-systems, domain → domains[])|Migrate \$id in all governance/schemas/*.json from set-apps.github.io to convergent-systems.co. Spec §18.1."
  "[Story] document-writer persona deprecation and migration|[Feature] Schema migrations (set-apps → convergent-systems, domain → domains[])|Remove document-writer from governance/personas/agentic/; create prose-writer.md and tech-writer.md as replacements. Spec §18.3."
)

declare -A STORY_NUMS

for def in "${STORY_DEFS[@]}"; do
  IFS='|' read -r title parent_feature body <<< "$def"
  parent_num="${FEATURE_NUMS[$parent_feature]:-}"
  num=$(create_issue "$title" "$body" "agile/story,status/triage")
  STORY_NUMS["$title"]="$num"
  if [[ -n "$parent_num" ]]; then
    add_sub_issue "$parent_num" "$num"
  fi
done

echo "Stories created: ${#STORY_NUMS[@]}"
echo "=== Done. Total issues created/verified. ==="
STORIES
```

- [ ] **Step 2: Dry-run to verify count**

```bash
cd /Users/itsfwcp/workspace/convergent-system-co/aiConstitution
bash scripts/create-issues.sh --dry-run 2>&1 | grep -c "would create"
```

Expected: a number ≥ 110. If significantly lower, check that the STORIES heredoc appended cleanly.

- [ ] **Step 3: Commit the script**

```bash
cd /Users/itsfwcp/workspace/convergent-system-co/aiConstitution
git add scripts/create-issues.sh
git commit -m "feat(issues): add idempotent agile hierarchy creation script

Creates 10 epics, 35 features, ~75 stories via gh CLI.
Idempotent: skips existing issues; wires native sub-issues.

Co-Authored-By: Claude Sonnet 4.6 (1M context) <noreply@anthropic.com>"
```

---

## Task 6: Run Pass 1 — create epics

**Files:** none (GitHub state only)

- [ ] **Step 1: Run the script for real (epics will be created)**

```bash
cd /Users/itsfwcp/workspace/convergent-system-co/aiConstitution
bash scripts/create-issues.sh 2>&1 | tee /tmp/create-issues.log
```

This creates all three passes in sequence. Watch for `created #N:` lines. If the script errors, check `/tmp/create-issues.log`.

- [ ] **Step 2: Verify all 10 epics exist**

```bash
gh issue list --repo convergent-systems-co/aiConstitution \
  --label "agile/epic" --json number,title --jq '.[] | "#\(.number) \(.title)"'
```

Expected: 10 lines, each starting with `[Epic]`.

---

## Task 7: Verify features and sub-issue wiring

- [ ] **Step 1: Count features**

```bash
gh issue list --repo convergent-systems-co/aiConstitution \
  --label "agile/feature" --json number,title --limit 50 | jq length
```

Expected: 35.

- [ ] **Step 2: Spot-check one epic's sub-issues**

```bash
# Get the issue number for E1
E1=$(jq '."[Epic] Constitution Layer & Wizard"' scripts/.issue-map.json)
gh api "repos/convergent-systems-co/aiConstitution/issues/$E1/sub_issues" \
  --jq '.[].title'
```

Expected: 4 feature titles starting with `[Feature]`.

---

## Task 8: Link existing open issues (#10–#19) to the hierarchy

- [ ] **Step 1: Classify and link each existing issue**

Run these in sequence (each takes ~1s):

```bash
# Issue #19: [Feature] Triage workflow → E4 (Persona & Panel, or E3 CLI)
# Actually this is about the triage GH Actions workflow → link to E3 CLI (F3 Governance commands)
E3_GOV=$(jq '."[Feature] Governance commands"' scripts/.issue-map.json)
gh api "repos/convergent-systems-co/aiConstitution/issues/$E3_GOV/sub_issues" \
  -X POST -f sub_issue_id=19
gh issue edit 19 --repo convergent-systems-co/aiConstitution \
  --add-label "agile/story" --remove-label "agile/feature"
sleep 1

# Issue #18: RAW: Break down questions → E1 Wizard TUI (wizard taxonomy)
WIZ=$(jq '."[Feature] Wizard TUI (ai setup)"' scripts/.issue-map.json)
gh api "repos/convergent-systems-co/aiConstitution/issues/$WIZ/sub_issues" \
  -X POST -f sub_issue_id=18
gh issue edit 18 --repo convergent-systems-co/aiConstitution \
  --add-label "agile/story"
sleep 1

# Issue #17: [Feature] Install plugins → E6 Plugin System
E6_INST=$(jq '."[Feature] Plugin install and lifecycle"' scripts/.issue-map.json)
gh api "repos/convergent-systems-co/aiConstitution/issues/$E6_INST/sub_issues" \
  -X POST -f sub_issue_id=17
gh issue edit 17 --repo convergent-systems-co/aiConstitution \
  --add-label "agile/story" --remove-label "agile/feature"
sleep 1

# Issue #16: [Finding] Surface 'waiting on user' state → E8 Notification (or doc)
E8_HOOK=$(jq '."[Feature] Agent hook integration (notify-me)"' scripts/.issue-map.json)
gh api "repos/convergent-systems-co/aiConstitution/issues/$E8_HOOK/sub_issues" \
  -X POST -f sub_issue_id=16
sleep 1

# Issues #10, #11, #13, #15 → E3 CLI (governance/doctor domain)
E3_HEALTH=$(jq '."[Feature] Setup and health commands"' scripts/.issue-map.json)
for n in 10 11 13 15; do
  gh api "repos/convergent-systems-co/aiConstitution/issues/$E3_HEALTH/sub_issues" \
    -X POST -f sub_issue_id=$n || true
  sleep 0.8
done
```

- [ ] **Step 2: Verify existing issues have parents**

```bash
gh issue view 19 --repo convergent-systems-co/aiConstitution --json labels,title
```

Expected: issue #19 has `agile/story` label.

---

## Task 9: Final verification

- [ ] **Step 1: Count totals**

```bash
echo "Epics:"
gh issue list --repo convergent-systems-co/aiConstitution --label "agile/epic" --json number | jq length

echo "Features:"
gh issue list --repo convergent-systems-co/aiConstitution --label "agile/feature" --limit 100 --json number | jq length

echo "Stories:"
gh issue list --repo convergent-systems-co/aiConstitution --label "agile/story" --limit 200 --json number | jq length
```

Expected: Epics=10, Features=35, Stories≥75.

- [ ] **Step 2: Push branch (hierarchy is live)**

```bash
cd /Users/itsfwcp/workspace/convergent-system-co/aiConstitution
git push
```

Done. The full agile hierarchy is live in GitHub with native sub-issue parent–child relationships.
