// Package wizard provides the Bubble Tea TUI model for the `ai setup` wizard.
//
// The model drives a sequence of questions drawn from a parsed wizard.Taxonomy.
// Questions are presented one at a time. Forward navigation (Enter / Right)
// advances; backward navigation ('b' / Left) returns to the previous question.
// The model is Done when every question in the active set has been answered.
//
// On completion, call AnswersToAnswerSet(m.Answers()) to convert the raw
// map[string]string into a typed AnswerSet, then WriteConstitutionFiles to
// materialise the output files.
//
// See SPEC.md §3.1 and §4.
package wizard

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	internalwizard "github.com/convergent-systems-co/aiConstitution/src/internal/wizard"
)

// AnswerSet holds the strongly-typed answers extracted from the wizard's raw
// answer map. Only the fields used by the post-wizard helpers are populated
// here; the full answers map is always accessible via Model.Answers().
type AnswerSet struct {
	// PrincipalName is the value of Q01 (what name the constitution recognises
	// as principal).
	PrincipalName string
	// Domains is the value of Q07 (code / writing / both).
	Domains string
	// AutonomyPosture is the value of Q09 (canonical / rephrase / weaken).
	AutonomyPosture string
}

// AnswersToAnswerSet converts the raw wizard answer map into a typed AnswerSet.
// Unknown keys are silently ignored so that callers do not break when the
// taxonomy version changes.
func AnswersToAnswerSet(answers map[string]string) AnswerSet {
	return AnswerSet{
		PrincipalName:   answers["Q01"],
		Domains:         answers["Q07"],
		AutonomyPosture: answers["Q09"],
	}
}

// Model is the Bubble Tea model for the constitution-setup wizard.
//
// It holds:
//   - a snapshot of the parsed Taxonomy
//   - the index of the current active question
//   - the accumulated answers
//   - per-type transient state (text buffer, select cursor, multi-select toggle set)
type Model struct {
	tax internalwizard.Taxonomy

	// qIdx is the index into the active-question slice returned by
	// tax.ActiveQuestions(answers). It is recomputed on each render.
	qIdx int

	// answers accumulates QID → value as the user progresses.
	answers map[string]string

	// done is set to true when all active questions have been answered.
	done bool

	// textBuf holds the in-progress text for TypeText questions.
	textBuf string

	// cursor is the highlighted option index for TypeSelect and
	// TypeMultiSelect questions.
	cursor int

	// selected tracks which option indices are toggled on for
	// TypeMultiSelect questions.
	selected map[int]bool
}

// NewModel constructs a Model from a parsed Taxonomy.
func NewModel(tax internalwizard.Taxonomy) Model {
	return Model{
		tax:      tax,
		answers:  make(map[string]string),
		selected: make(map[int]bool),
	}
}

// Init satisfies the tea.Model interface. The wizard needs no startup command.
func (m Model) Init() tea.Cmd {
	return nil
}

// Done reports whether the wizard has reached a terminal state (all active
// questions answered).
func (m Model) Done() bool {
	return m.done
}

// Answers returns a copy of the accumulated question→value map.
func (m Model) Answers() map[string]string {
	out := make(map[string]string, len(m.answers))
	for k, v := range m.answers {
		out[k] = v
	}
	return out
}

// activeQuestions returns the current set of active questions given the
// accumulated answers so far.
func (m Model) activeQuestions() []internalwizard.Question {
	return m.tax.ActiveQuestions(m.answers)
}

// currentQuestion returns the question at m.qIdx, or the zero value if the
// slice is empty or the index is out of range.
func (m Model) currentQuestion() (internalwizard.Question, bool) {
	active := m.activeQuestions()
	if len(active) == 0 || m.qIdx >= len(active) {
		return internalwizard.Question{}, false
	}
	return active[m.qIdx], true
}

// Update processes a message and returns the next model state.
//
// Handled messages:
//   - tea.KeyMsg — keyboard input routed by question type and key.
//
// Key bindings:
//
//	TypeText:        printable rune → append; Backspace → delete last;
//	                 Enter → commit text as answer, advance.
//	TypeSelect:      Up/Down → move cursor; Enter → commit selected option.
//	TypeMultiSelect: Up/Down → move cursor; Space → toggle; Enter → commit joined values.
//	TypeConfirm:     'y' → commit "yes"; 'n' → commit "no"; Enter → commit "yes".
//	All types:       'b' or Left → back; 'q' or Ctrl+C → quit without saving.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.done {
		return m, tea.Quit
	}

	q, ok := m.currentQuestion()
	if !ok {
		// No active questions — wizard is complete.
		m.done = true
		return m, tea.Quit
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case msg.Type == tea.KeyCtrlC || (msg.Type == tea.KeyRunes && string(msg.Runes) == "q"):
			return m, tea.Quit

		case msg.Type == tea.KeyLeft || (msg.Type == tea.KeyRunes && string(msg.Runes) == "b"):
			// Back navigation.
			if m.qIdx > 0 {
				m.qIdx--
				m.textBuf = ""
				m.cursor = 0
				m.selected = make(map[int]bool)
			}
			return m, nil
		}

		// Per-type input handling.
		switch q.Type() {
		case internalwizard.TypeText:
			m = m.handleTextKey(msg)
		case internalwizard.TypeSelect:
			m = m.handleSelectKey(msg, q)
		case internalwizard.TypeMultiSelect:
			m = m.handleMultiSelectKey(msg, q)
		case internalwizard.TypeConfirm:
			m = m.handleConfirmKey(msg)
		}
	}

	return m, nil
}

