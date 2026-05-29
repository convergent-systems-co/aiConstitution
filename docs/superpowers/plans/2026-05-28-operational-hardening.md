# Operational Hardening — Plan C

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Close three low-complexity gaps from the v2 review: fix the residual SPEC.md version-label inconsistency (B.3), add a `ai doctor` check that detects missing blocking wrapper hooks before they cause silent enforcement degradation (A.5), and update the command-wrappers.toml header comment to match the post-Plan-A enforcement model (A.6 + B.2).

**Architecture:** One-line doc fix; one new `checkWrapperHookDrift` function in `doctor.go` that reuses the existing `loadCommandWrappers()`, `hookSlug()`, and `paths.HooksDir()` from the same package; one comment edit in the TOML.

**Tech Stack:** Go 1.26. All production changes fit in `src/cmd/ai/cmd/doctor.go` and `src/cmd/ai/embed/hooks/command-wrappers.toml`.

---

## Context: what each item fixes

**B.3 residual — label mismatch (one line):**
`SPEC.md:3` currently says `**Status:** Draft v1.0.0` (word order: draft-then-version).
`README.md:15` says `Spec at v1.0.0-draft` (hyphenated, version-then-draft).
PR #441 fixed most version drift but missed this one. Fix: change SPEC.md line 3 to match the canonical form used everywhere else.

**A.5 — no doctor check for manifest-wired-but-missing hooks:**
`checkHookWiring` (doctor.go:163) verifies that *installed* hooks are wired in settings.json. The inverse — that hooks *required by command-wrappers.toml* are actually installed — is not checked. After Plan A, `runWrap` fails closed when a blocking hook is absent, so the user only discovers the gap when they invoke `git commit`. Doctor should surface this proactively.

**A.6 + B.2 — stale TOML comment:**
`command-wrappers.toml:6` still says "Defense-in-depth, not security control." After Plan A, blocking pre-hooks now fail closed (missing hook → exit 1 + ENFORCEMENT DEGRADED). The comment needs to reflect the new model without overstating: bypass via absolute path remains possible, but blocking hooks now actively prevent unguarded passthrough when properly installed.

---

## Files

**Modify:**
- `SPEC.md` — fix label on line 3 (B.3)
- `src/cmd/ai/embed/hooks/command-wrappers.toml` — update header comment (A.6/B.2)
- `src/cmd/ai/cmd/doctor.go` — add `checkWrapperHookDrift`, call from `runDoctor` (A.5)
- `src/cmd/ai/cmd/doctor_test.go` — tests for the new check (A.5)

---

## Task 1: Fix B.3 residual and A.6/B.2 TOML comment

These are two doc edits in one commit.

**Files:**
- Modify: `SPEC.md:3`
- Modify: `src/cmd/ai/embed/hooks/command-wrappers.toml:6–9`

- [ ] **Step 1.1: Confirm the current SPEC.md label**

  ```bash
  head -5 SPEC.md
  ```
  Expected: line 3 reads `**Status:** Draft v1.0.0`.

- [ ] **Step 1.2: Fix SPEC.md line 3**

  Read `SPEC.md` first (required before editing), then change:

  ```markdown
  **Status:** Draft v1.0.0  
  ```
  to:
  ```markdown
  **Status:** v1.0.0-draft  
  ```

- [ ] **Step 1.3: Update the command-wrappers.toml header comment**

  Read `src/cmd/ai/embed/hooks/command-wrappers.toml` first, then replace the header comment block (lines 6–9):

  **Current:**
  ```toml
  # Defense-in-depth, not security control — see SPEC.md §10.5.4.
  # Bypass via absolute path (/usr/bin/git) is always possible; the
  # wrapper produces an audit signal where one would normally appear,
  # so an absent audit line is itself visible in forensic review.
  ```

  **Replace with:**
  ```toml
  # Blocking pre-hooks (branch-guard, secret-precommit, no-verify-strip,
  # destructive-gh-guard) fail closed when not installed or when Python
  # is absent — enforcement is explicit, not accidental. Bypass via
  # absolute path (/usr/bin/git) remains possible; the wrapper produces
  # an audit signal so an absent log line is itself visible in forensic review.
  ```

- [ ] **Step 1.4: Verify build (embed picks up the updated TOML)**

  ```bash
  go build ./src/cmd/ai/...
  ```
  Expected: clean build.

- [ ] **Step 1.5: Commit**

  ```bash
  git add SPEC.md src/cmd/ai/embed/hooks/command-wrappers.toml
  git commit -m "docs: fix SPEC.md version label; update TOML header for fail-closed enforcement (B.3, A.6, B.2)

  - SPEC.md: 'Draft v1.0.0' → 'v1.0.0-draft' to match README and other files
  - command-wrappers.toml: replace 'defense-in-depth, not security control'
    with accurate description of post-Plan-A blocking enforcement

  Co-Authored-By: Claude Sonnet 4.6 (1M context) <noreply@anthropic.com>"
  ```

