# AI Constitution Spec — 1.0.0-draft

**Atom ID:** `schema-atoms/design-spec/ai-constitution-spec@1.0.0-draft`
**Status:** 1.0.0-draft · lifecycle: draft · Phase 1 bootstrap
**Conforms to:** Atom Spec v1.1.0
**Steward:** convergent-systems-co
**Federation:** convergent-systems.co
**License:** Apache-2.0 (spec prose) · CC-BY-4.0 (atom content)

---

## §0 — TL;DR

The AI Constitution is a version-controlled governance layer for AI-assisted
software development. The user maintains four Markdown files in `~/.ai/` that
define how every AI tool they use must behave. Hooks enforce rules deterministically.
An audit system captures violations. Violations become amendments that update the
files. The files become atoms, distributed and cached locally for immutable
version-pinned use. The `ai` binary is the CLI that orchestrates everything.

This spec defines what to build. The substrate (signing, caching, lifecycle,
distribution) is defined by the Atom Spec v1.1.0 and the per-catalog specs.
This spec does not repeat those definitions; it cites them.

---

## §1 — Purpose, scope, and non-goals

### §1.1 What this spec defines

This spec defines the AI Constitution methodology: the governance layer, the
persona and review systems, the hook enforcement mechanism, the memory-to-amendment
lifecycle, the skill and prompt systems, the CLI surface, the wizard, the
integration with AI tools, and the public sites that distribute all of it.

It defines the application. The substrate (atoms, signing, caching, CLI verbs
for atoms, distribution) is defined elsewhere and inherited.

### §1.2 What this spec does not define

- **The atom substrate.** Structure, signing, lifecycle states, content
  addressability, federation, mirrors, phase model, private deployments.
  → Atom Spec v1.1.0

- **The local atom cache.** Cache layout, TTL semantics, GC, content-hash
  verification on load, offline behavior, multi-version coexistence.
  → Atom Cache Spec (forthcoming)

- **The atom CLI verbs.** `atoms fetch`, `atoms verify`, `atoms publish`,
  `atoms list`, `atoms diff`, `atoms lineage`.
  → Atom CLI Spec (forthcoming)

- **The key system.** Key atoms, rotation, succession, revocation, trust bootstrap.
  → Atom Key Spec (forthcoming)

- **Per-catalog schemas.** The TOML shape of persona atoms, policy atoms,
  skill atoms, prompt atoms, constitution atoms, amendment atoms.
  → Atom Persona Spec, Atom Policy Spec, Atom Skill Spec, Atom Prompt Spec,
    Atom Constitution Spec, Atom Amendment Spec (forthcoming)

- **Olympus.** The AI OS is a peer application consuming the same atom substrate.
  → Olympus Spec (separate)

### §1.3 Relationship to the Atom Spec

The AI Constitution Spec is a `design-spec` atom in schema-atoms. It conforms
to Atom Spec v1.1.0. Its own atom reference:

```
schema-atoms/design-spec/ai-constitution-spec@1.0.0-draft
```

Every atom it produces (constitution atoms, amendment atoms, persona atoms,
policy atoms, skill atoms, prompt atoms) conforms to Atom Spec v1.1.0.

### §1.4 Peer relationship to Olympus

Olympus (the Convergent Systems AI OS) and the AI Constitution are peers. Both:

- Consume atoms from the same 25-catalog ecosystem under `convergent-systems.co`
- Cache atoms locally using the same cache substrate
- Run as Phase 1 bootstrap applications under single stewardship
- Advance together as the ecosystem matures toward Phase 2

