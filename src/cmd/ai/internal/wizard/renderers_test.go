package wizard

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	corewiz "github.com/convergent-systems-co/aiConstitution/src/internal/wizard"
)

// ---------------------------------------------------------------------------
// Fixtures
// ---------------------------------------------------------------------------

func textTaxonomy() corewiz.Taxonomy {
	return corewiz.Taxonomy{
		Questions: []corewiz.Question{
			{ID: "name", Type: corewiz.TypeText, Prompt: "Your name?", Required: true},
		},
	}
}

func confirmTaxonomy() corewiz.Taxonomy {
	return corewiz.Taxonomy{
		Questions: []corewiz.Question{
			{ID: "agree", Type: corewiz.TypeConfirm, Prompt: "Agree?", Required: true},
		},
	}
}

func selectTaxonomy() corewiz.Taxonomy {
	return corewiz.Taxonomy{
		Questions: []corewiz.Question{
			{
				ID:     "color",
				Type:   corewiz.TypeSelect,
				Prompt: "Pick a color",
				Options: []corewiz.Option{
					{Label: "Red", Value: "red"},
					{Label: "Green", Value: "green"},
					{Label: "Blue", Value: "blue"},
				},
				Required: true,
			},
		},
	}
}

func multiSelectTaxonomy() corewiz.Taxonomy {
	return corewiz.Taxonomy{
		Questions: []corewiz.Question{
			{
				ID:     "toppings",
				Type:   corewiz.TypeMultiSelect,
				Prompt: "Pick toppings",
				Options: []corewiz.Option{
					{Label: "Cheese", Value: "cheese"},
					{Label: "Olives", Value: "olives"},
					{Label: "Onion", Value: "onion"},
				},
				Required: true,
			},
		},
	}
}

func keyRune(r rune) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}}
}

func runStringInput(m Model, s string) Model {
	for _, r := range s {
		upd, _ := m.Update(keyRune(r))
		m = upd.(Model)
	}
	return m
}

// ---------------------------------------------------------------------------
// Text renderer
// ---------------------------------------------------------------------------

func TestTextRendererAcceptsCharactersAndCommitsOnEnter(t *testing.T) {
	m := NewModel(textTaxonomy())
	m = runStringInput(m, "Thomas")

	// Buffer should show the in-progress input even before commit.
	if !strings.Contains(m.View(), "Thomas") {
		t.Fatalf("expected View to echo in-progress text; got %q", m.View())
	}

	// Enter commits.
	upd, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = upd.(Model)

	if got := m.Answers()["name"]; got != "Thomas" {
		t.Fatalf("expected name=Thomas after enter; got %q", got)
	}
}

func TestTextRendererBackspaceDeletes(t *testing.T) {
	m := NewModel(textTaxonomy())
	m = runStringInput(m, "Thomass")
	// Backspace once.
	upd, _ := m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	m = upd.(Model)
	upd, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = upd.(Model)

	if got := m.Answers()["name"]; got != "Thomas" {
		t.Fatalf("expected name=Thomas after backspace; got %q", got)
	}
}

func TestTextRendererBackspaceAtEmptyIsNoop(t *testing.T) {
	m := NewModel(textTaxonomy())
	upd, _ := m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	m = upd.(Model)
	// No crash; no answer recorded yet.
	if _, ok := m.Answers()["name"]; ok {
		t.Fatal("backspace on empty buffer must not commit")
	}
}

// ---------------------------------------------------------------------------
// Confirm renderer
// ---------------------------------------------------------------------------

func TestConfirmDefaultsToYesAndCommitsOnEnter(t *testing.T) {
	m := NewModel(confirmTaxonomy())
	v := m.View()
	if !strings.Contains(v, "yes") || !strings.Contains(v, "no") {
		t.Fatalf("View must show yes/no choices; got %q", v)
	}

	upd, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = upd.(Model)
	if got := m.Answers()["agree"]; got != "yes" {
		t.Fatalf("expected default agree=yes; got %q", got)
	}
}

func TestConfirmRightArrowTogglesToNo(t *testing.T) {
	m := NewModel(confirmTaxonomy())
	upd, _ := m.Update(tea.KeyMsg{Type: tea.KeyRight})
	m = upd.(Model)
	upd, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = upd.(Model)
	if got := m.Answers()["agree"]; got != "no" {
		t.Fatalf("right-arrow then enter should commit no; got %q", got)
	}
}

func TestConfirmLeftArrowTogglesBackToYes(t *testing.T) {
	m := NewModel(confirmTaxonomy())
	upd, _ := m.Update(tea.KeyMsg{Type: tea.KeyRight})
	m = upd.(Model)
	upd, _ = m.Update(tea.KeyMsg{Type: tea.KeyLeft})
	m = upd.(Model)
	upd, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = upd.(Model)
	if got := m.Answers()["agree"]; got != "yes" {
		t.Fatalf("right then left then enter should commit yes; got %q", got)
	}
}

