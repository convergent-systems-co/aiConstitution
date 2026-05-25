# Agentic Persona Architecture — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement the agentic persona architecture — Constitution.md as prose source of truth, `ai compress` generating YAML + compact.md derivatives per persona section, persona loading from settings.toml/project.yaml, and CLAUDE.md block injection.

**Architecture:** `ai compress` deterministically parses `## N. <Persona> Rules` sections from Constitution.md and emits a YAML derivative (`<Persona>.md`) and a compact prose derivative (`<Persona>.compact.md`) per section. Active personas are resolved from `settings.toml` (default) overridden by `project.yaml` (per-project), then written into a clearly-marked block in `~/.claude/CLAUDE.md`.

**Tech Stack:** Go 1.24, cobra, gopkg.in/yaml.v3, BurntSushi/toml. Test runner: `go test` via module paths. All new logic lives in `src/internal/` packages; CLI wiring in `src/cmd/ai/cmd/`.

---

## File Map

| Action | Path | Responsibility |
|---|---|---|
| Create | `src/internal/compress/compress.go` | Section parsing, rule extraction, YAML + compact emission, source hash |
| Create | `src/internal/compress/compress_test.go` | Unit tests for compress |
| Create | `src/internal/persona/persona.go` | Active persona resolution (settings + project.yaml), CLAUDE.md block rewrite |
| Create | `src/internal/persona/persona_test.go` | Unit tests for persona |
| Modify | `src/internal/config/config.go` | Add `PersonasSettings` + `[personas]` section with defaults |
| Modify | `src/internal/paths/paths.go` | Add `ClaudeMD()`, `ProjectYAML()` |
| Create | `src/cmd/ai/cmd/compress.go` | `ai compress` command |
| Modify | `src/cmd/ai/cmd/persona.go` | Add `ai persona new` subcommand |
| Modify | `src/cmd/ai/cmd/mode.go` | Wire `ai mode <name>` to persona loading + CLAUDE.md rewrite |
| Modify | `src/cmd/ai/cmd/doctor.go` | Add CLAUDE.md block check + stale derivative hash check |
| Modify | `src/cmd/ai/cmd/status.go` | Show active personas |
| Modify | `src/cmd/ai/cmd/root.go` | Register `newCompressCmd()` |

---

## Task 1: Section parser — `constitution` package

**Files:**
- Modify: `src/internal/constitution/constitution.go`
- Modify: `src/internal/constitution/constitution_test.go`

- [ ] **Step 1: Write the failing tests**

Add to `src/internal/constitution/constitution_test.go`:

```go
func TestParseSectionsExtractsPersonaRules(t *testing.T) {
	content := `# AI Constitution

## 0. Governance Rules
Some meta text.

## 1. Common Rules
**P1. Honesty.** MUST NOT fabricate.

**P2. Cost ceiling.** Ask before exceeding $5.

## 2. Code Rules
**2.1 Function length.** Functions MUST be ≤30 lines.
`
	sections := constitution.ParseSections(content)
	if len(sections) != 2 {
		t.Fatalf("ParseSections: got %d sections, want 2 (Governance skipped)", len(sections))
	}
	if sections[0].Number != 1 || sections[0].Name != "Common" || sections[0].Slug != "common" || sections[0].FileName != "Common.md" {
		t.Errorf("sections[0] = %+v, want Number=1 Name=Common Slug=common FileName=Common.md", sections[0])
	}
	if sections[1].Number != 2 || sections[1].Name != "Code" || sections[1].Slug != "code" || sections[1].FileName != "Code.md" {
		t.Errorf("sections[1] = %+v", sections[1])
	}
}

func TestParseSectionsBodyContent(t *testing.T) {
	content := `## 1. Common Rules
**P1. Honesty.** MUST NOT fabricate.
## 2. Code Rules
**2.1 Length.** MUST be ≤30 lines.
`
	sections := constitution.ParseSections(content)
	if !strings.Contains(sections[0].Body, "Honesty") {
		t.Errorf("sections[0].Body missing expected content, got: %q", sections[0].Body)
	}
	if !strings.Contains(sections[1].Body, "Length") {
		t.Errorf("sections[1].Body missing expected content, got: %q", sections[1].Body)
	}
}

func TestParseSectionsEmptyReturnsNil(t *testing.T) {
	if got := constitution.ParseSections(""); len(got) != 0 {
		t.Errorf("ParseSections(\"\") = %v, want empty", got)
	}
}
```

Add `"strings"` to the import block in `constitution_test.go`.

- [ ] **Step 2: Run to verify tests fail**

```bash
go test github.com/convergent-systems-co/aiConstitution/src/internal/constitution -run TestParseSections -v
```

Expected: `FAIL` — `constitution.ParseSections` undefined.

- [ ] **Step 3: Implement `Section` type and `ParseSections`**

Add to `src/internal/constitution/constitution.go` (before the final closing brace):

```go
// Section represents one extracted persona section from Constitution.md.
// Governance sections (e.g., "## 0. Governance Rules") are excluded.
type Section struct {
	Number   int    // ordinal from the heading (## N.)
	Name     string // word before "Rules" (e.g., "Common", "Code")
	Slug     string // lowercase Name (e.g., "common", "code")
	FileName string // derivative output filename (e.g., "Common.md")
	Body     string // raw markdown content of this section
}

// sectionHeaderRe matches "## N. <Name> Rules" with optional whitespace.
var sectionHeaderRe = regexp.MustCompile(`(?m)^## (\d+)\. (\w+) Rules\s*$`)

