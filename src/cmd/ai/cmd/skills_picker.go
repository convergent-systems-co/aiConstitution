package cmd

// skills_picker.go — Bubble Tea checkbox multi-select for skill installation.
//
// Used by:
//   - `ai skills available -p` (standalone picker)
//   - `ai setup` skill-selection step
//
// UI: scrolling list of skills, each shown as two lines:
//   > [x] brainstorming (+2)
//          Explore user intent, requirements...
//   ──── scroll ────

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/term"
	"github.com/spf13/cobra"
	"os"
)

// skillPickerEntry holds one row in the picker.
type skillPickerEntry struct {
	slug     string
	fullDesc string
	subCount int
}

// skillPickerModel is the Bubble Tea model for multi-select skill installation.
type skillPickerModel struct {
	entries    []skillPickerEntry
	cursor     int          // index of highlighted skill
	selected   map[int]bool // indices of checked skills
	windowTop  int          // index of first visible skill
	windowSize int          // how many skills fit on screen (computed once)
	done       bool         // Enter pressed — install selections
	quit       bool         // q/Esc pressed — skip
}

func newSkillPickerModel(entries []skillPickerEntry) skillPickerModel {
	// Estimate visible items from terminal height (2 lines per skill + 6 chrome).
	h := 24
	if fd := os.Stdout.Fd(); term.IsTerminal(fd) {
		if _, rows, err := term.GetSize(fd); err == nil && rows > 0 {
			h = rows
		}
	}
	win := (h - 7) / 2 // 2 lines per skill, 7 lines for header + footer
	if win < 3 {
		win = 3
	}
	return skillPickerModel{
		entries:    entries,
		selected:   make(map[int]bool),
		windowSize: win,
	}
}

func (m skillPickerModel) Init() tea.Cmd { return nil }

func (m skillPickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch km.String() {
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
			if m.cursor < m.windowTop {
				m.windowTop = m.cursor
			}
		}
	case "down", "j":
		if m.cursor < len(m.entries)-1 {
			m.cursor++
			if m.cursor >= m.windowTop+m.windowSize {
				m.windowTop = m.cursor - m.windowSize + 1
			}
		}
	case "ctrl+u", "pgup":
		m.cursor -= m.windowSize
		if m.cursor < 0 {
			m.cursor = 0
		}
		m.windowTop = m.cursor
	case "ctrl+d", "pgdown":
		m.cursor += m.windowSize
		if m.cursor >= len(m.entries) {
			m.cursor = len(m.entries) - 1
		}
		if m.cursor >= m.windowTop+m.windowSize {
			m.windowTop = m.cursor - m.windowSize + 1
		}
	case " ":
		m.selected[m.cursor] = !m.selected[m.cursor]
	case "a":
		if len(m.selected) == len(m.entries) {
			m.selected = make(map[int]bool)
		} else {
			for i := range m.entries {
				m.selected[i] = true
			}
		}
	case "enter":
		m.done = true
		return m, tea.Quit
	case "q", "esc", "ctrl+c":
		m.quit = true
		return m, tea.Quit
	}
	return m, nil
}

func (m skillPickerModel) View() string {
	var sb strings.Builder

	selCount := len(m.selected)
	total := len(m.entries)

	sb.WriteString(fmt.Sprintf(
		"\n  Select skills to install  (%d/%d selected)\n",
		selCount, total,
	))
	sb.WriteString("  ─────────────────────────────────────────────────────────────\n\n")

	end := m.windowTop + m.windowSize
	if end > total {
		end = total
	}

	for i := m.windowTop; i < end; i++ {
		e := m.entries[i]

		cursor := "  "
		if i == m.cursor {
			cursor = "> "
		}
		checked := "[ ]"
		if m.selected[i] {
			checked = "[x]"
		}
		subs := ""
		if e.subCount > 0 {
			subs = fmt.Sprintf(" (+%d)", e.subCount)
		}

		// Slug line
		sb.WriteString(fmt.Sprintf("%s%s %s%s\n", cursor, checked, e.slug, subs))

		// Description — first sentence or 65 chars
		desc := e.fullDesc
		if dot := strings.Index(desc, ". "); dot > 0 && dot < 80 {
			desc = desc[:dot+1]
		} else if len(desc) > 65 {
			desc = desc[:62] + "..."
		}
		sb.WriteString(fmt.Sprintf("       %s\n", desc))
	}

	// Scroll indicator
	if total > m.windowSize {
		pct := 0
		if total > 1 {
			pct = m.cursor * 100 / (total - 1)
		}
		sb.WriteString(fmt.Sprintf(
			"\n  ── %d/%d (%d%%) ──────────────────────────────────────────────\n",
			m.cursor+1, total, pct,
		))
	}

	sb.WriteString("\n  ↑↓/jk move  •  space toggle  •  a all  •  ctrl+d/u page  •  enter install  •  q skip\n")
	return sb.String()
}

// selectedSlugs returns the slugs of all checked entries.
func (m skillPickerModel) selectedSlugs() []string {
	var out []string
	for i, e := range m.entries {
		if m.selected[i] {
			out = append(out, e.slug)
		}
	}
	return out
}

// runCatalogSkillPickerTUI runs the Bubble Tea checkbox picker and installs
// the selected skills. Falls back to a no-op when stdout is not a TTY.
func runCatalogSkillPickerTUI(cmd *cobra.Command, entries []catalogSkillEntry) error {
	if !term.IsTerminal(os.Stdout.Fd()) {
		return nil // non-interactive environment — skip
	}

	// Convert to picker entries.
	pickerEntries := make([]skillPickerEntry, len(entries))
	for i, e := range entries {
		pickerEntries[i] = skillPickerEntry{
			slug:     e.slug,
			fullDesc: e.fullDesc,
			subCount: e.subCount,
		}
	}

	m := newSkillPickerModel(pickerEntries)
	prog := tea.NewProgram(m, tea.WithAltScreen())
	result, err := prog.Run()
	if err != nil {
		return fmt.Errorf("skill picker: %w", err)
	}

	final, ok := result.(skillPickerModel)
	if !ok || final.quit || final.done && len(final.selected) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No skills selected.")
		return nil
	}

	out := cmd.OutOrStdout()
	fmt.Fprintln(out)
	for _, slug := range final.selectedSlugs() {
		fmt.Fprintf(out, "  Installing %-24s ", slug+"...")
		if installErr := runSkillsInstall(cmd, slug); installErr != nil {
			fmt.Fprintf(out, "warning: %v\n", installErr)
		} else {
			fmt.Fprintln(out, "done")
		}
	}
	return nil
}
