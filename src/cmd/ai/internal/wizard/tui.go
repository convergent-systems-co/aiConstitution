// Package wizard implements the Bubble Tea TUI scaffold for the
// constitution-setup wizard. It owns navigation, answer accumulation,
// and the completion signal. Question-type-specific input handling
// (text cursors, select highlights, multi-select toggles) is layered
// on top of this scaffold by sibling code (see issue #151).
//
// Question parsing and the active-question filter live in the upstream
// package github.com/convergent-systems-co/aiConstitution/src/internal/wizard;
// this package consumes Taxonomy + Question and adds the interactive
// shell.
//
// Per spec §13. Owner: domain:wizard.
package wizard

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	corewiz "github.com/convergent-systems-co/aiConstitution/src/internal/wizard"
)

// NextMsg advances the wizard to the next question. When sent past
// the final active question, the model checks completion and sets
// done=true if all required questions have been answered.
type NextMsg struct{}

// PrevMsg retreats to the previous question. Clamps at index 0.
type PrevMsg struct{}

// AnswerMsg records an answer for the currently displayed question.
// Type-specific renderers (issue #151) construct this message from
// the user's input; the scaffold treats Value as opaque.
type AnswerMsg struct {
	Value string
}

// Model is the bubbletea.Model for the wizard. The zero value is not
// usable — construct via NewModel.
type Model struct {
	tax     corewiz.Taxonomy
	answers map[string]string
	idx     int
	done    bool
}

// NewModel constructs a Model from a parsed Taxonomy.
func NewModel(tax corewiz.Taxonomy) Model {
	return Model{
		tax:     tax,
		answers: make(map[string]string),
		idx:     0,
		done:    false,
	}
}

// Index returns the current question index (into the active list as
// computed from the current answers map).
func (m Model) Index() int { return m.idx }

// Done reports whether the wizard has completed all required questions.
func (m Model) Done() bool { return m.done }

// Answers returns a copy of the accumulated answer map. The returned
// map is owned by the caller and safe to mutate.
func (m Model) Answers() map[string]string {
	out := make(map[string]string, len(m.answers))
	for k, v := range m.answers {
		out[k] = v
	}
	return out
}

// Init implements tea.Model. The wizard scaffold has no startup
// command — input flows in from the program's runtime.
func (m Model) Init() tea.Cmd { return nil }

// Update implements tea.Model. The scaffold tier handles only the
// three semantic messages (NextMsg, PrevMsg, AnswerMsg); keyboard
// input is translated into these messages by the type-specific
// renderers in issue #151. tea.KeyMsg arriving at this scaffold tier
// is a no-op (returns the model unchanged) so the program does not
// crash when a key is pressed before #151 lands.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case NextMsg:
		active := m.tax.ActiveQuestions(m.answers)
		if m.idx < len(active)-1 {
			m.idx++
			return m, nil
		}
		// At or past the last active question: check completion.
		if m.allRequiredAnswered(active) {
			m.done = true
			return m, tea.Quit
		}
		// Required gap — stay put.
		return m, nil

	case PrevMsg:
		if m.idx > 0 {
			m.idx--
		}
		return m, nil

	case AnswerMsg:
		active := m.tax.ActiveQuestions(m.answers)
		if m.idx >= 0 && m.idx < len(active) {
			m.answers[active[m.idx].ID] = msg.Value
		}
		return m, nil

	case tea.KeyMsg:
		// Scaffold tier: keys are inert. Renderers (#151) attach
		// here later.
		return m, nil
	}
	return m, nil
}

// View implements tea.Model. The scaffold renders the current
// question's Prompt as a single text line plus a position indicator.
// Per-type rendering is layered on by issue #151.
func (m Model) View() string {
	active := m.tax.ActiveQuestions(m.answers)
	if len(active) == 0 {
		return "(no active questions)"
	}
	if m.done {
		return "Done.\n"
	}
	if m.idx < 0 || m.idx >= len(active) {
		return "(out of range)"
	}
	q := active[m.idx]
	var b strings.Builder
	fmt.Fprintf(&b, "[%d/%d] %s\n", m.idx+1, len(active), q.Prompt)
	if ans, ok := m.answers[q.ID]; ok {
		fmt.Fprintf(&b, "  current: %s\n", ans)
	}
	return b.String()
}

// allRequiredAnswered reports whether every required question in the
// active list has a recorded answer.
func (m Model) allRequiredAnswered(active []corewiz.Question) bool {
	for _, q := range active {
		if !q.Required {
			continue
		}
		if _, ok := m.answers[q.ID]; !ok {
			return false
		}
	}
	return true
}