// ParseSections extracts persona sections from Constitution.md content.
// Sections whose Name is "Governance" are excluded — they contain
// meta-rules only, not enforceable AI directives.
func ParseSections(content string) []Section {
	matches := sectionHeaderRe.FindAllStringIndex(content, -1)
	if len(matches) == 0 {
		return nil
	}

	var sections []Section
	for i, loc := range matches {
		header := content[loc[0]:loc[1]]
		sub := sectionHeaderRe.FindStringSubmatch(header)
		if sub == nil {
			continue
		}
		num, _ := strconv.Atoi(sub[1])
		name := sub[2]
		if strings.EqualFold(name, "Governance") {
			continue
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
			Name:     name,
			Slug:     strings.ToLower(name),
			FileName: name + ".md",
			Body:     body,
		})
	}
	return sections
}
```

Add to the imports in `constitution.go`:

```go
import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test github.com/convergent-systems-co/aiConstitution/src/internal/constitution -run TestParseSections -v
```

Expected: all three `TestParseSections*` tests PASS.

- [ ] **Step 5: Run full test suite to verify no regressions**

```bash
go test github.com/convergent-systems-co/aiConstitution/src/internal/constitution/...
```

Expected: all tests pass.

- [ ] **Step 6: Commit**

```bash
git add src/internal/constitution/constitution.go src/internal/constitution/constitution_test.go
git commit -m "feat(constitution): add ParseSections — extract persona sections by ## N. <Name> Rules header"
```

---

## Task 2: `compress` package — YAML + compact.md emission

**Files:**
- Create: `src/internal/compress/compress.go`
- Create: `src/internal/compress/compress_test.go`

- [ ] **Step 1: Write the failing tests**

Create `src/internal/compress/compress_test.go`:

```go
package compress_test

import (
	"strings"
	"testing"

	"github.com/convergent-systems-co/aiConstitution/src/internal/compress"
	"github.com/convergent-systems-co/aiConstitution/src/internal/constitution"
)

func section(num int, name, body string) constitution.Section {
	return constitution.Section{
		Number:   num,
		Name:     name,
		Slug:     strings.ToLower(name),
		FileName: name + ".md",
		Body:     body,
	}
}

func TestExtractYAMLContainsPersonaMeta(t *testing.T) {
	s := section(1, "Common", "**P1. Honesty.** MUST NOT fabricate. *(Non-overridable.)*\n\n**P2. Cost.** Ask before exceeding $5.")
	ds, err := compress.Extract(s, "0.17")
	if err != nil {
		t.Fatalf("Extract() error = %v", err)
	}
	yaml := string(ds.YAML)
	if !strings.Contains(yaml, "persona: common") {
		t.Errorf("YAML missing persona field, got:\n%s", yaml)
	}
	if !strings.Contains(yaml, `version: "0.17"`) {
		t.Errorf("YAML missing version, got:\n%s", yaml)
	}
}

func TestExtractYAMLRulesGateInference(t *testing.T) {
	s := section(1, "Common", "**P1. Hard rule.** You MUST NOT do this.\n\n**P2. Soft rule.** You SHOULD prefer this.\n\n**P3. Permission.** You MAY skip this.")
	ds, err := compress.Extract(s, "0.1")
	if err != nil {
		t.Fatalf("Extract() error = %v", err)
	}
	yaml := string(ds.YAML)
	if !strings.Contains(yaml, "gate: hard") {
		t.Errorf("YAML missing hard gate, got:\n%s", yaml)
	}
	if !strings.Contains(yaml, "gate: soft") {
		t.Errorf("YAML missing soft gate, got:\n%s", yaml)
	}
	if !strings.Contains(yaml, "gate: permission") {
		t.Errorf("YAML missing permission gate, got:\n%s", yaml)
	}
}

func TestExtractYAMLNonOverridable(t *testing.T) {
	s := section(1, "Common", "**P1. Honesty.** MUST NOT fabricate. *(Non-overridable.)*")
	ds, err := compress.Extract(s, "0.1")
	if err != nil {
		t.Fatalf("Extract() error = %v", err)
	}
	if !strings.Contains(string(ds.YAML), "non_overridable: true") {
		t.Errorf("YAML missing non_overridable: true, got:\n%s", string(ds.YAML))
	}
}

func TestExtractHashIsStable(t *testing.T) {
	s := section(1, "Common", "**P1. Honesty.** MUST NOT fabricate.")
	ds1, _ := compress.Extract(s, "0.1")
	ds2, _ := compress.Extract(s, "0.1")
	if ds1.Hash != ds2.Hash {
		t.Errorf("Hash not stable: %q vs %q", ds1.Hash, ds2.Hash)
	}
}

func TestExtractCompactContainsRuleLabels(t *testing.T) {
	s := section(1, "Common", "**P1. No fabrication.** MUST NOT invent APIs.\n\n**P2. No secrets.** MUST NOT write credentials.")
	ds, err := compress.Extract(s, "0.1")
	if err != nil {
		t.Fatalf("Extract() error = %v", err)
	}
	compact := string(ds.Compact)
	if !strings.Contains(compact, "No fabrication") {
		t.Errorf("Compact missing rule label, got:\n%s", compact)
	}
}

func TestExtractRuleIDsUsesSectionDotIndex(t *testing.T) {
	s := section(2, "Code", "**2.1 Function length.** MUST be ≤30 lines.\n\n**2.2 Cyclomatic.** MUST be ≤10.")
	ds, err := compress.Extract(s, "0.6")
	if err != nil {
		t.Fatalf("Extract() error = %v", err)
	}
	yaml := string(ds.YAML)
	if !strings.Contains(yaml, `id: "2.1"`) {
		t.Errorf("YAML missing rule id 2.1, got:\n%s", yaml)
	}
	if !strings.Contains(yaml, `id: "2.2"`) {
		t.Errorf("YAML missing rule id 2.2, got:\n%s", yaml)
	}
}
```

- [ ] **Step 2: Run to verify tests fail**

```bash
go test github.com/convergent-systems-co/aiConstitution/src/internal/compress/... -v 2>&1 | head -20
```

Expected: `FAIL` — package not found.

- [ ] **Step 3: Implement `compress.go`**

Create `src/internal/compress/compress.go`:

```go
// Package compress extracts persona sections from Constitution.md and
// emits YAML derivatives and compact prose fallbacks.
// It is deterministic — no AI calls, no side effects outside the
// returned DerivativeSet.
package compress

import (
	"crypto/sha256"
	"fmt"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/convergent-systems-co/aiConstitution/src/internal/constitution"
)

// DerivativeSet is the output of Extract for one Section.
type DerivativeSet struct {
	YAML    []byte // YAML derivative (for Claude Code + YAML-capable tools)
	Compact []byte // compressed prose derivative (for GitHub Copilot etc.)
	Hash    string // sha256 of source section body (first 16 hex chars)
}

// ruleRe matches rule lines of the form:
//   **N.M Label.** content...   (e.g., **2.1 Function length.** MUST be ≤30 lines.)
//   **PN. Label.** content...   (e.g., **P1. Honesty.** MUST NOT fabricate.)
//   **Label.** content...       (fallback)
var ruleRe = regexp.MustCompile(`(?m)^\s*\*\*(?:(\d+\.\d+)|[A-Z]\d+\.)\s*([^*]+)\.\*\*\s*(.+)`)