Neither depends on the other at the substrate level. The `ai` binary (AI
Constitution's CLI) and Olympus's runtime are separate applications. They share
the local atom cache by convention, not by design dependency.

### §1.5 Phase 1 bootstrap status

This spec, all atoms it produces, and the `ai` binary are Phase 1 bootstrap.
Single steward: convergent-systems-co / Thomas Polliard. Single signing authority.
No independent runtimes in production yet. The phase model from the Atom Spec
applies: Phase 2 requires three or more independent organizations consuming
atoms from this ecosystem in production for six or more continuous months.

---

## §2 — System architecture

### §2.1 The four layers

```
Layer 4 — APPLICATION (this spec)
  AI Constitution methodology: constitution files, personas, panels,
  skills, prompts, hooks, amendments, wizard, CLI, sites

Layer 3 — CATALOG (per-catalog specs)
  constitution-atoms, amendment-atoms, persona-atoms, policy-atoms,
  skill-atoms, prompt-atoms, schema-atoms, brand-atoms, profile-atoms

Layer 2 — CACHE (Atom Cache Spec)
  ~/.config/aiConstitution/.atom-cache/
  Content-hash verified, TTL-managed, GC'd, offline-capable

Layer 1 — SUBSTRATE (Atom Spec v1.1.0)
  Signing (ML-DSA-65), lifecycle, content addressability,
  federation (convergent-systems.co), mirrors, phase model
```

AI Constitution Spec owns Layer 4 entirely. It references Layer 3 specs for
catalog contracts. It inherits Layers 1 and 2 without modification.

### §2.2 The three runtime contexts

**User's machine — interactive:**
- `ai` CLI loads the four constitution files from `~/.ai/`
- Modes, personas, profiles resolved from local files or atom cache
- Hooks fire on pre/post tool events and pre-commit
- Wizard runs for setup and migration
- Audit records written to `~/.ai/audit/`

**User's machine — CI (per-repo):**
- The repo's `CLAUDE.md`, `.github/copilot-instructions.md`, `AGENTS.md`,
  `.cursor/rules/` wire each AI tool to the constitution
- Review panels run as agents, scoring PRs against policy profiles
- Results feed the merge gate (confidence aggregation per default.yaml)

**Atom registries — distribution:**
- Constitution atoms, persona atoms, policy atoms, skill atoms, prompt atoms
  published to their respective catalogs
- Consumers (other developers, Olympus, any Atom Spec client) fetch and cache

### §2.3 Catalogs consumed by the AI Constitution

| Catalog | What AI Constitution consumes from it |
|---|---|
| `constitution-atoms.com` | The canonical constitution, common, code, writing files |
| `amendment-atoms.com` | Every accepted amendment to the constitution |
| `persona-atoms.com` | Agentic personas (`kind: agentic`) and reviewer personas (`kind: reviewer`) |
| `policy-atoms.com` | Policy profiles (default, fast-track, fin_pii_high, etc.) and panel configurations |
| `skill-atoms.com` | The 16 skills (commit, debug, refactor, etc.) |
| `prompt-atoms.com` | The 29 prompt templates paired with skills and workflows |
| `schema-atoms.com` | The JSON Schemas validating configuration files (panels.schema.json, project.schema.json, etc.) and this spec itself |
| `brand-atoms.com` | `[email protected]` — the canonical CS brand for all five sites |
| `profile-atoms.com` | Profile compositions pinning specific persona atom versions |
| `plugin-atoms.com` | The plugin candidates (amendment-author, hook-author, atom-publisher, review-panel, memory-curator) |

---

## §3 — The constitution layer

### §3.1 The four files

Four Markdown files are always loaded by the `ai` CLI at session start. Together
they define the user's governance layer for all AI tools.

| File | Purpose | Size (current) |
|---|---|---|
| `~/.ai/Constitution.md` | Root rules and overrides. Loaded first. Sets the tone, hierarchy, and override authority. | ~9KB |
| `~/.ai/Common.md` | Universal rules that apply regardless of task domain — communication, safety, audit triggers, overrides. | ~36KB |
| `~/.ai/Code.md` | Coding standards, language-specific rules, review expectations, testing requirements. | ~22KB |
| `~/.ai/Writing.md` | Writing standards, tone, structure, formatting. | ~16KB |

Loading order matters: Constitution.md declares hierarchy; conflicts resolve in
favor of more-specific files. The `ai` CLI injects all four into the system prompt
of every AI tool session.

### §3.2 GOALS.md

`~/.ai/GOALS.md` is loaded selectively — on `ai setup`, on `ai mode`, and when
the user explicitly requests it. It is NOT always loaded because it is personal
and often private. It is not a public atom.

### §3.3 Constitution.local.md

`~/.ai/Constitution.local.md` is gitignored. It holds personal overrides,
machine-specific rules, and experimental rules not yet ready for the canonical
files. Loaded last; has override authority for the local machine only.

`Constitution.local.md.example` lives in the repo and documents the pattern.

### §3.4 Constitution atoms

The canonical versions of the four files are published to `constitution-atoms.com`
as atoms. Individual users fork these atoms (or adopt them verbatim) as their
working copies in `~/.ai/`.

Three publication modes:

1. **Canonical.** The official Convergent Systems version of each file, maintained
   by convergent-systems-co. Any user can adopt these verbatim.

2. **Community fork.** A user or organization publishes their own fork of the
   canonical files under their own signing key. Others can adopt the fork.

3. **Local working copy.** The user's `~/.ai/` files. Not necessarily published.
   May differ from the canonical atom version. Migration path: `ai atoms publish`.

Constitution atoms reference their ancestor:

```toml
id          = "constitution-atoms/common/convergent-systems-core@2.0.0"
version     = "2.0.0"
lifecycle   = "published"

[constitution]
class       = "common"          # constitution | common | code | writing
title       = "Common Rules — Convergent Systems Core"
asset       = "Common.md"
supersedes  = "constitution-atoms/common/convergent-systems-core@1.0.0"
```

### §3.5 Amendment atoms and the amendment lifecycle

Amendments are how the constitution evolves. The full lifecycle:

```
1. Audit violation written to ~/.ai/audit/violations/<UTC>-<slug>.md
2. ai memory codify → structured finding added to ~/.ai/memory/
3. ai amend draft → proposed change to the affected file(s)
4. Human reviews and approves the draft
5. ai amend apply → change applied to the canonical file
6. ai atoms publish → amendment atom created in amendment-atoms.com
7. Amendment atom references the constitution atom it amends
8. New constitution atom version published (supersedes prior version)
```

Amendment atoms carry the full rationale, the diff, and the audit record that
triggered the amendment. The chain of amendment atoms is the authoritative
history of why the constitution is what it is.

```toml
id          = "amendment-atoms/amendment/common-override-scope-clarification@1.0.0"
version     = "1.0.0"
lifecycle   = "published"

[amendment]
amends      = "constitution-atoms/common/convergent-systems-core@2.0.0"
affects     = ["Common.md"]
diff_asset  = "diff.patch"
rationale   = "rationale.md"
audit_ref   = "audit/violations/2026-05-22T143408Z-override-scope.md"
proposed_by = "convergent-systems-co"
approved_at = "2026-05-23T00:00:00Z"
```

---

## §4 — The persona layer

### §4.1 Two kinds of personas

The `persona-atoms.com` catalog holds two distinct kinds of atoms. The kind
determines the URL path, the TOML schema, the loading mechanism, and the lifecycle.

| Aspect | Agentic (`kind: agentic`) | Reviewer (`kind: reviewer`) |
|---|---|---|
| URL path | `/agentic/<name>/<version>/` | `/reviewer/<name>/<version>/` |
| Payload | Markdown with frontmatter | YAML with structured fields |
| Loaded by | `ai mode <name>` — session-long | Review panels — per-pass |
| Mental model | "What role am I playing?" | "What lens am I reviewing through?" |
| Lifecycle | Session-long, conversational | Per-review-pass, deterministic |
| Additional metadata | — | `domains: [string]` array |

### §4.2 Agentic persona roster (current)

12 agentic personas in `governance/personas/agentic/`. Each is a `.md` file.

| Persona | Role | Spawnable by |
|---|---|---|
| `project-manager` | Orchestrates PM mode, coordinates phases | root |
| `tech-lead` | Architecture decisions, delegates to coder/tester | project-manager, devops-engineer |
| `coder` | Implements features and fixes | tech-lead |
| `devops-engineer` | Infrastructure, CI/CD, deployment | project-manager |
| `iac-engineer` | Infrastructure as code | tech-lead, devops-engineer |
| `test-writer` | Writes tests | tech-lead |
| `test-evaluator` | Evaluates test coverage and quality | tech-lead |
| `observer` | Passive monitoring, no writes | any |
| `executor` | Direct execution persona, minimal reasoning | any |
| `issue-refiner` | Refines GitHub issues for clarity | any |
| `documentation-reviewer` | Reviews documentation quality | tech-lead |
| `document-writer` | **DEPRECATED.** See §18.3. | — |

**Note on `document-writer`:** This persona was scheduled for removal in the v0.4
iteration of the methodology. It has not yet been removed from the repository. The
migration to `prose-writer` (creative prose) and `tech-writer` (factual/technical
prose) is a Phase E migration task. Until the migration is complete, `document-writer`
remains functional but is marked deprecated. New integrations MUST NOT reference it.

### §4.3 Reviewer persona roster (current)

7 reviewer personas in `governance/personas/domains/`. Each is a `.yaml` file.
The existing schema uses singular `domain: string`; the target schema uses
`domains: [string]`. See §18.2 for the migration.

| Reviewer | Domains | File |
|---|---|---|
| `code-reviewer` | `[engineering]` | `domains/engineering/code-reviewer.yaml` |
| `refactor-specialist` | `[engineering]` | `domains/engineering/refactor-specialist.yaml` |
| `security-reviewer` | `[security]` | `domains/security/security-reviewer.yaml` |
| `systems-architect` | `[architecture]` | `domains/architecture/systems-architect.yaml` |
| `data-governance-reviewer` | `[data]` | `domains/data/data-governance-reviewer.yaml` |
| `docs-reviewer` | `[documentation]` | `domains/documentation/docs-reviewer.yaml` |
| `cost-analyst` | `[finops]` | `domains/finops/cost-analyst.yaml` |

Reviewer YAML fields (per the existing `persona.schema.json`, migrating to `domains[]`):

```yaml
name: code-reviewer
domains: [engineering]          # TARGET: array. Current schema: domain: engineering (singular)
role: "Senior engineer performing strict production-level review..."
version: 1.0.0
capabilities:
  - correctness analysis
  - concurrency review
  - error handling review
evaluate_for:
  - Correctness under concurrent access
  - Edge cases and boundary conditions
principles:
  - Every finding must include concrete remediation
anti_patterns:
  - Style nitpicks unless they impact correctness
severity_weights:
  critical: 0.25
  high: 0.15
  medium: 0.05
  low: 0.01
```

### §4.4 Agent topology (spawn DAG)

Defined in `governance/policy/agent-topology.yaml`. Enforced by the orchestrator
in PM mode (`ai pm-mode`). In standard mode, topology is advisory only.

```
project-manager
├── devops-engineer (max 1 concurrent)
│   ├── tech-lead
│   └── iac-engineer
└── tech-lead (max 3 concurrent)
    ├── coder (max 5 concurrent) — MUST delegate; cannot self-implement
    ├── iac-engineer
    ├── test-evaluator
    ├── validation-tester
    ├── document-writer (DEPRECATED — migrate to tech-writer)
    └── documentation-reviewer
```

Phase bindings (which persona may complete each phase in PM mode):
- Phase 0, 1: `devops-engineer`
- Phase 2, 3: `tech-lead`
- Phase 4, 5, 6: `devops-engineer`

### §4.5 Agent containment

Defined in `governance/policy/agent-containment.yaml` (23KB). Specifies per-persona:

- `denied_paths` — glob patterns the persona may not write to (e.g., `coder`
  cannot touch `governance/policy/**`, `governance/schemas/**`,
  `governance/personas/**`)
- `denied_operations` — operations the persona may not perform (e.g., `coder`
  cannot `git_push`, `git_merge`, `approve_pr`, `modify_policy`)
- Enforcement mode: `enforced` (blocks) or `advisory` (logs only)

The shared denied-paths set (all worker agents):
- `governance/policy/**`
- `governance/schemas/**`
- `governance/personas/**`
- `governance/prompts/reviews/**`
- `.github/workflows/jm-compliance.yml`
- `.github/workflows/dark-factory-governance.yml`

Containment exists independently of the spawn DAG. A persona that can be spawned
may still be prevented from acting on specific paths.

### §4.6 Agent envelope protocol

Multi-agent communication follows the structured envelope protocol defined in
`governance/schemas/agent-envelope.schema.json` and documented in
`governance/prompts/agent-protocol.md`.

Every inter-agent message carries:
- `envelope` — metadata (version, message_id, timestamp, source_agent,
  target_agent, correlation_id, session_id)
- `persona` — the sending agent's active persona context
- `protocol_message` — the actual payload

Message IDs: `msg-<uuid-v4>`. Session IDs: `sess-<uuid-v4>`.

The schema `$id` URLs currently reference `set-apps.github.io/dark-factory-governance/`
— see §18.1 for the migration to `convergent-systems.co`.

---

## §5 — The panel and policy layer

### §5.1 Panels vs reviewer personas — the distinction

These two concepts exist at different layers and are frequently confused.

**Reviewer persona** (atom in `persona-atoms.com/reviewer/`):
- A *lens* applied during review — what aspects to evaluate, what principles to apply
- YAML file with `capabilities`, `evaluate_for`, `principles`, `anti_patterns`, `severity_weights`
- Loaded and unloaded per review pass
- Examples: `code-reviewer`, `security-reviewer`, `cost-analyst`

**Panel** (configuration in `governance/schemas/panels.defaults.json`):
- An *orchestration unit* — which reviewer personas to invoke, how to weight their
  findings, what pass/fail criteria apply, what tools the agent can use
- Has `enabled`, `pass_criteria`, `scoring`, `allowed_tools`
- 19 panels defined in `panels.defaults.json`
- A panel IS NOT a persona; it USES personas (or may invoke standalone review passes)

The relationship: a `code-review` panel invokes the `code-reviewer` persona.
A `security-review` panel invokes `security-reviewer`. The panel defines the
operational envelope (thresholds, scoring, tools); the persona defines the
intellectual lens (what to look for and how to reason about it).

### §5.2 The 19 panels

All defined in `governance/schemas/panels.defaults.json` (schema version 1.1.0):

| Panel | Pass criteria | Primary reviewer persona |
|---|---|---|
| `ai-expert-review` | confidence ≥ 0.70, 0 critical | (AI safety lens) |
| `api-design-review` | confidence ≥ 0.70, ≤ 2 high | (API design lens) |
| `architecture-review` | (per config) | `systems-architect` |
| `code-review` | (per config) | `code-reviewer` |
| `copilot-review` | (CI tool) | (external — GitHub Copilot) |
| `data-design-review` | (per config) | `data-governance-reviewer` |
| `documentation-review` | (per config) | `docs-reviewer` |
| `migration-review` | (per config) | (migration lens) |
| `performance-review` | (per config) | (performance lens) |
| `production-readiness-review` | (per config) | (ops lens) |
| `security-review` | (per config) | `security-reviewer` |
| `technical-debt-review` | (per config) | `refactor-specialist` |
| `testing-review` | (per config) | (testing lens) |
| `threat-modeling` | (per config) | `security-reviewer` |
| `threat-model-system` | (per config) | `security-reviewer` |
| `data-governance-review` | (per config) | `data-governance-reviewer` |
| `governance-compliance-review` | (per config) | (compliance lens) |
| `test-generation-review` | (per config) | (test-gen lens) |
| `cost-analysis` | (per config) | `cost-analyst` |

Each panel has a `scoring` block defining `confidence_base` and deductions per
severity (critical: -0.25, high: -0.15, medium: -0.05, low: -0.01). Pass criteria
specify `min_confidence`, `max_critical_findings`, `max_high_findings`,
`required_verdict`, and `min_compliance_score`.

Each panel's `allowed_tools.capabilities` constrains what the reviewing agent can
do. The `ai-expert-review` panel grants `file_read`, `file_search`, `code_analysis`.
No panel grants `file_write` or `git_push` — reviewers are read-only by default.

### §5.3 Policy profiles

14 YAML policy profiles in `governance/policy/`. Each profile selects a subset
of panels, weights their outputs for confidence aggregation, and defines risk
escalation rules. A project selects its profile in `project.yaml`.

| Profile | Use case | Panel weight model |
|---|---|---|
| `default.yaml` (v1.5.0) | Standard repos, moderate risk | Weighted average across 12+ panels |
| `fast-track.yaml` | Low-risk internal tools | Reduced panel set, lower thresholds |
| `reduced_touchpoint.yaml` | Automated pipelines | Minimal human-in-loop requirements |
| `fin_pii_high.yaml` | Financial / PII data | Stricter thresholds, security weight ↑ |
| `infrastructure_critical.yaml` | Infrastructure repos | IaC and ops panels weight ↑ |
| `multi-model.yaml` | Multi-model orchestration | Additional AI-safety panels |
| `agent-containment.yaml` | Per-persona sandboxing | Containment rules, not panel weights |
| `agent-context-boundaries.yaml` | Agent isolation rules | Context boundary enforcement |
| `agent-topology.yaml` | Spawn DAG | PM mode topology |
| `canary-calibration.yaml` | Gradual rollout | Confidence gating for deployment |
| `circuit-breaker.yaml` | Failure isolation | Automatic panel disabling on repeated failure |
| `collision-domains.yaml` | Conflict prevention | Merge conflict and concurrent-edit rules |
| `observer-rules.yaml` | Passive monitoring | Observer persona operational rules |
| `panel-timeout.yaml` | Operational bounds | Timeout and retry rules per panel |

**`default.yaml` confidence weighting model:**

```yaml
weighting:
  model: weighted_average
  weights:
    code-review: 0.16
    security-review: 0.16
    ai-expert-review: 0.10
    architecture-review: 0.10
    testing-review: 0.07
    copilot-review: 0.08
    dependabot-review: 0.08
    codeql-review: 0.08
    performance-review: 0.04
    documentation-review: 0.04
    threat-modeling: 0.04
    cost-analysis: 0.02
    data-governance-review: 0.02
    finops-review: 0.01
  missing_panel_behavior: redistribute
```

**Risk aggregation rules (default.yaml):**
- Any panel reporting `critical` → aggregate risk: `critical`
- Two or more panels reporting `high` → `high`
- Single `high` with others `low` → `medium`
- All panels `low` or `negligible` → `low`

### §5.4 Policy profile selection

Each repo selects its policy profile in `project.yaml`:

```yaml
governance:
  policy_profile: default      # maps to governance/policy/default.yaml
  containment_mode: enforced   # enforced | advisory
  parallel_tech_leads: 3
  parallel_coders: 5
```

The `ai` CLI reads `project.yaml` when running in PM mode or when spawning
review panels. If no `project.yaml` exists, `default` profile is used.

### §5.5 Policy atoms

The 14 policy profiles and the `panels.defaults.json` configuration will migrate
to `policy-atoms.com`. Each becomes an independent, versioned, signed atom.
Projects reference policy atoms by version, enabling pinned-version reproducibility.

```toml
id       = "policy-atoms/profile/default@1.5.0"
version  = "1.5.0"
lifecycle = "published"

[policy]
class    = "profile"
title    = "Default Policy Profile"
asset    = "default.yaml"
```

The `panels.defaults.json` content migrates as:

```toml
id       = "policy-atoms/panel-config/convergent-systems-core@1.1.0"
version  = "1.1.0"
lifecycle = "published"

[policy]
class    = "panel-config"
title    = "Convergent Systems Core Panel Defaults"
asset    = "panels.defaults.json"
```

---

## §6 — The skill layer

### §6.1 What a skill is

A skill is a bounded, invocable unit of AI-assisted behavior. Each skill has:
- A trigger (how the user or CI invokes it — slash command, git hook, etc.)
- A set of allowed tools (what the skill can read/write/execute)
- A prompt template paired with it in `governance/prompts/`
- A SKILL.md file defining all of the above

Skills are distinct from personas: a persona shapes the AI's *identity and
reasoning style* throughout a session; a skill defines a *specific task* with
a bounded scope and specific tool permissions.

### §6.2 Skill ↔ Prompt pairing

Every skill in `skills/<name>/SKILL.md` pairs with a prompt in
`governance/prompts/<name>.md`. The skill defines trigger and tooling;
the prompt defines the reasoning template the AI follows when executing the skill.

This separation means:
- Skills can be updated (new tool permissions, new triggers) without changing reasoning
- Prompts can be refined without changing the trigger surface
- Each can be atomized and versioned independently

### §6.3 The 16 skills

| Skill | Trigger | Paired prompt | Allowed tools |
|---|---|---|---|
| `checkpoint` | `/checkpoint` | `checkpoint-resumption-workflow.md` | Read, Bash |
| `cleanup` | `/cleanup` | — | Read, Write, Bash |
| `commit` | `/commit` | `commit.md` | Read, Bash |
| `debug` | `/debug` | `debug.md` | Read, Bash |
| `diagram` | `/diagram` | — | Read |
| `explain` | `/explain` | `explain.md` | Read |
| `onboard` | `/onboard` | — | Read, Bash |
| `pm-mode` | `/pm-mode` | `startup-pm-mode.md` | Read, Bash, Agent |
| `pr` | `/pr` | — | Read, Bash |
| `project` | `/project` | — | Read, Write, Bash |
| `project-workspace` | (trigger-eval.json) | — | Read |
| `refactor` | `/refactor` | `refactor.md` | Read, Write, Bash |
| `repo` | `/repo` | — | Read, Bash |
| `spawn` | `/spawn` | — | Read, Bash, Agent |
| `startup` | session start | `startup.md` | Read |
| `test` | `/test` | `write-tests.md` | Read, Write, Bash |

**Note:** `skills/project-workspace/` currently contains only `trigger-eval.json`
(no SKILL.md). This is a gap — SKILL.md is required for atomization. Phase E task.

### §6.4 Skill atoms

Each skill migrates to `skill-atoms.com` as a `skill` atom. The atom bundles
SKILL.md and references its paired prompt atom:

```toml
id       = "skill-atoms/skill/commit@1.0.0"
version  = "1.0.0"
lifecycle = "published"

[skill]
title           = "Commit"
description     = "Generate a conventional-commit message from staged diff and commit."
user_invocable  = true
trigger         = "/commit"
allowed_tools   = ["Read", "Bash"]
prompt_ref      = "prompt-atoms/skill-prompt/commit@1.0.0"
asset           = "SKILL.md"
```

---

## §7 — The prompt layer

### §7.1 Prompt types

Two categories of prompts in `governance/prompts/`:

**Skill prompts** — Paired with a specific skill. Define the reasoning template
the AI follows when that skill is invoked.

**Workflow prompts** — Define multi-step reasoning workflows not tied to a
single skill. These are used by PM mode, cross-repo workflows, governance
processes, and operational loops.

### §7.2 The 29 prompts

**Skill prompts (8):**

| Prompt | Paired skill | Purpose |
|---|---|---|
| `commit.md` | `commit` | Conventional commit message generation |
| `debug.md` | `debug` | Structured debugging reasoning |
| `explain.md` | `explain` | Code/concept explanation |
| `refactor.md` | `refactor` | Refactor reasoning and safety checks |
| `startup.md` | `startup` | Session startup behavior |
| `startup-pm-mode.md` | `pm-mode` | PM mode initialization |
| `write-tests.md` | `test` | Test authoring strategy |
| `checkpoint-resumption-workflow.md` | `checkpoint` | Checkpoint resumption reasoning |

**Workflow prompts (21):**

| Prompt | Workflow | Triggered by |
|---|---|---|
| `agent-protocol.md` | Multi-agent communication | Inter-agent message dispatch |
| `backward-compatibility-workflow.md` | Backward compat checking | Tech lead, breaking-change review |
| `ci-ai-panel-system.md` | CI panel orchestration | GitHub Actions workflow |
| `code-review.md` | Code review pass | Review panel |
| `cross-repo-escalation-workflow.md` | Cross-repo issues | Code reviewer, escalation trigger |
| `data-governance-workflow.md` | Data governance review | Data governance panel |
| `devops-operations-loop.md` | DevOps PM mode loop | DevOps engineer persona |
| `di-generation-workflow.md` | Dependency injection generation | Coder persona |
| `docx-generation.md` | Word document generation | Any |
| `github-pages-setup.md` | GitHub Pages setup | DevOps engineer |
| `governance-change-proposal.md` | Proposing constitution changes | Any |
| `governance-compliance-checklist.md` | Compliance verification | Governance compliance panel |
| `init.md` | Repo initialization | `ai setup` / `ai init` |
| `migrate.md` | Migration workflows | `ai update --migrate` |
| `moderator.md` | Multi-agent moderation | Moderator role in panel sessions |
| `observer.md` | Observer persona behavior | Observer persona |
| `plan.md` | Plan document generation | PM mode, `/plan` |
| `remediation-workflow.md` | Finding remediation | Post-review |
| `retrospective.md` | Sprint retrospective | `/retrospective` |
| `shared-perspectives.md` | Multi-persona synthesis | Panel aggregation |
| `test-coverage-gate.md` | Coverage gate enforcement | CI panel |

### §7.3 Prompt atoms

Each prompt migrates to `prompt-atoms.com`:

```toml
id       = "prompt-atoms/skill-prompt/commit@1.0.0"
version  = "1.0.0"
lifecycle = "published"

[prompt]
class    = "skill-prompt"    # skill-prompt | workflow-prompt
title    = "Commit message generation"
skill_ref = "skill-atoms/skill/commit@1.0.0"
asset    = "commit.md"
```

Workflow prompts use `class = "workflow-prompt"` and omit `skill_ref`.

---

## §8 — The hook system

### §8.1 What hooks are

Hooks are deterministic, executable rules that fire at defined lifecycle points.
Unlike AI reasoning (which is probabilistic), hooks are code — they produce
consistent outputs for consistent inputs. They are the enforcement layer,
not the reasoning layer.

Hook types by firing point:

| Hook type | Fires at | Example |
|---|---|---|
| `PreToolUse` | Before AI executes a tool | Block writes to governance/** |
| `PostToolUse` | After AI executes a tool | Audit log on file write |
| `pre-commit` | Before git commit | Block commits with staged secrets |
| `Stop` | When AI tries to stop | Require checkpoint before session end |
| `SubagentStop` | When a subagent completes | Verify outputs before handoff |

### §8.2 Command wrapper facade

`~/.ai/bin/` contains wrapper scripts for commands that need hook injection:
`git`, `gh`, and any other commands where pre/post behavior is needed. Wrappers
apply `preHooks` and `postHooks` uniformly, papering over tool-to-tool gaps in
native hook support.

The `ai` binary itself does NOT live in `~/.ai/bin/`. It installs on system PATH
via brew (macOS), winget or scoop (Windows). `~/.ai/bin/` is for wrapper scripts
only — not the binary.

### §8.3 Hook lifecycle

```
Author a hook → ai hooks validate → ai hooks install (registers with tool)
→ fires at lifecycle point → result logged to ~/.ai/audit/
→ if violation: ~/.ai/audit/violations/<UTC>-<slug>.md
→ if pattern emerges: ai memory codify → ai amend draft → amendment lifecycle
```

Hook contributions upstream: GitHub Issue using `hook.md` template
(YAML Issue Forms conversion deferred — tracked as a repo issue).

---

## §9 — The memory → amendment lifecycle (extended)

### §9.1 The full lifecycle

```
AI session produces behavior
       │
       ├─ Positive: logged to ~/.ai/audit/interactions/
       │
       └─ Violation: written to ~/.ai/audit/violations/<UTC>-<slug>.md
                                │
                     ai memory codify
                                │
               ~/.ai/memory/ structured finding
                                │
                    (pattern threshold met?)
                                │
                     ai amend draft
                                │
              proposed diff against one or more of
              {Constitution.md, Common.md, Code.md, Writing.md}
                                │
                    Human reviews and approves
                                │
                     ai amend apply
                                │
                     File updated in ~/.ai/
                                │
                  ai atoms publish (optional)
                                │
              amendment-atoms/amendment/<slug>@<version>
                                │
         new constitution-atoms/<file>/<slug>@<version+1>
```

### §9.2 Audit structure

```
~/.ai/audit/
├── interactions/          # Successful AI interactions (rotating, GC'd after 30 days)
├── violations/            # Rule violations (retained indefinitely until amend'd)
│   └── <UTC>-<slug>.md   # Format: 2026-05-23T143408Z-override-scope.md
├── overrides/             # Approved overrides to violations
└── snapshots/             # Memory snapshots before major changes
    └── <UTC>-<description>/
```

### §9.3 Memory structure

```
~/.ai/memory/
└── projects/
    └── <project-path-encoded>/   # e.g. -Users-itsfwcp-workspace-convergent-systems-co-olympus
        └── <findings, notes, context>
```

---

## §10 — The project layer

### §10.1 project.yaml

`project.yaml` is the per-repo configuration file. It declares:

- Repository identity (name, language, framework)
- Governance configuration (policy profile, containment mode, panel overrides)
- Repository GitHub settings (branch protection, CODEOWNERS, auto-merge)
- ADO integration settings (if applicable)

Validated by `governance/schemas/project.schema.json` (48KB — the largest schema,
covering all project configuration possibilities). The schema $id currently
references `set-apps.github.io/dark-factory-governance/` — see §18.1.

```yaml
name: olympus-central
language: go
governance:
  policy_profile: default
  containment_mode: enforced
  parallel_tech_leads: 3
  parallel_coders: 5
repository:
  auto_merge: true
  branch_protection:
    required_reviews: 1
    dismiss_stale: true
```

### §10.2 Repo-root integration files

Every repo that uses the AI Constitution wires each AI tool via repo-root files:

| File | Tool | Purpose |
|---|---|---|
| `CLAUDE.md` | Claude / Claude Code | Instructs Claude to load and follow the constitution |
| `.claude/` | Claude Code | Claude Code settings, project memory |
| `.github/copilot-instructions.md` | GitHub Copilot | Instructs Copilot to follow the constitution |
| `AGENTS.md` | OpenAI Codex / Agents | Instructs Codex agents to follow the constitution |
| `.cursor/rules/` | Cursor | Instructs Cursor to follow the constitution |

Each file is minimal — it points the tool to the canonical four files rather
than duplicating rules. Example `CLAUDE.md`:

```markdown
# Claude Instructions

Load and strictly follow the AI Constitution from ~/.ai/:
- Constitution.md (root rules)
- Common.md (universal rules)
- Code.md (coding standards, if coding task)
- Writing.md (writing standards, if writing task)

Do not override, summarize, or abbreviate these rules.
```

### §10.3 governance/.claude/

`governance/.claude/settings.local.json` holds Claude Code settings scoped to
the governance directory. This allows different behavior when Claude Code is
operating within `governance/` (e.g., allowed to read schemas and personas but
not modify containment policy without explicit override).

---

## §11 — Plugins

### §11.1 What plugins are

Plugins are multi-step, guided workflows. They sit above the CLI (too complex
for a single command) and above personas (not a behavioral shaping concern).
A plugin is invoked, asks questions, executes a sequence of CLI/AI actions, and
produces an artifact.

Plugin vs CLI vs persona vs skill:

| Layer | What it does | Example |
|---|---|---|
| CLI | Single deterministic operation | `ai amend apply` |
| Skill | Bounded AI task with defined tools | `/commit` |
| Persona | Session-long behavioral shaping | `ai mode tech-lead` |
| Plugin | Multi-step guided workflow | `amendment-author` |

#### Plugin availability by tool

| Plugin | Claude Code | GitHub Copilot CLI | Cursor | Notes |
|---|---|---|---|---|
| `superpowers` | ✓ | TBD — no native plugin system | TBD | Core subagent-driven-development patterns |
| `amendment-author` | ✓ | TBD | TBD | Guided `ai amend` flow |
| `hook-author` | ✓ | TBD | TBD | Guided `ai hooks propose` flow |
| `atom-publisher` | ✓ | TBD | TBD | Guided atom publication |
| `review-panel` | ✓ | TBD | TBD | Orchestrated panel review |
| `memory-curator` | ✓ | TBD | TBD | Guided `ai review` decision tree |

Plugin questions in the setup wizard are **tool-gated**: Q36b/Q36c only appear when Claude Code is selected at Q36. Users who select Copilot CLI without Claude Code see a note (Q36b-copilot) explaining that Copilot parity is planned. Users who select both tools see both the Claude plugin offer and the Copilot note.

### §11.2 The five plugin candidates

| Plugin | Workflow | Produces |
|---|---|---|
| `amendment-author` | Guides: violation → finding → draft amendment → review → apply → publish | Amendment atom |
| `hook-author` | Guides: describe behavior → write hook code → validate → install → test | Hook in `~/.ai/hooks/` |
| `atom-publisher` | Guides: draft atom → canonicalize → hash → sign → PR → publish | Atom in the appropriate catalog |
| `review-panel` | Guides: select panels → run passes → aggregate scores → produce report | Panel output report |
| `memory-curator` | Guides: review memory → identify patterns → propose amendments → archive | Curated memory + optional amendment |

### §11.3 Plugin atoms

Plugins live in `plugin-atoms.com`:

```toml
id       = "plugin-atoms/plugin/amendment-author@1.0.0"
version  = "1.0.0"
lifecycle = "draft"

[plugin]
title    = "Amendment Author"
summary  = "Guided workflow for authoring and publishing amendments."
asset    = "plugin.md"
produces = "amendment-atoms/amendment/*"
```

---

## §12 — The CLI surface

### §12.1 Core command groups

The `ai` binary provides commands in these groups. Atom substrate verbs
(`atoms fetch`, `atoms verify`, etc.) are defined in the Atom CLI Spec and
implemented by the same binary — they are not duplicated here.

**Setup and health:**
- `ai setup` — Initial wizard (installs hooks, creates config, fetches atoms)
- `ai doctor` — Checks config, hook health, atom cache integrity
- `ai update [--migrate]` — Updates constitution to new atom version

**Mode and personas:**
- `ai mode <name>` — Activates an agentic persona for the session
- `ai mode current` — Shows active mode
- `ai mode clear` — Clears active mode
- `ai mode list` — Lists available agentic personas
- `ai profile <list|show|new|edit|remove>` — Manages composition profiles
- `ai persona <list|show>` — Lists all personas (both kinds)

**Governance:**
- `ai amend <draft|apply|list|show>` — Amendment lifecycle
- `ai memory <codify|list|show|archive>` — Memory management
- `ai audit <list|show|rotate>` — Audit log management
- `ai hooks <list|install|validate|evaluate>` — Hook management
- `ai override <approve|list>` — Approved override management

**Sync and restore:**
- `ai sync <push|pull>` — Syncs ~/.ai/ with remote (git-based)
- `ai backup` — Creates a dated backup of ~/.ai/
- `ai restore <snapshot>` — Restores from a snapshot

**Work products:**
- `ai worktree <add|list|remove>` — Git worktree management
- `ai checkpoint <create|list|resume>` — Session checkpointing
- `ai issue <file>` — Files a GitHub issue from a template

**Project:**
- `ai init` — Initializes a new repo with project.yaml and integration files
- `ai pm-mode` — Launches PM mode (spawns project-manager persona)
- `ai spawn <persona>` — Spawns a sub-persona in PM mode

### §12.2 State files

Mode and session state live in `~/.config/aiConstitution/` (per-machine mutable,
NOT synced):

```
~/.config/aiConstitution/
├── mode.json          # current active mode
├── state.json         # consolidated session state
├── settings.toml      # per-machine settings
├── checkpoints/       # session checkpoints (migrated from ~/.ai/checkpoints/)
└── .atom-cache/       # local atom cache (see Atom Cache Spec)
    ├── persona/
    ├── reviewer/
    ├── policy/
    ├── skill/
    ├── prompt/
    ├── constitution/
    └── brand/
```

`~/.ai/` is the canonical synced tree — governance content, work products,
and completed artifacts. `~/.config/aiConstitution/` is per-machine mutable state.

### §12.3 The mutable-vs-sync carve-out

General rule: **mutable per-machine state → `~/.config/aiConstitution/`; immutable
governance content → `~/.ai/`.**

Sync-worthy work products (plans, specs) live in `~/.ai/` despite being technically
mutable during authoring, because the sync requirement is load-bearing and
`~/.config/` is per-machine by design. Plans and specs are append-mostly artifacts
whose lifecycle ends in immutability once shipped; the brief mutability window
during authoring is the cost of getting sync for free.

---

## §13 — The wizard

### §13.1 Purpose

The wizard is the guided onboarding experience for new users and the guided
migration experience for existing users upgrading to a new atom version. It is
a TUI (terminal UI) application invoked by `ai setup` and `ai update --migrate`.

### §13.2 Question taxonomy

The wizard presents questions in categories. The full taxonomy is in
`questions.yaml` (shipped with the `ai` binary). Categories:

- **Identity** — user name, organization, GitHub handle
- **Constitution** — which constitution atom to adopt, which canonical files to use
- **Personas** — which agentic personas to include, which reviewer personas
- **Profiles** — which composition profiles to set up
- **Tools** — which AI tools are in use (Claude, Copilot, Cursor, Codex)
- **Policy** — which policy profile to use, containment mode
- **Hooks** — which hooks to install, enforcement mode
- **Plugins** — which plugins to enable
- **Sync** — git remote for `~/.ai/`, sync frequency
- **Brand** — (for site builders) whether to pull CS brand tokens

Each question has a type (`select`, `multi-select`, `text`, `confirm`),
a default, and a dependency on prior answers. The wizard emits `settings.toml`
and initializes `~/.ai/` and `~/.config/aiConstitution/`.

---

## §14 — AI tool integrations

### §14.1 Claude / Claude Code

**CLAUDE.md** (repo root): Points Claude to the four files. Loaded automatically
by Claude Code.

**`.claude/`** (repo root): Claude Code settings. Includes project memory,
tool permissions, and any repo-specific overrides.

**`governance/.claude/settings.local.json`**: Scoped settings for when Claude
Code operates within the governance directory.

Hooks fire via Claude Code's `PreToolUse` and `PostToolUse` hook points.
The `Stop` hook fires before Claude ends a session.

### §14.2 GitHub Copilot

**`.github/copilot-instructions.md`**: Points Copilot to the four files.
Loaded automatically by GitHub Copilot in VS Code and JetBrains IDEs.

Copilot does not support the hook protocol. The Command Wrapper Facade in
`~/.ai/bin/` handles pre/post behavior for git operations regardless of
whether the git command was invoked through Copilot or directly.

The `copilot-review` panel in panels.defaults.json represents Copilot's code
review signal as an input to the aggregate confidence score.

### §14.3 Cursor

**`.cursor/rules/`**: Rules directory for Cursor. Each `.md` file in this
directory is loaded by Cursor. The `ai-constitution.md` rule points to the
four files.

### §14.4 OpenAI Codex / Agents

**`AGENTS.md`** (repo root): Loaded by OpenAI Codex agents and the OpenAI
Agents SDK. Points to the four files and declares allowed/denied operations.

---

## §15 — Sync and restore

### §15.1 ~/.ai/ layout (canonical, synced)

```
~/.ai/
├── Constitution.md              # Always loaded
├── Constitution.local.md        # Gitignored — local private overrides
├── Constitution.local.md.example
├── Common.md                    # Always loaded
├── Code.md                      # Always loaded
├── Writing.md                   # Always loaded
├── GOALS.md                     # Loaded selectively
├── CLAUDE.md                    # Claude integration
├── Makefile                     # Build and maintenance targets
├── README.md
├── .gitignore
├── .gitattributes
├── .editorconfig
├── go.mod                       # Go module for ai binary
├── mkdocs.yml                   # CURRENT: docs site config (migrating to Astro)
├── project.yaml                 # This repo's own project config
│
├── plans/                       # Sync-worthy plans (moved from governance/plans/)
│   └── <date>-<slug>.md
├── specs/                       # Sync-worthy specs (moved from docs/superpowers/specs/)
│   └── <date>-<slug>.md
│
├── governance/
│   ├── personas/
│   │   ├── agentic/             # 12 agentic persona .md files
│   │   └── domains/             # 7 reviewer persona .yaml files (by domain)
│   │       ├── architecture/
│   │       ├── data/
│   │       ├── documentation/
│   │       ├── engineering/
│   │       ├── finops/
│   │       └── security/
│   ├── policy/                  # 14 policy profile .yaml files
│   ├── prompts/                 # 29 prompt templates
│   ├── schemas/                 # 30+ JSON Schemas (migrating to schema-atoms)
│   ├── plans/                   # LEGACY: merge into ~/.ai/plans/ (Phase E)
│   └── templates/               # Issue and PR templates
│       └── .github/
│
├── skills/                      # 16 skill directories
│   ├── <name>/SKILL.md
│   └── project/templates/       # Project template files
│
├── cmd/                         # Go source for ai binary
│   └── ai/
│       ├── main.go
│       └── worktree_test.go
│
├── hooks/                       # Hook scripts (bash/go)
├── bin/                         # Command wrapper scripts (git, gh, etc.)
├── memory/                      # AI session memory
├── metadata/                    # Per-machine metadata (migrate to ~/.config/)
│   └── projects.json
├── audit/                       # Audit records (violations, interactions, overrides)
├── docs/                        # Mkdocs source (migrating to Astro)
├── site/                        # Mkdocs build output (gitignored post-migration)
├── worktrees/                   # Git worktree registry
└── .worktrees/                  # Worktree state
```

### §15.2 Pending migrations (Phase E)

| Current location | Target location | Reason |
|---|---|---|
| `checkpoints/` (57 dirs in synced tree) | `~/.config/aiConstitution/checkpoints/` | Mutable per-machine state |
| `governance/plans/` | `~/.ai/plans/` | Consolidate plan locations |
| `docs/superpowers/plans/` | `~/.ai/plans/` | Consolidate plan locations |
| `docs/superpowers/specs/` | `~/.ai/specs/` | Consolidate spec locations |
| `metadata/projects.json` | `~/.config/aiConstitution/metadata/` | Per-machine mutable |
| `governance/schemas/*.json` | `schema-atoms.com` (after atom publication) | Atoms substrate |
| `governance/policy/*.yaml` | `policy-atoms.com` (after atom publication) | Atoms substrate |
| `governance/prompts/*.md` | `prompt-atoms.com` (after atom publication) | Atoms substrate |
| `skills/*/SKILL.md` | `skill-atoms.com` (after atom publication) | Atoms substrate |
| `governance/personas/` | `persona-atoms.com` (after atom publication) | Atoms substrate |

---

## §16 — The public sites

### §16.1 Five sites

All five sites share the Convergent Systems brand, the Astro stack, and a
`/builder` route mirroring brand-atoms.com/builder. mkdocs is retired in favor
of Astro for all documentation.

| Site | Hosts | Repo |
|---|---|---|
| `ai-constitution.convergent-systems.co` | Documentation, guides, community | `convergent-systems-co/ai` |
| `persona-atoms.com` | Agentic + reviewer persona catalog | `convergent-systems-co/persona-atoms` |
| `profile-atoms.com` | Profile composition catalog | `convergent-systems-co/profile-atoms` |
| `skill-atoms.com` | Skill catalog | `convergent-systems-co/skill-atoms` |
| `brand-atoms.com` | Brand catalog (existing) | `convergent-systems-co/branding-library` |

### §16.2 Brand: [email protected]

All five sites consume `[email protected]`:

```
https://brand-atoms.com/brands/convergent-systems
https://brand-atoms.com/dist/brands/convergent-systems/1.0.0/css/tokens.css
https://brand-atoms.com/dist/brands/convergent-systems/1.0.0/tailwind/tailwind.config.cjs
```

Brand tokens (from the live brand atom):

| Role | Token | Hex |
|---|---|---|
| Primary action | `frost-cyan` | `#5CD6FF` |
| Brand mark | `solar-gold` | `#F4C75E` |
| Warmth | `ember-orange` | `#FF8A3D` |
| Canvas (dark) | `deep-space-0` | `#07090F` |
| Text (dark) | `snow-0` | `#EEF1F7` |
| Canvas (light) | `parchment-canvas` | `#F9F7F0` |
| Font | Inter | OFL-1.1 |

14 typed error-severity constraints are enforced at Astro build time:
- Heading weight: 700 or 800 only
- Wordmark letter-spacing: 0.18em (exact)
- Section-label letter-spacing: 0.32em uppercase
- Mark fill: `solar-gold` only — never soft variant, never cyan, never ember
- Mark treatments forbidden: stretched, rotated, recolored, drop-shadow, on-busy-photo
- WCAG 2.1 AA contrast (4.5:1 body, 3:1 primary)
- Ember-orange: forbidden in error states, validation failure, destructive action
- Heading-to-body ratio: ≥ 1.8

---

## §17 — Schema migrations

### §17.1 Schema $id migration (set-apps → convergent-systems)

All 30+ JSON Schemas in `governance/schemas/` carry `$id` URLs referencing
`https://set-apps.github.io/dark-factory-governance/schemas/`. This is the
predecessor project name ("Dark Factory"). These must migrate to the current
federation URL.

Target `$id` pattern: `https://schema-atoms.com/json-schema/<slug>/<version>/schema.json`

Migration path per schema:
1. Publish schema as an atom in schema-atoms (see §17.4)
2. Update `$id` in the schema file to the new URL
3. Update all references to the schema in `project.yaml`, `panels.defaults.json`, etc.
4. Update the JSON Schema `$schema` keyword where needed (draft-07 → 2020-12 where possible)

Priority order: `project.schema.json` (widely referenced), `panels.schema.json`,
`persona.schema.json`, `agent-envelope.schema.json`, `panel-output.schema.json`.

### §17.2 domain → domains[] migration

The existing `governance/schemas/persona.schema.json` defines:

```json
{
  "required": ["name", "domain", "role", "version"],
  "properties": {
    "domain": {
      "type": "string",
      "enum": ["security", "engineering", "operations", "architecture",
               "data", "documentation", "finops"]
    }
  }
}
```

The target schema defines `domains` as an array:

```json
{
  "required": ["name", "domains", "role", "version"],
  "properties": {
    "domains": {
      "type": "array",
      "items": {
        "type": "string",
        "enum": ["security", "engineering", "operations", "architecture",
                 "data", "documentation", "finops"]
      },
      "minItems": 1
    }
  }
}
```

Migration: update all 7 reviewer YAML files to use `domains: [<value>]` (array
of length 1 for current single-domain reviewers). Update `persona.schema.json`.
Update the CLI validation logic. This is a Phase E task; both schemas coexist
during migration, with the loader accepting either form.

### §17.3 document-writer deprecation

`document-writer.md` in `governance/personas/agentic/` and references in:
- `governance/schemas/agent-envelope.schema.json` (source_agent / target_agent enum)
- `governance/policy/agent-containment.yaml` (per-persona rules)
- `governance/policy/agent-topology.yaml` (can_spawn list)

Migration: introduce `prose-writer.md` (creative prose persona) and
`tech-writer.md` (factual/technical prose persona). Remove `document-writer.md`.
Update the agent-envelope schema to add the new personas and deprecate
`document-writer`. Update agent-topology to use the new personas. Phase E task.

### §17.4 Schema atomization workflow

Each of the 30+ JSON Schemas in `governance/schemas/` becomes a `json-schema`
atom in schema-atoms. The workflow per schema:

1. Create `schema-atoms/json-schema/<slug>@1.0.0` atom TOML
2. Asset: the schema JSON file with updated `$id`
3. Canonicalize, hash, sign, PR, publish
4. Update consuming files to reference the atom URL rather than the local file
5. The local copy in `governance/schemas/` becomes a cache of the canonical atom

Phase F: full schema atomization. Until then, local files are authoritative.

---

## §18 — Phase status

### §18.1 Current: Phase 1 bootstrap

This application, the `ai` binary, and all atoms it produces are Phase 1 bootstrap:

- Single steward: convergent-systems-co / Thomas Polliard
- Single signing authority (same entity holds root, editor, catalog-maintainer roles)
- No independent runtimes in production
- Spec changes stewarded by single party

**Acknowledged limitations:**
- Single point of failure
- No external verification of amendments
- All atoms verifiable only against the single root key chain
- No formal review process for spec changes

These are accepted as appropriate for the current size and trajectory.

### §18.2 Phase 2 entry criteria (inherited from Atom Spec)

- Three or more independent organizations operate runtimes consuming atoms
  from the convergent-systems ecosystem in production for ≥ 6 continuous months
- At least one has contributed a non-trivial atom accepted into a catalog
- The Atom Spec has had one revision via the Phase 2 amendment process

The AI Constitution application advances with the ecosystem. No separate
Phase 2 criteria specific to the AI Constitution application exist; the
substrate's phase model applies.

---

## §19 — Private deployments

Organizations may deploy the AI Constitution methodology privately. Per the
Atom Spec Part XIII:

- A private deployment is a conforming registry not publicly accessible
- Atoms are signed by the organization's keys (not the public root chain)
- Public atoms may be consumed by reference + content-hash verification
- The deployment defines its own trust boundary
- It MUST NOT claim public canonicity

AI Constitution private deployment specifically means:
- The four constitution files remain private (not published to constitution-atoms.com)
- Personas, policies, and skills may be private or forked from public atoms
- The `ai` binary operates against the private registry
- Panel results and amendment history remain within the organization

`constitution-atoms` and `amendment-atoms` are the first catalogs slated for
Docker-image distribution to support self-hosted private deployments.

---

## §20 — Open questions

**Q1: Constitution as one atom or four?**
Should the four files be four separate atoms (constitution, common, code, writing)
that compose, or one composite atom referencing all four? The current design leans
toward four separate atoms for independent versioning. A user might adopt
`common@2.0.0` while staying on `code@1.x`. Deferred to Atom Constitution Spec.

**Q2: Where do worktrees live?**
`~/.ai/worktrees/` and `~/.ai/.worktrees/` are both present. The `.claude/worktrees/`
directory also exists. The canonical location for worktree state is unclear.
The `ai worktree` command should own a single directory. Probably
`~/.config/aiConstitution/worktrees/` (per-machine) for state, with `~/.ai/.worktrees/`
as the git-level worktree directory. To be resolved in the Atom CLI Spec.

**Q3: Panel as a policy atom or its own kind?**
The 19 panels in panels.defaults.json are policy (they configure behavior).
But a user might want to define a custom panel (a new kind of review) rather
than just configure an existing one. If panels are a configurable `panel-config`
atom kind in policy-atoms, custom panels require creating a new kind of panel atom.
Deferred to Atom Policy Spec.

**Q4: CI integration specifics.**
The ci-ai-panel-system.md prompt exists but the CI GitHub Actions workflow is
not yet fully documented in this spec. How do panel results from CI feed into
the confidence aggregate? What is the merge gate mechanism? How does the workflow
trigger? To be documented in Phase D implementation.

**Q5: Skill plugin vs skill atom.**
`superpowers` is currently a skill mentioned in the wizard. But superpowers is
arguably a plugin (multi-step workflow). The skill/plugin boundary needs
clarification for the cases where a skill is sufficiently complex to be a plugin.

**Q6: ADO integration scope.**
Three ADO schemas exist (`ado-integration.schema.json`, `ado-sync-error.schema.json`,
`ado-sync-ledger.schema.json`). The scope of Azure DevOps integration is not yet
documented in this spec. ADO integration is a Phase G task; this spec notes it
as a known surface without specifying it.

**Q7: metadata/projects.json.**
Currently in `~/.ai/metadata/projects.json` (synced tree). Should be in
`~/.config/aiConstitution/metadata/` (per-machine). Migration task.

---

## §21 — Changelog

- **1.0.0-draft** — Initial formal spec. Full rewrite from the v0.1–v0.8 scratchwork
  iterations. Reframed as an application atop the Atom Spec substrate (peer to
  Olympus). Incorporates all findings from the May 2026 ai-backup.zip review:
  the panel system (19 panels, scoring, policy profiles), the 14 policy YAMLs,
  the 29 prompts, the 16 skills (with skill↔prompt pairing), agent topology and
  containment, the multi-agent envelope protocol, the document-writer deprecation,
  the domain→domains[] migration, and the set-apps→convergent-systems schema
  migration. Drops all v0.x patch numbering; adopts Atom Spec lifecycle states.
  Brand pinned to `[email protected]`. Kind terminology
  settled: `reviewer` (not `panel`). Domains as array. YAML Issue Forms deferred.
  Plans/specs in ~/.ai/ with explicit sync-carve-out rationale.
