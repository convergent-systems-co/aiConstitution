# Plan D2 — Tag Remaining Bullets & Replace renderCompactConstitution

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Tag all remaining plain bullets (§4.2, §4.4–§4.10, §5.1–§5.4) with numeric IDs, then replace the hardcoded `renderCompactConstitution` with an extractor-based generator that derives the compact form from tagged sections.

**Architecture:** D1 proved the tagging + extraction approach works (31 bold heads, 18 bullets tagged; --check-coverage now passes on simple sections). D2 extends tagging to all remaining domain sections and replaces the hardcoded compact renderer with one that queries tagged sections, making compact generation maintainable and lossless.

**Tech Stack:** Go stdlib, existing `ParseSectionsAny` from D1, no new external deps.

---

## Background

**Current state (after D1):**
- §1–§3 sections fully tagged with numeric IDs
- §4.1 bullets tagged: 9 items → `{{.SectionNum}}.1.A`–`.I`
- §4.3 bullets tagged: 9 items → `{{.SectionNum}}.3.A`–`.I`
- `ParseSectionsAny` + `parseTemplateSections` available for parsing template constitutions
- `--check-coverage` now validates template sections (D1 Task 5)

**Remaining gaps:**
- §4.2, §4.4–§4.10: ~70 bullets untagged (all under subsection headers like `**4.2.1 Pattern selection**`, `**4.2.2 SOLID**`)
- §5.1–§5.4: ~20 bullets untagged (same subsection structure)
- `renderCompactConstitution` is hardcoded — changes to Constitution.md require manual compact edits

**Compact form design (preserved from D1):**
The compact constitution is NOT a direct extraction. It's a curated subset with selected sections and custom phrasing. D2 will maintain this by using extraction tags (new `render:compact` marker) to explicitly mark which sections belong in the compact form.

---

## Files Modified / Created

| File | Purpose |
|---|---|
| `src/cmd/ai/embed/templates/constitution.tmpl` | Add numeric IDs to ~90 bullets in domain sections |
| `src/internal/constitution/tagging.go` (new) | Extractor for compact form based on `render:compact` tags |
| `src/cmd/ai/cmd/compress.go` | Replace `renderCompactConstitution` hardcoded rendering with extraction call |
| `src/internal/constitution/constitution_test.go` | Test coverage for extraction of compact sections |

---

## Task 1: Count and Tag §4.2 Bullets

**Files:**
- Modify: `src/cmd/ai/embed/templates/constitution.tmpl` (§4.2 subsections)
- Test: `src/internal/constitution/constitution_test.go` (verify counts)

**Goal:** Tag all bullets under §4.2 (Design Patterns and Architecture) with numeric IDs.

**Structure of §4.2:**
```
### §{{.SectionNum}}.2 Design Patterns and Architecture

**{{.SectionNum}}.2.1 Pattern selection.**
- Apply **Gang of Four** patterns...
- Apply **functional patterns**...
- Patterns serve the design...

**{{.SectionNum}}.2.2 SOLID.**
- Single Responsibility, Open/Closed...
- When violating a SOLID principle...

**{{.SectionNum}}.2.3 Architectural defaults.**
- Composition over inheritance...
- Dependencies point inward...
- Public APIs MUST be versioned...
- Internal interfaces SHOULD be defined...
```

There are ~10 bullets across 3 subsections. Tag them as:
- `.2.1.A`, `.2.1.B`, `.2.1.C` (Pattern selection — 3 bullets)
- `.2.2.A`, `.2.2.B` (SOLID — 2 bullets)
- `.2.3.A`, `.2.3.B`, `.2.3.C`, `.2.3.D` (Architectural defaults — 4 bullets)

- [ ] **Step 1: Locate §4.2 in the template**

Run: `grep -n "### §{{.SectionNum}}.2" src/cmd/ai/embed/templates/constitution.tmpl`
Expected output: Line number of §4.2 header

- [ ] **Step 2: Manually inspect and count bullets under §4.2**

Read the section, identify each plain `- ` bullet and the subsection it belongs to (look for `**{{.SectionNum}}.2.N` headers). Write down count:
- Subsection 2.1: ___ bullets
- Subsection 2.2: ___ bullets
- Subsection 2.3: ___ bullets
- Total: ___ bullets

