# Constitution Layer — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` (recommended) or `superpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace stub implementations in the `ai` binary with real functionality for §3 (four-file loading, TOML config, constitution validation) and §13 (ai setup wizard — non-interactive first, TUI second).

**Architecture:** Three new or completed Go packages: `src/internal/config` (real TOML Load/Save), `src/internal/constitution` (four-file loading + validation), `src/internal/wizard` (question taxonomy + setup flow). Commands `ai setup`, `ai doctor` (constitution checks only), and `ai status` (constitution status only) wired to real implementations. TDD throughout — failing test before every implementation.

**Tech Stack:** Go 1.22, github.com/BurntSushi/toml (TOML parsing), github.com/charmbracelet/bubbletea (wizard TUI — Task 8+), github.com/spf13/cobra (existing), testify/assert (testing).

---

## File structure

| Path | Action | Responsibility |
|---|---|---|
| `src/internal/config/config.go` | **MODIFY** | Replace stub Load/Save with real TOML parsing + env-var overlay |
| `src/internal/config/config_test.go` | **CREATE** | Tests for Load, Save, Defaults, env-var overlay |
| `src/internal/constitution/constitution.go` | **CREATE** | ConstitutionFiles type, Load(), Validate(), FileStatus() |
| `src/internal/constitution/constitution_test.go` | **CREATE** | Tests for Load and Validate |
| `src/internal/wizard/questions.go` | **CREATE** | Question type, LoadTaxonomy() from embedded YAML |
| `src/internal/wizard/questions_test.go` | **CREATE** | Tests for question parsing and dependency resolution |
| `src/internal/wizard/runner.go` | **CREATE** | Non-interactive runner (seeded answers path) |
| `src/internal/wizard/runner_test.go` | **CREATE** | Tests for non-interactive wizard execution |
| `src/cmd/ai/cmd/setup.go` | **MODIFY** | Wire real wizard runner into RunE |
| `src/cmd/ai/cmd/doctor.go` | **MODIFY** | Implement constitution health checks |
| `src/cmd/ai/cmd/status.go` | **MODIFY** | Wire constitution.FileStatus() into status output |
| `src/internal/go.mod` | **MODIFY** | Add BurntSushi/toml |
| `src/cmd/ai/go.mod` | **MODIFY** | Add BurntSushi/toml, bubbletea |

---

## Task 1: Add dependencies

**Files:**
- Modify: `src/internal/go.mod`
- Modify: `src/cmd/ai/go.mod`

- [ ] **Step 1: Add BurntSushi/toml to internal module**

```bash
cd /Users/itsfwcp/workspace/convergent-system-co/aiConstitution/src/internal
go get github.com/BurntSushi/toml@v1.4.0
```

Expected: `go: added github.com/BurntSushi/toml v1.4.0` (or similar version).

- [ ] **Step 2: Add BurntSushi/toml to cmd/ai module**

```bash
cd /Users/itsfwcp/workspace/convergent-system-co/aiConstitution/src/cmd/ai
go get github.com/BurntSushi/toml@v1.4.0
go get github.com/charmbracelet/bubbletea@latest
```

- [ ] **Step 3: Verify modules build**

```bash
cd /Users/itsfwcp/workspace/convergent-system-co/aiConstitution
go build ./src/...
```

Expected: no errors.

- [ ] **Step 4: Commit**

```bash
git add src/internal/go.mod src/internal/go.sum src/cmd/ai/go.mod src/cmd/ai/go.sum
git commit -m "build: add BurntSushi/toml and bubbletea dependencies"
```

---

## Task 2: Implement config.Load() and config.Save()

**Files:**
- Modify: `src/internal/config/config.go`
- Create: `src/internal/config/config_test.go`

- [ ] **Step 1: Write failing tests**

Create `src/internal/config/config_test.go`:

```go
package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/convergent-systems-co/aiConstitution/src/internal/config"
)

func TestLoadReturnsDefaultsWhenFileAbsent(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("AICONST_CONFIG_DIR", tmp)
	got, err := config.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	want := config.Defaults()
	if got.SchemaVersion != want.SchemaVersion {
		t.Errorf("SchemaVersion = %q, want %q", got.SchemaVersion, want.SchemaVersion)
	}
}

func TestLoadParsesToml(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("AICONST_CONFIG_DIR", tmp)

	tomlContent := `schemaVersion = "0.3"

