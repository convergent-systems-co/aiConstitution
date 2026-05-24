# Plan: TL1 Wave 2 — Audit + Memory Implementation

**Branch:** task/tl1-wave2
**Tasks:** #168, #169, #170
**Owner:** Tech Lead 1 (domain:internal)

## Objective

Implement the three stub functions that form the real audit and memory write path:
1. `audit.AppendEvent()` — JSONL interaction log writer
2. `audit.WriteViolation()` — violation markdown writer
3. `ai memory codify <path>` — promote a violation file to a memory finding

## Scope

| File | Action |
|---|---|
| `src/internal/audit/audit.go` | Replace `Append` stub with `AppendEvent`; add `WriteViolation` |
| `src/internal/audit/audit_test.go` | New — full test suite (written first) |
| `src/internal/memory/memory.go` | New package with `WriteFinding` |
| `src/internal/memory/memory_test.go` | New — full test suite (written first) |
| `src/cmd/ai/cmd/memory.go` | Replace `codify` stub with real implementation |
| `src/cmd/ai/cmd/memory_test.go` | New — integration test for `ai memory codify` |

## Approach

### Step 1 — Write failing tests for #168 + #169 (audit)
- `TestAppendEvent_CreatesJSONLFile`
- `TestAppendEvent_AppendsToExistingFile`
- `TestAppendEvent_PathIncludesYearMonth`
- `TestAppendEvent_MkdirAll`
- `TestWriteViolation_CreatesMarkdownFile`
- `TestWriteViolation_ContainsAllFields`
- `TestWriteViolation_SlugFromRule`

### Step 2 — Implement `AppendEvent` and `WriteViolation`
Both in `audit.go`. Keep existing `Append`/`RecordOverride`/`RecordViolation` stubs (public API surface) or update as needed.

### Step 3 — Write failing tests for #170 (memory)
- `TestWriteFinding_CreatesFile`
- `TestWriteFinding_ContainsFrontmatter`
- `TestWriteFinding_AppendsMEMORY`
- `TestWriteFinding_SlugFromRule`

### Step 4 — Implement `memory.WriteFinding`

### Step 5 — Write test for `ai memory codify`

### Step 6 — Implement `ai memory codify`

## Alternatives Considered

| Alternative | Pros | Cons | Verdict |
|---|---|---|---|
| Keep old stub API names (`Append`, `RecordViolation`) and add new functions | No callers to update | Two APIs for the same thing | Rejected — tasks specify new function names |
| Single file for memory package | Simpler | Violates SRP if memory grows | Accepted for now (small scope) |
| Use `fmt.Fprintf` for JSONL | Simpler | Loses json.Marshal validation | Rejected |

## Testing Strategy

- Use `t.TempDir()` and `AI_ROOT` env override to isolate filesystem writes
- Assert file contents with `os.ReadFile`
- Assert JSON round-trip for `AppendEvent`
- Assert markdown structure for `WriteViolation` and `WriteFinding`

## Risk Assessment

- `gosec G306`: file permissions flagged — mitigate with `//nolint:gosec` comments per lint rules
- Slug truncation at 32 chars: must handle rules shorter than 32 chars (no panic)

## Dependencies

- #169 depends on #168 types (both in same file — no import dependency)
- #170 depends on #168/#169 for the violation file format to parse

## Backward Compatibility

- `Append`, `RecordOverride`, `RecordViolation` are stubs with no callers beyond tests — safe to update signatures or keep as wrappers
