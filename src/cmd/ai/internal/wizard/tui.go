// Package wizard implements the Bubble Tea TUI for the
// constitution-setup wizard. It owns navigation, answer accumulation,
// the completion signal, and per-question-type input rendering.
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
// Renderers translate keyboard input into this message; callers may
// also send it directly (used by tests).
type AnswerMsg struct {
	Value string
}

// qstate holds per-question in-progress input that has not yet been
// committed into the answer map. One entry exists per visited question
// so that returning via PrevMsg restores the partial input.
type qstate struct {
	// text: buffer for KeyRunes / KeyBackspace.
	textBuf []rune
	// confirm: 0 = yes (default), 1 = no.
	confirmIdx int
	// select / multi-select: highlighted option index.
	highlight int
	// multi-select: bitmap of selected option indices.
	selected map[int]bool
}

// Model is the bubbletea.Model for the wizard. The zero value is not
// usable — construct via NewModel.
type Model struct {
	tax     corewiz.Taxonomy
	answers map[string]string
	idx     int
	done    bool
	state   map[string]*qstate // keyed by Question.ID
}

// NewModel constructs a Model from a parsed Taxonomy.
func NewModel(tax corewiz.Taxonomy) Model {
	return Model{
		tax:     tax,
		answers: make(map[string]string),
		idx:     0,
		done:    false,
		state:   make(map[string]*qstate),
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

// Init implements tea.Model. The wizard has no startup command —
// input flows in from the program's runtime.
func (m Model) Init() tea.Cmd { return nil }

// Update implements tea.Model. Handles the three semantic messages
// (NextMsg, PrevMsg, AnswerMsg) plus tea.KeyMsg, which is dispatched
// by the current question's Type to the matching renderer handler.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case NextMsg:
		active := m.tax.ActiveQuestions(m.answers)
		if m.idx < len(active)-1 {
			m.idx++
			return m, nil
		}
		if m.allRequiredAnswered(active) {
			m.done = true
			return m, tea.Quit
		}
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
		return m.handleKey(msg)
	}
	return m, nil
}

// handleKey dispatches a tea.KeyMsg to the renderer matching the
// current question's Type.
func (m Model) handleKey(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	active := m.tax.ActiveQuestions(m.answers)
	if m.idx < 0 || m.idx >= len(active) {
		return m, nil
	}
	q := active[m.idx]
	st := m.ensureState(q)

	switch q.Type {
	case corewiz.TypeText:
		return m.handleTextKey(k, q, st)
	case corewiz.TypeConfirm:
		return m.handleConfirmKey(k, q, st)
	case corewiz.TypeSelect:
		return m.handleSelectKey(k, q, st)
	case corewiz.TypeMultiSelect:
		return m.handleMultiSelectKey(k, q, st)
	}
	return m, nil
}

// ensureState returns the qstate for q, creating it on first visit.
func (m Model) ensureState(q corewiz.Question) *qstate {
	st, ok := m.state[q.ID]
	if ok {
		return st
	}
	st = &qstate{selected: make(map[int]bool)}
	m.state[q.ID] = st
	return st
}

// ---------------------------------------------------------------------------
// text renderer
// ---------------------------------------------------------------------------

func (m Model) handleTextKey(k tea.KeyMsg, q corewiz.Question, st *qstate) (tea.Model, tea.Cmd) {
	switch k.Type {
	case tea.KeyRunes, tea.KeySpace:
		st.textBuf = append(st.textBuf, k.Runes...)
		// Space key arrives with empty Runes on some terminals; coerce.
		if k.Type == tea.KeySpace && len(k.Runes) == 0 {
			st.textBuf = append(st.textBuf, ' ')
		}
	case tea.KeyBackspace:
		if n := len(st.textBuf); n > 0 {
			st.textBuf = st.textBuf[:n-1]
		}
	case tea.KeyEnter:
		m.answers[q.ID] = string(st.textBuf)
	}
	return m, nil
}