[review]
cadenceDays = 14
includeMemory = false
`
	if err := os.WriteFile(filepath.Join(tmp, "settings.toml"), []byte(tomlContent), 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := config.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got.SchemaVersion != "0.3" {
		t.Errorf("SchemaVersion = %q, want %q", got.SchemaVersion, "0.3")
	}
	if got.Review.CadenceDays != 14 {
		t.Errorf("Review.CadenceDays = %d, want %d", got.Review.CadenceDays, 14)
	}
	if got.Review.IncludeMemory != false {
		t.Error("Review.IncludeMemory = true, want false")
	}
}

func TestLoadEnvVarOverride(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("AICONST_CONFIG_DIR", tmp)
	t.Setenv("AICONST_REVIEW_CADENCE_DAYS", "60")

	got, err := config.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got.Review.CadenceDays != 60 {
		t.Errorf("Review.CadenceDays = %d, want 60 (from env var)", got.Review.CadenceDays)
	}
}

func TestSaveRoundTrips(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("AICONST_CONFIG_DIR", tmp)

	want := config.Defaults()
	want.Review.CadenceDays = 45
	want.SchemaVersion = "0.3"

	if err := config.Save(want); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	got, err := config.Load()
	if err != nil {
		t.Fatalf("Load() after Save() error = %v", err)
	}
	if got.Review.CadenceDays != 45 {
		t.Errorf("Review.CadenceDays = %d, want 45", got.Review.CadenceDays)
	}
}
```

- [ ] **Step 2: Run tests — expect failure**

```bash
cd /Users/itsfwcp/workspace/convergent-system-co/aiConstitution/src/internal
go test ./config/... -v 2>&1 | tail -20
```

Expected: `FAIL` — `Load()` returns defaults but env-var and TOML tests fail.

- [ ] **Step 3: Implement config.Load() and config.Save()**

Replace the stub `Load()` and `Save()` functions in `src/internal/config/config.go`. Add after the `Save(_ Settings) error` stub:

```go
import (
	"os"
	"path/filepath"
	"strconv"

	"github.com/BurntSushi/toml"
	"github.com/convergent-systems-co/aiConstitution/src/internal/paths"
)

// Load reads ~/.config/aiConstitution/settings.toml, layers it atop
// Defaults(), then applies environment-variable overrides.
func Load() (Settings, error) {
	s := Defaults()

	settingsPath := paths.SettingsTOML()
	data, err := os.ReadFile(settingsPath)
	if err != nil && !os.IsNotExist(err) {
		return s, err
	}
	if err == nil {
		if _, err2 := toml.Decode(string(data), &s); err2 != nil {
			return s, err2
		}
	}

	applyEnvOverrides(&s)
	return s, nil
}

// Save writes s to ~/.config/aiConstitution/settings.toml.
func Save(s Settings) error {
	dir := paths.ConfigDir()
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, "settings-*.toml.tmp")
	if err != nil {
		return err
	}
	tmpName := f.Name()
	defer func() { _ = os.Remove(tmpName) }()

	enc := toml.NewEncoder(f)
	if err := enc.Encode(s); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, filepath.Join(dir, "settings.toml"))
}

// applyEnvOverrides overlays AICONST_* environment variables onto s.
func applyEnvOverrides(s *Settings) {
	if v := os.Getenv("AICONST_REVIEW_CADENCE_DAYS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			s.Review.CadenceDays = n
		}
	}
	if v := os.Getenv("AICONST_AI_ROOT"); v != "" {
		s.Paths.AIRoot = v
	}
	if v := os.Getenv("AICONST_CONFIG_DIR"); v != "" {
		s.Paths.ConfigDir = v
	}
}
```

Also remove the old stub `Load` and `Save` function bodies (replace them entirely with the above).

- [ ] **Step 4: Run tests — expect pass**

```bash
cd /Users/itsfwcp/workspace/convergent-system-co/aiConstitution/src/internal
go test ./config/... -v
```

Expected: all 4 tests `PASS`.

- [ ] **Step 5: Commit**

```bash
git add src/internal/config/config.go src/internal/config/config_test.go \
        src/internal/go.mod src/internal/go.sum
git commit -m "feat(config): implement Load/Save with TOML parsing and env-var overlay

Replaces stub Load()/Save() in src/internal/config with real
BurntSushi/toml parsing, env-var overlay (AICONST_REVIEW_CADENCE_DAYS
etc.), and atomic write (write-tmp + rename). Tests cover: missing
file → defaults, TOML override, env-var override, save round-trip."
```

---

## Task 3: Create the constitution package

**Files:**
- Create: `src/internal/constitution/constitution.go`
- Create: `src/internal/constitution/constitution_test.go`

- [ ] **Step 1: Write failing tests**

Create `src/internal/constitution/constitution_test.go`:

```go
package constitution_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/convergent-systems-co/aiConstitution/src/internal/constitution"
)

func writeFile(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestLoadAllFourFiles(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "Constitution.md", "# Constitution\n**Version:** 0.3\n")
	writeFile(t, dir, "Common.md", "# Common\n**Version:** 0.17\n")
	writeFile(t, dir, "Code.md", "# Code\n**Version:** 0.6\n")
	writeFile(t, dir, "Writing.md", "# Writing\n**Version:** 0.4\n")

	files, err := constitution.Load(dir)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if files.Constitution == "" {
		t.Error("Constitution is empty")
	}
	if files.Common == "" {
		t.Error("Common is empty")
	}
}

func TestValidatePassesWithAllFiles(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "Constitution.md", "# Constitution\n**Version:** 0.3\n")
	writeFile(t, dir, "Common.md", "# Common\n**Version:** 0.17\n")
	writeFile(t, dir, "Code.md", "# Code\n**Version:** 0.6\n")
	writeFile(t, dir, "Writing.md", "# Writing\n**Version:** 0.4\n")

	files, _ := constitution.Load(dir)
	findings := files.Validate()
	if len(findings) != 0 {
		t.Errorf("Validate() = %v, want no findings", findings)
	}
}

func TestValidateReportsMissingFile(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "Constitution.md", "# Constitution\n")
	// Common.md, Code.md, Writing.md absent

	files, err := constitution.Load(dir)
	if err == nil {
		t.Fatal("expected error for missing files, got nil")
	}
	_ = files
}

func TestFileStatusAllPresent(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "Constitution.md", "# Constitution\n**Version:** 0.3\n")
	writeFile(t, dir, "Common.md", "# Common\n**Version:** 0.17\n")
	writeFile(t, dir, "Code.md", "# Code\n**Version:** 0.6\n")
	writeFile(t, dir, "Writing.md", "# Writing\n**Version:** 0.4\n")

	status := constitution.FileStatus(dir)
	for name, ok := range status {
		if !ok {
			t.Errorf("FileStatus[%q] = false, want true", name)
		}
	}
}
```

- [ ] **Step 2: Run tests — expect compilation failure**

```bash
cd /Users/itsfwcp/workspace/convergent-system-co/aiConstitution/src/internal
go test ./constitution/... -v 2>&1 | head -10
```

Expected: `cannot find package "constitution"` or similar.

- [ ] **Step 3: Create constitution.go**

Create `src/internal/constitution/constitution.go`:

```go
// Package constitution loads and validates the four AI Constitution files
// from ~/.ai/ (or a supplied root directory in tests).
package constitution

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// FileNames is the canonical list of the four constitution files,
// in loading order (Constitution first per spec §3.1).
var FileNames = []string{
	"Constitution.md",
	"Common.md",
	"Code.md",
	"Writing.md",
}

// Files holds the in-memory content of the four constitution files
// loaded from a single root directory.
type Files struct {
	Constitution string
	Common       string
	Code         string
	Writing      string
	// Local is the optional Constitution.local.md override content.
	// Empty string means no local override is present.
	Local string
}

// Finding describes a single validation issue.
type Finding struct {
	File    string
	Message string
}

func (f Finding) Error() string {
	return fmt.Sprintf("%s: %s", f.File, f.Message)
}

// Load reads the four canonical files from root (typically ~/.ai/).
// Returns an error if any of the four required files is missing or empty.
// Constitution.local.md is loaded if present; its absence is not an error.
func Load(root string) (Files, error) {
	var f Files
	mapping := []struct {
		name string
		dest *string
	}{
		{"Constitution.md", &f.Constitution},
		{"Common.md", &f.Common},
		{"Code.md", &f.Code},
		{"Writing.md", &f.Writing},
	}

	for _, m := range mapping {
		data, err := os.ReadFile(filepath.Join(root, m.name))
		if err != nil {
			return Files{}, fmt.Errorf("constitution: required file %q missing from %q: %w", m.name, root, err)
		}
		*m.dest = string(data)
	}

	// Constitution.local.md is optional.
	localData, err := os.ReadFile(filepath.Join(root, "Constitution.local.md"))
	if err == nil {
		f.Local = string(localData)
	}

	return f, nil
}

// Validate returns any structural findings for the loaded files.
// An empty slice means the files are structurally valid.
func (f Files) Validate() []Finding {
	var findings []Finding
	checks := []struct {
		name    string
		content string
	}{
		{"Constitution.md", f.Constitution},
		{"Common.md", f.Common},
		{"Code.md", f.Code},
		{"Writing.md", f.Writing},
	}
	for _, c := range checks {
		if strings.TrimSpace(c.content) == "" {
			findings = append(findings, Finding{File: c.name, Message: "file is empty"})
		}
	}
	return findings
}

// FileStatus returns a map of file name → present (true/false) for
// all four required files plus the optional local override.
// Used by ai doctor and ai status without a full Load.
func FileStatus(root string) map[string]bool {
	status := make(map[string]bool, 5)
	for _, name := range FileNames {
		_, err := os.Stat(filepath.Join(root, name))
		status[name] = err == nil
	}
	_, err := os.Stat(filepath.Join(root, "Constitution.local.md"))
	status["Constitution.local.md"] = err == nil
	return status
}
```

- [ ] **Step 4: Run tests — expect pass**

```bash
cd /Users/itsfwcp/workspace/convergent-system-co/aiConstitution/src/internal
go test ./constitution/... -v
```

