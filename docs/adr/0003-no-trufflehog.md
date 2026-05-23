# 0003. No trufflehog; `patterns.json` is the canonical secret pattern source

- Status: accepted
- Date: 2026-05-23
- Source: `SPEC.md §10.1`, `§10.4`

## Context

The original repo scaffold (from `convergent-systems-co/go-app-template`)
shipped a `secret-scan` GitHub Actions workflow built on `trufflehog`.
`SPEC.md §10` rejects this approach.

The user's requirement: no trufflehog. Their preferred approach: a
single canonical, versioned, auditable regex set
(`hooks/patterns.json`) consumed by **every** enforcement point — the
PreToolUse hook, the git pre-commit hook, the pre-sync scan, and the
pre-submission redaction in `ai issue file`.

## Decision

1. **Replace** `.github/workflows/secret-scan.yml`'s trufflehog
   invocation with a job that runs the `patterns.json` matcher
   against the diff.
2. **Ship** `hooks/patterns.json` as the single canonical pattern
   source.
3. **Optional CI net** — `gitleaks` is the documented opt-in for
   users who want server-side defense-in-depth
   (`settings.secret_scanning.ciScanner = "gitleaks"`). Not enabled
   by default in this repo.

## Consequences

- The pattern set is auditable: every regex is a deliberate decision,
  versioned, reviewable.
- Three enforcement points (pre-commit, PreToolUse, pre-sync) read
  the same patterns. A new pattern lands in one place and protects
  everywhere.
- False-positive rate is bounded by the regexes we ship; trufflehog's
  entropy heuristic produces false positives on UUIDs, hashes, and
  generated identifiers that the user's prose and code routinely
  contain.
- Lighter than trufflehog: no Go dependency, no service auth checks,
  no Docker pulls.
- If a finding later shows the regex approach misses a class of
  secrets that trufflehog would catch: write the missing pattern
  into `patterns.json` and file it upstream per `SPEC.md §9`.

## Alternatives considered

- **Trufflehog (status quo from scaffold).** Rejected per spec §10.4 —
  heavier than needed, opaque match logic, false positives on
  high-entropy non-secret content.
- **gitleaks as default.** Rejected — same multi-tool problem; the
  single-source-of-truth `patterns.json` is the spec's preferred
  invariant. Gitleaks is fine as an opt-in CI net.
- **No secret scanning in CI.** Rejected — defense-in-depth matters
  when local pre-commit hooks can be bypassed.