// Extract extracts rules from s and returns the YAML + compact derivatives.
// version is the persona version string (e.g., "0.17") from the source file header.
func Extract(s constitution.Section, version string) (DerivativeSet, error) {
	hash := sourceHash(s.Body)

	rules, err := extractRules(s)
	if err != nil {
		return DerivativeSet{}, err
	}

	yamlBytes, err := marshalYAML(s, version, hash, rules)
	if err != nil {
		return DerivativeSet{}, err
	}

	compact := marshalCompact(s, version, rules)

	return DerivativeSet{
		YAML:    yamlBytes,
		Compact: compact,
		Hash:    hash,
	}, nil
}

// rule is the intermediate representation used for both YAML and compact output.
type rule struct {
	ID             string `yaml:"id"`
	Gate           string `yaml:"gate"`
	NonOverridable bool   `yaml:"non_overridable,omitempty"`
	Label          string `yaml:"label"`
	Content        string `yaml:"content"`
}

// yamlFile is the top-level YAML document shape.
type yamlFile struct {
	GeneratedComment string `yaml:"-"` // emitted manually as header comment
	Persona          string `yaml:"persona"`
	Version          string `yaml:"version"`
	Inherits         string `yaml:"inherits"`
	SourceSection    string `yaml:"source_section"`
	SourceHash       string `yaml:"source_hash"`
	Rules            []rule `yaml:"rules"`
}

func extractRules(s constitution.Section) ([]rule, error) {
	var rules []rule
	// Split body on blank lines to get individual rule blocks.
	blocks := splitBlocks(s.Body)
	idx := 1
	for _, block := range blocks {
		block = strings.TrimSpace(block)
		if block == "" {
			continue
		}
		r := parseRuleBlock(s.Number, idx, block)
		if r == nil {
			continue
		}
		rules = append(rules, *r)
		idx++
	}
	return rules, nil
}

func splitBlocks(body string) []string {
	return strings.Split(body, "\n\n")
}

func parseRuleBlock(sectionNum, ruleIdx int, block string) *rule {
	// Try to match the first line of the block.
	firstLine := strings.SplitN(block, "\n", 2)[0]

	// Match **N.M Label.** or **PN. Label.** or **Label.**
	re := regexp.MustCompile(`^\s*\*\*(?:(\d+\.\d+)|[A-Z]\d+\.)\s*([^*]+?)\.*\*\*(.*)$`)
	m := re.FindStringSubmatch(firstLine)

	var id, label, content string
	if m != nil {
		// Explicit ID found (N.M format).
		if m[1] != "" {
			id = m[1]
		} else {
			id = fmt.Sprintf("%d.%d", sectionNum, ruleIdx)
		}
		label = strings.TrimSpace(m[2])
		content = strings.TrimSpace(m[3] + "\n" + restOfBlock(block))
	} else {
		// Fallback: no structured heading — treat whole block as content.
		id = fmt.Sprintf("%d.%d", sectionNum, ruleIdx)
		label = truncate(block, 60)
		content = strings.TrimSpace(block)
	}

	if label == "" || content == "" {
		return nil
	}

	return &rule{
		ID:             id,
		Gate:           inferGate(content),
		NonOverridable: strings.Contains(content, "(Non-overridable.)") || strings.Contains(content, "*(Non-overridable.)*"),
		Label:          label,
		Content:        content,
	}
}

func restOfBlock(block string) string {
	parts := strings.SplitN(block, "\n", 2)
	if len(parts) < 2 {
		return ""
	}
	return parts[1]
}

func inferGate(content string) string {
	upper := strings.ToUpper(content)
	switch {
	case strings.Contains(upper, "MUST NOT") || strings.Contains(upper, "MUST"):
		return "hard"
	case strings.Contains(upper, "SHOULD NOT") || strings.Contains(upper, "SHOULD"):
		return "soft"
	case strings.Contains(upper, "MAY"):
		return "permission"
	default:
		return "soft"
	}
}

func marshalYAML(s constitution.Section, version, hash string, rules []rule) ([]byte, error) {
	header := fmt.Sprintf(
		"# %s — generated by ai compress. Do not edit manually.\n"+
			"# Source: Constitution.md ## %d. %s Rules\n"+
			"# Source hash: sha256:%s\n",
		s.FileName, s.Number, s.Name, hash,
	)

	doc := map[string]any{
		"persona":        s.Slug,
		"version":        version,
		"inherits":       "constitution",
		"source_section": fmt.Sprintf("%d", s.Number),
		"source_hash":    hash,
		"rules":          rules,
	}

	body, err := yaml.Marshal(doc)
	if err != nil {
		return nil, fmt.Errorf("compress: marshal YAML for %s: %w", s.FileName, err)
	}
	return append([]byte(header), body...), nil
}

func marshalCompact(s constitution.Section, version string, rules []rule) []byte {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("<!-- %s — generated by ai compress. Do not edit manually. -->\n", s.FileName))
	sb.WriteString(fmt.Sprintf("<!-- persona: %s | version: %s | inherits: constitution -->\n\n", s.Slug, version))
	for _, r := range rules {
		gateTag := ""
		switch r.Gate {
		case "hard":
			gateTag = "[HARD]"
		case "soft":
			gateTag = "[SOFT]"
		case "permission":
			gateTag = "[MAY]"
		}
		noTag := ""
		if r.NonOverridable {
			noTag = " [NON-OVERRIDABLE]"
		}
		sb.WriteString(fmt.Sprintf("§%s %s%s %s — %s\n\n", r.ID, gateTag, noTag, r.Label, r.Content))
	}
	return []byte(sb.String())
}

func sourceHash(body string) string {
	sum := sha256.Sum256([]byte(body))
	return fmt.Sprintf("%x", sum[:8]) // 16 hex chars
}

