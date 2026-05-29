# Lossless Compaction D-1: Regex + Template Tagging Foundation

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix the regex layer so three-level IDs (`4.1.1`, `4.2.1`) are captured, strip the `§` prefix from template bold-subsection heads, tag §4.1 and §4.3 plain bullets with stable IDs, and add `ParseSectionsAny` so `--check-coverage` no longer errors on template-generated constitutions.

**Architecture:** Update `ruleHeadRe` and `bulletSubRuleRe` in `compress.go` to handle `\d+(?:\.\d+)+` (N.M.K) IDs; add `ParseSectionsAny` + `parseTemplateSections` to `constitution.go` that recognises `## §N Name` headers; strip `§` from 14 `**§N.M.K.**` template bold heads so the updated regex captures them; tag §4.1 (10 bullets) and §4.3 (8 bullets) with `- **{{.SectionNum}}.1.K.**` / `- **{{.SectionNum}}.3.K.**` IDs; wire `ParseSectionsAny` into `runCheckCoverage`.

**Tech Stack:** Go 1.26, `regexp`, `src/internal/compress/compress.go`, `src/internal/constitution/constitution.go`, `src/cmd/ai/embed/templates/constitution.tmpl`, `src/cmd/ai/cmd/compress.go`.

---

## Context: the four current gaps

Read these before touching any code.

**Gap 1 — Regex only handles two-level IDs** (`compress.go:58,64`):
```go
var ruleHeadRe      = regexp.MustCompile(`^\s*\*\*(?:(\d+\.\d+)|[A-Z]\d+\.|)([^*]+?)\.*\*\*(.*)$`)
var bulletSubRuleRe = regexp.MustCompile(`^[-*]\s+\*\*(\d+\.\d+)\s+([^*]+?)\.*\*\*(.*)$`)
```
`(\d+\.\d+)` matches `4.2` but stops before `.1` in `4.2.1`. Any three-level ID is silently truncated.

**Gap 2 — Template bold subsection heads use `§` prefix** (`constitution.tmpl`):
```
**§{{.SectionNum}}.2.1 Pattern selection.**
```
After template rendering, the line is `**§4.2.1 Pattern selection.**`. The `§` character is not a digit, so `(\d+(?:\.\d+)+)` won't match it — the ID is lost and an auto-ID is assigned instead.

**Gap 3 — `ParseSections` doesn't find template-generated sections** (`constitution.go:70`):
```go
var sectionHeaderRe = regexp.MustCompile(`(?m)^## (\d+)\. (\w+) Rules\s*$`)
```
Matches `## 4. Technical Rules` but NOT `## §4 Technical Work` (the template produces the latter). `--check-coverage` immediately returns "no persona sections found."

**Gap 4 — 98 plain bullets have no IDs** (`constitution.tmpl:381-390` etc.):
```
- Names MUST reveal intent. Reject `data`, `temp`...
```
No `**N.M.K.**` prefix → `parseBulletSubRule` can't capture it → missing from YAML derivatives and compact form.

**This plan (D-1)** fixes Gaps 1–3 and demonstrates Gap 4 with §4.1 (10 bullets) and §4.3 (8 bullets). Remaining §4.4–§4.10 and §5 tagging is Plan D-2.

---

## Files

**Modify:**
- `src/internal/compress/compress.go` — update two regexes
- `src/internal/constitution/constitution.go` — add `ParseSectionsAny`, `parseTemplateSections`
- `src/cmd/ai/embed/templates/constitution.tmpl` — strip `§` from bold heads; tag §4.1 and §4.3 bullets
- `src/cmd/ai/cmd/compress.go` — wire `ParseSectionsAny` into `runCheckCoverage`

**Tests:**
- `src/internal/compress/compress_test.go` — new tests for three-level ID capture
- `src/internal/constitution/constitution_test.go` — new tests for `ParseSectionsAny`

---

## Task 1: Update regexes for multi-level IDs in compress.go

**Files:**
- Modify: `src/internal/compress/compress.go:58,64`
- Modify: `src/internal/compress/compress_test.go`

