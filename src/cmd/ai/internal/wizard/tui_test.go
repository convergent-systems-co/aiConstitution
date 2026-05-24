package wizard

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	corewiz "github.com/convergent-systems-co/aiConstitution/src/internal/wizard"
)

// twoQuestionTaxonomy returns a deterministic taxonomy of two independent
// text questions used across navigation tests.
func twoQuestionTaxonomy() corewiz.Taxonomy {
	return corewiz.Taxonomy{
		Version: "test-1",
		Questions: []corewiz.Question{
			{ID: "name", Type: corewiz.TypeText, Prompt: "Your name?", Required: true},
			{ID: "color", Type: corewiz.TypeText, Prompt: "Favorite color?", Required: true},
		},
	}
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
	if got := m.Answers(); len(got) != 0 {
		t.Fatalf("expected empty answers map, got %v", got)
	}
}

// TestInitNoOp asserts Init() returns a nil command (no-op).
func TestInitNoOp(t *testing.T) {
	m := NewModel(twoQuestionTaxonomy())
	if cmd := m.Init(); cmd != nil {
		t.Fatalf("Init must return nil cmd, got %v", cmd)
	}
}

// TestForwardNavigation walks Update() with NextMsg.
func TestForwardNavigation(t *testing.T) {
	m := NewModel(twoQuestionTaxonomy())

	updated, _ := m.Update(NextMsg{})
	mm := updated.(Model)
	if mm.Index() != 1 {
		t.Fatalf("after one next, expected index 1, got %d", mm.Index())
	}
}

// TestBackNavigation walks forward then backward.
func TestBackNavigation(t *testing.T) {
	m := NewModel(twoQuestionTaxonomy())

	updated, _ := m.Update(NextMsg{})
	updated, _ = updated.(Model).Update(PrevMsg{})
	mm := updated.(Model)

	if mm.Index() != 0 {
		t.Fatalf("after next+prev expected index 0, got %d", mm.Index())
	}
}

// TestBackAtStartClamps asserts prev at index 0 does not underflow.
func TestBackAtStartClamps(t *testing.T) {
	m := NewModel(twoQuestionTaxonomy())

	updated, _ := m.Update(PrevMsg{})
	if updated.(Model).Index() != 0 {
		t.Fatalf("prev at index 0 must clamp; got %d", updated.(Model).Index())
	}
}

// TestAnswerStoresUnderCurrentID confirms AnswerMsg writes to the current
// question's ID.
func TestAnswerStoresUnderCurrentID(t *testing.T) {
	m := NewModel(twoQuestionTaxonomy())

	updated, _ := m.Update(AnswerMsg{Value: "Thomas"})
	mm := updated.(Model)

	if mm.Answers()["name"] != "Thomas" {
		t.Fatalf("expected answers[name]=Thomas, got %v", mm.Answers())
	}
}

// TestAnswerAccumulatesAcrossQuestions walks two questions and confirms
// both answers persist.
func TestAnswerAccumulatesAcrossQuestions(t *testing.T) {
	m := NewModel(twoQuestionTaxonomy())

	upd, _ := m.Update(AnswerMsg{Value: "Thomas"})
	upd, _ = upd.(Model).Update(NextMsg{})
	upd, _ = upd.(Model).Update(AnswerMsg{Value: "blue"})

	mm := upd.(Model)
	if mm.Answers()["name"] != "Thomas" || mm.Answers()["color"] != "blue" {
		t.Fatalf("expected both answers stored; got %v", mm.Answers())
	}
}

// TestDoneAfterFinalAnswer asserts that answering the last active question
// and advancing flips done=true and returns tea.Quit.
func TestDoneAfterFinalAnswer(t *testing.T) {
	m := NewModel(twoQuestionTaxonomy())

	// Answer Q1, advance.
	upd, _ := m.Update(AnswerMsg{Value: "Thomas"})
	upd, _ = upd.(Model).Update(NextMsg{})
	// Answer Q2, advance.
	upd, _ = upd.(Model).Update(AnswerMsg{Value: "blue"})
	updated, cmd := upd.(Model).Update(NextMsg{})

	mm := updated.(Model)
	if !mm.Done() {
		t.Fatal("expected done=true after answering all required questions and advancing past last")
	}
	if cmd == nil {
		t.Fatal("expected tea.Quit cmd at completion, got nil")
	}
}

// TestNotDoneIfRequiredUnanswered asserts that advancing past the last
// question with a required question still unanswered does NOT flip done.
func TestNotDoneIfRequiredUnanswered(t *testing.T) {
	m := NewModel(twoQuestionTaxonomy())

	// Skip past both questions without answering.
	upd, _ := m.Update(NextMsg{})
	upd, _ = upd.(Model).Update(NextMsg{})

	if upd.(Model).Done() {
		t.Fatal("done must not be true when required questions are unanswered")
	}
}

// TestViewRendersPrompt asserts View() outputs the current question's
// Prompt as a text line.
func TestViewRendersPrompt(t *testing.T) {
	m := NewModel(twoQuestionTaxonomy())
	v := m.View()
	if v == "" {
		t.Fatal("View must render non-empty output")
	}
	if !contains(v, "Your name?") {
		t.Fatalf("View output must include prompt; got %q", v)
	}
}

// TestProgramCanBeInstantiated confirms tea.NewProgram(m) does not panic
// or error during construction.
func TestProgramCanBeInstantiated(t *testing.T) {
	m := NewModel(twoQuestionTaxonomy())
	p := tea.NewProgram(m, tea.WithoutSignals(), tea.WithoutCatchPanics())
	if p == nil {
		t.Fatal("tea.NewProgram returned nil")
	}
}

func contains(haystack, needle string) bool {
	if len(needle) == 0 {
		return true
	}
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
