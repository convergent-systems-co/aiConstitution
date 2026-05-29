# Compact Coverage Gate — Plan B

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix the rule extractor to capture bullet sub-rules (C.3) and add a rule-ID coverage check to `ai compress --check` (C.2), so that missing rules in the compact form are a visible, testable failure rather than silent omission.

**Architecture:** Add `bulletSubRuleRe` + `parseBulletSubRule()` to `compress.go` to handle the `- **N.M Label.**` format that `ruleHeadRe` currently misses; update `extractRules` to try the new parser for blocks that fail `parseRuleBlock`; export `RuleIDs(section)` for test and CLI use; extend `--check` to report rule counts per section and add `--check-coverage` to compare compact-form rule IDs against full-extraction IDs.

**Tech Stack:** Go 1.26, `regexp`, `src/internal/compress/compress.go`, `src/cmd/ai/cmd/compress.go`.

---

## Context: the exact gap

The constitution template has two block shapes for rules:

**Shape A — top-level, matched by current `ruleHeadRe`:**
```markdown
**U13. Context-window discipline.** Treat it as a budget, not a buffer.
```
`parseRuleBlock` matches this. ✓

**Shape B — bullet sub-rules, NOT matched:**
```markdown
- **13.1 Capacity gate.** At or above 80% utilization, stop. MUST stop.
- **13.2 Clean tree.** You MUST NOT auto-compact on a dirty tree.
- **13.3 Checkpoint then summarize.** Update HANDOFF.md per U10.
```
The blank line before this block causes `extractRules` to split it into its own paragraph. `parseRuleBlock` takes the first line (`- **13.1 Capacity gate.**...`), finds `m == nil` because `ruleHeadRe` requires `^\s*\*\*` but the line starts with `- `, and returns nil. The entire block is silently discarded.

**Same issue applies to:** 15.1/15.2/15.3, 16.1/16.2/16.3, and any future `- **N.M Label.**` sub-rules.

**Current `ruleHeadRe`** (`compress.go:58`):
```go
var ruleHeadRe = regexp.MustCompile(`^\s*\*\*(?:(\d+\.\d+)|[A-Z]\d+\.|)([^*]+?)\.*\*\*(.*)$`)
```
Requires line to start with optional whitespace then `**`. Does not handle `- **`.

**`--check` current behavior** (`cmd/compress.go:90–98`): compares the source hash embedded in the derivative file against the current source hash. Does not count or compare rule IDs.

---

## Files

**Modify:**
- `src/internal/compress/compress.go` — add `bulletSubRuleRe`, `parseBulletSubRule`, `parseBulletSubRuleBlock`, update `extractRules`, add `RuleIDs`
- `src/internal/compress/compress_test.go` — tests for bullet sub-rule capture and `RuleIDs`
- `src/cmd/ai/cmd/compress.go` — extend `--check` with rule-count report + `--check-coverage` flag

---

## Task 1: Add bullet sub-rule parser to compress.go

**Files:**
- Modify: `src/internal/compress/compress.go`

- [ ] **Step 1.1: Write the failing test first**

  Append to `src/internal/compress/compress_test.go`:

  ```go
  // TestExtract_CapturesBulletSubRules verifies that a block of bullet-format
  // sub-rules (e.g. "- **13.1 Capacity gate.** ...") is captured as individual
  // rules with their explicit N.M IDs.
  func TestExtract_CapturesBulletSubRules(t *testing.T) {
  	body := "**U13. Context-window discipline.** Treat it as a budget, not a buffer.\n\n" +
  		"- **13.1 Capacity gate.** At or above 80% utilization, stop. MUST stop.\n" +
  		"- **13.2 Clean tree.** You MUST NOT auto-compact on a dirty tree.\n" +
  		"- **13.3 Checkpoint then summarize.** Update HANDOFF.md per U10."
  	s := section(3, "Common", body)
  	ds, err := compress.Extract(s, "1.0")
  	if err != nil {
  		t.Fatalf("Extract() error = %v", err)
  	}
  	yaml := string(ds.YAML)
  	for _, wantID := range []string{`"13.1"`, `"13.2"`, `"13.3"`} {
  		if !strings.Contains(yaml, "id: "+wantID) {
  			t.Errorf("YAML missing sub-rule id %s:\n%s", wantID, yaml)
  		}
  	}
  }

  // TestExtract_BulletSubRuleGateInference verifies that MUST/SHOULD in a
  // bullet sub-rule are correctly inferred as hard/soft gates.
  func TestExtract_BulletSubRuleGateInference(t *testing.T) {
  	body := "**U15. Bounded self-correction.** When not converging, stop.\n\n" +
  		"- **15.1 Three-cycle local cap.** After three failed attempts MUST stop.\n" +
  		"- **15.2 Five-cycle total cap.** After five total attempts, escalate.\n" +
  		"- **15.3 No silent retries.** A retry MUST be visible in your output."
  	s := section(3, "Common", body)
  	ds, err := compress.Extract(s, "1.0")
  	if err != nil {
  		t.Fatalf("Extract() error = %v", err)
  	}
  	yaml := string(ds.YAML)
  	if !strings.Contains(yaml, "id: \"15.1\"") {
  		t.Errorf("YAML missing 15.1:\n%s", yaml)
  	}
  	if !strings.Contains(yaml, "gate: hard") {
  		t.Errorf("YAML missing hard gate for MUST rules:\n%s", yaml)
  	}
  }
  ```