- [ ] **Step 1.1: Write the failing tests**

  Append to `src/internal/compress/compress_test.go`:

  ```go
  // TestExtract_ThreeLevelBoldHead verifies that **N.M.K Label.** heads
  // (e.g. 4.2.1 after stripping § from template) are captured with the full ID.
  func TestExtract_ThreeLevelBoldHead(t *testing.T) {
  	body := "**4.2.1 Pattern selection.** Apply Gang-of-Four patterns where they fit. MUST apply."
  	s := section(4, "Technical", body)
  	ds, err := compress.Extract(s, "1.0")
  	if err != nil {
  		t.Fatalf("Extract() error = %v", err)
  	}
  	yaml := string(ds.YAML)
  	if !strings.Contains(yaml, `id: "4.2.1"`) {
  		t.Errorf("YAML missing three-level id 4.2.1:\n%s", yaml)
  	}
  }

  // TestExtract_ThreeLevelBulletSubRule verifies that - **N.M.K Label.** bullets
  // are captured with the full three-level ID.
  func TestExtract_ThreeLevelBulletSubRule(t *testing.T) {
  	body := "**§4.1 Clean Code.** Rules:\n\n- **4.1.1 Names reveal intent.** MUST reveal intent.\n- **4.1.2 Function length.** SHOULD stay under 30 lines."
  	s := section(4, "Technical", body)
  	ds, err := compress.Extract(s, "1.0")
  	if err != nil {
  		t.Fatalf("Extract() error = %v", err)
  	}
  	yaml := string(ds.YAML)
  	for _, wantID := range []string{`"4.1.1"`, `"4.1.2"`} {
  		if !strings.Contains(yaml, "id: "+wantID) {
  			t.Errorf("YAML missing id %s:\n%s", wantID, yaml)
  		}
  	}
  }
  ```

- [ ] **Step 1.2: Run tests to confirm they fail**

  ```bash
  go test -run 'TestExtract_ThreeLevelBoldHead|TestExtract_ThreeLevelBulletSubRule' -v ./src/internal/compress/
  ```
  Expected: FAIL — `YAML missing three-level id 4.2.1` (current regex captures `4.2` not `4.2.1`).

- [ ] **Step 1.3: Update both regexes in compress.go**

  Change `(\d+\.\d+)` to `(\d+(?:\.\d+)+)` in BOTH regex declarations:

  ```go
  // ruleHeadRe matches rule opening lines:
  //   **N.M Label.** rest...         (two-level, e.g. §3.4)
  //   **N.M.K Label.** rest...       (three-level, e.g. §4.2.1)
  //   **PN. Label.** rest...         (letter-digit, e.g. P1, U1)
  //   **Label.** rest...             (unlabelled — auto-ID assigned)
  var ruleHeadRe = regexp.MustCompile(`^\s*\*\*(?:(\d+(?:\.\d+)+)|[A-Z]\d+\.|)([^*]+?)\.*\*\*(.*)$`)

  // bulletSubRuleRe matches bullet-prefixed sub-rules with explicit N.M[.K] IDs:
  //   - **13.1 Capacity gate.** rest...
  //   - **4.1.1 Names reveal intent.** rest...
  var bulletSubRuleRe = regexp.MustCompile(`^[-*]\s+\*\*(\d+(?:\.\d+)+)\s+([^*]+?)\.*\*\*(.*)$`)
  ```

- [ ] **Step 1.4: Run tests — expect PASS**

  ```bash
  go test -run 'TestExtract_ThreeLevelBoldHead|TestExtract_ThreeLevelBulletSubRule' -v ./src/internal/compress/
  ```
  Expected: both PASS.

- [ ] **Step 1.5: Run full compress suite (no regressions)**

  ```bash
  go test -v ./src/internal/compress/
  ```
  Expected: all existing tests plus the two new ones PASS.

- [ ] **Step 1.6: Commit**

  ```bash
  git add src/internal/compress/compress.go src/internal/compress/compress_test.go
  git commit -m "fix(compress): capture N.M.K three-level IDs in ruleHeadRe and bulletSubRuleRe

  (\d+\.\d+) → (\d+(?:\.\d+)+) in both regexes.
  4.2.1, 4.1.1, 10.2.1 etc. now capture their full ID instead of truncating.

  Co-Authored-By: Claude Sonnet 4.6 (1M context) <noreply@anthropic.com>"
  ```

---

## Task 2: Add `ParseSectionsAny` to constitution.go

**Files:**
- Modify: `src/internal/constitution/constitution.go`
- Modify: `src/internal/constitution/constitution_test.go` (find existing test file)

