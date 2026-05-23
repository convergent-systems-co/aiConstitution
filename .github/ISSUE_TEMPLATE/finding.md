---
name: Finding
about: A recurring pattern, gap, or behavior worth tracking — not yet a hook
title: "[Finding] "
labels: ["kind/finding", "status/triage"]
---

<!-- File this when a pattern recurs in a way that warrants tracking
     but isn't yet a hook proposal. See SPEC.md §9.4. Findings can
     escalate to hooks (file a `hook` issue), to spec amendments (PR
     against SPEC.md), or stay as observations. -->

## Summary

<!-- One sentence. What is the recurring pattern? -->

## Severity

- [ ] **Major** — affects core guarantees (correctness, security, audit
      trail integrity); recurring; or principal-flagged.
- [ ] **Minor** — quality-of-life or efficiency; not a guarantee failure.

## Recurrence

<!-- How many times observed? Across how many sessions / days? -->

## Originating audit records

<!-- Paths inside ~/.ai/audit/violations/, ~/.ai/audit/overrides/, or
     ~/.ai/memory/. Redacted excerpts only — per ~/.ai/Common.md §4.5
     redact secrets, employer names matching Q04, and any
     local-only-tagged content before pasting. -->

## Proposed remediation

<!-- Pick zero or more. -->

- [ ] No remediation — just observation.
- [ ] Spec amendment (`SPEC.md`, four-file constitution, or
      `governance/policy/*.json`).
- [ ] New hook (file a `hook` issue).
- [ ] Existing hook strengthening.
- [ ] Other (specify).

## Context

<!-- Any additional context the maintainers should know. -->
