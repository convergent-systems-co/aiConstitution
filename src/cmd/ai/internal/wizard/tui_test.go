package wizard_test

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	internalwizard "github.com/convergent-systems-co/aiConstitution/src/internal/wizard"

	"github.com/convergent-systems-co/aiConstitution/src/cmd/ai/internal/wizard"
)

// minimalTaxonomy builds a Taxonomy with exactly nQuestions text questions,
// all mandatory, no dependencies. Used to drive navigation tests without
// relying on the real questions.yaml.
func minimalTaxonomy(nQuestions int) internalwizard.Taxonomy {
	qs := make([]internalwizard.Question, nQuestions)
	for i := range qs {
		qs[i] = internalwizard.Question{
			ID:     qidFor(i),
			Prompt: "Question " + qidFor(i) + "?",
			Type:   internalwizard.TypeText,
		}
	}
	return internalwizard.Taxonomy{
		Version:   "test",
		Questions: qs,
	}
}

func qidFor(i int) string {
	return string(rune('A' + i))
}

// pressEnter simulates an Enter key press through Update.
func pressEnter(m wizard.Model) (wizard.Model, tea.Cmd) {
	result, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	return result.(wizard.Model), cmd
}

// pressBack simulates the 'b' key (back navigation).
func pressBack(m wizard.Model) (wizard.Model, tea.Cmd) {
	result, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b'}})
	return result.(wizard.Model), cmd
}

// pressRune simulates a single printable rune.
func pressRune(m wizard.Model, r rune) (wizard.Model, tea.Cmd) {
	result, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	return result.(wizard.Model), cmd
}

// pressKey simulates a named key.
func pressKey(m wizard.Model, k tea.KeyType) (wizard.Model, tea.Cmd) {
	result, cmd := m.Update(tea.KeyMsg{Type: k})
	return result.(wizard.Model), cmd
}

// TestNewModelInitializesCorrectly verifies the initial state of a freshly
// created Model: not done, no answers, first question active.
func TestNewModelInitializesCorrectly(t *testing.T) {
	tax := minimalTaxonomy(3)
	m := wizard.NewModel(tax)

	if m.Done() {
		t.Error("NewModel: Done() should be false initially")
	}
	if len(m.Answers()) != 0 {
		t.Errorf("NewModel: Answers() should be empty initially, got %v", m.Answers())
	}
}

// TestModelDoneAfterAllQuestionsAnswered drives Enter through all questions and
// asserts Done() flips to true — the primary acceptance criterion for #192.
func TestModelDoneAfterAllQuestionsAnswered(t *testing.T) {
	tax := minimalTaxonomy(3)
	m := wizard.NewModel(tax)

	// For TypeText questions: type a character, then Enter to submit.
	for i := 0; i < 3; i++ {
		m, _ = pressRune(m, 'x') // accumulate text
		m, _ = pressEnter(m)     // submit + advance
	}

	if !m.Done() {
		t.Errorf("Done() = false after answering all %d questions; answers = %v", 3, m.Answers())
	}
}

// TestModelForwardNavigation confirms that Enter advances to the next question.
func TestModelForwardNavigation(t *testing.T) {
	tax := minimalTaxonomy(3)
	m := wizard.NewModel(tax)

	// Answer question 0.
	m, _ = pressRune(m, 'a')
	m, _ = pressEnter(m)

	answers := m.Answers()
	if _, ok := answers["A"]; !ok {
		t.Errorf("ForwardNavigation: answer for Q0 (A) not recorded; answers = %v", answers)
	}
	if m.Done() {
		t.Error("ForwardNavigation: Done() should be false after first question")
	}
}

// TestModelBackNavigation confirms that pressing 'b' returns to the previous
// question without losing accumulated answers.
func TestModelBackNavigation(t *testing.T) {
	tax := minimalTaxonomy(3)
	m := wizard.NewModel(tax)

	// Answer question 0, advance.
	m, _ = pressRune(m, 'z')
	m, _ = pressEnter(m)

	// Now on question 1 — go back.
	m, _ = pressBack(m)

	// Should be on question 0 again and not done.
	if m.Done() {
		t.Error("BackNavigation: Done() should be false after going back")
	}
	// Previous answer should be preserved.
	if m.Answers()["A"] != "z" {
		t.Errorf("BackNavigation: answer for A = %q, want %q", m.Answers()["A"], "z")
	}
}

// TestModelStoresTextAnswer verifies that rune input accumulates and Enter
// submits the accumulated text as the answer.
func TestModelStoresTextAnswer(t *testing.T) {
	tax := minimalTaxonomy(1)
	m := wizard.NewModel(tax)

	for _, r := range "hello" {
		m, _ = pressRune(m, r)
	}
	m, _ = pressEnter(m)

	if got := m.Answers()["A"]; got != "hello" {
		t.Errorf("TextAnswer: got %q, want %q", got, "hello")
	}
}