- [ ] **Step 2.1: Check if constitution_test.go exists**

  ```bash
  ls src/internal/constitution/
  ```
  Expected: `constitution.go` and `constitution_test.go`. If the test file is missing, create it with `package constitution_test`.

- [ ] **Step 2.2: Write failing tests for ParseSectionsAny**

  Append to `src/internal/constitution/constitution_test.go`:

  ```go
  // TestParseSectionsAny_LegacyFormat verifies backward compatibility with
  // the "## N. Name Rules" format.
  func TestParseSectionsAny_LegacyFormat(t *testing.T) {
  	content := "## 1. Common Rules\nSome rules.\n\n## 2. Code Rules\nMore rules.\n"
  	secs := constitution.ParseSectionsAny(content)
  	if len(secs) != 2 {
  		t.Fatalf("expected 2 sections, got %d", len(secs))
  	}
  	if secs[0].Name != "Common" || secs[0].Number != 1 {
  		t.Errorf("unexpected first section: %+v", secs[0])
  	}
  }

  // TestParseSectionsAny_TemplateFormat verifies the "## §N Name" format
  // generated by the constitution template.
  func TestParseSectionsAny_TemplateFormat(t *testing.T) {
  	content := "## §3 Universal Rules\n**U1. State assumptions.** MUST name gaps.\n\n## §4 Technical Work\n**P1. Clean code.** MUST be readable.\n"
  	secs := constitution.ParseSectionsAny(content)
  	if len(secs) != 2 {
  		t.Fatalf("expected 2 sections for template format, got %d: %+v", len(secs), secs)
  	}
  	if secs[0].Number != 3 {
  		t.Errorf("expected section 3, got %d", secs[0].Number)
  	}
  	if secs[1].Number != 4 {
  		t.Errorf("expected section 4, got %d", secs[1].Number)
  	}
  }

  // TestParseSectionsAny_GovernanceExcluded verifies that §1 Governance is
  // excluded from both formats (governance contains meta-rules, not AI directives).
  func TestParseSectionsAny_GovernanceExcluded(t *testing.T) {
  	content := "## §1 Governance\nSome meta-rules.\n\n## §3 Universal Rules\n**U1. MUST.** Do this.\n"
  	secs := constitution.ParseSectionsAny(content)
  	for _, s := range secs {
  		if strings.EqualFold(s.Name, "Governance") || s.Number == 1 {
  			t.Errorf("Governance section should be excluded: %+v", s)
  		}
  	}
  }
  ```

  Check the import at the top of the test file includes `"strings"` — add if missing.

- [ ] **Step 2.3: Run tests to confirm they fail**

  ```bash
  go test -run 'TestParseSectionsAny' -v ./src/internal/constitution/
  ```
  Expected: compile error — `constitution.ParseSectionsAny undefined`.