- [ ] **Step 1.2: Run tests to confirm they fail**

  ```bash
  go test -run 'TestExtract_CapturesBulletSubRules|TestExtract_BulletSubRuleGateInference' -v ./src/internal/compress/
  ```
  Expected: FAIL — `YAML missing sub-rule id "13.1"` (the current extractor drops bullet blocks).

- [ ] **Step 1.3: Add `bulletSubRuleRe` and `parseBulletSubRule` to compress.go**

  In `compress.go`, after the `ruleHeadRe` declaration (line 58), add:

  ```go
  // bulletSubRuleRe matches bullet-prefixed sub-rules with explicit N.M IDs:
  //   - **13.1 Capacity gate.** rest...
  //   - **16.1 TUI / terminal sessions** — rest...
  // The ID (e.g. "13.1") is captured in group 1; the label in group 2;
  // the trailing content (after the closing **) in group 3.
  var bulletSubRuleRe = regexp.MustCompile(`^[-*]\s+\*\*(\d+\.\d+)\s+([^*]+?)\.*\*\*(.*)$`)

  // parseBulletSubRule attempts to parse one line as a bullet sub-rule.
  // Returns nil when the line does not match.
  func parseBulletSubRule(line string) *rule {
  	m := bulletSubRuleRe.FindStringSubmatch(strings.TrimSpace(line))
  	if m == nil {
  		return nil
  	}
  	label := strings.TrimSpace(m[2])
  	content := strings.TrimSpace(strings.TrimLeft(m[3], " —-:"))
  	if label == "" {
  		return nil
  	}
  	return &rule{
  		ID:             m[1],
  		Gate:           inferGate(content),
  		NonOverridable: strings.Contains(content, "(Non-overridable.)") || strings.Contains(content, "*(Non-overridable.)*"),
  		Label:          label,
  		Content:        content,
  	}
  }
  ```

- [ ] **Step 1.4: Update `extractRules` to try the bullet parser for unmatched blocks**

  Replace the `extractRules` function (lines 60–77) with:

  ```go
  func extractRules(s constitution.Section) []rule {
  	var rules []rule
  	blocks := strings.Split(s.Body, "\n\n")
  	idx := 1
  	for _, block := range blocks {
  		block = strings.TrimSpace(block)
  		if block == "" {
  			continue
  		}
  		r := parseRuleBlock(s.Number, idx, block)
  		if r != nil {
  			rules = append(rules, *r)
  			idx++
  			continue
  		}
  		// Block didn't match as a top-level rule — scan lines for bullet sub-rules.
  		for _, line := range strings.Split(block, "\n") {
  			if sr := parseBulletSubRule(line); sr != nil {
  				rules = append(rules, *sr)
  				idx++
  			}
  		}
  	}
  	return rules
  }
  ```

- [ ] **Step 1.5: Run the failing tests — expect PASS**

  ```bash
  go test -run 'TestExtract_CapturesBulletSubRules|TestExtract_BulletSubRuleGateInference' -v ./src/internal/compress/
  ```
  Expected:
  ```
  --- PASS: TestExtract_CapturesBulletSubRules (0.00s)
  --- PASS: TestExtract_BulletSubRuleGateInference (0.00s)
  ```