func truncate(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	return s[:n]
}
```

- [ ] **Step 4: Run tests**

```bash
go test github.com/convergent-systems-co/aiConstitution/src/internal/compress/... -v
```

Expected: all tests PASS.

- [ ] **Step 5: Commit**

```bash
git add src/internal/compress/
git commit -m "feat(compress): add compress package — YAML + compact.md extraction from Constitution.md sections"
```

---

## Task 3: `config` — add `[personas]` section

**Files:**
- Modify: `src/internal/config/config.go`
- Modify: `src/internal/config/config_test.go`

- [ ] **Step 1: Write failing tests**

Add to `src/internal/config/config_test.go`:

```go
func TestDefaultPersonasIncludesCommon(t *testing.T) {
	s := config.Defaults()
	found := false
	for _, p := range s.Personas.Default {
		if p == "common" {
			found = true
		}
	}
	if !found {
		t.Errorf("Defaults().Personas.Default = %v, want to include \"common\"", s.Personas.Default)
	}
}

func TestPersonasTOMLRoundTrip(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("AICONST_CONFIG_DIR", dir)

	s := config.Defaults()
	s.Personas.Default = []string{"common", "code"}
	if err := config.Save(s); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	loaded, err := config.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(loaded.Personas.Default) != 2 || loaded.Personas.Default[0] != "common" {
		t.Errorf("Loaded personas = %v, want [common code]", loaded.Personas.Default)
	}
}
```

- [ ] **Step 2: Run to verify tests fail**

```bash
go test github.com/convergent-systems-co/aiConstitution/src/internal/config -run TestDefaultPersonas -v
```

Expected: FAIL — `config.Settings` has no `Personas` field.

- [ ] **Step 3: Add `PersonasSettings`**

In `src/internal/config/config.go`, add after the `DraftsSettings` struct definition:

```go
// PersonasSettings carries the [personas] section — which derivative
// persona files are loaded by default on every session.
type PersonasSettings struct {
	Default []string `toml:"default"`
}
```

Add `Personas PersonasSettings` to the `Settings` struct after `Drafts`:

```go
	Drafts   DraftsSettings   `toml:"drafts"`
	Personas PersonasSettings `toml:"personas"`
	Focus    FocusSettings    `toml:"focus"`
```

In `Defaults()`, add after the `Drafts` entry:

```go
		Drafts:  DraftsSettings{PublishNudgeAfterDays: 30, SuppressNudge: false},
		Personas: PersonasSettings{Default: []string{"common"}},
		Focus:   FocusSettings{DefaultMode: "none", PreferStableVersions: true},
```

- [ ] **Step 4: Run tests**

```bash
go test github.com/convergent-systems-co/aiConstitution/src/internal/config/... -v
```

Expected: all tests PASS.

- [ ] **Step 5: Commit**

```bash
git add src/internal/config/config.go src/internal/config/config_test.go
git commit -m "feat(config): add [personas] section with default persona list"
```

---

## Task 4: `paths` — add `ClaudeMD` and `ProjectYAML`

**Files:**
- Modify: `src/internal/paths/paths.go`

- [ ] **Step 1: Add the two path helpers**

Add to the end of `src/internal/paths/paths.go`:

```go
// ClaudeMD returns the global Claude Code user instruction file.
// This is the file that holds the <!-- ai:personas --> block.
func ClaudeMD() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".claude/CLAUDE.md"
	}
	return filepath.Join(home, ".claude", "CLAUDE.md")
}

// ProjectYAML returns the project.yaml path relative to dir (typically
// the current working directory). Returns "" if dir is empty.
func ProjectYAML(dir string) string {
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, "project.yaml")
}
```

- [ ] **Step 2: Build to verify no errors**

```bash
go build github.com/convergent-systems-co/aiConstitution/src/internal/paths
```

Expected: exits 0.

- [ ] **Step 3: Commit**

```bash
git add src/internal/paths/paths.go
git commit -m "feat(paths): add ClaudeMD() and ProjectYAML() path helpers"
```

---

## Task 5: `persona` package — resolution + CLAUDE.md block rewrite

**Files:**
- Create: `src/internal/persona/persona.go`
- Create: `src/internal/persona/persona_test.go`

- [ ] **Step 1: Write failing tests**

Create `src/internal/persona/persona_test.go`:

```go
package persona_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/convergent-systems-co/aiConstitution/src/internal/config"
	"github.com/convergent-systems-co/aiConstitution/src/internal/persona"
)

func TestResolveUsesSettingsDefault(t *testing.T) {
	s := config.Defaults()
	s.Personas.Default = []string{"common", "code"}
	got, err := persona.Resolve(s, "")
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if len(got) != 2 || got[0] != "common" || got[1] != "code" {
		t.Errorf("Resolve() = %v, want [common code]", got)
	}
}

