package cmd

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	cbterm "github.com/charmbracelet/x/term"
	"github.com/spf13/cobra"
)

// fetchDirFn is the injectable type for fetching the skill-atoms directory listing.
type fetchDirFn func() ([]skillAtomDirEntry, error)

// fetchAtomFn is the injectable type for fetching a single skill atom by download URL.
type fetchAtomFn func(url string) (*skillAtom, error)

// installFn is the injectable type for installing a skill by slug.
type installFn func(cmd *cobra.Command, slug string) error

// skillRow holds a display-ready entry for the skill selection prompt.
type skillRow struct {
	slug        string
	description string
}

// runSkillSelectionPrompt shows a numbered skill list on w, reads a selection
// from r, and installs the chosen skills via the install function.
//
// isTTY must be true for the prompt to run. When false (non-interactive
// terminal, piped output, CI), the function returns immediately without
// fetching or installing anything. Pass cbterm.IsTerminal(os.Stdout.Fd())
// in production; pass false in tests that cover the non-TTY path.
//
// Input parsing:
//   - "all"         → install every listed skill
//   - "1,3,5"       → install skills at those 1-based positions
//   - ""  (Enter)   → skip, install nothing
//   - invalid token → warn on w, skip that token only
//
// Install errors are non-fatal: a warning is printed and setup continues.
func runSkillSelectionPrompt(
	w io.Writer,
	r io.Reader,
	isTTY bool,
	fetchDir fetchDirFn,
	fetchAtom fetchAtomFn,
	install installFn,
	cmd *cobra.Command,
) error {
	// TTY guard: skip entirely when not an interactive terminal.
	if !isTTY {
		return nil
	}

	// Fetch directory listing.
	entries, err := fetchDir()
	if err != nil {
		fmt.Fprintf(w, "\nNote: could not fetch available skills (%v). Skipping skill selection.\n", err)
		return nil
	}

	// Hydrate each entry into a display row, skipping deprecated/retired.
	var rows []skillRow
	for _, e := range entries {
		atom, fetchErr := fetchAtom(e.DownloadURL)
		if fetchErr != nil || atom == nil {
			continue
		}
		lc := strings.ToLower(atom.Lifecycle)
		if lc == "deprecated" || lc == "retired" {
			continue
		}
		slug := atom.Name
		if slug == "" {
			slug = strings.TrimSuffix(e.Name, ".json")
		}
		rows = append(rows, skillRow{slug: slug, description: atom.Description})
	}

	if len(rows) == 0 {
		fmt.Fprintln(w, "\n(no skills available — skipping skill selection)")
		return nil
	}

	// Print the selection UI.
	fmt.Fprintln(w)
	fmt.Fprintln(w, "┌─────────────────────────────────────────────────────────┐")
	fmt.Fprintln(w, "│  Available skills (press Enter to skip, or type numbers)│")
	fmt.Fprintln(w, "└─────────────────────────────────────────────────────────┘")
	for i, row := range rows {
		desc := row.description
		if desc == "" {
			desc = "(no description)"
		}
		fmt.Fprintf(w, " %2d. %-16s — %s\n", i+1, row.slug, desc)
	}
	fmt.Fprintln(w)
	fmt.Fprint(w, `Install which? (e.g. 1,3,5 or "all" or Enter to skip): `)

	// Read one line from r.
	scanner := bufio.NewScanner(r)
	scanner.Scan()
	line := strings.TrimSpace(scanner.Text())

	if line == "" {
		fmt.Fprintln(w, "Skipping skill installation.")
		return nil
	}

	// Determine which slugs to install.
	var toInstall []string
	if strings.EqualFold(line, "all") {
		for _, row := range rows {
			toInstall = append(toInstall, row.slug)
		}
	} else {
		for _, token := range strings.Split(line, ",") {
			token = strings.TrimSpace(token)
			if token == "" {
				continue
			}
			n, parseErr := strconv.Atoi(token)
			if parseErr != nil || n < 1 || n > len(rows) {
				fmt.Fprintf(w, "Warning: %q is not a valid selection — skipping.\n", token)
				continue
			}
			toInstall = append(toInstall, rows[n-1].slug)
		}
	}

	// Install each selected skill. Errors are non-fatal.
	fmt.Fprintln(w)
	for _, slug := range toInstall {
		fmt.Fprintf(w, "  Installing %s... ", slug)
		if installErr := install(cmd, slug); installErr != nil {
			fmt.Fprintf(w, "warning: %v\n", installErr)
		} else {
			fmt.Fprintln(w, "done")
		}
	}

	return nil
}

// runSkillSelectionPromptReal is the production entry point for the skill
// selection step. It checks whether stdout is a real terminal and wires the
// concrete fetch and install functions into runSkillSelectionPrompt.
//
// Called from runSetupTUI after the wizard completes successfully.
// Errors are non-fatal: the caller should warn but not abort setup.
func runSkillSelectionPromptReal(cmd *cobra.Command) error {
	isTTY := cbterm.IsTerminal(os.Stdout.Fd())
	return runSkillSelectionPrompt(
		os.Stdout,
		os.Stdin,
		isTTY,
		fetchSkillsDirectory,
		fetchSkillAtomFromURL,
		runSkillsInstall,
		cmd,
	)
}