- [ ] **Step 2.4: Add `ParseSectionsAny` and `parseTemplateSections` to constitution.go**

  Append after the existing `ParseSections` function (after line 112):

  ```go
  // templateSectionRe matches section headers generated by the constitution template.
  // Format: "## §N Name" where N is a number and Name is one or more words.
  var templateSectionRe = regexp.MustCompile(`(?m)^## §(\d+) (.+)$`)

  // parseTemplateSections extracts sections from content using the template-generated
  // "## §N Name" header format. Returns nil when no such headers are found.
  func parseTemplateSections(content string) []Section {
  	matches := templateSectionRe.FindAllStringIndex(content, -1)
  	if len(matches) == 0 {
  		return nil
  	}
  	var sections []Section
  	for i, loc := range matches {
  		header := content[loc[0]:loc[1]]
  		sub := templateSectionRe.FindStringSubmatch(header)
  		if sub == nil {
  			continue
  		}
  		num, _ := strconv.Atoi(sub[1])
  		fullName := strings.TrimSpace(sub[2])
  		// Use the first word as the canonical Name for slug/file compatibility.
  		firstName := strings.Fields(fullName)[0]
  		if strings.EqualFold(firstName, "Governance") || num == 1 {
  			continue // meta-rules only, not enforceable AI directives
  		}
  		bodyStart := loc[1]
  		var bodyEnd int
  		if i+1 < len(matches) {
  			bodyEnd = matches[i+1][0]
  		} else {
  			bodyEnd = len(content)
  		}
  		body := strings.TrimSpace(content[bodyStart:bodyEnd])
  		sections = append(sections, Section{
  			Number:   num,
  			Name:     firstName,
  			Slug:     strings.ToLower(firstName),
  			FileName: firstName + ".md",
  			Body:     body,
  		})
  	}
  	return sections
  }

  // ParseSectionsAny extracts persona sections from Constitution.md content,
  // handling both:
  //   - "## N. Name Rules" — the legacy/test format (ParseSections)
  //   - "## §N Name"       — the format generated by the constitution template
  //
  // This is the function to use in CLI commands; ParseSections is kept for
  // backward compatibility with existing tests and older constitutions.
  func ParseSectionsAny(content string) []Section {
  	if secs := ParseSections(content); len(secs) > 0 {
  		return secs
  	}
  	return parseTemplateSections(content)
  }
  ```

- [ ] **Step 2.5: Run tests — expect PASS**

  ```bash
  go test -run 'TestParseSectionsAny' -v ./src/internal/constitution/
  ```
  Expected: all three PASS.

- [ ] **Step 2.6: Full constitution suite**

  ```bash
  go test -v ./src/internal/constitution/
  ```
  Expected: all PASS.

- [ ] **Step 2.7: Commit**

  ```bash
  git add src/internal/constitution/constitution.go src/internal/constitution/constitution_test.go
  git commit -m "feat(constitution): add ParseSectionsAny for template-generated '## §N Name' headers

  ParseSections (legacy '## N. Name Rules') stays unchanged.
  ParseSectionsAny tries legacy first, falls back to templateSectionRe.
  Needed by --check-coverage and future compact form generator.

  Co-Authored-By: Claude Sonnet 4.6 (1M context) <noreply@anthropic.com>"
  ```

---

## Task 3: Strip `§` from template bold-subsection heads

The 14 `**§{{.SectionNum}}.M.K.**` heads in the template render to `**§4.2.1.**` which breaks ID extraction. Removing the `§` makes them `**4.2.1.**` — captured cleanly by the updated `ruleHeadRe`.

**Files:**
- Modify: `src/cmd/ai/embed/templates/constitution.tmpl`

- [ ] **Step 3.1: Confirm the current count**

  ```bash
  grep -c '\*\*§{{' src/cmd/ai/embed/templates/constitution.tmpl
  ```
  Expected: `14`.

- [ ] **Step 3.2: Strip `§` from all bold subsection heads**

  ```bash
  sed -i '' 's/\*\*§{{/\*\*{{/g' src/cmd/ai/embed/templates/constitution.tmpl
  ```
  On Linux, omit the `''`: `sed -i 's/\*\*§{{/\*\*{{/g' ...`

- [ ] **Step 3.3: Verify the replacement**

  ```bash
  grep -c '\*\*§{{' src/cmd/ai/embed/templates/constitution.tmpl
  ```
  Expected: `0`.

  ```bash
  grep -c '\*\*{{' src/cmd/ai/embed/templates/constitution.tmpl
  ```
  Expected: `14` (all converted).

  Check one line looks right:
  ```bash
  grep -n '\*\*{{.SectionNum}}.2.1' src/cmd/ai/embed/templates/constitution.tmpl
  ```
  Expected: line like `**{{.SectionNum}}.2.1 Pattern selection.**`

- [ ] **Step 3.4: Build to confirm embed picks up the changed template**

  ```bash
  go build ./src/cmd/ai/...
  ```
  Expected: clean build.

- [ ] **Step 3.5: Commit**

  ```bash
  git add src/cmd/ai/embed/templates/constitution.tmpl
  git commit -m "fix(template): strip § prefix from bold subsection heads (**§N.M.K** → **N.M.K**)

  § is not a digit so ruleHeadRe could not extract the ID.
  After this change, **4.2.1 Pattern selection.** is captured with ID 4.2.1.

  Co-Authored-By: Claude Sonnet 4.6 (1M context) <noreply@anthropic.com>"
  ```

---

## Task 4: Tag §4.1 and §4.3 plain bullets in the template

Plain bullets like `- Names MUST reveal intent.` have no ID so they are silently dropped. This task converts them to `- **{{.SectionNum}}.1.K Label.** ...` format, which `parseBulletSubRule` captures.

The `{{.SectionNum}}` Go template variable expands to the section's number at `ai setup` time, so `4.1.1` is stable for a principal whose technical domain is §4.

**Files:**
- Modify: `src/cmd/ai/embed/templates/constitution.tmpl` (lines 381–390 for §4.1, lines 413–420 for §4.3)

- [ ] **Step 4.1: Read the current §4.1 block to confirm line numbers**

  ```bash
  grep -n 'Names MUST reveal intent\|Functions SHOULD stay\|Comments MUST explain\|Three repetitions\|Dead code MUST\|Every.*catch\|Side effects MUST\|I/O.*persistence\|Magic numbers\|Public functions MUST have type' \
    src/cmd/ai/embed/templates/constitution.tmpl
  ```
  Note the exact line numbers — they determine where to place the edits.

- [ ] **Step 4.2: Replace the §4.1 plain-bullet block**

  Find the 10-line block starting with `- Names MUST reveal intent.` and ending with `- Public functions MUST have type signatures` and replace it with:

  ```
  - **{{.SectionNum}}.1.1 Names reveal intent.** MUST reveal intent. Reject `data`, `temp`, `mgr`, `util`, `helper`, `manager`, `handler` unless that word is genuinely the most precise.
  - **{{.SectionNum}}.1.2 Function length.** SHOULD stay under ~30 lines and cyclomatic complexity ≤ 10. Exceed only with a stated reason.
  - **{{.SectionNum}}.1.3 Comments explain why.** MUST explain *why*, not *what*. The code says what.
  - **{{.SectionNum}}.1.4 Extract at three repetitions.** Three repetitions is the limit. On the third, extract.
  - **{{.SectionNum}}.1.5 Delete dead code.** MUST be deleted, not commented out. Version control is the graveyard.
  - **{{.SectionNum}}.1.6 Deliberate error handling.** Every `catch` / `except` / `recover` MUST contain a deliberate decision: recover, retry, escalate, or fail loudly. Silent swallowing is forbidden.
  - **{{.SectionNum}}.1.7 Isolate side effects.** MUST be isolated and named. Prefer pure functions where reasonable.
  - **{{.SectionNum}}.1.8 Validate at boundaries.** I/O, persistence, third-party calls, and user input MUST cross labeled boundaries with validation.
  - **{{.SectionNum}}.1.9 Name magic values.** Magic numbers and magic strings MUST be named constants with explanatory context.
  - **{{.SectionNum}}.1.10 Type signatures.** Public functions MUST have type signatures (or equivalent doc-comment types in dynamic languages).
  ```

- [ ] **Step 4.3: Replace the §4.3 testing plain-bullet block**

  Find the 8-line block starting with `- Tests MUST exist before a feature` and replace it with:

  ```
  - **{{.SectionNum}}.3.1 Tests before done.** MUST exist before a feature is called done. Not after the release.
  - **{{.SectionNum}}.3.2 Testing pyramid.** Follow the testing pyramid: many fast unit tests, fewer integration tests, a small number of end-to-end tests. Inverted pyramids are a smell.
  - **{{.SectionNum}}.3.3 Behavior not implementation.** MUST describe behavior, not implementation. A behavior-preserving refactor MUST NOT break tests.
  - **{{.SectionNum}}.3.4 Bug fix starts with red test.** Every bug fix begins with a failing test that reproduces the bug.
  - **{{.SectionNum}}.3.5 No assertion-free tests.** A test without assertions is not a test. Flag and remove.
  - **{{.SectionNum}}.3.6 CI mandatory.** MUST run on every change. "Works on my machine" is not verification.
  - **{{.SectionNum}}.3.7 Spec-form names.** SHOULD read as specifications: `it_rejects_orders_below_minimum`, not `test_order_1`.
  - **{{.SectionNum}}.3.8 Fix flaky tests.** MUST be quarantined or fixed within one sprint of being identified. Ignored flakiness erodes trust in the whole suite.
  ```

- [ ] **Step 4.4: Build to confirm template still compiles**

  ```bash
  go build ./src/cmd/ai/...
  ```
  Expected: clean build (the `//go:embed` picks up the modified template automatically).

