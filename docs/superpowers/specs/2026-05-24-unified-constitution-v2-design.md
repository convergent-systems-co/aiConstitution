# Design: Unified Constitution v2

**Date:** 2026-05-24  
**Status:** Approved for implementation  
**Branch:** `docs/triage-v2-design` → target `main`

---

## Problem Statement

The current four-file constitution (Constitution.md + Common.md + Code.md + Writing.md) has three structural problems:

1. **Cannot be recreated by the wizard.** Only ~20% of the content is personal (name, domains, autonomy level). The other 80% is governance machinery that the wizard has no questions for — meaning `ai setup` produces an incomplete constitution for new users.

2. **Context load is unmanaged.** The four files total ~22,500 tokens. At 200K context (Copilot CLI, standard Claude), that is 11% of the window consumed before the user types a word. There is no compact path for constrained tools.

3. **Domains are hardcoded.** Exactly two domain files exist. A data scientist, lawyer, or researcher has no path to a third domain without manually editing governance files the wizard doesn't know about.

A secondary problem: the behavioral rules governing sycophancy, directness, and conviction are scattered across P5, U2, and Writing §1.3. They exist but are invisible as a named section — making them easy to miss and hard to amend.

---

## Decisions

| Question | Decision |
|---|---|
| Wizard output | Generates full Constitution.md (not just a thin personalization layer) |
| Invariant machinery | Bundled verbatim — no questions asked, always included |
| Domain count | Arbitrary — user names and creates any number of domain sections |
| Document shape | Single unified Constitution.md with numbered sections |
| Context strategy | Full doc for Claude Code (1M ctx); auto-generated compact runtime for constrained tools |
| Behavioral rules | Promoted to a named §2 Behavioral Standards section |
| Drift detection | First-class alongside violations and overrides |
| Cross-references | Section numbers only — no file prefix (§2.U17 not Common.md §U17) |

---

## Section 1 — Document Architecture

### 1.1 Unified Constitution.md Structure

All four current files collapse into one document at `~/.ai/Constitution.md`:

```
§1  Governance                    [invariant]
    Override protocol, audit log schema, amendment process,
    inheritance rules, tool integration conventions, RFC 2119 keywords

§2  Behavioral Standards          [invariant core + personal overlay]
    §2.1  Conviction
    §2.2  Directness
    §2.3  Uncertainty
    §2.4  Disagreement
    §2.5  Helpfulness

§3  Universal Rules               [invariant core + thin personal overlay]
    Prime Directives (P1–P5), Autonomy Gates, Operating Rules (U1–U17+),
    Secret Handling, Governance Storage
    Personal overlay: principal name, cost ceiling, tool selection,
    protected branches, attribution preference

§4–N  Domain Sections             [invariant skeleton + full personal content]
    Each domain has:
      - Preamble: what work this section governs
      - Invariant floor: non-negotiable rules for this domain
      - Personal rules: domain-specific norms authored by the principal
    
    Standard domains (names are user-chosen):
      §4  Technical Work     (previously Code.md)
      §5  Prose Work         (previously Writing.md)
      §N  [any additional domains — Data, Legal, Research, etc.]
```

### 1.2 Section Numbering

Cross-references use `§<top>.<sub>` within the document:

- `§3.U17` — previously `Common.md §U17`
- `§4.11.2` — previously `Code.md §11.2`
- `§2.1` — new Conviction rule (no prior equivalent)

The amendment log uses the same numbering. `ai amend draft --section §3.U17` targets one precise location.

### 1.3 Tool Loading

```
CLAUDE.md (Claude Code):   @~/.ai/Constitution.md
Copilot instructions:      symlink → ~/.ai/Constitution.runtime.md
Cursor rules:              symlink → ~/.ai/Constitution.runtime.md
```

One @-include replaces four. All derived integration files are managed by the CLI — `ai hooks install` writes them; `ai doctor` validates them.

---

## Section 2 — Behavioral Standards

This section is new. It promotes and consolidates the anti-sycophancy rules currently scattered across §3 P5, §3 U2, and §5 §1.3.

### Why a named section

Behavioral rules are the most visible part of the constitution to the user. When an AI flatters instead of corrects, the user sees the failure immediately — but the rule enforcing it is buried three levels deep. Promoting these rules to §2 makes them the first substantive section after governance, signals their importance, and makes them easy to find, amend, and hold the AI accountable to.

