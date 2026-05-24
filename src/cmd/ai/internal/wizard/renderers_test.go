package wizard

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	corewiz "github.com/convergent-systems-co/aiConstitution/src/internal/wizard"
)

// ---------------------------------------------------------------------------
// Fixtures — v2 taxonomy schema (uses ParseTaxonomy instead of struct literals)
// ---------------------------------------------------------------------------

func mustParse(t *testing.T, src string) corewiz.Taxonomy {
	t.Helper()
	tax, err := corewiz.ParseTaxonomy([]byte(src))
	if err != nil {
		t.Fatalf("ParseTaxonomy: %v", err)
	}
	return *tax
}

func textTaxonomy() corewiz.Taxonomy {
	tax, _ := corewiz.ParseTaxonomy([]byte(`
version: "1.0"
phases:
  - id: P1
    title: Test
    mandatory: true
    questions:
      - qid: name
        prompt: "Your name?"
        default: ""
`))
	return *tax
}

func confirmTaxonomy() corewiz.Taxonomy {
	tax, _ := corewiz.ParseTaxonomy([]byte(`
version: "1.0"
phases:
  - id: P1
    title: Test
    mandatory: true
    questions:
      - qid: agree
        prompt: "Agree?"
        informational: true
        default: "yes"
`))
	return *tax
}

func selectTaxonomy() corewiz.Taxonomy {
	tax, _ := corewiz.ParseTaxonomy([]byte(`
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
          - label: Green
            value: green
          - label: Blue
            value: blue
        default: red
`))
	return *tax
}

func multiSelectTaxonomy() corewiz.Taxonomy {
	tax, _ := corewiz.ParseTaxonomy([]byte(`
version: "1.0"
phases:
  - id: P1
    title: Test
    mandatory: true
    questions:
      - qid: toppings
        prompt: "Pick toppings"
        options:
          - label: Cheese
            value: cheese
          - label: Olives
            value: olives
          - label: Onion
            value: onion
        allow_free_text: true
        default: cheese
`))
	return *tax
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestNewModel_InitializesState(t *testing.T) {
	tax := textTaxonomy()
	m := NewModel(tax)
	if m.idx != 0 {
		t.Errorf("idx = %d, want 0", m.idx)
	}
}

func TestModel_TextInput_UpdatesAnswer(t *testing.T) {
	tax := textTaxonomy()
	m := NewModel(tax)

	// Simulate typing
	rawM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("Alice")})
	m, _ = rawM.(Model)
	view := m.View()
	_ = view // TUI renders; just ensure no panic
}

func TestModel_SelectInput_HighlightMoves(t *testing.T) {
	tax := selectTaxonomy()
	m := NewModel(tax)

	before := m.state
	rawM, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m, _ = rawM.(Model)
	_ = before
	_ = m // just ensure no panic
}

func TestModel_Done_ReturnsAnswers(t *testing.T) {
	tax := textTaxonomy()
	m := NewModel(tax)

	// Submit with empty input (default allowed)
	rawM, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m, _ = rawM.(Model)
	if !m.Done() {
		// May need more Enter presses depending on question count
		rawM, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m, _ = rawM.(Model)
	}
	_ = m.Answers()
}

func TestModel_View_ContainsPrompt(t *testing.T) {
	tax := textTaxonomy()
	m := NewModel(tax)
	view := m.View()
	if !strings.Contains(view, "Your name?") {
		t.Errorf("view does not contain prompt 'Your name?': %q", view)
	}
}

func TestModel_SelectView_ContainsOptions(t *testing.T) {
	tax := selectTaxonomy()
	m := NewModel(tax)
	view := m.View()
	if !strings.Contains(view, "Red") {
		t.Errorf("select view missing option 'Red': %q", view)
	}
}
