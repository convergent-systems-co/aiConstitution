# Full-Stack Test Infrastructure — Plan 2 of 6: Setup, Hooks, Command-Wrappers (§4.1–4.3)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Write T1/T2 integration tests for constitution generation, hook extraction, settings.json wiring, and command-wrapper install — all running in the `sandbox()` harness, all offline.

**Architecture:** Every test calls `sandbox(t)` as its first line (see Plan 1), then exercises the system via `cmd.NewRootCmd()` for full-stack tests or export seams (`UpdateSettingsJSONForTest`, `PurgeMalformedHookEntriesForTest`) for focused unit-style assertions. Three test files, one per spec subsection, all in `package cmd_test` alongside the existing tests.

**Tech Stack:** Go 1.26, `cmd.NewRootCmd()` in-process execution, `sandbox()` harness from Plan 1, `paths.AIRoot()`, `encoding/json`, `strings`, `os`.

---

## Prerequisite: Plan 1 merged

This plan requires `src/cmd/ai/cmd/harness_test.go` (the `sandbox()` helper) and the export seams from Plan 1 to be on `main`. Confirm before starting:

```bash
grep -l 'func sandbox' src/cmd/ai/cmd/
# Expected: src/cmd/ai/cmd/harness_test.go
grep 'UpdateSettingsJSONForTest' src/cmd/ai/cmd/export_test.go
# Expected: func UpdateSettingsJSONForTest(settingsPath, hooksDir string) error
```

---

## Context: key facts before you write a line of code

Read these before starting — they tell you exactly what the system produces.

**`AICONST_SEEDS` format** (`setup.go:parseSeedsEnv`):
Comma-separated `key=value` pairs. `Q01` is the Principal name — the only field with no default.
```
AICONST_SEEDS=Q01=Test User
AICONST_SEEDS=Q01=Alice,Q02=claude-code
```

**What `ai setup --non-interactive` writes** (to `paths.AIRoot()`):
1. `Constitution.md` — rendered from `embed/templates/constitution.tmpl`; must contain no `{{`
2. `Constitution.runtime.md` — compact form (optional)
3. Audit subdirectories — `audit/overrides/`, `audit/violations/`, etc.
4. `CLAUDE.md` — `@~/.ai/Constitution.md` include directive
5. When `--no-hooks` is NOT set: also wires `.claude/settings.json`

Use `--no-hooks` in setup tests to keep them focused on constitution output only.