- [ ] **Step 4.5: Commit**

  ```bash
  git add src/cmd/ai/embed/templates/constitution.tmpl
  git commit -m "feat(template): tag §4.1 (clean code) and §4.3 (testing) bullets with stable IDs

  10 + 8 plain bullets converted to '- **{{.SectionNum}}.N.K Label.** ...' format.
  parseBulletSubRule() now captures them with IDs 4.1.1-4.1.10 and 4.3.1-4.3.8
  in constitutions generated by 'ai setup'.

  Co-Authored-By: Claude Sonnet 4.6 (1M context) <noreply@anthropic.com>"
  ```

---

## Task 5: Wire `ParseSectionsAny` into `--check-coverage`

**Files:**
- Modify: `src/cmd/ai/cmd/compress.go` (function `runCheckCoverage`)

- [ ] **Step 5.1: Find the ParseSections call in runCheckCoverage**

  ```bash
  grep -n 'ParseSections\|constitution\.Parse' src/cmd/ai/cmd/compress.go
  ```
  Expected: one call to `constitution.ParseSections(string(data))` inside `runCheckCoverage`.

- [ ] **Step 5.2: Replace `ParseSections` with `ParseSectionsAny`**

  In `runCheckCoverage`, change:
  ```go
  sections := constitution.ParseSections(string(data))
  if len(sections) == 0 {
  	return fmt.Errorf("compress --check-coverage: no persona sections found in Constitution.md")
  }
  ```
  to:
  ```go
  sections := constitution.ParseSectionsAny(string(data))
  if len(sections) == 0 {
  	return fmt.Errorf("compress --check-coverage: no persona sections found in Constitution.md\n" +
  		"  Constitution.md must have sections in '## N. Name Rules' or '## §N Name' format")
  }
  ```