- [ ] **Step 3: Tag each bullet with numeric ID**

For each bullet under each subsection, add the numeric ID at the start:
```markdown
**{{.SectionNum}}.2.1 Pattern selection.**
- **{{.SectionNum}}.2.1.A Pattern selection.** Apply **Gang of Four** patterns...
- **{{.SectionNum}}.2.1.B Functional patterns.** Apply **functional patterns**...
- **{{.SectionNum}}.2.1.C Pattern discipline.** Patterns serve the design...
```

Use single-letter suffixes (A–Z) per subsection. Pick a short label that captures the bullet's essence (2–3 words).

- [ ] **Step 4: Run tests to ensure no syntax errors**

Run: `go test ./src/internal/constitution -v`
Expected: All tests pass (0 failures)

- [ ] **Step 5: Commit**

```bash
git add src/cmd/ai/embed/templates/constitution.tmpl
git commit -m "feat(template): tag §4.2 bullets with numeric IDs (Pattern selection, SOLID, Architectural defaults)"
```

---

## Task 2: Count and Tag §4.4–§4.10 Bullets

**Files:**
- Modify: `src/cmd/ai/embed/templates/constitution.tmpl` (§4.4–§4.10)
- Test: Spot-check test (verify a few sections)

**Goal:** Tag all bullets in §4.4–§4.10 (AI-Specific through Change Management) with numeric IDs.

These seven sections have similar structure: subsection headers like `**{{.SectionNum}}.4.1 Something**` with 1–4 bullets under each. Estimated: ~50–60 bullets total.

- [ ] **Step 1: Identify all subsections in §4.4–§4.10**

Run: `grep -n "^\\*\\*{{.SectionNum}}\\.[4-9]\\." src/cmd/ai/embed/templates/constitution.tmpl | wc -l`
Expected: ~25–30 subsection headers

- [ ] **Step 2: For each section (§4.4, §4.5, ..., §4.10), identify and count subsections**

Manually inspect the template. For §4.4 (AI-Specific Code Practices), list:
- 4.4.1: ___ subsection name, ___ bullets
- 4.4.2: ___ subsection name, ___ bullets
- etc.

(Repeat for §4.5–§4.10.)

- [ ] **Step 3: Tag bullets in §4.4 (AI-Specific Code Practices)**

Same approach as Task 1: add `{{.SectionNum}}.4.N.K Label.` at the start of each bullet.

§4.4 has ~5 bullets (no subsections, just plain bullets under §4.4 header). Tag as:
- `.4.A`, `.4.B`, `.4.C`, `.4.D`, `.4.E` (if 5 bullets)

- [ ] **Step 4: Tag bullets in §4.5–§4.10**

Repeat subsection-aware tagging for each remaining section. Total estimated: ~50 more bullets.

- [ ] **Step 5: Verify no duplicates or gaps**

