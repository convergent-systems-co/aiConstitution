// Package wizard — renderers.go
//
// RenderQuestion dispatches to a type-specific renderer based on q.Type.
// Each renderer returns a self-contained string suitable for printing to the
// terminal. No external styling library is used; ASCII art only, per the
// TUI / terminal output-medium rule (Common.md §U16.1).
package wizard

import (
	"fmt"
	"strings"

	internalwizard "github.com/convergent-systems-co/aiConstitution/src/internal/wizard"
)

// RenderQuestion renders the current state of the Model for question q.
// It is exported so that renderer tests can call it directly without going
// through the full Update/View cycle.
func RenderQuestion(m Model, q internalwizard.Question) string {
	switch q.Type {
	case internalwizard.TypeText:
		return renderText(m, q)
	case internalwizard.TypeSelect:
		return renderSelect(m, q)
	case internalwizard.TypeMultiSelect:
		return renderMultiSelect(m, q)
	case internalwizard.TypeConfirm:
		return renderConfirm(m, q)
	default:
		return renderText(m, q)
	}
}

// renderText renders a free-text input question.
//
// Format:
//
//	<prompt>
//	> <accumulated input>_
//	(Enter to continue)
func renderText(m Model, q internalwizard.Question) string {
	var sb strings.Builder
	sb.WriteString(q.Prompt)
	sb.WriteString("\n")
	sb.WriteString("> ")
	sb.WriteString(m.textBuf)
	sb.WriteString("_\n")
	sb.WriteString("(Enter to continue, b to go back)\n")
	return sb.String()
}

// renderSelect renders a single-choice option list with a cursor.
//
// Format:
//
//	<prompt>
//	> Option A
//	  Option B
//	  Option C
//	(Up/Down to move, Enter to select)
func renderSelect(m Model, q internalwizard.Question) string {
	var sb strings.Builder
	sb.WriteString(q.Prompt)
	sb.WriteString("\n")
	for i, opt := range q.Options {
		if i == m.cursor {
			sb.WriteString("> ")
		} else {
			sb.WriteString("  ")
		}
		sb.WriteString(opt.Label)
		if opt.Note != "" {
			sb.WriteString(fmt.Sprintf(" (%s)", opt.Note))
		}
		sb.WriteString("\n")
	}
	if len(q.Options) == 0 {
		sb.WriteString("  (no options)\n")
	}
	sb.WriteString("(Up/Down to move, Enter to select, b to go back)\n")
	return sb.String()
}

// renderMultiSelect renders a multi-choice option list with checkboxes.
//
// Format:
//
//	<prompt>
//	> [x] Option A
//	  [ ] Option B
//	  [x] Option C
//	(Up/Down to move, Space to toggle, Enter to confirm)
func renderMultiSelect(m Model, q internalwizard.Question) string {
	var sb strings.Builder
	sb.WriteString(q.Prompt)
	sb.WriteString("\n")
	for i, opt := range q.Options {
		cursor := "  "
		if i == m.cursor {
			cursor = "> "
		}
		checked := "[ ]"
		if m.selected[i] {
			checked = "[x]"
		}
		sb.WriteString(fmt.Sprintf("%s%s %s\n", cursor, checked, opt.Label))
	}
	if len(q.Options) == 0 {
		sb.WriteString("  (no options)\n")
	}
	sb.WriteString("(Up/Down to move, Space to toggle, Enter to confirm, b to go back)\n")
	return sb.String()
}

// renderConfirm renders a yes/no confirmation question.
//
// Format:
//
//	<prompt>
//	[y]es / [n]o
func renderConfirm(_ Model, q internalwizard.Question) string {
	return fmt.Sprintf("%s\n[y]es / [n]o (b to go back)\n", q.Prompt)
}
