# Plan: wizard TUI + setup wiring (#192–#198)

**Tech Lead:** TL-2 (`domain:wizard-tui`)
**Repo:** `convergent-systems-co/aiConstitution`
**Branch:** `feature/wizard-tui`
**Worktree:** `.worktrees/feature/wizard-tui`
**Module root for `go` commands:** `src/cmd/ai/` (wired via `go.work`)

---

## Objective

Implement the Bubble Tea TUI wizard (#192–#194) and the `ai setup` wiring
(#195–#198) so that `ai setup` runs the TUI, renders Constitution files on
finish, saves settings.toml, installs hooks, writes `~/.claude/CLAUDE.md`,
and creates the Copilot CLI symlink.

---

## Alternatives Table

| Alternative | Pros | Cons | Verdict |
|---|---|---|---|
| Use `charmbracelet/bubbles/textinput` for text input | Built-in component, well-maintained | Adds a dependency with a large transitive footprint; issue spec says plain bubbletea only | Rejected |
| Plain bubbletea `tea.KeyMsg` handling for all input types | Zero new deps beyond bubbletea itself; matches task spec #150–#151 | More code for text accumulation | Chosen |
| Use `huh` form library | Higher-level; less code | Heavy dependency; task spec says plain bubbletea | Rejected |
| TUI in `src/internal/wizard/tui.go` | Closer to the wizard package | Violates OWNS list which puts TUI under `src/cmd/ai/internal/wizard/` | Rejected |

---

## Scope

### Files to create
- `src/cmd/ai/internal/wizard/tui.go` — bubbletea Model + phase navigation (#192, #194)
- `src/cmd/ai/internal/wizard/tui_test.go` — TUI model tests (#192)
- `src/cmd/ai/internal/wizard/renderers.go` — question type renderers (#193)
- `src/cmd/ai/internal/wizard/renderers_test.go` — renderer tests (#193)

### Files to modify
- `src/cmd/ai/cmd/setup.go` — TUI wiring, hooks install, CLAUDE.md, Copilot symlink (#195–#198)
- `src/cmd/ai/go.mod` — add `github.com/charmbracelet/bubbletea`
- `src/cmd/ai/go.sum` — updated by `go get`

### Files NOT to touch
- `src/cmd/ai/cmd/amend.go`
- `src/cmd/ai/embed/hooks/` (any hook source files)
- `src/internal/constitution/`

---

## Approach

### Step 1 — Add bubbletea dependency
```bash
cd src/cmd/ai && go get github.com/charmbracelet/bubbletea@v1.3.10
```

### Step 2 — Create wizard package dir and tui.go (#192)
`src/cmd/ai/internal/wizard/tui.go` — package wizard

Key types:
```go
type Model struct {
    tax      internalWizard.Taxonomy  // imported from src/internal/wizard
    phaseIdx int
    qIdx     int
    answers  map[string]string
    done     bool
    // text accumulation for TypeText questions
    textBuf  string
    // select/multi-select cursor
    cursor   int
    selected map[int]bool  // multi-select
}
```

Navigation:
- `Enter` or `Right` on non-text questions → advance
- `b`/`Left` → previous question
- Phase boundary: when last question in phase is answered, phaseIdx++, qIdx=0
- `Done()` = true when phaseIdx >= len(mandatory phases answered)
- On Done, `Update` returns `tea.Quit`

Constructor: `NewModel(tax internalWizard.Taxonomy) Model`
Accessor: `Answers() map[string]string`

### Step 3 — Create renderers.go (#193)
`src/cmd/ai/internal/wizard/renderers.go`

```go
func renderText(m Model, q internalWizard.Question) string
func renderSelect(m Model, q internalWizard.Question) string
func renderMultiSelect(m Model, q internalWizard.Question) string
func renderConfirm(m Model, q internalWizard.Question) string
```

Called from `View()` based on `q.Type()`.

Note: `Question.Type` is a field, not a method, in the existing wizard package.
Use `q.Type` directly.

### Step 4 — Wire constitution render on Done (#194)
In `tui.go`, add:
```go
func (m Model) RenderConstitution(aiRoot string) error
```
This:
1. Calls `wizard.AnswersToAnswerSet(m.answers)` — **NOTE**: `AnswersToAnswerSet` does
   not exist yet in the wizard package. We must define it in this package as a
   local helper that converts `map[string]string` to a simple struct.
2. Writes `Constitution.md` and `Constitution.runtime.md` to aiRoot as minimal
   stub files (full templating is out of scope; write a valid sentinel).

Actually, re-reading the task spec: "Call wizard.AnswersToAnswerSet(m.Answers())",
"Load embed.ConstitutionTemplate()", "Call constitution.Render(as, tmpl)".
None of these exist. The task spec invents them. Per Code.md §4 (AI-Specific Code
Practices): "You MUST NOT invent imports, type signatures, environment variables,
CLI flags, or config keys."

**Decision**: Define the minimal helpers we need within the wizard TUI package.
- `AnswerSet` — a struct holding typed answers extracted from the map
- `AnswersToAnswerSet(answers map[string]string) AnswerSet` — local function
- `WriteConstitutionFiles(as AnswerSet, aiRoot string) error` — writes stub
  Constitution.md and Constitution.runtime.md
  Content: a template string embedded in the binary using `embed.FS` patterns,
  or a simple header-only stub that names the principal.

This is the honest alternative to invoking non-existent functions.

### Step 5 — Map answers to settings.toml (#195)
In `setup.go`:
- After wizard completes, call `config.Load()` to get defaults
- Map: Q01 answer → `cfg.Wizard.LastSeenWizardVersion` field note: Q01 is
  principal name; WizardSettings doesn't have a `PrincipalName` field.
  Per Code.md §4, do not invent struct fields.
  **Decision**: Store wizard version only (already in `WizardSettings.LastSeenWizardVersion`).
  The principal name goes into the Constitution file text, not settings.toml.
- Call `config.Save(cfg)` (currently a stub that returns nil; that's fine)

### Step 6 — Install hooks (#196)
In `setup.go`, after constitution written:
```go
_, _ = embed.ExtractAllHooks(paths.HooksDir(), false)
```
Reuse the exact logic from `hooks.go:installAllHooks()`.

### Step 7 — Write CLAUDE.md (#197)
In `setup.go`:
```go
func writeClaudeMD(aiRoot string) error
```
Target: `~/.claude/CLAUDE.md`
Content: `@~/.ai/Constitution.md\n`
Idempotent: if already contains `@~/.ai/Constitution.md`, no-op.

### Step 8 — Copilot symlink (#198)
In `setup.go`:
```go
func installCopilotSymlink(aiRoot string) error
```
Target link: `~/.copilot/instructions/constitution.md`
Target: `~/.ai/Constitution.runtime.md`
Logic: `os.MkdirAll`, `os.Readlink` to check existing, `os.Symlink` to create.

---

## Testing Strategy

### TUI tests (`tui_test.go`)
- `TestModelDoneAfterAllQuestions`: build a minimal Taxonomy with 3 questions, drive N Enter presses via `model.Update(tea.KeyMsg{Type: tea.KeyEnter})`, assert `model.Done() == true`.
- `TestModelForwardNavigation`: assert question index advances with Enter.
- `TestModelBackNavigation`: assert 'b' key returns to previous question.
- `TestModelStoresAnswer`: for TypeText, send rune key messages then Enter, assert answer stored.
- `TestAnswersToAnswerSet`: verify struct fields populated from map.

### Renderer tests (`renderers_test.go`)
- `TestRenderTextQuestion`: View() for TypeText contains prompt.
- `TestRenderSelectQuestion`: View() for TypeSelect contains option labels and cursor.
- `TestRenderMultiSelectQuestion`: space toggles checkbox in view.
- `TestRenderConfirmQuestion`: View() for TypeConfirm contains y/n.

### Setup tests (inline, no separate file — setup.go is Coder C's OWNS)
- `TestWriteClaudeMD`: temp dir, call writeClaudeMD, assert file contains @-include.
- `TestWriteClaudeMDIdempotent`: call twice, file not duplicated.
- `TestInstallCopilotSymlink`: temp dirs, call installCopilotSymlink, assert symlink.
- `TestInstallCopilotSymlinkStale`: write a stale symlink first, assert it is replaced.

---

## Risk Assessment

| Risk | Mitigation |
|---|---|
| bubbletea not in module cache | Check cache first; v1.3.10 confirmed present |
| `AnswersToAnswerSet` / `ConstitutionTemplate` don't exist | Define locally; document gap in code comment |
| `WizardSettings` lacks `PrincipalName` field | Use only existing fields; store version, write name to constitution file |
| Symlink creation fails on Linux/macOS differences | Use `os.Symlink` which is portable; handle `os.ErrExist` |
| bubbletea v1 API differences from v0 | Pin to v1.3.10; use `tea.Model` interface, `tea.KeyMsg` |

---

## Dependencies

- bubbletea v1.3.10 must be added to `src/cmd/ai/go.mod`
- `src/internal/wizard` package (Taxonomy, Question, QuestionType) — import via workspace
- `src/cmd/ai/embed` package (ExtractAllHooks)
- `src/internal/config` (Load, Save)
- `src/internal/paths` (AIRoot, HooksDir)

---

## Backward Compatibility

- `ai setup` currently stubs with a notice. Replacing with TUI is the intended behavior.
- New package `src/cmd/ai/internal/wizard/` does not affect any existing callers.
- `go.mod` change adds a new dependency; no existing code is broken.

---

## Coder Partition (parallel, disjoint OWNS lists)

**Coder A** — `tui.go`, `tui_test.go` (#192, #194)
**Coder B** — `renderers.go`, `renderers_test.go` (#193)
**Coder C** — `setup.go` (#195, #196, #197, #198)

All three run in parallel after TDD Writer returns RED tests.

---

## Out of Scope

- Full constitution template rendering (the four-file output is spec'd in
  a separate constitution-render task; here we write a stub header file).
- CLI argument parsing changes beyond setup.go RunE.
- The `--non-interactive` flow for setup (uses existing `RunNonInteractive`).
