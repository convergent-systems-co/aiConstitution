# Plan: Persona & Panel System (#250–#255)

**Objective:** Implement persona YAML loading, panel JSON loading, weighted panel scoring, and `ai review --pr <n>` diff-based review report.

**Epic:** #26 Persona & Panel System

---

## Alternatives Table

| Alternative | Pros | Cons | Verdict |
|---|---|---|---|
| Custom YAML unmarshaler for domain/domains normalization | Idiomatic Go, single parse pass | Slightly more code | Chosen |
| Pre-parse YAML bytes with regex/string manipulation | Simpler initial approach | Fragile, breaks on multiline YAML | Rejected |
| Separate loaders for agentic vs. reviewer personas | Clear separation | Code duplication for shared fields | Rejected — single `LoadPersonas` with type filtering |
| Embed panels.defaults.json via `//go:embed` | No file dependency at runtime | Requires embed in cmd/ai module, not internal | Chosen for internal package; fallback to file read |

---

## Scope

**Files to create:**
- `src/internal/personas/personas.go` — PersonaFile, LoadPersonas (#250, #251, #252)
- `src/internal/personas/personas_test.go` — tests for #250, #251, #252
- `src/internal/panels/panels.go` — Panel, LoadDefaultPanels, PanelResult, ScorePanels (#253, #254)
- `src/internal/panels/panels_test.go` — tests for #253, #254
- `src/internal/panels/panels.defaults.json` — minimal 3-panel defaults

**Files to modify:**
- `src/cmd/ai/cmd/review.go` — add `--pr <n>` flag and panel report (#255)
- `src/cmd/ai/go.mod` — no new deps (using standard library only for JSON; yaml dep added to src/internal/go.mod)
- `src/internal/go.mod` — add `go.yaml.in/yaml/v3`

---

## Approach

1. **TDD Writer** — write failing tests in personas_test.go and panels_test.go
2. **Coder A** — implement personas.go (LoadPersonas, custom domain unmarshaler)
3. **Coder B** — implement panels.go + panels.defaults.json (LoadDefaultPanels, ScorePanels)
4. **Coder C** — extend review.go with --pr flag (depends on A+B completing)
5. **Adversarial Tester** — verify GREEN + no deception

---

## Testing Strategy

- `LoadPersonas`: temp dir with fixture YAML files; verify correct field parsing
- `domain`/`domains` normalization: two fixture files, one per form
- `LoadPersonas` skip-on-error: fixture with invalid YAML alongside valid ones
- `LoadDefaultPanels`: embedded JSON, verify panel count and weight sum ~1.0
- `ScorePanels`: 2 pass + 1 fail, verify weighted arithmetic
- `ai review --pr`: mock gh subprocess, verify stdout report format

---

## Risk Assessment

- **yaml dependency not in go.mod** — mitigated by adding `go.yaml.in/yaml/v3` to `src/internal/go.mod`
- **cmd/ai module doesn't know about internal/personas** — cmd/ai is a separate module; review.go will use os/exec for gh and fmt for output only; no import from internal required for #255
- **panels.defaults.json embed in internal module** — use `//go:embed` in panels package; requires Go 1.16+ (workspace uses 1.22, safe)

---

## Dependencies

- #252 (domain/domains normalization) is a prerequisite for #250 and #251 — solved in the same `personas.go` file
- #253 (LoadDefaultPanels) is a prerequisite for #254 (ScorePanels) — same file
- Coder C (#255) depends on the types from A+B being importable — review.go stubs panel invocation; actual import handled at integration time (review.go stays self-contained for this PR)

---

## Backward Compatibility

- New packages only; no existing package modified
- `review.go` gains a new `--pr` flag; existing flags unchanged

---

## Out of Scope

- Live HTTP fetch of persona atoms
- Actual panel logic against real diff content (panels print placeholder scores)
- `ai review` existing memory-to-amendment behavior unchanged
