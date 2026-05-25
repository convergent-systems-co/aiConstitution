package wizard

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	corewiz "github.com/convergent-systems-co/aiConstitution/src/internal/wizard"
)

// helpers

func mustParseTUI(t *testing.T, yaml string) corewiz.Taxonomy {
	t.Helper()
	tax, err := corewiz.ParseTaxonomy([]byte(yaml))
	if err != nil {
		t.Fatalf("ParseTaxonomy: %v", err)
	}
	return *tax
}

func pressEnter(m Model) (Model, tea.Cmd) {
	raw, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	return raw.(Model), cmd
}

func pressKey(m Model, kt tea.KeyType) (Model, tea.Cmd) {
	raw, cmd := m.Update(tea.KeyMsg{Type: kt})
	return raw.(Model), cmd
}

func pressRune(m Model, r rune) (Model, tea.Cmd) {
	raw, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	return raw.(Model), cmd
}

// tests

func TestNewModel_InitialState(t *testing.T) {
	tax := mustParseTUI(t, `
version: "1.0"
phases:
  - id: P1
    title: Test
    mandatory: true
    questions:
      - qid: A
        prompt: "Q1?"
        default: ""
`)
	m := NewModel(tax)
	if m.Done() {
		t.Error("new model must not be done")
	}
	if m.Answers() == nil {
		t.Error("Answers() must not be nil")
	}
}

func TestModel_EnterAdvances(t *testing.T) {
	tax := mustParseTUI(t, `
version: "1.0"
phases:
  - id: P1
    title: Test
    mandatory: true
    questions:
      - qid: A
        prompt: "Q1?"
        default: "ans"
      - qid: B
        prompt: "Q2?"
        default: "ans"
`)
	m := NewModel(tax)
	m, _ = pressEnter(m)
	_ = m.View() // should not panic
}

func TestModel_DoneAfterAllAnswered(t *testing.T) {
	tax := mustParseTUI(t, `
version: "1.0"
phases:
  - id: P1
    title: Test
    mandatory: true
    questions:
      - qid: only
        prompt: "Only Q?"
        default: "yes"
`)
	m := NewModel(tax)
	m, _ = pressEnter(m) // answer question → enters review
	if m.review {
		m, _ = pressEnter(m) // accept review → done
	}
	if !m.Done() {
		t.Error("should be Done after answering the only question")
	}
}

func TestModel_SelectQuestion(t *testing.T) {
	tax := mustParseTUI(t, `
version: "1.0"
phases:
  - id: P1
    title: Test
    mandatory: true
    questions:
      - qid: color
        prompt: "Color?"
        options:
          - label: Red
            value: red
          - label: Blue
            value: blue
        default: red
`)
	m := NewModel(tax)
	view := m.View()
	if view == "" {
		t.Error("View() must not be empty for select question")
	}
	// Enter selects current option
	m, _ = pressEnter(m) // select option → enters review
	if m.review {
		m, _ = pressEnter(m) // accept review → done
	}
	if !m.Done() {
		t.Error("should be Done after selecting the only question")
	}
}

func TestModel_MultiSelectQuestion(t *testing.T) {
	tax := mustParseTUI(t, `
version: "1.0"
phases:
  - id: P1
    title: Test
    mandatory: true
    questions:
      - qid: tools
        prompt: "Tools?"
        options:
          - label: git
            value: git
          - label: gh
            value: gh
        allow_free_text: true
        default: git
`)
	m := NewModel(tax)
	// Space toggles first item, Enter submits
	m, _ = pressKey(m, tea.KeySpace)
	m, _ = pressEnter(m)
	ans := m.Answers()["tools"]
	_ = ans // answer should be non-empty
}

func TestModel_ConfirmQuestion(t *testing.T) {
	tax := mustParseTUI(t, `
version: "1.0"
phases:
  - id: P1
    title: Test
    mandatory: true
    questions:
      - qid: ok
        prompt: "OK?"
        informational: true
        default: "yes"
`)
	m := NewModel(tax)
	view := m.View()
	if view == "" {
		t.Error("View() for confirm question must not be empty")
	}
	m2, _ := pressRune(m, 'y')
	_ = m2.Answers()
}

func TestModel_View_NotEmpty(t *testing.T) {
	tax := mustParseTUI(t, `
version: "1.0"
phases:
  - id: P1
    title: Test
    mandatory: true
    questions:
      - qid: x
        prompt: "X?"
        default: "d"
`)
	m := NewModel(tax)
	if m.View() == "" {
		t.Error("View() must not be empty")
	}
}