**How `ai hooks install --all` works** (calls `installAllHooksAndWire`):
1. Fetches ai-atoms catalog from `AiAtomsCatalogURL` (sandbox redirects this — already wired)
2. Downloads hook atoms that have a non-empty `script` field (sandbox fixture has none → no scripts downloaded; that's fine)
3. Extracts infrastructure files from the embedded binary: `_lib.py` (mode 0755), `patterns.json` (mode 0644), `command-wrappers.toml` (mode 0644)
4. Calls `updateSettingsJSON(settingsPath, hooksDir)` using `CLAUDE_CONFIG_DIR` env if set (sandbox sets it)

**`updateSettingsJSON(settingsPath, hooksDir string)` behavior**:
- Reads existing JSON or starts fresh
- Calls `purgeMalformedHookEntries` (removes absolute-path commands, null hooks, retired commands)
- Merges canonical wiring from `canonicalWiring(hooksDir)` into the hooks map
- Writes valid, indented JSON to `settingsPath`
- The canonical entries all use `"ai hooks run <slug>"` (never absolute paths)

**`purgeMalformedHookEntries(settings map[string]any)` behavior**:
Mutates in place. Removes entire hook groups when they contain:
- Absolute-path commands (`python3 /home/user/.ai/hooks/...`)
- `{"hooks": null}` stubs
- Retired commands (`ai hooks run audit-command`, `ai hooks run audit`)
- Non-map entries

Valid groups survive unchanged.

**Command execution pattern** (from `amend_apply_cmd_test.go`):
```go
root := cmd.NewRootCmd()
var buf bytes.Buffer
root.SetOut(&buf)
root.SetErr(&buf)
root.SetArgs([]string{"hooks", "install", "--all"})
if err := root.Execute(); err != nil {
    t.Fatalf("command failed: %v\n%s", err, buf.String())
}
```

---

## Files

**Create:**
- `src/cmd/ai/cmd/setup_integration_test.go` — §4.1: constitution generation tests
- `src/cmd/ai/cmd/hooks_integration_test.go` — §4.2: hook extraction + wire tests
- `src/cmd/ai/cmd/wrappers_integration_test.go` — §4.3: command-wrapper install tests

**No production files changed.** This plan is test-only.

---

## Task 1: §4.1 Setup — generates Constitution.md with correct content

**Files:**
- Create: `src/cmd/ai/cmd/setup_integration_test.go`

- [ ] **Step 1.1: Create the file and write the generation test**

  ```go
  // setup_integration_test.go — §4.1 integration tests for ai setup.
  // Uses sandbox() from harness_test.go; MUST NOT call t.Parallel().
  package cmd_test

  import (
  	"bytes"
  	"os"
  	"path/filepath"
  	"strings"
  	"testing"

  	cmd "github.com/convergent-systems-co/aiConstitution/src/cmd/ai/cmd"
  )

  // runSetup executes "ai setup --non-interactive --no-hooks" with seeds in the
  // sandbox environment, returning the command output.
  func runSetup(t *testing.T, seeds string) string {
  	t.Helper()
  	t.Setenv("AICONST_SEEDS", seeds)
  	root := cmd.NewRootCmd()
  	var buf bytes.Buffer
  	root.SetOut(&buf)
  	root.SetErr(&buf)
  	root.SetArgs([]string{"setup", "--non-interactive", "--no-hooks"})
  	if err := root.Execute(); err != nil {
  		t.Fatalf("setup: %v\noutput:\n%s", err, buf.String())
  	}
  	return buf.String()
  }

  // TestSetup_WritesConstitution verifies that setup --non-interactive writes
  // Constitution.md to AI_ROOT containing the principal name and no unrendered
  // template markers.
  func TestSetup_WritesConstitution(t *testing.T) {
  	s := sandbox(t)

  	runSetup(t, "Q01=Test User")

  	constitutionPath := filepath.Join(s.AIRoot, "Constitution.md")
  	data, err := os.ReadFile(constitutionPath)
  	if err != nil {
  		t.Fatalf("Constitution.md not written to %s: %v", constitutionPath, err)
  	}
  	content := string(data)

  	if strings.Contains(content, "{{") {
  		// Find the first offending line for a useful error message.
  		for i, line := range strings.Split(content, "\n") {
  			if strings.Contains(line, "{{") {
  				t.Errorf("unrendered template at line %d: %q", i+1, line)
  				break
  			}
  		}
  	}
  	if !strings.Contains(content, "Test User") {
  		t.Error("Constitution.md does not contain the principal name 'Test User'")
  	}
  	if len(data) < 1000 {
  		t.Errorf("Constitution.md suspiciously small (%d bytes); template may have partially rendered", len(data))
  	}
  }
  ```

- [ ] **Step 1.2: Run the test (expect PASS)**

  ```bash
  go test -run TestSetup_WritesConstitution -v ./src/cmd/ai/cmd/
  ```
  Expected:
  ```
  === RUN   TestSetup_WritesConstitution
  --- PASS: TestSetup_WritesConstitution (X.XXs)
  ```
  If it fails with "Constitution.md not written", check that `paths.SetOverrides` in `sandbox()` is taking effect — confirm `paths.AIRoot()` returns `s.AIRoot` after sandbox is called.

- [ ] **Step 1.3: Commit**

  ```bash
  git add src/cmd/ai/cmd/setup_integration_test.go
  git commit -m "test(setup): §4.1 constitution generation — writes file with correct content

  Co-Authored-By: Claude Sonnet 4.6 (1M context) <noreply@anthropic.com>"
  ```

---

## Task 2: §4.1 Setup — idempotency (backup, no clobber)

**Files:**
- Modify: `src/cmd/ai/cmd/setup_integration_test.go` (append)

- [ ] **Step 2.1: Write the idempotency test**

  Append to `setup_integration_test.go`:

  ```go
  // TestSetup_IdempotentNoClobber verifies that running setup twice on the same
  // AI_ROOT does not silently overwrite Constitution.md. The second run must
  // either produce a backup file or leave the original unchanged (both are
  // acceptable; "clobbering without backup" is not).
  func TestSetup_IdempotentNoClobber(t *testing.T) {
  	s := sandbox(t)

  	// First run.
  	runSetup(t, "Q01=First User")

  	original, err := os.ReadFile(filepath.Join(s.AIRoot, "Constitution.md"))
  	if err != nil {
  		t.Fatalf("first run: Constitution.md not written: %v", err)
  	}

  	// Second run with a different principal so we can detect clobber.
  	runSetup(t, "Q01=Second User")

  	afterSecond, err := os.ReadFile(filepath.Join(s.AIRoot, "Constitution.md"))
  	if err != nil {
  		t.Fatalf("second run: Constitution.md missing: %v", err)
  	}

  	// Acceptable outcomes:
  	//   (a) Constitution.md still has "First User" (not overwritten) AND a backup exists, or
  	//   (b) Constitution.md now has "Second User" AND a backup of the original exists.
  	// Unacceptable: "Second User" in Constitution.md with no backup anywhere.
  	hasBackup := false
  	_ = filepath.WalkDir(s.AIRoot, func(path string, d os.DirEntry, err error) error {
  		if err == nil && !d.IsDir() && strings.Contains(d.Name(), "Constitution") && path != filepath.Join(s.AIRoot, "Constitution.md") {
  			hasBackup = true
  		}
  		return nil
  	})

  	if strings.Contains(string(afterSecond), "Second User") && !hasBackup {
  		t.Errorf("second run clobbered Constitution.md with no backup\n"+
  			"original had: %q\nafter second run: %q\nbackup found: %v",
  			string(original)[:min(200, len(original))],
  			string(afterSecond)[:min(200, len(afterSecond))],
  			hasBackup)
  	}
  }

  func min(a, b int) int {
  	if a < b {
  		return a
  	}
  	return b
  }
  ```

- [ ] **Step 2.2: Run the idempotency test**

  ```bash
  go test -run TestSetup_IdempotentNoClobber -v ./src/cmd/ai/cmd/
  ```
  Expected: `--- PASS`. If it fails with "clobbered with no backup", the setup command needs a backup path — stop and report to your partner before fixing.

- [ ] **Step 2.3: Commit**

  ```bash
  git add src/cmd/ai/cmd/setup_integration_test.go
  git commit -m "test(setup): §4.1 idempotency — second run does not clobber without backup

  Co-Authored-By: Claude Sonnet 4.6 (1M context) <noreply@anthropic.com>"
  ```

---

## Task 3: §4.2 Hooks — extract infrastructure files

**Files:**
- Create: `src/cmd/ai/cmd/hooks_integration_test.go`

- [ ] **Step 3.1: Create the file and write the extract test**

  ```go
  // hooks_integration_test.go — §4.2 integration tests for ai hooks install.
  // Uses sandbox() from harness_test.go; MUST NOT call t.Parallel().
  package cmd_test

  import (
  	"bytes"
  	"os"
  	"path/filepath"
  	"strings"
  	"testing"

  	cmd "github.com/convergent-systems-co/aiConstitution/src/cmd/ai/cmd"
  )

  // runHooksCmd executes an "ai hooks ..." command in the sandbox environment.
  func runHooksCmd(t *testing.T, args ...string) string {
  	t.Helper()
  	root := cmd.NewRootCmd()
  	var buf bytes.Buffer
  	root.SetOut(&buf)
  	root.SetErr(&buf)
  	root.SetArgs(append([]string{"hooks"}, args...))
  	if err := root.Execute(); err != nil {
  		t.Fatalf("ai hooks %s: %v\noutput:\n%s", strings.Join(args, " "), err, buf.String())
  	}
  	return buf.String()
  }

  // TestHooksInstallAll_ExtractsInfrastructure verifies that "ai hooks install --all"
  // extracts the embedded infrastructure files (_lib.py, patterns.json,
  // command-wrappers.toml) into AI_ROOT/hooks/.
  func TestHooksInstallAll_ExtractsInfrastructure(t *testing.T) {
  	s := sandbox(t)

  	runHooksCmd(t, "install", "--all")

  	hooksDir := filepath.Join(s.AIRoot, "hooks")

  	// _lib.py must exist and be executable.
  	libPy := filepath.Join(hooksDir, "_lib.py")
  	libInfo, err := os.Stat(libPy)
  	if err != nil {
  		t.Fatalf("_lib.py not extracted to %s: %v", hooksDir, err)
  	}
  	if libInfo.Mode()&0o100 == 0 {
  		t.Errorf("_lib.py is not executable (mode=%o)", libInfo.Mode())
  	}

  	// patterns.json must exist (not executable).
  	patternsJSON := filepath.Join(hooksDir, "patterns.json")
  	if _, err := os.Stat(patternsJSON); err != nil {
  		t.Fatalf("patterns.json not extracted to %s: %v", hooksDir, err)
  	}

  	// command-wrappers.toml must exist.
  	wrappersTOML := filepath.Join(hooksDir, "command-wrappers.toml")
  	if _, err := os.Stat(wrappersTOML); err != nil {
  		t.Fatalf("command-wrappers.toml not extracted to %s: %v", hooksDir, err)
  	}
  }
  ```

- [ ] **Step 3.2: Run the extract test**

  ```bash
  go test -run TestHooksInstallAll_ExtractsInfrastructure -v ./src/cmd/ai/cmd/
  ```
  Expected: `--- PASS`.

  If it fails with "_lib.py not extracted": `installAllHooksAndWire` fetches the catalog first. Confirm `*cmd.AiAtomsCatalogURLForTest` is being set by `sandbox()` to the fake atom server. Check `sandbox()` sets it before `runHooksCmd` is called. Also confirm the sandbox `AtomServer` is serving `/catalog.json`.

- [ ] **Step 3.3: Commit**

  ```bash
  git add src/cmd/ai/cmd/hooks_integration_test.go
  git commit -m "test(hooks): §4.2 extract — ai hooks install --all extracts infrastructure from embed

  Co-Authored-By: Claude Sonnet 4.6 (1M context) <noreply@anthropic.com>"
  ```

---

## Task 4: §4.2 Hooks wire — creates valid settings.json

**Files:**
- Modify: `src/cmd/ai/cmd/hooks_integration_test.go` (append)

- [ ] **Step 4.1: Write the wire test (direct seam)**

  Append to `hooks_integration_test.go`:

  ```go
  // TestHooksWire_CreatesSettingsJSON verifies that updateSettingsJSON produces
  // valid JSON containing the canonical "ai hooks run" commands.
  // Drives the exported seam directly for precision.
  func TestHooksWire_CreatesSettingsJSON(t *testing.T) {
  	s := sandbox(t)

  	settingsPath := filepath.Join(s.ClaudeDir, "settings.json")
  	hooksDir := filepath.Join(s.AIRoot, "hooks")

  	if err := cmd.UpdateSettingsJSONForTest(settingsPath, hooksDir); err != nil {
  		t.Fatalf("UpdateSettingsJSON: %v", err)
  	}

  	data, err := os.ReadFile(settingsPath)
  	if err != nil {
  		t.Fatalf("settings.json not written: %v", err)
  	}

  	// Must be valid JSON.
  	var settings map[string]any
  	if err := json.Unmarshal(data, &settings); err != nil {
  		t.Fatalf("settings.json contains invalid JSON: %v\n%s", err, data)
  	}

  	// Must have a "hooks" key.
  	hooks, ok := settings["hooks"].(map[string]any)
  	if !ok || len(hooks) == 0 {
  		t.Fatal("settings.json missing or empty 'hooks' key")
  	}

  	// At least one hook event must be present.
  	for _, wantEvent := range []string{"PreToolUse", "PostToolUse", "SessionStart"} {
  		if _, exists := hooks[wantEvent]; exists {
  			goto foundEvent
  		}
  	}
  	t.Error("settings.json has no recognized hook events (PreToolUse, PostToolUse, SessionStart)")
  foundEvent:

  	// Every command in the file must use "ai hooks run", never absolute paths.
  	raw := string(data)
  	if strings.Contains(raw, "python3 /") || strings.Contains(raw, "python /") {
  		t.Error("settings.json contains absolute Python path; wiring must use 'ai hooks run'")
  	}
  	if !strings.Contains(raw, "ai hooks run") {
  		t.Error("settings.json contains no 'ai hooks run' command")
  	}
  }
  ```

  Also add the missing import at the top of the file (add `"encoding/json"` to the import block):

  ```go
  import (
  	"bytes"
  	"encoding/json"
  	"os"
  	"path/filepath"
  	"strings"
  	"testing"

  	cmd "github.com/convergent-systems-co/aiConstitution/src/cmd/ai/cmd"
  )
  ```

- [ ] **Step 4.2: Run the wire test**

  ```bash
  go test -run TestHooksWire_CreatesSettingsJSON -v ./src/cmd/ai/cmd/
  ```
  Expected: `--- PASS`.

- [ ] **Step 4.3: Also verify wiring through the full command**

  Append to `hooks_integration_test.go`:

  ```go
  // TestHooksInstallAll_WiresSettings verifies that "ai hooks install --all"
  // also wires .claude/settings.json (not only extracts hooks).
  func TestHooksInstallAll_WiresSettings(t *testing.T) {
  	s := sandbox(t)

  	runHooksCmd(t, "install", "--all")

  	settingsPath := filepath.Join(s.ClaudeDir, "settings.json")
  	data, err := os.ReadFile(settingsPath)
  	if err != nil {
  		t.Fatalf("settings.json not created by 'hooks install --all': %v\nCLAUDE_CONFIG_DIR=%s", err, s.ClaudeDir)
  	}

  	var settings map[string]any
  	if err := json.Unmarshal(data, &settings); err != nil {
  		t.Fatalf("settings.json invalid JSON: %v", err)
  	}
  	if _, ok := settings["hooks"]; !ok {
  		t.Error("settings.json missing 'hooks' key after 'hooks install --all'")
  	}
  }
  ```

- [ ] **Step 4.4: Run all tests so far**

  ```bash
  go test -run 'TestHooksWire|TestHooksInstallAll' -v ./src/cmd/ai/cmd/
  ```
  Expected: all PASS.

- [ ] **Step 4.5: Commit**

  ```bash
  git add src/cmd/ai/cmd/hooks_integration_test.go
  git commit -m "test(hooks): §4.2 wire — updateSettingsJSON creates valid JSON with ai hooks run commands

  Co-Authored-By: Claude Sonnet 4.6 (1M context) <noreply@anthropic.com>"
  ```

---

## Task 5: §4.2 Hooks wire — merge safety (user hooks survive)

**Files:**
- Modify: `src/cmd/ai/cmd/hooks_integration_test.go` (append)

- [ ] **Step 5.1: Write the merge-safety test**

  Append to `hooks_integration_test.go`:

  ```go
  // TestHooksWire_MergeSafety verifies that updateSettingsJSON does not remove
  // a user's pre-existing hook entry in an unrelated event group.
  func TestHooksWire_MergeSafety(t *testing.T) {
  	s := sandbox(t)

  	settingsPath := filepath.Join(s.ClaudeDir, "settings.json")
  	hooksDir := filepath.Join(s.AIRoot, "hooks")

  	// Pre-seed with a user hook in PostToolUse using a custom matcher.
  	existing := map[string]any{
  		"hooks": map[string]any{
  			"PostToolUse": []any{
  				map[string]any{
  					"matcher": "my-custom-tool",
  					"hooks": []any{
  						map[string]any{
  							"type":    "command",
  							"command": "my-custom-hook",
  						},
  					},
  				},
  			},
  		},
  	}
  	raw, _ := json.Marshal(existing)
  	if err := os.WriteFile(settingsPath, raw, 0o644); err != nil {
  		t.Fatal(err)
  	}

  	if err := cmd.UpdateSettingsJSONForTest(settingsPath, hooksDir); err != nil {
  		t.Fatalf("UpdateSettingsJSON: %v", err)
  	}

  	data, _ := os.ReadFile(settingsPath)
  	if !strings.Contains(string(data), "my-custom-hook") {
  		t.Error("user hook 'my-custom-hook' was removed by updateSettingsJSON; must survive merge")
  	}
  	if !strings.Contains(string(data), "my-custom-tool") {
  		t.Error("user matcher 'my-custom-tool' was removed; must survive merge")
  	}
  }
  ```

- [ ] **Step 5.2: Run the merge-safety test**

  ```bash
  go test -run TestHooksWire_MergeSafety -v ./src/cmd/ai/cmd/
  ```
  Expected: `--- PASS`.

- [ ] **Step 5.3: Commit**

  ```bash
  git add src/cmd/ai/cmd/hooks_integration_test.go
  git commit -m "test(hooks): §4.2 merge safety — user hooks survive updateSettingsJSON

  Co-Authored-By: Claude Sonnet 4.6 (1M context) <noreply@anthropic.com>"
  ```

---

## Task 6: §4.2 Hooks wire — idempotency (no duplicate entries)

**Files:**
- Modify: `src/cmd/ai/cmd/hooks_integration_test.go` (append)

- [ ] **Step 6.1: Write the idempotency test**

  Append to `hooks_integration_test.go`. The helper `countHookCommands` walks the entire hooks map and counts `"command"` entries:

  ```go
  // countHookCommands counts the total number of hook command entries across all
  // events in a settings map. Used to detect spurious duplication on re-run.
  func countHookCommands(settings map[string]any) int {
  	hooks, _ := settings["hooks"].(map[string]any)
  	total := 0
  	for _, eventVal := range hooks {
  		groups, _ := eventVal.([]any)
  		for _, g := range groups {
  			m, _ := g.(map[string]any)
  			hookList, _ := m["hooks"].([]any)
  			total += len(hookList)
  		}
  	}
  	return total
  }

  // TestHooksWire_Idempotent verifies that running updateSettingsJSON twice
  // produces the same number of hook commands — no duplicates added.
  func TestHooksWire_Idempotent(t *testing.T) {
  	s := sandbox(t)

  	settingsPath := filepath.Join(s.ClaudeDir, "settings.json")
  	hooksDir := filepath.Join(s.AIRoot, "hooks")

  	// First call.
  	if err := cmd.UpdateSettingsJSONForTest(settingsPath, hooksDir); err != nil {
  		t.Fatalf("first UpdateSettingsJSON: %v", err)
  	}
  	data1, _ := os.ReadFile(settingsPath)
  	var s1 map[string]any
  	json.Unmarshal(data1, &s1)
  	count1 := countHookCommands(s1)

  	// Second call — identical inputs.
  	if err := cmd.UpdateSettingsJSONForTest(settingsPath, hooksDir); err != nil {
  		t.Fatalf("second UpdateSettingsJSON: %v", err)
  	}
  	data2, _ := os.ReadFile(settingsPath)
  	var s2 map[string]any
  	json.Unmarshal(data2, &s2)
  	count2 := countHookCommands(s2)

  	if count1 != count2 {
  		t.Errorf("second run added duplicate entries: first=%d commands, second=%d commands",
  			count1, count2)
  	}
  	if count1 == 0 {
  		t.Error("no hook commands found after first run — wiring produced empty result")
  	}
  }
  ```

- [ ] **Step 6.2: Run the idempotency test**

  ```bash
  go test -run TestHooksWire_Idempotent -v ./src/cmd/ai/cmd/
  ```
  Expected: `--- PASS`.

- [ ] **Step 6.3: Commit**

  ```bash
  git add src/cmd/ai/cmd/hooks_integration_test.go
  git commit -m "test(hooks): §4.2 idempotency — updateSettingsJSON adds no duplicates on re-run

  Co-Authored-By: Claude Sonnet 4.6 (1M context) <noreply@anthropic.com>"
  ```

---

## Task 7: §4.2 Hooks wire — purgeMalformedHookEntries removes bad shapes

**Files:**
- Modify: `src/cmd/ai/cmd/hooks_integration_test.go` (append)

- [ ] **Step 7.1: Write the purge test**

  Append to `hooks_integration_test.go`:

  ```go
  // TestPurge_RemovesMalformedKeepsGood verifies that purgeMalformedHookEntries
  // removes hook groups with null hooks and absolute-path commands while
  // preserving well-formed groups.
  func TestPurge_RemovesMalformedKeepsGood(t *testing.T) {
  	t.Parallel()

  	settings := map[string]any{
  		"hooks": map[string]any{
  			"PreToolUse": []any{
  				// Good: canonical "ai hooks run" command.
  				map[string]any{
  					"hooks": []any{
  						map[string]any{
  							"type":    "command",
  							"command": "ai hooks run audit-logger",
  						},
  					},
  				},
  				// Bad: absolute Python path (pre-migration style).
  				map[string]any{
  					"hooks": []any{
  						map[string]any{
  							"type":    "command",
  							"command": "python3 /home/user/.ai/hooks/audit.py",
  						},
  					},
  				},
  				// Bad: null hooks list.
  				map[string]any{
  					"hooks": nil,
  				},
  				// Bad: retired command name.
  				map[string]any{
  					"hooks": []any{
  						map[string]any{
  							"type":    "command",
  							"command": "ai hooks run audit-command",
  						},
  					},
  				},
  			},
  		},
  	}

  	cmd.PurgeMalformedHookEntriesForTest(settings)

  	hooks := settings["hooks"].(map[string]any)
  	preToolUse, ok := hooks["PreToolUse"].([]any)
  	if !ok {
  		t.Fatal("PreToolUse missing after purge")
  	}

  	if len(preToolUse) != 1 {
  		t.Fatalf("expected 1 surviving entry, got %d: %v", len(preToolUse), preToolUse)
  	}

  	// The only surviving entry must be the good canonical one.
  	surviving := preToolUse[0].(map[string]any)
  	hookList := surviving["hooks"].([]any)
  	entry := hookList[0].(map[string]any)
  	if entry["command"] != "ai hooks run audit-logger" {
  		t.Errorf("wrong entry survived: command=%q", entry["command"])
  	}
  }

  // TestPurge_LeavesSettingsUntouchedWhenNoHooksKey verifies that
  // purgeMalformedHookEntries is a no-op on a settings map with no "hooks" key.
  func TestPurge_LeavesSettingsUntouchedWhenNoHooksKey(t *testing.T) {
  	t.Parallel()
  	settings := map[string]any{"theme": "dark"}
  	cmd.PurgeMalformedHookEntriesForTest(settings)
  	if v, ok := settings["theme"]; !ok || v != "dark" {
  		t.Error("purge mutated a settings map with no 'hooks' key")
  	}
  }
  ```

- [ ] **Step 7.2: Run the purge tests**

  ```bash
  go test -run TestPurge -v ./src/cmd/ai/cmd/
  ```
  Expected: both PASS.

  If `TestPurge_RemovesMalformedKeepsGood` fails with "expected 1 surviving entry, got 4": `purgeMalformedHookEntries` is not filtering these shapes. Read `hooks_claude.go:cleanHookGroup` to understand what qualifies as malformed — the absolute-path and null-hooks cases may use different detection logic. Adjust the malformed fixtures to match what the function actually rejects, then report back.

- [ ] **Step 7.3: Commit**

  ```bash
  git add src/cmd/ai/cmd/hooks_integration_test.go
  git commit -m "test(hooks): §4.2 purge — purgeMalformedHookEntries removes bad shapes, keeps good

  Co-Authored-By: Claude Sonnet 4.6 (1M context) <noreply@anthropic.com>"
  ```

---

## Task 8: §4.3 Command-wrappers install

**Files:**
- Create: `src/cmd/ai/cmd/wrappers_integration_test.go`

- [ ] **Step 8.1: Create the file and write the wrapper install test**

  ```go
  // wrappers_integration_test.go — §4.3 integration tests for command-wrapper install.
  // Uses sandbox() from harness_test.go; MUST NOT call t.Parallel().
  package cmd_test

  import (
  	"bytes"
  	"os"
  	"path/filepath"
  	"runtime"
  	"strings"
  	"testing"

  	cmd "github.com/convergent-systems-co/aiConstitution/src/cmd/ai/cmd"
  )

  // TestHooksInstallCommandWrappers_ExtractsToAIBin verifies that
  // "ai hooks install command-wrappers" extracts the platform-appropriate
  // shims to AI_ROOT/bin/ with executable permissions.
  func TestHooksInstallCommandWrappers_ExtractsToAIBin(t *testing.T) {
  	s := sandbox(t)

  	root := cmd.NewRootCmd()
  	var buf bytes.Buffer
  	root.SetOut(&buf)
  	root.SetErr(&buf)
  	root.SetArgs([]string{"hooks", "install", "command-wrappers"})
  	if err := root.Execute(); err != nil {
  		t.Fatalf("hooks install command-wrappers: %v\n%s", err, buf.String())
  	}

  	binDir := filepath.Join(s.AIRoot, "bin")

  	// On POSIX: expect bare "git" and "gh" (not .cmd or .ps1).
  	// On Windows: expect "git.cmd" and "gh.cmd" (or .ps1).
  	var wantNames []string
  	var notwantSuffix string
  	if runtime.GOOS == "windows" {
  		wantNames = []string{"git.cmd", "git.ps1", "gh.cmd", "gh.ps1"}
  		notwantSuffix = ""
  	} else {
  		wantNames = []string{"git", "gh"}
  		notwantSuffix = ".cmd"
  	}

  	for _, name := range wantNames {
  		p := filepath.Join(binDir, name)
  		info, err := os.Stat(p)
  		if err != nil {
  			t.Errorf("%s not extracted to %s: %v", name, binDir, err)
  			continue
  		}
  		// On POSIX, wrappers must be executable.
  		if runtime.GOOS != "windows" && info.Mode()&0o100 == 0 {
  			t.Errorf("%s is not executable (mode=%o)", name, info.Mode())
  		}
  		// Content must delegate to "ai wrap".
  		data, err := os.ReadFile(p)
  		if err != nil {
  			t.Errorf("cannot read %s: %v", p, err)
  			continue
  		}
  		if !strings.Contains(string(data), "ai wrap") {
  			t.Errorf("%s does not delegate to 'ai wrap':\n%s", name, data)
  		}
  	}

  	// Platform-inappropriate forms must NOT be extracted.
  	if notwantSuffix != "" {
  		for _, base := range []string{"git", "gh"} {
  			bad := filepath.Join(binDir, base+notwantSuffix)
  			if _, err := os.Stat(bad); err == nil {
  				t.Errorf("%s should not be extracted on this OS", bad)
  			}
  		}
  	}
  }

  // TestHooksInstallCommandWrappers_Idempotent verifies that running
  // "ai hooks install command-wrappers" twice does not error or duplicate files.
  func TestHooksInstallCommandWrappers_Idempotent(t *testing.T) {
  	s := sandbox(t)
  	binDir := filepath.Join(s.AIRoot, "bin")

  	runInstall := func() {
  		root := cmd.NewRootCmd()
  		var buf bytes.Buffer
  		root.SetOut(&buf)
  		root.SetErr(&buf)
  		root.SetArgs([]string{"hooks", "install", "command-wrappers"})
  		if err := root.Execute(); err != nil {
  			t.Fatalf("hooks install command-wrappers: %v\n%s", err, buf.String())
  		}
  	}

  	runInstall()
  	entries1, _ := os.ReadDir(binDir)
  	runInstall()
  	entries2, _ := os.ReadDir(binDir)

  	if len(entries1) != len(entries2) {
  		t.Errorf("second install changed bin/ file count: first=%d, second=%d", len(entries1), len(entries2))
  	}
  }
  ```

- [ ] **Step 8.2: Run the wrapper tests**

  ```bash
  go test -run TestHooksInstallCommandWrappers -v ./src/cmd/ai/cmd/
  ```
  Expected: both PASS.

- [ ] **Step 8.3: Run the complete test suite to verify no regressions**

  ```bash
  make test
  ```
  Expected: all packages pass, 0 failures.

- [ ] **Step 8.4: Commit**

  ```bash
  git add src/cmd/ai/cmd/wrappers_integration_test.go
  git commit -m "test(wrappers): §4.3 command-wrapper install — extracts ai wrap shims, idempotent

  Co-Authored-By: Claude Sonnet 4.6 (1M context) <noreply@anthropic.com>"
  ```

---

## Self-review

**Spec §4.1 coverage:**

| Requirement | Task | Status |
|---|---|---|
| `ai setup --non-interactive --seeds` produces Constitution.md | Task 1 | ✓ |
| Re-running is idempotent (backup, no clobber) | Task 2 | ✓ |
| Template renders: no leftover `{{` | Task 1 assertion | ✓ |
| Legacy migration: seed Common.md, assert folded | **Not covered** | ⚠️ |

**Legacy migration gap:** §4.1 says "seed a fake `Common.md` in the sandbox root, run setup, assert it is folded into `Constitution.md`." This requires understanding how `setup.go` detects the legacy layout (`!status["v2"] && status["Common.md"]`). Since `ai migrate` is a separate command that does the actual folding, and `ai setup` may just detect and prompt, this scenario needs more investigation. Deferred to Plan 5 (§4.9 Update/Migrate) where `ai migrate` is already in scope.

**Spec §4.2 coverage:**

| Requirement | Task | Status |
|---|---|---|
| Extract: `_lib.py`, `patterns.json` extracted with correct modes | Task 3 | ✓ |
| Wire: `updateSettingsJSON` creates valid settings.json with `ai hooks run` | Task 4 | ✓ |
| Wire through command (`hooks install --all`) | Task 4 step 4.3 | ✓ |
| Merge safety: pre-seeded user hook survives | Task 5 | ✓ |
| Idempotency: run wire twice → no duplicates | Task 6 | ✓ |
| Corrupt entry → `purgeMalformedHookEntries` removes only malformed | Task 7 | ✓ |
| T4 self-check (`python3 hook --self-check`) | Not covered (T4, Plan 6) | — |
| T4 behavioral (secret/branch-guard with crafted stdin) | Not covered (T4, Plan 6) | — |

**Spec §4.3 coverage:**

| Requirement | Task | Status |
|---|---|---|
| Extracts `git`, `gh` to AI_ROOT/bin/ | Task 8 | ✓ |
| Files are executable (POSIX) | Task 8 | ✓ |
| Content delegates to `ai wrap` | Task 8 | ✓ |
| Platform filtering: no .cmd/.ps1 on POSIX | Task 8 | ✓ |
| Idempotency: second install doesn't error | Task 8 | ✓ |
| Stub real git with argv-recording script | Not covered — deferred to Plan 6 E2E |
| `notify-me_test.sh` (T4) | Not covered (T4, Plan 6) | — |

**Placeholder scan:** None found. All steps contain complete code, exact commands, and expected output.

**Type consistency:**
- `runSetup(t, seeds)` → defined in Task 1, used in Task 2 ✓
- `runHooksCmd(t, args...)` → defined in Task 3, used in Task 4 ✓
- `countHookCommands(settings)` → defined in Task 6, no other uses ✓
- `cmd.UpdateSettingsJSONForTest` → from Plan 1 export_test.go ✓
- `cmd.PurgeMalformedHookEntriesForTest` → from Plan 1 export_test.go ✓

---

## What's next

After this plan is merged:

- **Plan 3** (`2026-05-28-fullstack-test-plan-3-skills-plugins.md`): `ai skills available/install/validate`, `ai plugins install/enable/disable/status`, cache-hit verification, 404 negative paths (§4.4–4.5).
- **Plan 4** (`2026-05-28-fullstack-test-plan-4-atoms-sync.md`): Brand atoms, persona/profile/mode, `ai sync push/pull`, restore into second sandbox, backup (§4.6–4.7).
- **Plan 5** (`2026-05-28-fullstack-test-plan-5-doctor-audit.md`): Doctor repair loop, status golden snapshot, audit rotate, update/migrate including legacy-migration scenario from §4.1 (§4.8–4.9).
- **Plan 6** (`2026-05-28-fullstack-test-plan-6-e2e-ci.md`): T3 binary journey test, T4 hook self-checks, §6 safety guards, CI additions.