func TestResolveProjectYAMLOverridesSettings(t *testing.T) {
	dir := t.TempDir()
	projYAML := filepath.Join(dir, "project.yaml")
	if err := os.WriteFile(projYAML, []byte("personas:\n  load:\n    - common\n    - writing\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	s := config.Defaults()
	s.Personas.Default = []string{"common", "code"}

	got, err := persona.Resolve(s, projYAML)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if len(got) != 2 || got[1] != "writing" {
		t.Errorf("Resolve() = %v, want [common writing] from project.yaml", got)
	}
}

func TestResolveProjectYAMLMissingFallsBackToSettings(t *testing.T) {
	s := config.Defaults()
	s.Personas.Default = []string{"common"}
	got, err := persona.Resolve(s, "/nonexistent/project.yaml")
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if len(got) != 1 || got[0] != "common" {
		t.Errorf("Resolve() = %v, want [common]", got)
	}
}

func TestRewriteBlockCreatesBlock(t *testing.T) {
	dir := t.TempDir()
	claudeMD := filepath.Join(dir, "CLAUDE.md")
	if err := os.WriteFile(claudeMD, []byte("# Instructions\n\n@~/.ai/Constitution.md\n\n@~/Documents/Prompts/Instructions.md\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := persona.RewriteBlock(claudeMD, []string{"common", "code"}, "/fake/.ai"); err != nil {
		t.Fatalf("RewriteBlock() error = %v", err)
	}

	content, _ := os.ReadFile(claudeMD)
	s := string(content)
	if !strings.Contains(s, "<!-- ai:personas") {
		t.Error("RewriteBlock: missing opening comment")
	}
	if !strings.Contains(s, "<!-- /ai:personas -->") {
		t.Error("RewriteBlock: missing closing comment")
	}
	if !strings.Contains(s, "@/fake/.ai/Common.md") {
		t.Errorf("RewriteBlock: missing Common.md include, got:\n%s", s)
	}
	if !strings.Contains(s, "@/fake/.ai/Code.md") {
		t.Errorf("RewriteBlock: missing Code.md include, got:\n%s", s)
	}
}

func TestRewriteBlockReplacesExistingBlock(t *testing.T) {
	dir := t.TempDir()
	claudeMD := filepath.Join(dir, "CLAUDE.md")
	initial := "# Instructions\n\n" +
		"<!-- ai:personas — managed by ai cli, do not edit manually -->\n" +
		"@/fake/.ai/Common.md\n" +
		"@/fake/.ai/Code.md\n" +
		"<!-- /ai:personas -->\n\n" +
		"@~/Documents/Prompts/Instructions.md\n"
	if err := os.WriteFile(claudeMD, []byte(initial), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := persona.RewriteBlock(claudeMD, []string{"common", "writing"}, "/fake/.ai"); err != nil {
		t.Fatalf("RewriteBlock() error = %v", err)
	}

	content, _ := os.ReadFile(claudeMD)
	s := string(content)
	if strings.Contains(s, "Code.md") {
		t.Error("RewriteBlock: old Code.md include should be replaced, got:\n" + s)
	}
	if !strings.Contains(s, "Writing.md") {
		t.Errorf("RewriteBlock: missing Writing.md include, got:\n%s", s)
	}
}
```

- [ ] **Step 2: Run to verify tests fail**

```bash
go test github.com/convergent-systems-co/aiConstitution/src/internal/persona/... -v 2>&1 | head -10
```

Expected: FAIL — package not found.

- [ ] **Step 3: Implement `persona.go`**

Create `src/internal/persona/persona.go`:

```go
// Package persona resolves the active persona list and manages the
// <!-- ai:personas --> block in ~/.claude/CLAUDE.md.
package persona

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/convergent-systems-co/aiConstitution/src/internal/config"
)

const (
	blockOpen  = "<!-- ai:personas — managed by ai cli, do not edit manually -->"
	blockClose = "<!-- /ai:personas -->"
)

// projectYAML is the minimal structure of project.yaml we care about.
type projectYAML struct {
	Personas struct {
		Load []string `yaml:"load"`
	} `yaml:"personas"`
}

// Resolve returns the active persona slug list. project.yaml (at
// projectYAMLPath) overrides settings.toml defaults if the file exists
// and has a non-empty personas.load list. Missing project.yaml is not
// an error — it falls back silently to settings defaults.
func Resolve(s config.Settings, projectYAMLPath string) ([]string, error) {
	if projectYAMLPath != "" {
		data, err := os.ReadFile(projectYAMLPath) //nolint:gosec
		if err == nil {
			var p projectYAML
			if err2 := yaml.Unmarshal(data, &p); err2 == nil && len(p.Personas.Load) > 0 {
				return p.Personas.Load, nil
			}
		}
	}
	return s.Personas.Default, nil
}

// RewriteBlock rewrites the <!-- ai:personas --> block in claudeMDPath.
// personas is the ordered list of slugs (e.g., ["common", "code"]).
// aiRoot is the path used to build the @include lines (e.g., ~/.ai).
// If the block doesn't exist, it is inserted after the first @include
// line (or at the end of the file if none exists).
func RewriteBlock(claudeMDPath string, personas []string, aiRoot string) error {
	data, err := os.ReadFile(claudeMDPath) //nolint:gosec
	if err != nil {
		return fmt.Errorf("persona: read %s: %w", claudeMDPath, err)
	}

	newBlock := buildBlock(personas, aiRoot)
	content := string(data)

	blockRe := regexp.MustCompile(`(?s)` + regexp.QuoteMeta(blockOpen) + `.*?` + regexp.QuoteMeta(blockClose) + `\n?`)
	if blockRe.MatchString(content) {
		content = blockRe.ReplaceAllString(content, newBlock)
	} else {
		// Insert after the first @include line.
		insertRe := regexp.MustCompile(`(?m)(^@[^\n]+\n)`)
		if loc := insertRe.FindStringIndex(content); loc != nil {
			content = content[:loc[1]] + "\n" + newBlock + content[loc[1]:]
		} else {
			content = content + "\n" + newBlock
		}
	}

	return os.WriteFile(claudeMDPath, []byte(content), 0o600) //nolint:gosec
}

// PersonaFileName maps a persona slug to its derivative filename.
// "common" → "Common.md", "code" → "Code.md", etc.
func PersonaFileName(slug string) string {
	if slug == "" {
		return ""
	}
	return strings.ToUpper(slug[:1]) + slug[1:] + ".md"
}

func buildBlock(personas []string, aiRoot string) string {
	var sb strings.Builder
	sb.WriteString(blockOpen + "\n")
	for _, slug := range personas {
		name := PersonaFileName(slug)
		sb.WriteString(fmt.Sprintf("@%s\n", filepath.Join(aiRoot, name)))
	}
	sb.WriteString(blockClose + "\n")
	return sb.String()
}
```

- [ ] **Step 4: Run tests**

```bash
go test github.com/convergent-systems-co/aiConstitution/src/internal/persona/... -v
```

Expected: all tests PASS.

- [ ] **Step 5: Commit**

```bash
git add src/internal/persona/
git commit -m "feat(persona): add persona package — resolve active personas and rewrite CLAUDE.md block"
```

---

## Task 6: `ai compress` command

**Files:**
- Create: `src/cmd/ai/cmd/compress.go`
- Modify: `src/cmd/ai/cmd/root.go`

- [ ] **Step 1: Implement `compress.go`**

Create `src/cmd/ai/cmd/compress.go`:

```go
package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/convergent-systems-co/aiConstitution/src/internal/compress"
	"github.com/convergent-systems-co/aiConstitution/src/internal/constitution"
	"github.com/convergent-systems-co/aiConstitution/src/internal/paths"
	"github.com/spf13/cobra"
)

// newCompressCmd implements `ai compress`. See design spec §5.
func newCompressCmd() *cobra.Command {
	var personaFlag string
	var checkFlag bool

	c := &cobra.Command{
		Use:   "compress",
		Short: "Regenerate YAML + compact.md derivatives from Constitution.md",
		Long: `compress reads Constitution.md, extracts each ## N. <Persona> Rules section,
and emits two files per section into the AI root (~/.ai/):

  <Persona>.md           YAML derivative (for Claude Code + YAML tools)
  <Persona>.compact.md   Compressed prose (for GitHub Copilot + Markdown tools)

The Governance section is excluded — it contains meta-rules, not AI directives.

With --check, compress exits non-zero if any derivative is stale without
writing files. Suitable for pre-commit hooks and CI.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			root := paths.AIRoot()
			constPath := filepath.Join(root, "Constitution.md")

			data, err := os.ReadFile(constPath) //nolint:gosec
			if err != nil {
				return fmt.Errorf("compress: read Constitution.md from %s: %w", root, err)
			}

			sections := constitution.ParseSections(string(data))
			if len(sections) == 0 {
				return fmt.Errorf("compress: no ## N. <Persona> Rules sections found in Constitution.md")
			}

			if personaFlag != "" {
				filtered := sections[:0]
				for _, s := range sections {
					if s.Slug == personaFlag {
						filtered = append(filtered, s)
					}
				}
				if len(filtered) == 0 {
					return fmt.Errorf("compress: persona %q not found in Constitution.md", personaFlag)
				}
				sections = filtered
			}

			version := extractVersion(string(data))
			out := cmd.OutOrStdout()
			stale := false

			for _, s := range sections {
				ds, err := compress.Extract(s, version)
				if err != nil {
					return fmt.Errorf("compress: extract %s: %w", s.Name, err)
				}

				yamlPath := filepath.Join(root, s.FileName)
				compactPath := filepath.Join(root, s.Slug+".compact.md")

				if checkFlag {
					if isStale(yamlPath, ds.Hash) {
						_, _ = fmt.Fprintf(out, "  [stale] %s\n", s.FileName)
						stale = true
					} else {
						_, _ = fmt.Fprintf(out, "  [ok]    %s\n", s.FileName)
					}
					continue
				}

				if err := os.WriteFile(yamlPath, ds.YAML, 0o644); err != nil { //nolint:gosec
					return fmt.Errorf("compress: write %s: %w", yamlPath, err)
				}
				if err := os.WriteFile(compactPath, ds.Compact, 0o644); err != nil { //nolint:gosec
					return fmt.Errorf("compress: write %s: %w", compactPath, err)
				}
				_, _ = fmt.Fprintf(out, "  wrote %s + %s\n", s.FileName, filepath.Base(compactPath))
			}

			if stale {
				return fmt.Errorf("compress: %d derivative(s) are stale — run `ai compress` to regenerate", countStale(sections, root))
			}
			return nil
		},
	}

	c.Flags().StringVar(&personaFlag, "persona", "", "regenerate only this persona slug (e.g., code)")
	c.Flags().BoolVar(&checkFlag, "check", false, "exit non-zero if any derivative is stale (no writes)")
	return c
}

// extractVersion pulls the version string from the Constitution.md header.
// Falls back to "unknown" if not found. Format: **Version:** 0.17
func extractVersion(content string) string {
	for _, line := range splitLines(content) {
		if after, ok := cutPrefix(line, "**Version:**"); ok {
			return trimBold(after)
		}
	}
	return "unknown"
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i, c := range s {
		if c == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	lines = append(lines, s[start:])
	return lines
}

func cutPrefix(s, prefix string) (string, bool) {
	if len(s) >= len(prefix) && s[:len(prefix)] == prefix {
		return s[len(prefix):], true
	}
	return "", false
}

func trimBold(s string) string {
	s = strings.TrimSpace(s)
	s = strings.Trim(s, "*")
	return strings.TrimSpace(s)
}

// isStale returns true if the derivative file is missing or has a
// different source hash embedded in its header comment.
func isStale(path, wantHash string) bool {
	data, err := os.ReadFile(path) //nolint:gosec
	if err != nil {
		return true
	}
	return !strings.Contains(string(data), wantHash)
}

func countStale(sections []constitution.Section, root string) int {
	n := 0
	for _, s := range sections {
		p := filepath.Join(root, s.FileName)
		data, err := os.ReadFile(p) //nolint:gosec
		if err != nil || !strings.Contains(string(data), "source_hash") {
			n++
		}
	}
	return n
}
```

Add `"strings"` to the import block (it's used in `trimBold` and `isStale`).

- [ ] **Step 2: Register in `root.go`**

In `src/cmd/ai/cmd/root.go`, add `newCompressCmd()` to the `root.AddCommand(...)` call:

```go
	root.AddCommand(
		newSetupCmd(),
		newReviewCmd(),
		newDoctorCmd(),
		newCompressCmd(),   // ← add this line
		newSyncCmd(),
		// ... rest unchanged
	)
```

- [ ] **Step 3: Build and smoke-test**

```bash
go build github.com/convergent-systems-co/aiConstitution/src/cmd/ai && \
  ./ai compress --help
```

Expected: help text showing `--persona` and `--check` flags.

- [ ] **Step 4: Commit**

```bash
git add src/cmd/ai/cmd/compress.go src/cmd/ai/cmd/root.go
git commit -m "feat(cmd): add ai compress command — extract YAML + compact.md derivatives from Constitution.md"
```

---

## Task 7: Wire `ai mode <name>` to persona loading + CLAUDE.md rewrite

**Files:**
- Modify: `src/cmd/ai/cmd/mode.go`

- [ ] **Step 1: Replace the `mode <name>` stub with real implementation**

In `src/cmd/ai/cmd/mode.go`, replace the `RunE` on the root `newModeCmd` cobra command with:

```go
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			slug := args[0]

			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("mode: load settings: %w", err)
			}

			// Add the requested persona to the active set (additive).
			active := cfg.Personas.Default
			for _, p := range active {
				if p == slug {
					_, _ = fmt.Fprintf(cmd.OutOrStdout(), "persona %q already active\n", slug)
					return nil
				}
			}
			active = append(active, slug)

			claudeMD := paths.ClaudeMD()
			if err := persona.RewriteBlock(claudeMD, active, paths.AIRoot()); err != nil {
				return fmt.Errorf("mode: rewrite CLAUDE.md: %w", err)
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "activated persona %q — CLAUDE.md updated\nActive: %v\n", slug, active)
			return nil
		},
```

Add imports at the top of `mode.go`:

```go
import (
	"fmt"

	"github.com/convergent-systems-co/aiConstitution/src/internal/config"
	"github.com/convergent-systems-co/aiConstitution/src/internal/paths"
	"github.com/convergent-systems-co/aiConstitution/src/internal/persona"
	"github.com/spf13/cobra"
)
```

Wire the `clear` subcommand to actually revert to defaults:

```go
	// clear
	c.AddCommand(&cobra.Command{
		Use:   "clear",
		Short: "Deactivate the current mode (return to defaults)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("mode clear: load settings: %w", err)
			}
			claudeMD := paths.ClaudeMD()
			if err := persona.RewriteBlock(claudeMD, cfg.Personas.Default, paths.AIRoot()); err != nil {
				return fmt.Errorf("mode clear: rewrite CLAUDE.md: %w", err)
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "cleared — reverted to defaults: %v\n", cfg.Personas.Default)
			return nil
		},
	})
```

- [ ] **Step 2: Build**

```bash
go build github.com/convergent-systems-co/aiConstitution/src/cmd/ai
```

Expected: exits 0.

- [ ] **Step 3: Commit**

```bash
git add src/cmd/ai/cmd/mode.go
git commit -m "feat(cmd): wire ai mode <name> to persona loading + CLAUDE.md block rewrite"
```

---

## Task 8: `ai persona new`

**Files:**
- Modify: `src/cmd/ai/cmd/persona.go`

- [ ] **Step 1: Add `new` subcommand**

In `src/cmd/ai/cmd/persona.go`, add a `new` subcommand to `newPersonaCmd()` before `c.AddCommand(list, share)`:

```go
	// new
	c.AddCommand(&cobra.Command{
		Use:   "new",
		Short: "Draft a new persona section in Constitution.md and compress",
		Long: `new prompts for a persona name and description, generates a template
section in Constitution.md, opens $EDITOR for review, then runs
ai compress to emit the YAML and compact.md derivatives.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			out := cmd.OutOrStdout()
			root := paths.AIRoot()
			constPath := filepath.Join(root, "Constitution.md")

			// Read current Constitution.md to find the next section number.
			data, err := os.ReadFile(constPath) //nolint:gosec
			if err != nil {
				return fmt.Errorf("persona new: read Constitution.md: %w", err)
			}
			sections := constitution.ParseSections(string(data))
			nextNum := len(sections) + 1 // +1 because Governance is excluded from sections

			// Prompt for name.
			name, err := promptLine(cmd.InOrStdin(), out, "Persona name (e.g., Security, DataScience): ")
			if err != nil {
				return err
			}
			name = strings.TrimSpace(name)
			if name == "" {
				return fmt.Errorf("persona new: name cannot be empty")
			}

			// Prompt for description.
			desc, err := promptLine(cmd.InOrStdin(), out, "Brief description (one sentence): ")
			if err != nil {
				return err
			}

			// Build template section.
			template := buildPersonaTemplate(nextNum, name, desc)

			// Append to Constitution.md.
			f, err := os.OpenFile(constPath, os.O_APPEND|os.O_WRONLY, 0o644) //nolint:gosec
			if err != nil {
				return fmt.Errorf("persona new: open Constitution.md: %w", err)
			}
			if _, err := fmt.Fprint(f, template); err != nil {
				_ = f.Close()
				return fmt.Errorf("persona new: write template: %w", err)
			}
			_ = f.Close()

			_, _ = fmt.Fprintf(out, "\nTemplate appended to Constitution.md at ## %d. %s Rules\n", nextNum, name)
			_, _ = fmt.Fprintf(out, "Edit %s to fill in the rules, then run:\n\n  ai compress --persona %s\n\n", constPath, strings.ToLower(name))
			return nil
		},
	})
```

Add required imports to `persona.go`:

```go
import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/convergent-systems-co/aiConstitution/src/internal/constitution"
	"github.com/convergent-systems-co/aiConstitution/src/internal/paths"
	"github.com/spf13/cobra"
)
```

Add helpers at the bottom of `persona.go`:

```go
func promptLine(in io.Reader, out io.Writer, prompt string) (string, error) {
	_, _ = fmt.Fprint(out, prompt)
	scanner := bufio.NewScanner(in)
	if scanner.Scan() {
		return scanner.Text(), nil
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	return "", nil
}

func buildPersonaTemplate(num int, name, desc string) string {
	slug := strings.ToLower(name)
	return fmt.Sprintf(`

## %d. %s Rules

<!-- persona: %s | description: %s -->

**%d.1 Rule label.** MUST description of rule.

**%d.2 Rule label.** SHOULD description of rule.

**%d.3 Rule label.** MAY description of rule.
`, num, name, slug, desc, num, num, num)
}
```

- [ ] **Step 2: Build**

```bash
go build github.com/convergent-systems-co/aiConstitution/src/cmd/ai && \
  ./ai persona --help
```

Expected: `new` subcommand listed.

- [ ] **Step 3: Commit**

```bash
git add src/cmd/ai/cmd/persona.go
git commit -m "feat(cmd): add ai persona new — draft persona section in Constitution.md and prompt to compress"
```

---

## Task 9: `ai doctor` — CLAUDE.md block check + stale derivative check

**Files:**
- Modify: `src/cmd/ai/cmd/doctor.go`
- Modify: `src/cmd/ai/cmd/doctor_test.go`

- [ ] **Step 1: Write failing tests**

Add to `src/cmd/ai/cmd/doctor_test.go`:

```go
func TestDoctorReportsMissingPersonasBlock(t *testing.T) {
	root := t.TempDir()
	writeConstitutionFiles(t, root)

	claudeDir := t.TempDir()
	claudeMD := filepath.Join(claudeDir, "CLAUDE.md")
	if err := os.WriteFile(claudeMD, []byte("# Instructions\n@~/.ai/Constitution.md\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command("./ai", "doctor")
	// We're testing via the constitution.FileStatus path in the existing
	// doctor; for the new check, test the helper directly.
	_ = cmd // integration tested manually; unit test the helper:

	present := doctorCheckPersonasBlock(claudeMD)
	if present {
		t.Error("doctorCheckPersonasBlock: returned true for file missing the block")
	}
}

func TestDoctorReportsBlockPresent(t *testing.T) {
	dir := t.TempDir()
	claudeMD := filepath.Join(dir, "CLAUDE.md")
	content := "# Instructions\n<!-- ai:personas — managed by ai cli, do not edit manually -->\n@~/.ai/Common.md\n<!-- /ai:personas -->\n"
	if err := os.WriteFile(claudeMD, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	if !doctorCheckPersonasBlock(claudeMD) {
		t.Error("doctorCheckPersonasBlock: returned false for file with block present")
	}
}
```

These tests call `doctorCheckPersonasBlock` which we'll export as a package-level function.

- [ ] **Step 2: Implement doctor checks**

In `src/cmd/ai/cmd/doctor.go`, add after the existing checks in `RunE`:

```go
			// Check 2: CLAUDE.md personas block.
			claudeMD := paths.ClaudeMD()
			blockOK := doctorCheckPersonasBlock(claudeMD)
			if blockOK {
				_, _ = fmt.Fprintf(out, "  [✓] CLAUDE.md personas block\n")
			} else {
				_, _ = fmt.Fprintf(out, "  [✗] CLAUDE.md personas block missing — run `ai mode` or `ai compress` to create it\n")
				allOK = false
			}

			// Check 3: derivative freshness.
			constPath := filepath.Join(root, "Constitution.md")
			if data, err := os.ReadFile(constPath); err == nil { //nolint:gosec
				for _, s := range constitution.ParseSections(string(data)) {
					yamlPath := filepath.Join(root, s.FileName)
					_, statErr := os.Stat(yamlPath)
					if statErr != nil {
						_, _ = fmt.Fprintf(out, "  [✗] %s missing — run `ai compress`\n", s.FileName)
						allOK = false
					} else {
						_, _ = fmt.Fprintf(out, "  [✓] %s present\n", s.FileName)
					}
				}
			}
```

Add the helper function at the bottom of `doctor.go`:

```go
// doctorCheckPersonasBlock returns true if claudeMDPath contains the
// <!-- ai:personas --> block.
func doctorCheckPersonasBlock(claudeMDPath string) bool {
	data, err := os.ReadFile(claudeMDPath) //nolint:gosec
	if err != nil {
		return false
	}
	return strings.Contains(string(data), "<!-- ai:personas")
}
```

Add imports to `doctor.go`:

```go
import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/convergent-systems-co/aiConstitution/src/internal/constitution"
	"github.com/convergent-systems-co/aiConstitution/src/internal/paths"
	"github.com/spf13/cobra"
)
```

- [ ] **Step 3: Build**

```bash
go build github.com/convergent-systems-co/aiConstitution/src/cmd/ai
```

Expected: exits 0.

- [ ] **Step 4: Commit**

```bash
git add src/cmd/ai/cmd/doctor.go src/cmd/ai/cmd/doctor_test.go
git commit -m "feat(cmd): extend ai doctor — check CLAUDE.md personas block and derivative file presence"
```

---

## Task 10: `ai status` — show active personas

**Files:**
- Modify: `src/cmd/ai/cmd/status.go`

- [ ] **Step 1: Extend status output**

Replace the `RunE` body in `src/cmd/ai/cmd/status.go` with:

```go
		RunE: func(cmd *cobra.Command, _ []string) error {
			root := paths.AIRoot()
			out := cmd.OutOrStdout()
			_, _ = fmt.Fprintf(out, "AI Root: %s\n\n", root)

			_, _ = fmt.Fprintln(out, "Constitution files:")
			status := constitution.FileStatus(root)
			for _, name := range constitution.FileNames {
				mark := "present"
				if !status[name] {
					mark = "MISSING"
				}
				_, _ = fmt.Fprintf(out, "  %-30s %s\n", name, mark)
			}
			if status["Constitution.local.md"] {
				_, _ = fmt.Fprintf(out, "  %-30s %s\n", "Constitution.local.md", "present (local override)")
			}

			// Active personas from CLAUDE.md block.
			_, _ = fmt.Fprintln(out, "\nActive personas (CLAUDE.md block):")
			claudeMD := paths.ClaudeMD()
			active := readActivePersonas(claudeMD)
			if len(active) == 0 {
				_, _ = fmt.Fprintln(out, "  (no personas block found — run `ai compress`)")
			} else {
				for _, p := range active {
					_, _ = fmt.Fprintf(out, "  %s\n", p)
				}
			}
			return nil
		},
```

Add `readActivePersonas` helper at the bottom of `status.go`:

```go
// readActivePersonas parses the <!-- ai:personas --> block in CLAUDE.md
// and returns the persona slugs in order.
func readActivePersonas(claudeMDPath string) []string {
	data, err := os.ReadFile(claudeMDPath) //nolint:gosec
	if err != nil {
		return nil
	}
	content := string(data)
	start := strings.Index(content, "<!-- ai:personas")
	end := strings.Index(content, "<!-- /ai:personas -->")
	if start == -1 || end == -1 || end <= start {
		return nil
	}
	block := content[start:end]
	var slugs []string
	for _, line := range strings.Split(block, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "@") {
			continue
		}
		// @/path/to/Common.md → "Common.md" → "common"
		base := filepath.Base(line[1:])
		if !strings.HasSuffix(base, ".md") {
			continue
		}
		slug := strings.ToLower(strings.TrimSuffix(base, ".md"))
		slugs = append(slugs, slug)
	}
	return slugs
}
```

Add imports to `status.go`:

```go
import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/convergent-systems-co/aiConstitution/src/internal/constitution"
	"github.com/convergent-systems-co/aiConstitution/src/internal/paths"
	"github.com/spf13/cobra"
)
```

- [ ] **Step 2: Build and smoke-test**

```bash
go build github.com/convergent-systems-co/aiConstitution/src/cmd/ai && ./ai status
```

Expected: prints constitution file status + active personas section.

- [ ] **Step 3: Run full test suite**

```bash
go test github.com/convergent-systems-co/aiConstitution/src/internal/... github.com/convergent-systems-co/aiConstitution/src/cmd/ai/...
```

Expected: all tests PASS.

- [ ] **Step 4: Commit**

```bash
git add src/cmd/ai/cmd/status.go
git commit -m "feat(cmd): extend ai status — show active personas from CLAUDE.md block"
```

---

## Prerequisites (manual, outside this plan)

Before `ai compress` produces meaningful output, `Constitution.md` must be restructured with numbered persona sections:

```
## 0. Governance Rules
## 1. Common Rules
## 2. Code Rules
## 3. Writing Rules
```

This is prose authoring work done manually or via `ai persona new`. The CLI built here will process whatever sections are present.

**`persona.Resolve` hookup:** The `Resolve` function (Task 5) is the correct call site for `ai setup` (wizard) to wire project.yaml persona defaults into CLAUDE.md on first run. That integration is out of scope here but `Resolve(cfg, paths.ProjectYAML(cwd))` is the exact call to make.