### §2.1 Conviction (invariant floor + personal overlay)

**Invariant:**
> Agreement is not the goal — correctness is. Sycophancy is a form of dishonesty: it tells the principal what they want to hear rather than what is true. The AI MUST NOT fabricate agreement, soften a true answer to avoid friction, or add qualifiers it does not mean. Performative pushback is the mirror violation: equally dishonest.

**Personal overlay — pushback persistence:**
```
Push until explicitly told "noted, move on"
Push once clearly, then defer
Flag disagreement once, then follow the principal's lead  ← default
Flag only when the error is safety-critical
```

### §2.2 Directness (invariant)

No preamble restating the prompt. No closing summary of what was just said. No `"Great question!"` or `"Certainly!"`. Lead with the answer.

**Personal overlay — response length default:**
```
Shortest path to the answer
Short with reasoning visible
Full reasoning shown  ← default for complex work
Match complexity of the request
```

### §2.3 Uncertainty (invariant, no overlay)

> When the AI does not know, it says so. Confident phrasing applied to uncertain content is a form of fabrication. `"I don't know"` and `"I'm guessing, but..."` are correct responses, not failures.

No personal overlay. This rule is non-negotiable.

### §2.4 Disagreement (invariant floor + personal overlay)

**Invariant:**
> Disagreement MUST be surfaced before complying, not after. A disagreement disclosed after execution is a disclosure, not a warning. If the AI believes an instruction is wrong, it says so first, then — if the principal confirms — executes.

**Personal overlay — disagreement tone:**
```
Direct, no softening ("That approach has a flaw:")
Direct with framing ("I'd push back on this:")  ← default
Collaborative framing ("One concern worth considering:")
```

### §2.5 Helpfulness (invariant, no overlay)

> Helpfulness is compliance with the principal's *actual intent*, not their stated request. When those diverge — when what was asked would undermine what is wanted — the AI raises the gap. Compliance that buries the gap is not helpfulness; it is flattery dressed as execution.

This is the root-cause definition. It explains why §2.1–§2.4 exist. No personal overlay — this definition is why the section exists.

---

## Section 3 — Wizard Design

The wizard generates a complete `Constitution.md` in ~15 questions across five phases. It never asks about governance machinery — only the personal layer.

### 3.1 Question Flow

```
Phase 1: Identity (~2 min, 3 questions)
  Q1  Your name / handle              → §3 Principal field, audit logs
  Q2  Primary AI tools                → §1 Tool Integration table
  Q3  Work context (org, project)     → §3 U4 provenance preference

Phase 2: Domains (~3 min, 3 questions)
  Q4  What kinds of work do you do?   → Creates §4, §5, §N section stubs
        (multi-select + free text)
  Q5  Name each domain                → Section title + preamble
  Q6  Key norms for each domain       → Domain personal rules block
        (chat-assisted — not a fixed option list)

Phase 3: Autonomy (~3 min, 4 questions)
  Q7   Cost ceiling per task          → §3 Autonomy Gate §3.6
  Q8   File-count blast radius        → §3.6
  Q9   Protected branch names         → §3.2 + branch-guard.json
  Q10  Destructive action posture     → §3 Autonomy posture

Phase 4: Behavioral Style (~2 min, 3 questions)
  Q11  Pushback persistence           → §2.1 personal overlay
  Q12  Response length default        → §2.2 personal overlay
  Q13  Disagreement tone              → §2.4 personal overlay

Phase 5: Review (not a question — shown)
  Full Constitution.md rendered in TUI
  User edits inline before writing to disk
```

### 3.2 What the Wizard Pre-fills

The wizard writes the following sections verbatim from template, without asking:

- All of §1 Governance (override format, audit schema, RFC 2119 keywords, amendment protocol)
- All of §2 Behavioral Standards invariant floors
- All of §3 invariant rules (U1–U17, Secret Handling §4, Governance Storage §5)
- Domain section skeletons (Clean Code §4.1, Testing §4.3 for a Technical domain, etc.)
- Changelogs seeded with `v1.0 — initial wizard generation, <date>`

### 3.3 Chat-Assisted Domain Rules (Q6)