- [ ] **Step 1.6: Run the full compress test suite (no regressions)**

  ```bash
  go test -v ./src/internal/compress/
  ```
  Expected: all existing tests plus the two new ones PASS.

- [ ] **Step 1.7: Commit**

  ```bash
  git add src/internal/compress/compress.go src/internal/compress/compress_test.go
  git commit -m "fix(compress): capture bullet sub-rules with explicit N.M IDs (C.3)

  - **13.1 Capacity gate.**, **15.1 Three-cycle cap.**, **16.1 TUI sessions**,
  and any future - **N.M Label.** sub-rules are now extracted as individual
  rules with their stable IDs, not silently discarded.

  Co-Authored-By: Claude Sonnet 4.6 (1M context) <noreply@anthropic.com>"
  ```

---

## Task 2: Export `RuleIDs` for coverage checks

**Files:**
- Modify: `src/internal/compress/compress.go` (append)
- Modify: `src/internal/compress/compress_test.go` (append)

- [ ] **Step 2.1: Add `RuleIDs` to compress.go**

  Append to `compress.go` (before the final `}`):

  ```go
  // RuleIDs returns the stable IDs of all rules extracted from section s,
  // in extraction order. Used by ai compress --check-coverage to verify
  // that the compact form covers every rule.
  func RuleIDs(s constitution.Section) []string {
  	rules := extractRules(s)
  	ids := make([]string, 0, len(rules))
  	for _, r := range rules {
  		ids = append(ids, r.ID)
  	}
  	return ids
  }
  ```

- [ ] **Step 2.2: Write tests for `RuleIDs`**

  Append to `compress_test.go`:

  ```go
  // TestRuleIDs_IncludesBulletSubRules verifies that RuleIDs returns the N.M
  // sub-rule IDs as well as top-level rule IDs.
  func TestRuleIDs_IncludesBulletSubRules(t *testing.T) {
  	body := "**U13. Context discipline.** Treat it as a budget.\n\n" +
  		"- **13.1 Capacity gate.** MUST stop at 80%.\n" +
  		"- **13.2 Clean tree.** MUST NOT compact on dirty tree.\n\n" +
  		"**U14. Independent verification.** MUST cross-reference."
  	s := section(3, "Common", body)
  	ids := compress.RuleIDs(s)

  	want := map[string]bool{"13.1": true, "13.2": true}
  	for _, id := range ids {
  		delete(want, id)
  	}
  	for id := range want {
  		t.Errorf("RuleIDs missing expected ID %q; got %v", id, ids)
  	}
  	if len(ids) < 3 { // U13, 13.1, 13.2, U14 = at least 4 but U13 auto-IDed
  		t.Errorf("RuleIDs returned too few IDs (%d): %v", len(ids), ids)
  	}
  }

  // TestRuleIDs_StableOrder verifies that RuleIDs returns IDs in consistent
  // extraction order (deterministic for coverage diffing).
  func TestRuleIDs_StableOrder(t *testing.T) {
  	body := "**P1. Honesty.** MUST NOT fabricate.\n\n**P2. Cost.** Ask before exceeding."
  	s := section(1, "Common", body)
  	ids1 := compress.RuleIDs(s)
  	ids2 := compress.RuleIDs(s)
  	if len(ids1) != len(ids2) {
  		t.Fatalf("RuleIDs not stable: %v vs %v", ids1, ids2)
  	}
  	for i := range ids1 {
  		if ids1[i] != ids2[i] {
  			t.Errorf("RuleIDs[%d] differs: %q vs %q", i, ids1[i], ids2[i])
  		}
  	}
  }
  ```

- [ ] **Step 2.3: Run tests**

  ```bash
  go test -run 'TestRuleIDs' -v ./src/internal/compress/
  ```
  Expected: both PASS.

- [ ] **Step 2.4: Full compress suite**

  ```bash
  go test -v ./src/internal/compress/
  ```
  Expected: all PASS.

- [ ] **Step 2.5: Commit**

  ```bash
  git add src/internal/compress/compress.go src/internal/compress/compress_test.go
  git commit -m "feat(compress): export RuleIDs for coverage checking (C.2 foundation)

  Co-Authored-By: Claude Sonnet 4.6 (1M context) <noreply@anthropic.com>"
  ```

---

## Task 3: Extend `--check` with rule counts + add `--check-coverage`

