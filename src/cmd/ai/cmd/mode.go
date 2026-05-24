package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/convergent-systems-co/aiConstitution/src/internal/paths"
	"github.com/convergent-systems-co/aiConstitution/src/internal/state"

	"github.com/spf13/cobra"
)

// personasAgenticDir is the shipped-persona directory walked by
// `ai mode list` and used for existence checks on `ai mode <name>`.
func personasAgenticDir() string {
	return filepath.Join(paths.GovernanceDir(), "personas", "agentic")
}

// newModeCmd implements `ai mode {current,list,clear,show,share}` and
// `ai mode <name>`. See SPEC.md §3.7 + §7.
func newModeCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "mode [name]",
		Short: "Activate a persona or profile (additive; not exclusive)",
		Long: `mode loads a persona or profile on top of the always-loaded
four-file constitution. Personas are additive emphasis, not
replacements — see SPEC.md §7 for the rationale.

Subcommands:
  current   Print the active mode.
  list      Enumerate available personas (governance/personas/agentic/).
  clear     Deactivate the current mode (return to four-file only).
  show      Show resolved content for a name.
  share     File a draft as an upstream atom PR.

See SPEC.md §3.7 + §7.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			return activateMode(cmd, args[0])
		},
	}

	c.AddCommand(&cobra.Command{
		Use:   "current",
		Short: "Print the active mode",
		RunE: func(cmd *cobra.Command, _ []string) error {
			m, err := state.LoadMode()
			if err != nil {
				return fmt.Errorf("mode current: %w", err)
			}
			name := m.Name
			if name == "" {
				name = "(none)"
			}
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), name)
			return nil
		},
	})

	c.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "Enumerate shipped agentic personas",
		RunE: func(cmd *cobra.Command, _ []string) error {
			names, err := listAgenticPersonas()
			if err != nil {
				return fmt.Errorf("mode list: %w", err)
			}
			if len(names) == 0 {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "(no personas found)")
				return nil
			}
			for _, n := range names {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), n)
			}
			return nil
		},
	})

	c.AddCommand(&cobra.Command{
		Use:   "clear",
		Short: "Deactivate the current mode (return to four-file only)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := state.SaveMode(state.Mode{}); err != nil {
				return fmt.Errorf("mode clear: %w", err)
			}
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Mode cleared.")
			return nil
		},
	})

	c.AddCommand(&cobra.Command{
		Use:   "show <name>",
		Short: "Show resolved persona/profile content + metadata",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			notice("mode show:", args[0])
			return stub("mode show", "§7.8.5")
		},
	})

	c.AddCommand(&cobra.Command{
		Use:   "share <name>",
		Short: "File a draft as an upstream atom PR",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			notice("mode share:", args[0])
			return stub("mode share", "§7.9.3")
		},
	})

	return c
}

// newFocusCmd is the documented alias of `ai mode` (SPEC.md §3.7).
func newFocusCmd() *cobra.Command {
	c := newModeCmd()
	c.Use = "focus [name]"
	c.Short = "Alias of `ai mode`"
	c.Aliases = nil
	return c
}

// listAgenticPersonas returns the sorted basenames of every *.md file
// under paths.GovernanceDir()/personas/agentic/. Returns an empty
// slice (not an error) if the directory does not yet exist — fresh
// installs don't have governance/ until `ai setup`.
func listAgenticPersonas() ([]string, error) {
	dir := personasAgenticDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".md") {
			continue
		}
		out = append(out, strings.TrimSuffix(name, ".md"))
	}
	sort.Strings(out)
	return out, nil
}

// activateMode writes mode.json for the named agentic persona. Refuses
// to activate an unknown name rather than silently saving a bogus
// reference; the rule per Common.md P2 (Honesty Over Compliance) is to
// fail loudly.
func activateMode(cmd *cobra.Command, name string) error {
	candidate := filepath.Join(personasAgenticDir(), name+".md")
	if _, err := os.Stat(candidate); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("mode: persona %q not found at %s", name, candidate)
		}
		return fmt.Errorf("mode: stat %s: %w", candidate, err)
	}
	m := state.Mode{
		Name:         name,
		Type:         "persona",
		Source:       "shipped",
		ActivatedAt:  time.Now().UTC(),
		ActivatedVia: "cli",
		SourcePath:   candidate,
	}
	if err := state.SaveMode(m); err != nil {
		return fmt.Errorf("mode: save: %w", err)
	}
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Mode set: %s\n", name)
	return nil
}