Q6 opens a short conversation rather than a fixed option list. The user describes their domain norms in plain language; the assistant drafts rule text; the user approves or revises. This produces rationale-first rules — each rule explains the failure mode it prevents before stating the constraint.

Rationale-first format:
```
[Failure mode] is [consequence]. Therefore: [rule].

Example:
Mocked integration tests passed while the production migration failed
because the mock didn't model the real schema constraints. Therefore:
integration tests MUST hit a real database; mocks are forbidden for
DB-backed acceptance criteria.
```

This design is grounded in Anthropic's finding that natural-language rules written with explicit motivation generalize better to novel situations than option-selected rules.

---

## Section 4 — Compact Runtime

### 4.1 What It Is

`Constitution.runtime.md` is a ~3,000-token derived document auto-generated from `Constitution.md`. It is never hand-edited.

```
Constitution.runtime.md
├── §1 header          Principal, tools, version
├── §2 full            Behavioral Standards (invariant only, ~800 tokens)
├── §3 condensed       Prime Directives P1–P5 + Autonomy Gates §3.1–§3.6
└── §N domain lines    One paragraph per domain stating scope +
                       any rules that strengthen §3 (e.g. protected branches)
```

### 4.2 Tool Loading Strategy

| Tool | Context window | Loads | Tokens |
|---|---|---|---|
| Claude Code | 1M | `Constitution.md` | ~22,500 |
| Copilot CLI | 200K | `Constitution.runtime.md` | ~3,000 |
| Cursor | 200K | `Constitution.runtime.md` | ~3,000 |
| Any tool (review/amend session) | Any | `Constitution.md` | ~22,500 |

### 4.3 CLI Management

```bash
ai generate runtime          # regenerate Constitution.runtime.md
ai doctor                    # check 11: runtime is current (hash match)
ai update                    # regenerates runtime after any ai amend apply
```

`ai doctor` check #11 (new): compares a hash of the sections that feed the runtime against the runtime file. Stale runtime = `[⚠]` warning, not `[✗]` error (runtime is advisory, not authoritative).

---

## Section 5 — Drift Detection

Drift detection extends the existing violation/override audit system with a third record type and a proactive review layer.

### 5.1 Three Audit Record Types

```
~/.ai/audit/
├── violations/     Rule was broken         [exists today]
├── overrides/      Rule was explicitly relaxed  [exists today]
└── drift/          Rule was nearly triggered or pattern detected  [new]
```

Drift records are written by hooks and the `ai review` cycle — not by the AI itself. They are advisory, not blocking.

### 5.2 Drift Record Triggers

| Trigger | Example |
|---|---|
| Near-miss on a quantitative gate | Blast radius hit 98 files; limit is 100 |
| Same rule invoked ≥3 times in one session without formal gate | Branch-guard checked but passed 4 times |
| Pattern cluster across sessions | §3.U17 near-triggered in 5 of last 10 sessions |
| `ai review` behavioral analysis | Responses trending longer despite U7 terse setting |

### 5.3 Drift Record Format

```markdown
# Drift — <UTC timestamp>

- **Rule:** §<ref> — <name>
- **Trigger:** near-miss | pattern | cluster | behavioral
- **Evidence:** <what was observed>
- **Sessions affected:** <count or date range>
- **Proposed action:** strengthen enforcement | add hook | acknowledge variance | amend rule
```

### 5.4 The Review Cycle (`ai review`)

Default cadence: 30 days. Produces a Governance Report at `.ai/governance/reports/<date>.md`.

```
Four scans:

1. Violation scan
   Rules that broke → propose amendments to the rule or add enforcement hooks

2. Override scan
   Rules relaxed ≥2 times → propose either tightening or promoting the
   relaxation to a permanent policy change

3. Drift scan
   Near-miss clusters → flag for enforcement review; propose §G4-style
   hook additions for patterns that recur

4. Dead-rule scan
   Rules not referenced in any audit record in 90 days → flag as
   candidates for pruning or consolidation into a general principle
```

### 5.5 Amendment Pathway

```
audit/violations/ ─┐
audit/overrides/  ─┼──► ai review ──► Governance Report ──► ai amend draft ──► ai amend apply
audit/drift/      ─┘
```

`ai amend apply` updates the constitution, bumps the version, appends the changelog, and triggers `ai generate runtime` to keep the compact file current.

---

## Section 6 — Migration Path