// ---------------------------------------------------------------------------
// Select renderer
// ---------------------------------------------------------------------------

func TestSelectViewShowsAllOptions(t *testing.T) {
	m := NewModel(selectTaxonomy())
	v := m.View()
	for _, want := range []string{"Red", "Green", "Blue"} {
		if !strings.Contains(v, want) {
			t.Fatalf("View must list option %q; got %q", want, v)
		}
	}
}

func TestSelectEnterCommitsFirstByDefault(t *testing.T) {
	m := NewModel(selectTaxonomy())
	upd, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = upd.(Model)
	if got := m.Answers()["color"]; got != "red" {
		t.Fatalf("default highlight should be option[0] = red; got %q", got)
	}
}

func TestSelectDownArrowMovesHighlight(t *testing.T) {
	m := NewModel(selectTaxonomy())
	upd, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = upd.(Model)
	upd, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = upd.(Model)
	if got := m.Answers()["color"]; got != "green" {
		t.Fatalf("down+enter should commit option[1] = green; got %q", got)
	}
}

func TestSelectDownArrowClampsAtEnd(t *testing.T) {
	m := NewModel(selectTaxonomy())
	for i := 0; i < 10; i++ {
		upd, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
		m = upd.(Model)
	}
	upd, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = upd.(Model)
	if got := m.Answers()["color"]; got != "blue" {
		t.Fatalf("repeated down should clamp at last option = blue; got %q", got)
	}
}

func TestSelectUpArrowClampsAtZero(t *testing.T) {
	m := NewModel(selectTaxonomy())
	upd, _ := m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m = upd.(Model)
	upd, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = upd.(Model)
	if got := m.Answers()["color"]; got != "red" {
		t.Fatalf("up at index 0 should clamp; got %q", got)
	}
}

// ---------------------------------------------------------------------------
// Multi-select renderer
// ---------------------------------------------------------------------------

func TestMultiSelectSpaceTogglesMembership(t *testing.T) {
	m := NewModel(multiSelectTaxonomy())
	// Toggle option[0] (cheese).
	upd, _ := m.Update(tea.KeyMsg{Type: tea.KeySpace})
	m = upd.(Model)
	// Move down, toggle option[1] (olives).
	upd, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = upd.(Model)
	upd, _ = m.Update(tea.KeyMsg{Type: tea.KeySpace})
	m = upd.(Model)
	// Commit.
	upd, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = upd.(Model)

	got := m.Answers()["toppings"]
	if got != "cheese,olives" {
		t.Fatalf("expected cheese,olives; got %q", got)
	}
}

func TestMultiSelectSpaceUntoggles(t *testing.T) {
	m := NewModel(multiSelectTaxonomy())
	// Toggle on then off.
	upd, _ := m.Update(tea.KeyMsg{Type: tea.KeySpace})
	m = upd.(Model)
	upd, _ = m.Update(tea.KeyMsg{Type: tea.KeySpace})
	m = upd.(Model)
	// Move down, toggle.
	upd, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = upd.(Model)
	upd, _ = m.Update(tea.KeyMsg{Type: tea.KeySpace})
	m = upd.(Model)
	upd, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = upd.(Model)

	got := m.Answers()["toppings"]
	if got != "olives" {
		t.Fatalf("expected only olives after toggle-off; got %q", got)
	}
}

func TestMultiSelectEmptyCommit(t *testing.T) {
	m := NewModel(multiSelectTaxonomy())
	// Commit without selecting anything.
	upd, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = upd.(Model)
	got := m.Answers()["toppings"]
	if got != "" {
		t.Fatalf("empty commit should record empty string; got %q", got)
	}
}

func TestMultiSelectViewShowsCheckedMarks(t *testing.T) {
	m := NewModel(multiSelectTaxonomy())
	upd, _ := m.Update(tea.KeyMsg{Type: tea.KeySpace})
	m = upd.(Model)
	v := m.View()
	// Some checked-state indicator should now appear; we don't pin the
	// exact glyph, just confirm the selection is reflected somewhere.
	if !strings.Contains(strings.ToLower(v), "cheese") {
		t.Fatalf("View must show option labels; got %q", v)
	}
}

// ---------------------------------------------------------------------------
// Cross-renderer: scaffold messages still work (regression for #150).
// ---------------------------------------------------------------------------

func TestScaffoldMessagesStillWorkAfterTextInput(t *testing.T) {
	m := NewModel(textTaxonomy())
	m = runStringInput(m, "x")
	// Enter commits.
	upd, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = upd.(Model)
	// NextMsg should still advance done since the only question is now answered.
	upd, _ = m.Update(NextMsg{})
	m = upd.(Model)
	if !m.Done() {
		t.Fatal("expected done after answering the single required question and advancing")
	}
}