// ---------------------------------------------------------------------------
// confirm renderer
// ---------------------------------------------------------------------------

func (m Model) handleConfirmKey(k tea.KeyMsg, q corewiz.Question, st *qstate) (tea.Model, tea.Cmd) {
	switch k.Type {
	case tea.KeyLeft:
		st.confirmIdx = 0 // yes
	case tea.KeyRight:
		st.confirmIdx = 1 // no
	case tea.KeyEnter:
		if st.confirmIdx == 1 {
			m.answers[q.ID] = "no"
		} else {
			m.answers[q.ID] = "yes"
		}
	}
	return m, nil
}

// ---------------------------------------------------------------------------
// select renderer
// ---------------------------------------------------------------------------

func (m Model) handleSelectKey(k tea.KeyMsg, q corewiz.Question, st *qstate) (tea.Model, tea.Cmd) {
	switch k.Type {
	case tea.KeyUp:
		if st.highlight > 0 {
			st.highlight--
		}
	case tea.KeyDown:
		if st.highlight < len(q.Options)-1 {
			st.highlight++
		}
	case tea.KeyEnter:
		if st.highlight >= 0 && st.highlight < len(q.Options) {
			m.answers[q.ID] = q.Options[st.highlight].Value
		}
	}
	return m, nil
}

// ---------------------------------------------------------------------------
// multi-select renderer
// ---------------------------------------------------------------------------

func (m Model) handleMultiSelectKey(k tea.KeyMsg, q corewiz.Question, st *qstate) (tea.Model, tea.Cmd) {
	switch k.Type {
	case tea.KeyUp:
		if st.highlight > 0 {
			st.highlight--
		}
	case tea.KeyDown:
		if st.highlight < len(q.Options)-1 {
			st.highlight++
		}
	case tea.KeySpace:
		if st.highlight >= 0 && st.highlight < len(q.Options) {
			st.selected[st.highlight] = !st.selected[st.highlight]
		}
	case tea.KeyEnter:
		vals := make([]string, 0, len(q.Options))
		for i, opt := range q.Options {
			if st.selected[i] {
				vals = append(vals, opt.Value)
			}
		}
		m.answers[q.ID] = strings.Join(vals, ",")
	}
	return m, nil
}

// ---------------------------------------------------------------------------
// View
// ---------------------------------------------------------------------------

// View implements tea.Model. Renders the current question per its
// Type. Position indicator is shown above the renderer output.
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
	st := m.state[q.ID]
	if st == nil {
		st = &qstate{selected: make(map[int]bool)}
	}

	var b strings.Builder
	fmt.Fprintf(&b, "[%d/%d] %s\n", m.idx+1, len(active), q.Prompt)

	switch q.Type {
	case corewiz.TypeText:
		fmt.Fprintf(&b, "  > %s_\n", string(st.textBuf))
	case corewiz.TypeConfirm:
		yes, no := "yes", "no"
		if st.confirmIdx == 0 {
			yes = "[yes]"
		} else {
			no = "[no]"
		}
		fmt.Fprintf(&b, "  %s   %s\n", yes, no)
	case corewiz.TypeSelect:
		for i, opt := range q.Options {
			marker := "  "
			if i == st.highlight {
				marker = "> "
			}
			fmt.Fprintf(&b, "  %s%s\n", marker, opt.Label)
		}
	case corewiz.TypeMultiSelect:
		for i, opt := range q.Options {
			highlight := "  "
			if i == st.highlight {
				highlight = "> "
			}
			check := "[ ]"
			if st.selected[i] {
				check = "[x]"
			}
			fmt.Fprintf(&b, "  %s%s %s\n", highlight, check, opt.Label)
		}
	default:
		if ans, ok := m.answers[q.ID]; ok {
			fmt.Fprintf(&b, "  current: %s\n", ans)
		}
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