// TestAnswersToAnswerSet verifies that AnswersToAnswerSet maps the known
// Q01 key to the PrincipalName field.
func TestAnswersToAnswerSet(t *testing.T) {
	answers := map[string]string{
		"Q01": "Alice",
		"Q07": "both",
		"Q09": "canonical",
	}
	as := wizard.AnswersToAnswerSet(answers)
	if as.PrincipalName != "Alice" {
		t.Errorf("AnswersToAnswerSet: PrincipalName = %q, want %q", as.PrincipalName, "Alice")
	}
	if as.Domains != "both" {
		t.Errorf("AnswersToAnswerSet: Domains = %q, want %q", as.Domains, "both")
	}
}

// TestModelSelectQuestion verifies that up/down arrows move the cursor and
// Enter submits the selected option's Value for TypeSelect questions.
func TestModelSelectQuestion(t *testing.T) {
	tax := internalwizard.Taxonomy{
		Version: "test",
		Questions: []internalwizard.Question{
			{
				ID:     "color",
				Prompt: "Pick a color?",
				Type:   internalwizard.TypeSelect,
				Options: []internalwizard.Option{
					{Label: "Red", Value: "red"},
					{Label: "Blue", Value: "blue"},
					{Label: "Green", Value: "green"},
				},
			},
		},
	}
	m := wizard.NewModel(tax)

	// Move down once → cursor on index 1 (Blue).
	m, _ = pressKey(m, tea.KeyDown)
	m, _ = pressEnter(m)

	if got := m.Answers()["color"]; got != "blue" {
		t.Errorf("SelectQuestion: answer = %q, want %q", got, "blue")
	}
	if !m.Done() {
		t.Error("SelectQuestion: Done() should be true after answering the single question")
	}
}

// TestModelMultiSelectQuestion verifies that spacebar toggles items and Enter
// submits the comma-joined Value list for TypeMultiSelect questions.
func TestModelMultiSelectQuestion(t *testing.T) {
	tax := internalwizard.Taxonomy{
		Version: "test",
		Questions: []internalwizard.Question{
			{
				ID:     "tools",
				Prompt: "Select tools?",
				Type:   internalwizard.TypeMultiSelect,
				Options: []internalwizard.Option{
					{Label: "git", Value: "git"},
					{Label: "gh", Value: "gh"},
					{Label: "curl", Value: "curl"},
				},
			},
		},
	}
	m := wizard.NewModel(tax)

	// Toggle index 0 (git) — on.
	m, _ = pressKey(m, tea.KeySpace)
	// Move down; toggle index 1 (gh) — on.
	m, _ = pressKey(m, tea.KeyDown)
	m, _ = pressKey(m, tea.KeySpace)
	// Submit.
	m, _ = pressEnter(m)

	got := m.Answers()["tools"]
	if got != "git,gh" {
		t.Errorf("MultiSelect: answer = %q, want %q", got, "git,gh")
	}
}

// TestModelConfirmQuestion verifies that the 'y' key submits "yes" and 'n'
// submits "no" for TypeConfirm questions.
func TestModelConfirmQuestion(t *testing.T) {
	tax := internalwizard.Taxonomy{
		Version: "test",
		Questions: []internalwizard.Question{
			{
				ID:     "ok",
				Prompt: "OK?",
				Type:   internalwizard.TypeConfirm,
			},
		},
	}

	m := wizard.NewModel(tax)
	m2, _ := pressRune(m, 'y')
	if got := m2.Answers()["ok"]; got != "yes" {
		t.Errorf("ConfirmQuestion y: answer = %q, want %q", got, "yes")
	}

	m3 := wizard.NewModel(tax)
	m3, _ = pressRune(m3, 'n')
	if got := m3.Answers()["ok"]; got != "no" {
		t.Errorf("ConfirmQuestion n: answer = %q, want %q", got, "no")
	}
}

// TestModelViewNotEmpty verifies that View() returns a non-empty string for
// each question type without panicking.
func TestModelViewNotEmpty(t *testing.T) {
	types := []internalwizard.QuestionType{
		internalwizard.TypeText,
		internalwizard.TypeSelect,
		internalwizard.TypeMultiSelect,
		internalwizard.TypeConfirm,
	}
	for _, qt := range types {
		t.Run(string(qt), func(t *testing.T) {
			tax := internalwizard.Taxonomy{
				Questions: []internalwizard.Question{
					{
						ID:     "q",
						Prompt: "Prompt?",
						Type:   qt,
						Options: []internalwizard.Option{
							{Label: "A", Value: "a"},
						},
					},
				},
			}
			m := wizard.NewModel(tax)
			if v := m.View(); v == "" {
				t.Errorf("View() is empty for type %s", qt)
			}
		})
	}
}