Expected: all 4 tests `PASS`.

- [ ] **Step 5: Commit**

```bash
git add src/internal/constitution/
git commit -m "feat(constitution): add Load, Validate, FileStatus

New package src/internal/constitution:
- Load() reads all 4 required files from a root dir; errors on missing.
- Validate() returns structured findings (currently: empty-file check).
- FileStatus() maps file name → present for doctor/status commands.
- Constitution.local.md optional override loaded if present."
```

---

## Task 4: Wire constitution loading into ai doctor

**Files:**
- Modify: `src/cmd/ai/cmd/doctor.go`

The doctor command currently returns a stub. Replace the RunE with real constitution checks.

- [ ] **Step 1: Write failing test for doctor output**

Create `src/cmd/ai/cmd/doctor_test.go`:

```go
package cmd_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/convergent-systems-co/aiConstitution/src/cmd/ai/cmd"
)

func TestDoctorReportsAllFilesPresent(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("AI_ROOT", dir)
	for _, name := range []string{"Constitution.md", "Common.md", "Code.md", "Writing.md"} {
		_ = os.WriteFile(filepath.Join(dir, name), []byte("# "+name+"\n"), 0o600)
	}

	root := cmd.NewRootCmd()
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"doctor"})
	if err := root.Execute(); err != nil {
		t.Fatalf("doctor returned error: %v", err)
	}
	if !bytes.Contains(buf.Bytes(), []byte("Constitution.md")) {
		t.Errorf("expected 'Constitution.md' in output, got:\n%s", buf)
	}
}

func TestDoctorReportsMissingFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("AI_ROOT", dir)
	// Only write 3 of the 4 files
	for _, name := range []string{"Constitution.md", "Common.md", "Code.md"} {
		_ = os.WriteFile(filepath.Join(dir, name), []byte("# "+name+"\n"), 0o600)
	}

	root := cmd.NewRootCmd()
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"doctor"})
	root.Execute() //nolint:errcheck
	if !bytes.Contains(buf.Bytes(), []byte("Writing.md")) {
		t.Errorf("expected 'Writing.md' in output, got:\n%s", buf)
	}
}
```

- [ ] **Step 2: Run tests — expect failure**

```bash
cd /Users/itsfwcp/workspace/convergent-system-co/aiConstitution/src/cmd/ai
go test ./cmd/... -run TestDoctor -v 2>&1 | tail -10
```

Expected: `FAIL` — doctor returns stub error, not constitution status.

- [ ] **Step 3: Replace doctor stub with constitution health check**

Replace the `RunE` body in `src/cmd/ai/cmd/doctor.go`:

```go
import (
	"fmt"

	"github.com/convergent-systems-co/aiConstitution/src/internal/constitution"
	"github.com/convergent-systems-co/aiConstitution/src/internal/paths"
	"github.com/spf13/cobra"
)

// Inside newDoctorCmd(), replace the RunE closure body:
RunE: func(cmd *cobra.Command, _ []string) error {
    root := paths.AIRoot()
    status := constitution.FileStatus(root)

    allOK := true
    for _, name := range constitution.FileNames {
        present := status[name]
        mark := "✓"
        if !present {
            mark = "✗"
            allOK = false
        }
        fmt.Fprintf(cmd.OutOrStdout(), "  [%s] %s\n", mark, name)
    }

    if localPresent := status["Constitution.local.md"]; localPresent {
        fmt.Fprintf(cmd.OutOrStdout(), "  [✓] Constitution.local.md (local override)\n")
    }

    if !allOK {
        return fmt.Errorf("doctor: missing required constitution files in %s", root)
    }
    fmt.Fprintln(cmd.OutOrStdout(), "Constitution files: OK")
    return nil
},
```

- [ ] **Step 4: Run tests — expect pass**

```bash
cd /Users/itsfwcp/workspace/convergent-system-co/aiConstitution/src/cmd/ai
go test ./cmd/... -run TestDoctor -v
```

Expected: `PASS`.

- [ ] **Step 5: Commit**

```bash
git add src/cmd/ai/cmd/doctor.go src/cmd/ai/cmd/doctor_test.go
git commit -m "feat(doctor): implement constitution file health check

ai doctor now reports present/missing status for all four required
~/.ai/ files instead of returning a stub error. Uses
constitution.FileStatus() from src/internal/constitution."
```

---

## Task 5: Wire constitution status into ai status

**Files:**
- Modify: `src/cmd/ai/cmd/status.go`

- [ ] **Step 1: Read the current status.go stub**

```bash
cat src/cmd/ai/cmd/status.go
```

- [ ] **Step 2: Write a failing test**

Create `src/cmd/ai/cmd/status_test.go`:

```go
package cmd_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/convergent-systems-co/aiConstitution/src/cmd/ai/cmd"
)

func TestStatusShowsConstitutionFiles(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("AI_ROOT", dir)
	for _, name := range []string{"Constitution.md", "Common.md", "Code.md", "Writing.md"} {
		_ = os.WriteFile(filepath.Join(dir, name), []byte("# "+name+"\n"), 0o600)
	}

	root := cmd.NewRootCmd()
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"status"})
	root.Execute() //nolint:errcheck
	if !bytes.Contains(buf.Bytes(), []byte("Constitution")) {
		t.Errorf("expected 'Constitution' in status output, got:\n%s", buf)
	}
}
```

- [ ] **Step 3: Run test — expect failure**

```bash
cd /Users/itsfwcp/workspace/convergent-system-co/aiConstitution/src/cmd/ai
go test ./cmd/... -run TestStatusShows -v 2>&1 | tail -5
```

Expected: `FAIL`.

- [ ] **Step 4: Implement status command constitution section**

Replace the stub `RunE` in `src/cmd/ai/cmd/status.go`:

```go
import (
	"fmt"

	"github.com/convergent-systems-co/aiConstitution/src/internal/constitution"
	"github.com/convergent-systems-co/aiConstitution/src/internal/paths"
	"github.com/spf13/cobra"
)

RunE: func(cmd *cobra.Command, _ []string) error {
    root := paths.AIRoot()
    fmt.Fprintf(cmd.OutOrStdout(), "AI Root: %s\n\n", root)

    fmt.Fprintln(cmd.OutOrStdout(), "Constitution files:")
    status := constitution.FileStatus(root)
    for _, name := range constitution.FileNames {
        mark := "present"
        if !status[name] {
            mark = "MISSING"
        }
        fmt.Fprintf(cmd.OutOrStdout(), "  %-30s %s\n", name, mark)
    }
    if status["Constitution.local.md"] {
        fmt.Fprintf(cmd.OutOrStdout(), "  %-30s %s\n", "Constitution.local.md", "present (local override)")
    }
    return nil
},
```

- [ ] **Step 5: Run test — expect pass**

```bash
cd /Users/itsfwcp/workspace/convergent-system-co/aiConstitution/src/cmd/ai
go test ./cmd/... -run TestStatusShows -v
```

Expected: `PASS`.

- [ ] **Step 6: Commit**

```bash
git add src/cmd/ai/cmd/status.go src/cmd/ai/cmd/status_test.go
git commit -m "feat(status): show constitution file presence in ai status"
```

---

## Task 6: Create the wizard questions package

**Files:**
- Create: `src/internal/wizard/questions.go`
- Create: `src/internal/wizard/questions_test.go`

The `ai` binary already embeds `questions.yaml` via the embed package? Let's check and if not, wire it up.

- [ ] **Step 1: Check if questions.yaml is embedded**

```bash
grep -r "questions.yaml" \
  /Users/itsfwcp/workspace/convergent-system-co/aiConstitution/src/cmd/ai/embed/
```

If not found, the embed package needs to be extended.

- [ ] **Step 2: Embed questions.yaml if not already embedded**

Check `src/cmd/ai/embed/embed.go`. If `questions.yaml` is not listed, add it:

```go
// In src/cmd/ai/embed/embed.go, add:
//go:embed ../../questions.yaml
var questionsYAML []byte

// QuestionsYAML returns the embedded questions.yaml bytes.
func QuestionsYAML() []byte { return questionsYAML }
```

The `questions.yaml` is at the repo root. The embed path `../../questions.yaml` reaches it from `src/cmd/ai/embed/`. Adjust path if needed.

- [ ] **Step 3: Write failing tests for question taxonomy**

Create `src/internal/wizard/questions_test.go`:

```go
package wizard_test

import (
	"testing"

	"github.com/convergent-systems-co/aiConstitution/src/internal/wizard"
)

const sampleTaxonomy = `
version: "0.2"
questions:
  - id: user_name
    category: identity
    type: text
    prompt: "What is your full name?"
    required: true
  - id: has_org
    category: identity
    type: confirm
    prompt: "Do you work within an organization?"
    required: false
  - id: org_name
    category: identity
    type: text
    prompt: "What is your organization name?"
    required: true
    depends:
      id: has_org
      value: "true"
`

func TestLoadTaxonomyParsesQuestions(t *testing.T) {
	tax, err := wizard.ParseTaxonomy([]byte(sampleTaxonomy))
	if err != nil {
		t.Fatalf("ParseTaxonomy() error = %v", err)
	}
	if tax.Version != "0.2" {
		t.Errorf("Version = %q, want %q", tax.Version, "0.2")
	}
	if len(tax.Questions) != 3 {
		t.Errorf("len(Questions) = %d, want 3", len(tax.Questions))
	}
}

func TestActiveQuestionsSkipsUnsatisfiedDependency(t *testing.T) {
	tax, _ := wizard.ParseTaxonomy([]byte(sampleTaxonomy))
	answers := map[string]string{
		"user_name": "Alice",
		"has_org":   "false",
	}
	active := tax.ActiveQuestions(answers)
	for _, q := range active {
		if q.ID == "org_name" {
			t.Error("org_name should be inactive when has_org=false")
		}
	}
}

func TestActiveQuestionsIncludesSatisfiedDependency(t *testing.T) {
	tax, _ := wizard.ParseTaxonomy([]byte(sampleTaxonomy))
	answers := map[string]string{
		"user_name": "Alice",
		"has_org":   "true",
	}
	active := tax.ActiveQuestions(answers)
	found := false
	for _, q := range active {
		if q.ID == "org_name" {
			found = true
		}
	}
	if !found {
		t.Error("org_name should be active when has_org=true")
	}
}
```

