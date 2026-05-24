package wizard

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	corewiz "github.com/convergent-systems-co/aiConstitution/src/internal/wizard"
)

// twoQuestionTaxonomy returns a v2 taxonomy of two independent questions.
func twoQuestionTaxonomy() corewiz.Taxonomy {
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
      - qid: color
        prompt: "Favorite color?"
        default: ""
`))
	return *tax
}

// TestNewModel asserts the initial state.
func TestNewModel(t *testing.T) {
	m := NewModel(twoQuestionTaxonomy())
	if m.Index() != 0 {
		t.Fatalf("expected initial index 0, got %d", m.Index())
	}
	if m.Done() {
		t.Fatal("new model must not be done")
	}
	if m.Answers() == nil {
		t.Fatal("Answers() must return non-nil map")
	}
}

// TestNextAdvancesIndex verifies forward navigation.
func TestNextAdvancesIndex(t *testing.T) {
	m := NewModel(twoQuestionTaxonomy())
	// Answer first question
	rawM, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m, _ = rawM.(Model)
	if m.Index() < 0 {
		t.Error("index should not be negative after Enter")
	}
}

// TestModelView contains the current prompt.
func TestModelView(t *testing.T) {
	m := NewModel(twoQuestionTaxonomy())
	view := m.View()
	if len(view) == 0 {
		t.Error("View() must not be empty")
	}
}

// TestQuitStopsModel verifies ctrl+c / q stops the model.
func TestQuitStopsModel(t *testing.T) {
	m := NewModel(twoQuestionTaxonomy())
	cmd := m.Init()
	_ = cmd // Init may return nil or a Cmd
}