---

## Task 2: Add `checkWrapperHookDrift` to doctor.go (A.5)

**Files:**
- Modify: `src/cmd/ai/cmd/doctor.go`
- Modify: `src/cmd/ai/cmd/doctor_test.go`

`checkWrapperHookDrift` reuses three functions already in the `cmd` package:
- `loadCommandWrappers()` — from `wrap.go`, loads and parses `command-wrappers.toml`
- `hookSlug()` — from `wrap.go`, extracts the basename slug from a script path
- `paths.HooksDir()` — returns `~/.ai/hooks/`

All are in `package cmd`, so no export seam is needed.

- [ ] **Step 2.1: Write the failing tests first**

  Append to `src/cmd/ai/cmd/doctor_test.go` (note: the file is `package cmd`, not `package cmd_test`):

  ```go
  func TestCheckWrapperHookDrift_AllInstalled(t *testing.T) {
  	tmp := t.TempDir()
  	t.Setenv("AI_ROOT", tmp)
  	paths.SetOverrides(tmp, "")
  	t.Cleanup(func() { paths.SetOverrides("", "") })

  	hooksDir := filepath.Join(tmp, "hooks")
  	if err := os.MkdirAll(hooksDir, 0o755); err != nil {
  		t.Fatal(err)
  	}
  	// Install all blocking hooks referenced by the embedded TOML.
  	for _, slug := range []string{
  		"branch-guard", "secret-precommit", "no-verify-strip", "destructive-gh-guard",
  	} {
  		if err := os.WriteFile(filepath.Join(hooksDir, slug+".py"), []byte("# stub\n"), 0o644); err != nil {
  			t.Fatal(err)
  		}
  	}

  	var buf bytes.Buffer
  	checkWrapperHookDrift(&buf)
  	out := buf.String()

  	if strings.Contains(out, "⚠") {
  		t.Errorf("expected no warnings when all hooks installed, got:\n%s", out)
  	}
  	if !strings.Contains(out, "[✓]") {
  		t.Errorf("expected [✓] confirmation, got:\n%s", out)
  	}
  }

  func TestCheckWrapperHookDrift_MissingHook(t *testing.T) {
  	tmp := t.TempDir()
  	t.Setenv("AI_ROOT", tmp)
  	paths.SetOverrides(tmp, "")
  	t.Cleanup(func() { paths.SetOverrides("", "") })

  	// Create hooks dir but leave it empty (no hook files installed).
  	if err := os.MkdirAll(filepath.Join(tmp, "hooks"), 0o755); err != nil {
  		t.Fatal(err)
  	}

  	var buf bytes.Buffer
  	checkWrapperHookDrift(&buf)
  	out := buf.String()

  	if !strings.Contains(out, "⚠") {
  		t.Errorf("expected drift warning when hooks missing, got:\n%s", out)
  	}
  	if !strings.Contains(out, "ai hooks install --all") {
  		t.Errorf("expected remediation hint 'ai hooks install --all', got:\n%s", out)
  	}
  }
  ```

  Also confirm that `paths` is imported in `doctor_test.go`:

  ```bash
  grep '"github.com/convergent-systems-co/aiConstitution/src/internal/paths"' src/cmd/ai/cmd/doctor_test.go
  ```
  If not present, add it to the import block at the top of the file.

- [ ] **Step 2.2: Run tests to confirm they fail**

  ```bash
  go test -run 'TestCheckWrapperHookDrift' -v ./src/cmd/ai/cmd/
  ```
  Expected: compile error — `checkWrapperHookDrift` is not defined yet.

- [ ] **Step 2.3: Add `checkWrapperHookDrift` to doctor.go**

  Find the end of `checkHookWiring` (around line 191) and insert after it:

  ```go
  // checkWrapperHookDrift verifies that every blocking pre-hook referenced in
  // command-wrappers.toml is installed on disk. A missing hook means runWrap
  // will fail closed (ENFORCEMENT DEGRADED) on the next invocation; doctor
  // surfaces this proactively so the user can fix it before it blocks work.
  func checkWrapperHookDrift(w io.Writer) {
  	cfg, err := loadCommandWrappers()
  	if err != nil {
  		fmt.Fprintf(w, "[⚠] command-wrappers.toml unreadable: %v — run: ai hooks install --all\n", err)
  		return
  	}

  	hooksDir := paths.HooksDir()
  	var missing []string

  	for _, entry := range cfg.Command {
  		if !entry.isEnabled() {
  			continue
  		}
  		for _, h := range entry.PreHooks {
  			if !h.isBlocking() {
  				continue
  			}
  			slug := hookSlug(h.Script)
  			hookPath := filepath.Join(hooksDir, slug+".py")
  			if _, err := os.Stat(hookPath); os.IsNotExist(err) {
  				missing = append(missing, slug)
  			}
  		}
  	}

  	if len(missing) == 0 {
  		fmt.Fprintln(w, "[✓] All blocking wrapper hooks installed")
  		return
  	}
  	for _, slug := range missing {
  		fmt.Fprintf(w, "[⚠] Blocking hook %q not installed — run: ai hooks install --all\n", slug)
  	}
  }
  ```