- [ ] **Step 5.3: Build and smoke-test**

  ```bash
  go build ./src/cmd/ai/... && echo "build OK"
  ```

  Now test against the actual live constitution:
  ```bash
  go run ./src/cmd/ai compress --check-coverage 2>&1 | head -10
  ```
  Expected: output now shows `[checked] Universal: N rule IDs` (or similar) instead of "no persona sections found". If `Constitution.compact.md` is missing, you'll see that error instead — that's correct behaviour.

- [ ] **Step 5.4: Run the full test suite**

  ```bash
  make test
  ```
  Expected: all packages pass, 0 failures.

- [ ] **Step 5.5: Commit**

  ```bash
  git add src/cmd/ai/cmd/compress.go
  git commit -m "feat(compress): --check-coverage uses ParseSectionsAny for template-generated constitutions

  constitution.ParseSections only matched '## N. Name Rules' (legacy format).
  Template generates '## §N Name'. ParseSectionsAny handles both, so
  --check-coverage no longer errors with 'no persona sections found'.

  Co-Authored-By: Claude Sonnet 4.6 (1M context) <noreply@anthropic.com>"
  ```

---

## Self-review

**Gap coverage:**

| Gap | Fix | Task |
|---|---|---|
| `ruleHeadRe` truncates `4.2.1` to `4.2` | `(\d+(?:\.\d+)+)` captures full multi-level ID | 1 |
| `bulletSubRuleRe` same issue | Same fix | 1 |
| `**§N.M.K.**` heads can't be ID-extracted | Strip `§` from 14 template bold heads | 3 |
| `ParseSections` misses `## §N Name` headers | `ParseSectionsAny` + `parseTemplateSections` | 2 |
| Plain bullets have no IDs (98 total) | Tag §4.1 (10) + §4.3 (8) as proof-of-concept | 4 |
| `--check-coverage` errors on template constitutions | Wire `ParseSectionsAny` | 5 |

**Deliberately deferred to Plan D-2:**
- Replace `renderCompactConstitution` with extractor-based generator
- Tag remaining §4.4–§4.10, §4.2.x sub-bullets, §5 bullets
- Make `--check-coverage` pass CI (requires compact form to contain all tagged IDs)

**Placeholder scan:** None found. All steps show exact code, commands, expected output.

**Type consistency:**
- `ParseSectionsAny(content string) []Section` — defined in Task 2, called in Task 5 ✓
- `parseTemplateSections(content string) []Section` — defined in Task 2, called by `ParseSectionsAny` in Task 2 ✓
- `constitution.Section` struct — unchanged, both parsers return it ✓
- `compress.RuleIDs(s)` — unchanged, still called in `runCheckCoverage` ✓
