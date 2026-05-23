# 0004. Markdown issue templates in v0.8; YAML Issue Forms deferred

- Status: accepted
- Date: 2026-05-23
- Source: `SPEC.md §14.3`, `§19` (v0.8 changelog), Q-O resolution table

## Context

The repo scaffold shipped GitHub YAML Issue Forms
(`.github/ISSUE_TEMPLATE/*.yml`: `bug`, `chore`, `rfc`, `feature`,
`config`). The spec mandates a different set:
`epic`, `feature`, `story`, `task`, `hook`, `finding` — and v0.7 of
the spec proposed converting these to YAML Issue Forms, which v0.8
**defers**.

The deferral is deliberate: YAML Issue Forms warrant their own filed
issue and design pass (field types, dropdowns, validation), and are
not a blocker for shipping atoms, plugins, or the v0.8 surface.

## Decision

1. **Ship six Markdown templates** at `.github/ISSUE_TEMPLATE/`:
   `epic.md`, `feature.md`, `story.md`, `task.md`, `hook.md`,
   `finding.md`.
2. **Retain `config.yml`** — it carries the `security@convergent-systems.co`
   contact link, which is unrelated to the form-vs-markdown question.
3. **Leave the legacy `bug.yml`, `chore.yml`, `rfc.yml`,
   `feature.yml`** in place for now. They contradict the spec's
   epic/feature/story/task taxonomy and SHOULD be removed in a
   follow-up commit, but deletion is gated by `Common.md §2.2` and
   is appropriately a separate, principal-approved step.
4. **File a `feature` issue** against this repo to track the YAML
   conversion as a v1.x candidate.

## Consequences

- Submitters get the spec's intended taxonomy
  (epic → feature → story → task hierarchy plus hook + finding).
- The free-form Markdown templates capture full context; users
  can include code, audit references, and motivation prose.
- Future YAML conversion will improve structured submission but is
  not blocking v0.8.
- The repo carries duplicate template sets (legacy + spec) until
  the legacy ones are removed by the principal. The legacy
  templates are functional, just off-spec.

## Alternatives considered

- **Ship YAML Issue Forms now.** Rejected per spec §14.3 — deferred
  to v1.x. The field design needs its own pass.
- **Delete the legacy templates as part of this change.** Rejected —
  deletion is in `~/.ai/Common.md §2.2` and warrants explicit
  approval. Leaving them in place is the safer interim state.
- **Strip `config.yml`.** Rejected — it's the security-disclosure
  contact link, not a form template.