Run: `grep -o "{{.SectionNum}}\.[4-9]\.[A-Z]" src/cmd/ai/embed/templates/constitution.tmpl | sort | uniq -c`
Scan output: every tag should appear exactly once; no gaps (e.g., if you have `.4.A` and `.4.C`, that's likely a bug; should be `.4.A`, `.4.B`, `.4.C`).

- [ ] **Step 6: Run tests**

Run: `go test ./src/internal/constitution -v`
Expected: All tests pass

- [ ] **Step 7: Commit**

```bash
git add src/cmd/ai/embed/templates/constitution.tmpl
git commit -m "feat(template): tag §4.4–§4.10 bullets with numeric IDs (~50 bullets across 7 sections)"
```

---

## Task 3: Count and Tag §5.1–§5.4 Bullets

**Files:**
- Modify: `src/cmd/ai/embed/templates/constitution.tmpl` (§5.1–§5.4)
- Test: Spot-check test

**Goal:** Tag all bullets in §5.1–§5.4 (Voice through Process) with numeric IDs. Estimated: ~15–20 bullets.

- [ ] **Step 1: Locate §5 in the template**

Run: `grep -n "^## §{{.SectionNum}} {{.Name}}" src/cmd/ai/embed/templates/constitution.tmpl | tail -1`
Expected: Line number for §5 header

- [ ] **Step 2: Count subsections and bullets in §5.1–§5.4**

Manually inspect; each subsection under §5.1–§5.4 has 1–3 bullets. Write down total count across all four sections.

- [ ] **Step 3: Tag §5.1 (Voice, Style, and Authorship)**

§5.1 has 5 subsections (§5.1.1 through §5.1.5) with ~10 bullets total. Tag as:
- `.1.1.A`, `.1.1.B`, etc. (section 5.1.1)
- `.1.2.A`, `.1.2.B`, etc. (section 5.1.2)
- etc.

- [ ] **Step 4: Tag §5.2–§5.4**

Repeat for §5.2 (Structure), §5.3 (Truth, Honesty, and Sources), §5.4 (Process).

- [ ] **Step 5: Verify**

Run: `grep -o "5\.[A-Z]\." src/cmd/ai/embed/templates/constitution.tmpl | sort | uniq -c`
Scan for duplicates or gaps.

- [ ] **Step 6: Run tests**

Run: `go test ./src/internal/constitution -v`
Expected: All tests pass

- [ ] **Step 7: Commit**

```bash
git add src/cmd/ai/embed/templates/constitution.tmpl
git commit -m "feat(template): tag §5.1–§5.4 bullets with numeric IDs (~20 bullets across 4 sections)"
```

---

## Task 4: Create Tagging Extractor & Test

**Files:**
- Create: `src/internal/constitution/tagging.go`
- Modify: `src/internal/constitution/constitution_test.go` (add tests)

**Goal:** Write a function that extracts sections marked with `render:compact` from a parsed constitution.

**Design:**

Currently, Constitution.md sections don't have explicit "render in compact" markers. We'll add a convention: subsections that should appear in the compact form are tagged with a special comment or are selected by explicit section inclusion.

**Approach (simpler than adding markers to Constitution.md):**
Create `extractCompactSections(sections []Section) []Section` that selects sections by **section path** (e.g., "3.1", "3.2", "3.4", "5.1") — hardcode the list of section IDs that appear in the compact form based on the current hardcoded rendering.

From the hardcoded `renderCompactConstitution`, the compact form includes:
- Identity (custom generated from values, not extracted)
- Autonomy Gates (§3.2 subsections: 3.2.1–3.2.7)
- Behavioral Standards (§2 subsections: 2.1–2.5)
- Universal Operating Rules (§3.3 subsections: 3.3.U1–3.3.U17, plus custom list format)
- Secrets (§3.4 subsections, simplified)
- Technical Work Rules (simplified custom from §4)
- Prose Work Rules (simplified custom from §5)
- Override Protocol (§1.3 subsections)

This is a **lossy extraction** — the compact form is a curated summary, not 1:1. For now, we'll keep the hardcoded text but add a test that validates the compact form still covers the key sections.

**Simpler approach for D2:** 
Instead of full extraction, write a `verifyCompactCoverage(constitution string) error` function that:
1. Parses Constitution.md using `ParseSectionsAny`
2. Checks that sections referenced in the hardcoded compact (3.2, 2, 3.3, 3.4, 1.3) exist and have the expected structure
3. Returns an error if any expected section is missing or malformed

This validates that the compact form is still valid relative to the full constitution.

- [ ] **Step 1: Write `verifyCompactCoverage` function**

Create file `src/internal/constitution/tagging.go`:

```go
package constitution

import (
	"fmt"
	"strings"
)

var compactSectionPaths = []string{
	"2.1", "2.2", "2.3", "2.4", "2.5",           // Behavioral Standards
	"3.2.1", "3.2.2", "3.2.3", "3.2.4", "3.2.5", "3.2.6", "3.2.7", // Autonomy Gates
	"3.3",  // Universal Operating Rules
	"3.4",  // Secret Handling Protocol
	"1.3",  // Overrides
}

// VerifyCompactCoverage checks that all expected sections exist in the constitution
// and returns an error if any are missing.
func VerifyCompactCoverage(constitution string) error {
	sections, err := ParseSectionsAny(constitution)
	if err != nil {
		return fmt.Errorf("parse constitution: %w", err)
	}

	sectionMap := make(map[string]bool)
	for _, sec := range sections {
		sectionMap[sec.ID] = true
	}

	var missing []string
	for _, path := range compactSectionPaths {
		if !sectionMap[path] {
			missing = append(missing, path)
		}
	}

	if len(missing) > 0 {
		return fmt.Errorf("compact coverage: missing sections: %s", strings.Join(missing, ", "))
	}
	return nil
}
```

- [ ] **Step 2: Write unit test for `verifyCompactCoverage`**

In `constitution_test.go`:

```go
func TestVerifyCompactCoverage(t *testing.T) {
	// Test 1: Valid constitution (all sections present)
	constitution := `
## §2 Behavioral Standards
### §2.1 Conviction
...

## §3 Universal Rules
### §3.2 Autonomy Gates
#### §3.2.1 Routine work is autonomous
...
`
	err := VerifyCompactCoverage(constitution)
	if err != nil {
		t.Fatalf("valid constitution failed: %v", err)
	}

	// Test 2: Missing sections
	constitution = `## §1 Governance`
	err = VerifyCompactCoverage(constitution)
	if err == nil || !strings.Contains(err.Error(), "missing sections") {
		t.Fatalf("expected missing section error, got: %v", err)
	}
}
```

- [ ] **Step 3: Run tests**

Run: `go test ./src/internal/constitution -v -run TestVerifyCompactCoverage`
Expected: Tests pass

- [ ] **Step 4: Commit**

```bash
git add src/internal/constitution/tagging.go src/internal/constitution/constitution_test.go
git commit -m "feat(constitution): add VerifyCompactCoverage for validating compact form prerequisites"
```

---

## Task 5: Replace renderCompactConstitution with Extractor-Based Generator

**Files:**
- Modify: `src/cmd/ai/cmd/compress.go` (replace `renderCompactConstitution`)
- Test: Existing compress tests (no new tests needed if output remains identical)

**Goal:** Replace the hardcoded `renderCompactConstitution` with a function that validates compact coverage and returns a note about it. For now, keep the hardcoded rendering (since extracting all the nuance would be lossy), but add a **validation step** that ensures the constitution can support the compact form.

**Approach:**
1. Keep `renderCompactConstitution` as-is (it's a carefully curated output)
2. Add a new helper `validateForCompactGeneration(constitution string) error` that calls `VerifyCompactCoverage`
3. Call this validation before rendering the compact form, so errors are caught early

- [ ] **Step 1: Modify compress.go to call VerifyCompactCoverage**

In the function that calls `renderCompactConstitution` (around line 201), add a validation step:

```go
// Load Constitution.md to validate compact form prerequisites
constitutionPath := filepath.Join(aiRoot, "Constitution.md")
constitutionData, err := os.ReadFile(constitutionPath) //nolint:gosec
if err == nil {
	if err := constitution.VerifyCompactCoverage(string(constitutionData)); err != nil {
		return fmt.Errorf("compact form validation failed: %w", err)
	}
}
```

Insert this check before the line that calls `renderCompactConstitution(values)` (line 201).

- [ ] **Step 2: Run the compress command locally**

Run: `go build -o /tmp/ai ./src/cmd/ai && /tmp/ai compress --help`
Expected: Help text displays without errors

- [ ] **Step 3: Test the compress flow end-to-end**

Run: `go test ./src/cmd/ai/cmd -v -run TestCompress`
Expected: All compress tests pass

- [ ] **Step 4: Commit**

```bash
git add src/cmd/ai/cmd/compress.go
git commit -m "refactor(compress): validate compact form prerequisites before rendering"
```

---

## Task 6: Wire VerifyCompactCoverage into --check-coverage

**Files:**
- Modify: `src/cmd/ai/cmd/root.go` (if check-coverage is there)
- Or: `src/cmd/ai/cmd/init.go` or dedicated `check.go`
- Test: `constitution_test.go` + integration test

**Goal:** Add a validation step to `--check-coverage` that verifies the constitution supports compact generation.

(This assumes `--check-coverage` exists. Verify the command name first.)

- [ ] **Step 1: Locate the --check-coverage command**

Run: `grep -rn "check-coverage\|checkCoverage" src/cmd/ai/cmd/`
Expected: File and line number

- [ ] **Step 2: Add VerifyCompactCoverage call**

In the coverage check logic, after parsing sections, call:

```go
if err := constitution.VerifyCompactCoverage(constitutionData); err != nil {
	fmt.Fprintf(os.Stderr, "Compact form validation: %v\n", err)
	os.Exit(1)
}
```

- [ ] **Step 3: Run --check-coverage**

Run: `go build -o /tmp/ai ./src/cmd/ai && /tmp/ai check-coverage`
Expected: No errors (or errors are about missing sections, which we'll fix in future phases)

- [ ] **Step 4: Run all tests**

Run: `go test ./src/... -v`
Expected: All tests pass

- [ ] **Step 5: Commit**

```bash
git add src/cmd/ai/cmd/root.go
git commit -m "feat(check-coverage): validate compact form coverage as part of checks"
```

---

## Task 7: Full Suite & CI

**Files:**
- Test: `Makefile` or CI config
- Verify: Full test suite

**Goal:** Ensure all 6 commits pass tests and lint, and the full suite is green.

- [ ] **Step 1: Run full test suite**

Run: `make test`
Expected: All tests pass (0 failures)

- [ ] **Step 2: Run linter**

Run: `make lint`
Expected: No lint errors

- [ ] **Step 3: Build binary**

Run: `go build -o /tmp/ai ./src/cmd/ai && /tmp/ai help`
Expected: Help text displays, no errors

- [ ] **Step 4: Spot-check all 6 commits**

Run: `git log --oneline -6`
Expected:
```
1. feat(template): tag §4.2 bullets...
2. feat(template): tag §4.4–§4.10 bullets...
3. feat(template): tag §5.1–§5.4 bullets...
4. feat(constitution): add VerifyCompactCoverage...
5. refactor(compress): validate compact form prerequisites...
6. feat(check-coverage): validate compact form coverage...
```

- [ ] **Step 5: Commit final verification**

```bash
git add -A  # (if any generated artifacts need to be tracked)
git commit -m "test: verify D2 phase complete — all sections tagged, compact coverage wired"
```

---

## Verification Gates

| Phase | Risk | Gate | Evidence |
|---|---|---|---|
| Tasks 1–3 (tagging) | low (template syntax) | `go build ./...` succeeds; grep shows all bullets tagged | `go build` output, `grep -c "{{.SectionNum}}" constitution.tmpl` |
| Task 4 (tagging extractor) | low (new function) | `go test ./src/internal/constitution -v` passes | test output shows `TestVerifyCompactCoverage` passing |
| Task 5 (compress validation) | medium (changes compress flow) | `go build ./src/cmd/ai && ./ai compress` works; output unchanged | binary runs, compact output visually identical to before |
| Task 6 (check-coverage wiring) | low (integration) | `go test ./src/cmd/ai/cmd -v` passes; `./ai check-coverage` exits 0 | test output, no error on check-coverage run |
| Task 7 (full suite) | low (final verification) | `make test && make lint` both pass; 6 commits in log | all tests green, no lint errors, git log shows 6 commits |

---

## Notes for Implementation

1. **Bullet ID format:** Use `{{.SectionNum}}.N.K Label.` where:
   - `{{.SectionNum}}` is the section number (auto-substituted, e.g., `4` or `5`)
   - `N` is the subsection (e.g., `2.1`, `4.10`)
   - `K` is the letter (A–Z for bullets within a subsection)
   - `Label` is a 2–3 word description of the bullet

2. **Subsection-aware tagging:**
   - If a subsection (like `**4.2.1**`) has 3 bullets, tag them as `.2.1.A`, `.2.1.B`, `.2.1.C`
   - If the next subsection (`.2.2`) has 2 bullets, tag them as `.2.2.A`, `.2.2.B` (restart the letter count)

3. **VerifyCompactCoverage strategy:**
   - Keep it simple: just check that expected section IDs exist
   - Don't try to extract and render the compact form (that's lossy and fragile)
   - This is a **validation layer**, not a replacement for the hand-crafted compact form

4. **Testing philosophy:**
   - Spot-check a few bullets after tagging to ensure format is correct
   - Unit test `VerifyCompactCoverage` with real Constitution.md data
   - Integration test compress and check-coverage workflows

---

## Post-D2 Work (Not in This Plan)

- **Task 8 (D3 follow-up):** Replace `renderCompactConstitution` with extractor-based generation (this would make the compact form auto-derived and lossless)
- **Task 9 (D3):** Tag remaining custom sections (§4.11+, if any are added post-1.0)
- **Ongoing:** Update compact form generation if the constitution structure changes