// handleTextKey processes a key press for TypeText questions.
func (m Model) handleTextKey(msg tea.KeyMsg) Model {
	switch msg.Type {
	case tea.KeyEnter:
		// Commit the accumulated text and advance.
		q, ok := m.currentQuestion()
		if !ok {
			return m
		}
		m.answers[effectiveID(q)] = m.textBuf
		m.textBuf = ""
		m.cursor = 0
		m.selected = make(map[int]bool)
		return m.advance()

	case tea.KeyBackspace, tea.KeyDelete:
		if len(m.textBuf) > 0 {
			// Trim the last UTF-8 character.
			runes := []rune(m.textBuf)
			m.textBuf = string(runes[:len(runes)-1])
		}

	case tea.KeyRunes:
		m.textBuf += string(msg.Runes)
	}
	return m
}

// handleSelectKey processes a key press for TypeSelect questions.
func (m Model) handleSelectKey(msg tea.KeyMsg, q internalwizard.Question) Model {
	switch msg.Type {
	case tea.KeyUp:
		if m.cursor > 0 {
			m.cursor--
		}
	case tea.KeyDown:
		if m.cursor < len(q.Options)-1 {
			m.cursor++
		}
	case tea.KeyEnter:
		val := ""
		if m.cursor < len(q.Options) {
			val = q.Options[m.cursor].Value
		}
		m.answers[effectiveID(q)] = val
		m.cursor = 0
		m.selected = make(map[int]bool)
		return m.advance()
	}
	return m
}

// handleMultiSelectKey processes a key press for TypeMultiSelect questions.
func (m Model) handleMultiSelectKey(msg tea.KeyMsg, q internalwizard.Question) Model {
	switch msg.Type {
	case tea.KeyUp:
		if m.cursor > 0 {
			m.cursor--
		}
	case tea.KeyDown:
		if m.cursor < len(q.Options)-1 {
			m.cursor++
		}
	case tea.KeySpace:
		// Toggle current option.
		if m.selected[m.cursor] {
			delete(m.selected, m.cursor)
		} else {
			m.selected[m.cursor] = true
		}
	case tea.KeyEnter:
		// Commit as comma-joined values in option order.
		var vals []string
		for i, opt := range q.Options {
			if m.selected[i] {
				vals = append(vals, opt.Value)
			}
		}
		m.answers[effectiveID(q)] = strings.Join(vals, ",")
		m.cursor = 0
		m.selected = make(map[int]bool)
		return m.advance()
	}
	return m
}

// handleConfirmKey processes a key press for TypeConfirm questions.
func (m Model) handleConfirmKey(msg tea.KeyMsg) Model {
	switch {
	case msg.Type == tea.KeyEnter || (msg.Type == tea.KeyRunes && string(msg.Runes) == "y"):
		q, ok := m.currentQuestion()
		if !ok {
			return m
		}
		m.answers[effectiveID(q)] = "yes"
		m.cursor = 0
		m.selected = make(map[int]bool)
		return m.advance()

	case msg.Type == tea.KeyRunes && string(msg.Runes) == "n":
		q, ok := m.currentQuestion()
		if !ok {
			return m
		}
		m.answers[effectiveID(q)] = "no"
		m.cursor = 0
		m.selected = make(map[int]bool)
		return m.advance()
	}
	return m
}

// advance moves to the next question, or marks the model Done if all active
// questions have been answered.
func (m Model) advance() Model {
	active := m.activeQuestions()
	// Count answered active questions.
	answered := 0
	for _, q := range active {
		if _, ok := m.answers[effectiveID(q)]; ok {
			answered++
		}
	}
	if answered >= len(active) {
		m.done = true
		return m
	}
	// Find the next unanswered question.
	for i, q := range active {
		if _, ok := m.answers[effectiveID(q)]; !ok {
			m.qIdx = i
			return m
		}
	}
	m.done = true
	return m
}

// View renders the current question for the terminal.
func (m Model) View() string {
	if m.done {
		return "Setup complete. Press any key to continue.\n"
	}
	q, ok := m.currentQuestion()
	if !ok {
		return ""
	}
	return RenderQuestion(m, q)
}

// effectiveID returns the stable ID for a question, preferring ID over QID.
func effectiveID(q internalwizard.Question) string {
	if q.QID != "" {
		return q.QID
	}
	return q.QID
}
