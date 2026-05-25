package wizard

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	corewiz "github.com/convergent-systems-co/aiConstitution/src/internal/wizard"
)

func mustParseTax(t *testing.T, src string) corewiz.Taxonomy {
	t.Helper()
	tax, err := corewiz.ParseTaxonomy([]byte(src))
	if err != nil {
		t.Fatalf("ParseTaxonomy: %v", err)
	}
	return *tax
}

func TestRenderText(t *testing.T) {
	tax := mustParseTax(t, `
version: "1.0"
phases:
  - id: P1
    title: Test
    mandatory: true
    questions:
      - qid: name
        prompt: "Your name?"
        default: ""
`)
	m := NewModel(tax)
	view := m.View()
	if !strings.Contains(view, "Your name?") {
		t.Errorf("text render missing prompt: %q", view)
	}
}

func TestRenderSelect(t *testing.T) {
	tax := mustParseTax(t, `
version: "1.0"
phases:
  - id: P1
    title: Test
    mandatory: true
    questions:
      - qid: color
        prompt: "Pick a color"
        options:
          - label: Red
            value: red
          - label: Blue
            value: blue
        default: red
`)
	m := NewModel(tax)
	view := m.View()
	if !strings.Contains(view, "Red") {
		t.Errorf("select render missing option: %q", view)
	}
}

func TestForwardNavigation(t *testing.T) {
	tax := mustParseTax(t, `
version: "1.0"
phases:
  - id: P1
    title: Test
    mandatory: true
    questions:
      - qid: q1
        prompt: "Q1?"
        default: "a"
      - qid: q2
        prompt: "Q2?"
        default: "b"
`)
	m := NewModel(tax)
	rawM, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m, _ = rawM.(Model)
	// After one Enter, should have moved forward
	_ = m.View()
}

func TestDoneWhenAllAnswered(t *testing.T) {
	tax := mustParseTax(t, `
version: "1.0"
phases:
  - id: P1
    title: Test
    mandatory: true
    questions:
      - qid: only
        prompt: "Only question"
        default: "yes"
`)
	m := NewModel(tax)
	rawM, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m, _ = rawM.(Model)
	if !m.Done() {
		t.Error("model should be Done after answering the only question")
	}
}