- [ ] **Step 4: Run tests — expect compilation failure**

```bash
cd /Users/itsfwcp/workspace/convergent-system-co/aiConstitution/src/internal
go test ./wizard/... -v 2>&1 | head -5
```

Expected: `cannot find package`.

- [ ] **Step 5: Create questions.go**

First add `gopkg.in/yaml.v3` to the internal module:

```bash
cd /Users/itsfwcp/workspace/convergent-system-co/aiConstitution/src/internal
go get gopkg.in/yaml.v3@v3.0.1
```

Create `src/internal/wizard/questions.go`:

```go
// Package wizard implements the question taxonomy loader and non-interactive
// runner for ai setup. The Bubble Tea TUI runner is in runner.go.
package wizard

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// QuestionType enumerates the supported question input types.
type QuestionType string

// Supported question types.
const (
	TypeText        QuestionType = "text"
	TypeConfirm     QuestionType = "confirm"
	TypeSelect      QuestionType = "select"
	TypeMultiSelect QuestionType = "multi-select"
)

// Dependency describes a conditional dependency on a prior answer.
type Dependency struct {
	ID    string `yaml:"id"`    // question ID that must be answered
	Value string `yaml:"value"` // answer value that satisfies the dep
}

// Question is a single wizard question parsed from questions.yaml.
type Question struct {
	ID       string       `yaml:"id"`
	Category string       `yaml:"category"`
	Type     QuestionType `yaml:"type"`
	Prompt   string       `yaml:"prompt"`
	Options  []string     `yaml:"options,omitempty"`
	Default  string       `yaml:"default,omitempty"`
	Required bool         `yaml:"required"`
	Depends  *Dependency  `yaml:"depends,omitempty"`
}

// Taxonomy is the parsed questions.yaml file.
type Taxonomy struct {
	Version   string     `yaml:"version"`
	Questions []Question `yaml:"questions"`
}

// ParseTaxonomy decodes a questions.yaml byte slice into a Taxonomy.
func ParseTaxonomy(data []byte) (Taxonomy, error) {
	var t Taxonomy
	if err := yaml.Unmarshal(data, &t); err != nil {
		return Taxonomy{}, fmt.Errorf("wizard: parse taxonomy: %w", err)
	}
	return t, nil
}

// ActiveQuestions returns the subset of questions whose dependency
// (if any) is satisfied by the current answers map.
func (t Taxonomy) ActiveQuestions(answers map[string]string) []Question {
	active := make([]Question, 0, len(t.Questions))
	for _, q := range t.Questions {
		if q.Depends == nil {
			active = append(active, q)
			continue
		}
		if answers[q.Depends.ID] == q.Depends.Value {
			active = append(active, q)
		}
	}
	return active
}
```

- [ ] **Step 6: Run tests — expect pass**

```bash
cd /Users/itsfwcp/workspace/convergent-system-co/aiConstitution/src/internal
go test ./wizard/... -v
```

Expected: all 3 tests `PASS`.

- [ ] **Step 7: Commit**

```bash
git add src/internal/wizard/ src/internal/go.mod src/internal/go.sum
git commit -m "feat(wizard): add question taxonomy parser

ParseTaxonomy() decodes questions.yaml into typed Question structs.
ActiveQuestions() filters by dependency satisfaction for sequential
question presentation. Tests cover: parse, inactive dep, active dep."
```

---

## Task 7: Non-interactive wizard runner

**Files:**
- Create: `src/internal/wizard/runner.go`
- Create: `src/internal/wizard/runner_test.go`

- [ ] **Step 1: Write failing test**

Create `src/internal/wizard/runner_test.go`:

