# 0000. Record architecture decisions

- Status: accepted
- Date: 2026-05-22

## Context

This project records architecture decisions as ADRs (MADR format) so that the
rationale survives even after the decision is normal.

## Decision

We will record each significant architecture decision in `docs/adr/` with a
sequence number. ADRs are immutable once accepted; we supersede rather than
edit.

## Consequences

- Future contributors can read the why, not just the what.
- The cost is small: one short markdown file per decision.
- We commit to keeping the index in sync as decisions land.

## Alternatives considered

- No ADRs. Rejected — context loss as team turns over.
- Pull-request description only. Rejected — not searchable; PR descriptions drift.
