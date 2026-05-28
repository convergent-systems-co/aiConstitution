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

type fetchDirFn func() ([]skillAtomDirEntry, error)
type fetchAtomFn func(url string) (*skillAtom, error)
type installFn func(cmd *cobra.Command, slug string) error

type skillRow struct {
	slug        string
	name        string
	description string
}

// runSkillSelectionPrompt shows a deduplicated, screen-aware numbered skill
// list, reads a selection, and installs the chosen skills.
//
// Sub-skills declared in any atom's depends_on are hidden — selecting the
// parent skill installs them automatically.
func runSkillSelectionPrompt(
	w io.Writer,
	r io.Reader,
	isTTY bool,
	fetchDir fetchDirFn,
	fetchAtom fetchAtomFn,
	install installFn,
	cmd *cobra.Command,
) error {
	if !isTTY {
		return nil
	}

	entries, err := fetchDir()
	if err != nil {
		fmt.Fprintf(w, "\nNote: could not fetch available skills (%v). Skipping.\n", err)
		return nil
	}

	// First pass: hydrate all atoms, collect sub-skill slugs from depends_on.
	type entry struct {
		slug string
		atom *skillAtom
	}
	var all []entry
	subSkills := map[string]bool{}

	for _, e := range entries {
		atom, fetchErr := fetchAtom(e.DownloadURL)
		if fetchErr != nil || atom == nil {
			continue
		}
		lc := strings.ToLower(atom.Lifecycle)
		if lc == "deprecated" || lc == "retired" {
			continue
		}
		slug := strings.TrimSuffix(e.Name, ".json")
		all = append(all, entry{slug: slug, atom: atom})
		for _, dep := range atom.DependsOn {
			subSkills[dep] = true
		}
	}

	// Second pass: build display rows, excluding sub-skills.
	var rows []skillRow
	for _, e := range all {
		if subSkills[e.slug] {
			continue // hidden — installed as part of a parent skill
		}
		name := e.atom.Name
		if name == "" {
			name = e.slug
		}
		desc := e.atom.Description
		// Truncate description to 60 chars for display.
		if len(desc) > 60 {
			desc = desc[:57] + "..."
		}
		rows = append(rows, skillRow{slug: e.slug, name: name, description: desc})
	}

	if len(rows) == 0 {
		fmt.Fprintln(w, "\n(no skills available)")
		return nil
	}

	// Clear screen so the list is always visible from the top.
	fmt.Fprint(w, "\033[2J\033[H")

	fmt.Fprintln(w, "╔══════════════════════════════════════════════════════════════╗")
	fmt.Fprintln(w, "║  Install skills                                              ║")
	fmt.Fprintln(w, "║  Sub-skills install automatically with their parent.         ║")
	fmt.Fprintln(w, "╚══════════════════════════════════════════════════════════════╝")
	fmt.Fprintln(w)

	for i, row := range rows {
		deps := ""
		// Find depends_on for this slug to show sub-skill count
		for _, e := range all {
			if e.slug == row.slug && len(e.atom.DependsOn) > 0 {
				deps = fmt.Sprintf(" (+%d sub-skills)", len(e.atom.DependsOn))
				break
			}
		}
		fmt.Fprintf(w, "  %2d. %-18s%s\n      %s\n", i+1, row.slug+deps, "", row.description)
	}

	fmt.Fprintln(w)
	fmt.Fprintf(w, `Install which? (e.g. 1,3,5 or "all" or Enter to skip): `)

	scanner := bufio.NewScanner(r)
	scanner.Scan()
	line := strings.TrimSpace(scanner.Text())

	if line == "" {
		fmt.Fprintln(w, "\nSkipping skill installation.")
		return nil
	}

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

	fmt.Fprintln(w)
	for _, slug := range toInstall {
		fmt.Fprintf(w, "  Installing %-20s ", slug+"...")
		if installErr := install(cmd, slug); installErr != nil {
			fmt.Fprintf(w, "warning: %v\n", installErr)
		} else {
			fmt.Fprintln(w, "done")
		}
	}

	return nil
}

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