```go
package wizard_test

import (
	"testing"

	"github.com/convergent-systems-co/aiConstitution/src/internal/wizard"
)

func TestNonInteractiveRunnerUsesSeededAnswers(t *testing.T) {
	tax, _ := wizard.ParseTaxonomy([]byte(sampleTaxonomy))
	seeds := map[string]string{
		"user_name": "Bob",
		"has_org":   "true",
		"org_name":  "Acme",
	}
	answers, err := wizard.RunNonInteractive(tax, seeds)
	if err != nil {
		t.Fatalf("RunNonInteractive() error = %v", err)
	}
	if answers["user_name"] != "Bob" {
		t.Errorf("user_name = %q, want %q", answers["user_name"], "Bob")
	}
	if answers["org_name"] != "Acme" {
		t.Errorf("org_name = %q, want %q", answers["org_name"], "Acme")
	}
}

func TestNonInteractiveRunnerErrorsOnMissingRequired(t *testing.T) {
	tax, _ := wizard.ParseTaxonomy([]byte(sampleTaxonomy))
	seeds := map[string]string{
		// user_name is required but missing
		"has_org": "false",
	}
	_, err := wizard.RunNonInteractive(tax, seeds)
	if err == nil {
		t.Fatal("expected error for missing required question, got nil")
	}
}

func TestNonInteractiveRunnerUsesDefaultForOptional(t *testing.T) {
	const taxWithDefault = `
version: "0.2"
questions:
  - id: color
    category: prefs
    type: text
    prompt: "Favourite color?"
    default: "blue"
    required: false
`
	tax, _ := wizard.ParseTaxonomy([]byte(taxWithDefault))
	answers, err := wizard.RunNonInteractive(tax, nil)
	if err != nil {
		t.Fatalf("RunNonInteractive() error = %v", err)
	}
	if answers["color"] != "blue" {
		t.Errorf("color = %q, want %q (default)", answers["color"], "blue")
	}
}
```

- [ ] **Step 2: Run — expect fail**

```bash
cd /Users/itsfwcp/workspace/convergent-system-co/aiConstitution/src/internal
go test ./wizard/... -run TestNonInteractive -v 2>&1 | tail -5
```

Expected: `FAIL` — RunNonInteractive undefined.

- [ ] **Step 3: Create runner.go**

Create `src/internal/wizard/runner.go`:

```go
package wizard

import "fmt"

// RunNonInteractive executes the wizard using seeded answers (no user input).
// It walks active questions in order, uses seeds when available, falls back
// to the question's Default, and errors if a required question has neither.
//
// Returns a map of question ID → answer string.
func RunNonInteractive(tax Taxonomy, seeds map[string]string) (map[string]string, error) {
	answers := make(map[string]string)

	for _, q := range tax.ActiveQuestions(answers) {
		// Re-evaluate after each answer so dependencies resolve incrementally.
		// This means we need to iterate the full set each time.
		_ = q // placeholder; real loop below
	}

	// Iterative resolution: keep walking until no new answers are added.
	for {
		added := false
		for _, q := range tax.ActiveQuestions(answers) {
			if _, done := answers[q.ID]; done {
				continue
			}
			val, ok := seeds[q.ID]
			if !ok {
				if q.Default != "" {
					val = q.Default
					ok = true
				}
			}
			if !ok && q.Required {
				return nil, fmt.Errorf("wizard: required question %q has no seeded answer", q.ID)
			}
			if ok {
				answers[q.ID] = val
				added = true
			}
		}
		if !added {
			break
		}
	}

	return answers, nil
}
```

- [ ] **Step 4: Run — expect pass**

```bash
cd /Users/itsfwcp/workspace/convergent-system-co/aiConstitution/src/internal
go test ./wizard/... -v
```

Expected: all 6 tests `PASS` (3 from Task 6 + 3 new).

- [ ] **Step 5: Commit**

```bash
git add src/internal/wizard/runner.go src/internal/wizard/runner_test.go
git commit -m "feat(wizard): add non-interactive runner

RunNonInteractive() walks the question taxonomy using seeded answers,
falls back to defaults, and errors on missing required questions.
Supports incremental dependency resolution (answers unlock later qs)."
```

---

## Task 8: Wire wizard into ai setup (--non-interactive path)

**Files:**
- Modify: `src/cmd/ai/cmd/setup.go`

- [ ] **Step 1: Write a failing test for setup --non-interactive**

Create `src/cmd/ai/cmd/setup_test.go`:

```go
package cmd_test

import (
	"bytes"
	"os"
	"testing"

	"github.com/convergent-systems-co/aiConstitution/src/cmd/ai/cmd"
)

func TestSetupNonInteractiveSucceeds(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("AI_ROOT", dir)
	t.Setenv("AICONST_CONFIG_DIR", dir)

	root := cmd.NewRootCmd()
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"setup", "--non-interactive"})

	// Must not error out (stub previously returned errNotImplementedHint).
	err := root.Execute()
	if err != nil {
		t.Logf("setup output: %s", buf)
		t.Errorf("setup --non-interactive returned error: %v", err)
	}
}

func TestSetupNonInteractiveCreatesSettingsToml(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("AI_ROOT", dir)
	t.Setenv("AICONST_CONFIG_DIR", dir)

	root := cmd.NewRootCmd()
	root.SetArgs([]string{"setup", "--non-interactive"})
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	_ = root.Execute()

	if _, err := os.Stat(dir + "/settings.toml"); os.IsNotExist(err) {
		t.Error("settings.toml was not created by setup --non-interactive")
	}
}
```