The current `--check` reports `[ok]` / `[stale]` per derivative file based on the embedded hash. This task extends it to also report how many rules were extracted per section, and adds `--check-coverage` that compares compact-form rule IDs against full-extraction IDs.

**`--check-coverage` algorithm:**
1. Parse all persona sections from Constitution.md (using `constitution.ParseSections`).
2. For each section, call `compress.RuleIDs(s)` → full set.
3. Read `Constitution.compact.md`.
4. For each rule ID, check whether `§<ID>` appears in the compact file.
5. Report IDs present in full but absent from compact as missing.
6. Exit non-zero if any are missing.

The compact form emits lines like `§13.1 [HARD] Capacity gate — ...` (see `marshalCompact` in compress.go). So checking for `§<ID>` is a reliable presence test.

**Files:**
- Modify: `src/cmd/ai/cmd/compress.go`

- [ ] **Step 3.1: Read the existing `--check` block in compress.go**

  ```bash
  grep -n 'checkFlag\|isStale\|stale\|--check' src/cmd/ai/cmd/compress.go | head -15
  ```
  Confirm the flag is at line ~19 and the check block is at ~90–98.

- [ ] **Step 3.2: Add the `--check-coverage` flag and `checkCoverage` function**

  In `compress.go`, add the new flag in `newCompressCmd()` after the existing `checkFlag` declaration:

  ```go
  var checkCoverage bool
  ```

  After the `checkFlag` flag declaration (line ~45):

  ```go
  c.Flags().BoolVar(&checkCoverage, "check-coverage", false, "exit non-zero if compact form is missing rule IDs present in full constitution")
  ```

  Update the `RunE` dispatch to pass `checkCoverage`:

  ```go
  RunE: func(cmd *cobra.Command, _ []string) error {
      if personasFlag || personaFlag != "" || checkFlag {
          return runCompressPersonas(cmd, personaFlag, checkFlag)
      }
      if checkCoverage {
          return runCheckCoverage(cmd)
      }
      return runCompress(cmd, wire, output)
  },
  ```

- [ ] **Step 3.3: Extend `runCompressPersonas` to report rule counts**

  In `runCompressPersonas`, inside the `if check {` block (around line 90), extend the output to include rule count:

  ```go
  if check {
      ids := compress.RuleIDs(s)
      if isStale(yamlPath, ds.Hash) {
          _, _ = fmt.Fprintf(out, "  [stale] %s (%d rules)\n", s.FileName, len(ids))
          stale++
      } else {
          _, _ = fmt.Fprintf(out, "  [ok]    %s (%d rules)\n", s.FileName, len(ids))
      }
      continue
  }
  ```

- [ ] **Step 3.4: Add `runCheckCoverage` function**

  Add after `runCompressPersonas`:

  ```go
  // runCheckCoverage implements --check-coverage: reads all persona sections
  // from Constitution.md, extracts their rule IDs, and verifies each ID
  // appears in Constitution.compact.md. Exits non-zero on any missing ID.
  func runCheckCoverage(cmd *cobra.Command) error {
  	root := paths.AIRoot()
  	constPath := filepath.Join(root, "Constitution.md")

  	data, err := os.ReadFile(constPath) //nolint:gosec
  	if err != nil {
  		return fmt.Errorf("compress --check-coverage: read Constitution.md: %w", err)
  	}
  	sections := constitution.ParseSections(string(data))
  	if len(sections) == 0 {
  		return fmt.Errorf("compress --check-coverage: no persona sections found in Constitution.md")
  	}

  	compactPath := filepath.Join(root, "Constitution.compact.md")
  	compactData, err := os.ReadFile(compactPath) //nolint:gosec
  	if err != nil {
  		return fmt.Errorf("compress --check-coverage: read Constitution.compact.md: %w\n"+
  			"  Run 'ai compress' first to generate it.", err)
  	}
  	compactContent := string(compactData)

  	out := cmd.OutOrStdout()
  	var missing []string
  	for _, s := range sections {
  		ids := compress.RuleIDs(s)
  		for _, id := range ids {
  			if !strings.Contains(compactContent, "§"+id) {
  				missing = append(missing, fmt.Sprintf("%s §%s", s.Name, id))
  			}
  		}
  		_, _ = fmt.Fprintf(out, "  [checked] %s: %d rule IDs\n", s.Name, len(ids))
  	}

  	if len(missing) > 0 {
  		_, _ = fmt.Fprintf(out, "\nMissing from Constitution.compact.md (%d):\n", len(missing))
  		for _, m := range missing {
  			_, _ = fmt.Fprintf(out, "  - %s\n", m)
  		}
  		return fmt.Errorf("compress --check-coverage: %d rule ID(s) missing from compact form", len(missing))
  	}
  	_, _ = fmt.Fprintln(out, "\n[ok] All extracted rule IDs present in Constitution.compact.md")
  	return nil
  }
  ```