For existing users (including the current `~/.ai` installation).

### 6.1 Migration Steps

```bash
ai migrate --flatten
```
Merges Constitution.md + Common.md + Code.md + Writing.md into a single `~/.ai/Constitution.md`. Section mapping:

| Old file | New section |
|---|---|
| Constitution.md | §1 |
| Common.md §1 (Prime Directives) | §3 (Universal Rules) |
| Common.md §2–§5 | §3 |
| Code.md | §4 |
| Writing.md | §5 |

All cross-references rewritten. `Common.md §U17` → `§3.U17`. The original files are archived to `.ai/archive/pre-v2/` (not deleted).

```bash
ai migrate --add-behavioral
```
Inserts §2 Behavioral Standards. Populated from existing clauses:
- §2.1 Conviction ← from old Common.md §1 P5
- §2.4 Disagreement ← from old Common.md §3 U2
- §2.2 Directness ← from old Writing.md §1.3 (AI tells list)

These rules are not new — they are promoted and given a name.

```bash
ai migrate --generate-runtime
```
Produces `Constitution.runtime.md`. Updates CLAUDE.md to a single @-include. Updates Copilot and Cursor symlinks.

### 6.2 Backward Compatibility

The amendment log carries forward verbatim. Old section numbers appear in the log as `[pre-v2: Common.md §U17]` — preserved for audit continuity, replaced in the live document.

---

## Section 7 — Research Foundations

Four Anthropic papers validate the architecture and contribute two concrete design decisions:

### 7.1 Design Decisions from Research

**Decision: Rationale-first rule writing** (all four papers)

Every constitutional rule should state the failure mode it prevents before stating the constraint. This is embedded in the wizard's Q6 (chat-assisted domain rules) and in the invariant boilerplate template.

> Sourced from: *Claude's New Constitution* — "Principles work best when they explain motivation, not just behavior. An AI that understands the *why* generalizes better to novel situations."

**Decision: Dead-rule pruning in the review cycle** (Specific vs. General Principles)

> "A finite rule list fails the moment a situation falls outside it." Rules that haven't fired in 90 days are either redundant (covered by a general principle already) or stale (the work has changed). The `ai review` dead-rule scan flags these.

### 7.2 Website Research Section

Include a "Research foundations" section on `aiConstitution.convergent-systems.co` with these four links and one-sentence framing:

| Article | Framing |
|---|---|
| [Next-Generation Constitutional Classifiers](https://www.anthropic.com/research/next-generation-constitutional-classifiers) | How layered, principle-based enforcement outperforms reactive pattern matching — the empirical case for explicit constitutional rules |
| [Claude's New Constitution](https://www.anthropic.com/news/claude-new-constitution) | Anthropic's own rationale for why AI needs explicit governing documents rather than implicit training alone |
| [Constitutional Classifiers](https://www.anthropic.com/research/constitutional-classifiers) | Writing the constitution first, then building enforcement, produces measurably more robust AI behavior |
| [Specific vs. General Principles for Constitutional AI](https://www.anthropic.com/research/specific-versus-general-principles-for-constitutional-ai) | Why a foundational general principle plus targeted specific rules outperforms either approach alone — the theoretical basis for the invariant core + personal overlay architecture |

---

## Summary of Changes

| Area | Current | v2 |
|---|---|---|
| Files | 4 (Constitution, Common, Code, Writing) | 1 (Constitution.md) + 1 derived (runtime) |
| @-includes | 4 lines in CLAUDE.md | 1 line |
| Behavioral rules | Scattered in P5, U2, Writing §1.3 | Named §2 Behavioral Standards |
| Domain count | Hardcoded 2 | Arbitrary, user-named |
| Context (constrained tools) | ~22,500 tokens | ~3,000 tokens |
| Drift detection | None | `audit/drift/` + 4-scan review cycle |
| Wizard output | Partial personalization | Full Constitution.md |
| Rule format | Rule stated, why implied | Failure mode → rule (rationale-first) |
| Migration | N/A | `ai migrate --flatten --add-behavioral --generate-runtime` |

---

## Out of Scope

- Multi-user constitutions (each user has their own `~/.ai/`)
- Cloud sync of audit logs (local only per §3.5.2)
- Model-specific rule variants (one constitution governs all tools)
- Automatic amendment application without principal approval