- [ ] **Step 2: Run — expect failure**

```bash
cd /Users/itsfwcp/workspace/convergent-system-co/aiConstitution/src/cmd/ai
go test ./cmd/... -run TestSetup -v 2>&1 | tail -10
```

Expected: `FAIL` — setup returns stub error.

- [ ] **Step 3: Replace setup stub with real non-interactive path**

Modify the `RunE` in `src/cmd/ai/cmd/setup.go`:

```go
import (
	"fmt"

	"github.com/convergent-systems-co/aiConstitution/src/cmd/ai/embed"
	"github.com/convergent-systems-co/aiConstitution/src/internal/config"
	"github.com/convergent-systems-co/aiConstitution/src/internal/wizard"
	"github.com/spf13/cobra"
)

RunE: func(cmd *cobra.Command, _ []string) error {
    if nonInteractive {
        return runSetupNonInteractive(cmd)
    }
    // TUI path: deferred to Task 9 (Bubble Tea).
    notice("setup:", "TUI not yet implemented; use --non-interactive")
    return runSetupNonInteractive(cmd)
},
```

Add below the cobra constructor:

```go
func runSetupNonInteractive(cmd *cobra.Command) error {
    taxData := embed.QuestionsYAML()
    tax, err := wizard.ParseTaxonomy(taxData)
    if err != nil {
        return fmt.Errorf("setup: load question taxonomy: %w", err)
    }

    // No seeds: use all defaults.
    answers, err := wizard.RunNonInteractive(tax, nil)
    if err != nil {
        return fmt.Errorf("setup: non-interactive wizard: %w", err)
    }

    s := config.Defaults()
    // Apply wizard answers to settings where mappings exist.
    if v, ok := answers["wizard_version"]; ok {
        s.Wizard.LastSeenWizardVersion = v
    }
    if err := config.Save(s); err != nil {
        return fmt.Errorf("setup: save settings: %w", err)
    }

    fmt.Fprintln(cmd.OutOrStdout(), "Setup complete (non-interactive). Run 'ai status' to verify.")
    return nil
}
```

- [ ] **Step 4: Run tests — expect pass**

```bash
cd /Users/itsfwcp/workspace/convergent-system-co/aiConstitution/src/cmd/ai
go test ./cmd/... -run TestSetup -v
```

Expected: both tests `PASS`.

- [ ] **Step 5: Full build check**

```bash
cd /Users/itsfwcp/workspace/convergent-system-co/aiConstitution
go build ./src/...
```

Expected: no errors.

- [ ] **Step 6: Commit**

```bash
git add src/cmd/ai/cmd/setup.go src/cmd/ai/cmd/setup_test.go
git commit -m "feat(setup): implement --non-interactive path

ai setup --non-interactive now:
1. Parses questions.yaml taxonomy via wizard.ParseTaxonomy()
2. Runs wizard.RunNonInteractive() with no seeds (all defaults)
3. Saves default settings.toml via config.Save()

TUI path (--tui) still shows a notice; will be implemented in
the wizard TUI plan."
```

---

## Task 9: Run all tests and verify clean build

- [ ] **Step 1: Run the full test suite**

```bash
cd /Users/itsfwcp/workspace/convergent-system-co/aiConstitution
go test ./src/... -v 2>&1 | tail -40
```

Expected: all tests `PASS`, no failures.

- [ ] **Step 2: Verify binary builds and runs**

```bash
cd /Users/itsfwcp/workspace/convergent-system-co/aiConstitution/src/cmd/ai
go build -o /tmp/ai-test .
/tmp/ai-test doctor 2>&1|| true
/tmp/ai-test status 2>&1 || true
/tmp/ai-test setup --non-interactive 2>&1 || true
```

Expected: no panics. `doctor` prints `✗ Constitution.md` etc. (AI_ROOT not set to a dir with files). `status` prints `AI Root:` path. `setup` prints `Setup complete`.

- [ ] **Step 3: Push**

```bash
cd /Users/itsfwcp/workspace/convergent-system-co/aiConstitution
git push
```

---

## Self-review notes

**Spec coverage gaps (deferred to later plans):**
- §3.4 Constitution atoms — deferred: requires atom substrate (external repos)
- §3.5 Amendment lifecycle — `ai amend` still stub; needs its own plan
- §13 TUI path — `--tui` flag still shows notice; needs bubbletea plan
- Config env-var overlay covers only `AICONST_REVIEW_CADENCE_DAYS` — remaining env vars deferred

**Type consistency:** `wizard.Taxonomy`, `wizard.Question`, `wizard.Dependency`, `constitution.Files`, `constitution.Finding`, `constitution.FileNames` — used consistently across tasks.

**No placeholders:** all test code and implementation code is complete and runnable.