- [ ] **Step 3.5: Build to confirm no errors**

  ```bash
  go build ./src/cmd/ai/...
  ```
  Expected: clean build. If you see `compress.RuleIDs undefined`, confirm Task 2 was committed and `RuleIDs` is exported (capital R) in `src/internal/compress/compress.go`.

- [ ] **Step 3.6: Run the full test suite**

  ```bash
  make test
  ```
  Expected: all packages pass, 0 failures.

- [ ] **Step 3.7: Smoke-test the new flag**

  ```bash
  # The command will error because no Constitution.md exists in the test env,
  # but it should compile and give a clear error, not a panic.
  AI_ROOT=/tmp/nonexistent go run ./src/cmd/ai compress --check-coverage 2>&1 | head -3
  ```
  Expected: `compress --check-coverage: read Constitution.md: open /tmp/nonexistent/Constitution.md: no such file or directory`

- [ ] **Step 3.8: Commit**

  ```bash
  git add src/cmd/ai/cmd/compress.go
  git commit -m "feat(compress): --check reports rule counts; --check-coverage finds missing IDs (C.2)

  --check now shows (N rules) per section alongside ok/stale.
  --check-coverage compares full-extraction rule IDs against
  Constitution.compact.md and exits non-zero on any missing ID.

  Running against the current hand-written compact form will report
  missing sub-rule IDs (13.1, 15.1, 16.x etc.) until the compact
  form is regenerated from the extractor (C.4 work).

  Co-Authored-By: Claude Sonnet 4.6 (1M context) <noreply@anthropic.com>"
  ```

---

## Task 4: Full suite verification

- [ ] **Step 4.1: Run the complete test suite**

  ```bash
  make test
  ```
  Expected: all packages pass, 0 failures.

- [ ] **Step 4.2: Verify rule count improvement**

  Run a quick check to confirm the extractor now finds more rules than before. The body of `§3.3 Universal Operating Rules` in the template has U13's sub-rules (13.1–13.6), U15 (15.1–15.3), U16 (16.1–16.4):

  ```bash
  go test -run TestExtract_Captures -v ./src/internal/compress/
  ```
  Expected: all tests PASS including the two new bullet sub-rule tests.

---

## Self-review

**Spec C.3 coverage:**

| Gap | Fix | Task |
|---|---|---|
| `- **13.1 Capacity gate.**` not captured | `parseBulletSubRule` + updated `extractRules` | 1 |
| `- **15.1 Three-cycle cap.**` not captured | Same fix covers all `- **N.M Label.**` formats | 1 |
| `- **16.1 TUI / terminal sessions**` not captured | Same | 1 |
| Plain-bullet rules (`- Names MUST reveal intent.`) | Not fixed — requires C.4 explicit IDs on each bullet | deferred |

**Spec C.2 coverage:**

| Gap | Fix | Task |
|---|---|---|
| `--check` only checks hash staleness | Extended to also report rule counts per section | 3 |
| No rule-ID coverage check | `--check-coverage` compares full IDs vs compact content | 3 |
| `RuleIDs` not accessible | Exported function in compress package | 2 |

**C.4 foundation:**
- `RuleIDs` is the function that a future compact generator will call to enumerate rules.
- `--check-coverage` is the gate that will fail CI once the compact is generated from the extractor.
- Plan B intentionally leaves the hand-written `renderCompactConstitution` in place; replacing it is C.4's scope (requires tagging all plain-bullet normative rules in the template first).

**Placeholder scan:** None found. All steps have complete code, commands, and expected output.

**Type consistency:**
- `parseBulletSubRule(line string) *rule` — defined in Task 1, called in updated `extractRules` in Task 1 ✓
- `compress.RuleIDs(s constitution.Section) []string` — defined in Task 2, called in `runCheckCoverage` in Task 3 ✓
- `runCheckCoverage(cmd *cobra.Command) error` — defined in Task 3, dispatched from `RunE` in Task 3 ✓