- [ ] **Step 2.4: Wire `checkWrapperHookDrift` into `runDoctor`**

  In `runDoctor` (around line 68), add the call after `checkHookWiring`:

  ```go
  func runDoctor(w io.Writer, fix bool, resetHead string) error {
  	_ = fix
  	_ = resetHead

  	checkTerminalNotifier(w)
  	checkPersonasBlock(w)
  	checkDerivativeFiles(w)
  	checkHookWiring(w, paths.AIRoot(), homeDir())
  	checkWrapperHookDrift(w)
  	_ = checkInstalledSkills(w)

  	return nil
  }
  ```

- [ ] **Step 2.5: Run the new tests**

  ```bash
  go test -run 'TestCheckWrapperHookDrift' -v ./src/cmd/ai/cmd/
  ```
  Expected:
  ```
  --- PASS: TestCheckWrapperHookDrift_AllInstalled (0.00s)
  --- PASS: TestCheckWrapperHookDrift_MissingHook (0.00s)
  ```

  If `TestCheckWrapperHookDrift_AllInstalled` fails with "expected no warnings" but you see branch-guard or secret-precommit in the warning: the embedded TOML has more blocking hooks than the four listed in the test fixtures. Read `src/cmd/ai/embed/hooks/command-wrappers.toml` and install ALL of them in the test setup. The current blocking pre-hooks across all enabled commands are: `branch-guard`, `secret-precommit`, `no-verify-strip`, `destructive-gh-guard`. Check the TOML if this list differs.

- [ ] **Step 2.6: Run the full test suite**

  ```bash
  make test
  ```
  Expected: all packages pass, 0 failures.

- [ ] **Step 2.7: Commit**

  ```bash
  git add src/cmd/ai/cmd/doctor.go src/cmd/ai/cmd/doctor_test.go
  git commit -m "feat(doctor): check that blocking wrapper hooks are installed (A.5)

  checkWrapperHookDrift reads command-wrappers.toml and verifies that
  every enabled blocking pre-hook slug exists in paths.HooksDir().
  Missing hooks emit [⚠] with 'ai hooks install --all' remediation hint.

  Co-Authored-By: Claude Sonnet 4.6 (1M context) <noreply@anthropic.com>"
  ```

---

## Task 3: Full suite verification

- [ ] **Step 3.1: Run the complete test suite**

  ```bash
  make test
  ```
  Expected: all packages pass, 0 failures.

- [ ] **Step 3.2: Smoke-test doctor output includes the new check**

  ```bash
  AI_ROOT=/tmp/doctor-smoke go run ./src/cmd/ai doctor 2>&1 | grep -E 'wrapper|hook|DEGRADED|✓' | head -5
  ```
  Expected: output contains either `[✓] All blocking wrapper hooks installed` (if hooks are extracted) or `[⚠] Blocking hook ... not installed` (if AI_ROOT is fresh). No panic.

---

## Self-review

**B.3 coverage:**

| Gap | Fix | Task |
|---|---|---|
| SPEC.md says "Draft v1.0.0", README says "v1.0.0-draft" | Change SPEC.md line 3 to "v1.0.0-draft" | 1 |

**A.5 coverage:**

| Gap | Fix | Task |
|---|---|---|
| Doctor doesn't check manifest-wired-but-missing blocking hooks | `checkWrapperHookDrift` reads TOML + verifies each blocking slug on disk | 2 |
| `--self-check` (Python runtime) | Deferred — belongs in T4 hook runtime tests (full-stack test spec Plan 6) | — |

**A.6 + B.2 coverage:**

| Gap | Fix | Task |
|---|---|---|
| TOML comment says "not security control" but blocking hooks now fail closed | Updated header comment reflects post-Plan-A model | 1 |
| Constitution template §3.2 doesn't mention wrapper enforcement | Deferred — cross-platform portability spec Plan 2 already covers §3.2 rewrite | — |

**Placeholder scan:** None found.

**Type consistency:**
- `checkWrapperHookDrift(w io.Writer)` — defined in Task 2, called in `runDoctor` in Task 2 ✓
- Uses `loadCommandWrappers()`, `hookSlug()` from `wrap.go` (same package, no seam needed) ✓
- Uses `paths.HooksDir()`, `paths.SetOverrides()` from `paths` package ✓
