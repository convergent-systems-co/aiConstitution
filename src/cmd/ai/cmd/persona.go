package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/convergent-systems-co/aiConstitution/src/internal/paths"
	"github.com/spf13/cobra"
)

// newPersonaCmd implements `ai persona {list,show,share}`.
// Issues #217 (list) and #218 (show). See SPEC.md §3 + §7.9.
func newPersonaCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "persona",
		Short: "Inspect persona atoms (agentic + reviewer)",
		Long: `persona surfaces user-installed personas from ~/.ai/personas/.

  agentic   (type: "agentic")  — loaded by `+"`"+`ai mode <name>`+"`"+`
  reviewer  (type: "reviewer") — invoked by /spawn review panels

See SPEC.md §3, §7.9.`,
	}

	c.AddCommand(
		newPersonaListCmd(),
		newPersonaShowCmd(),
		newPersonaShareCmd(),
	)
	return c
}

// personasDir returns the canonical user-installed personas directory.
func personasDir() string {
	return filepath.Join(paths.AIRoot(), "personas")
}

// personaPath returns the full path for a persona YAML file by name.
func personaPath(name string) string {
	return filepath.Join(personasDir(), name+".yaml")
}

// newPersonaListCmd implements `ai persona list [--type agentic|reviewer]`.
func newPersonaListCmd() *cobra.Command {
	var typeFilter string

	list := &cobra.Command{
		Use:   "list",
		Short: "List personas, grouped by type",
		RunE: func(cmd *cobra.Command, _ []string) error {
			dir := personasDir()
			entries, err := os.ReadDir(dir)
			if err != nil {
				if errors.Is(err, os.ErrNotExist) {
					fmt.Fprintln(cmd.OutOrStdout(), "(no personas installed)")
					return nil
				}
				return fmt.Errorf("persona list: read %s: %w", dir, err)
			}

			type personaRow struct {
				name string
				kind string
				desc string
			}
			var rows []personaRow
			for _, e := range entries {
				if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
					continue
				}
				data, readErr := os.ReadFile(filepath.Join(dir, e.Name()))
				if readErr != nil {
					continue
				}
				name := strings.TrimSuffix(e.Name(), ".yaml")
				kind := parseYAMLField(data, "type")
				desc := parseYAMLField(data, "description")

				// Apply --type filter when set.
				if typeFilter != "" && kind != typeFilter {
					continue
				}
				rows = append(rows, personaRow{name: name, kind: kind, desc: desc})
			}

			if len(rows) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "(no personas installed)")
				return nil
			}

			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "NAME\tTYPE\tDESCRIPTION")
			for _, r := range rows {
				fmt.Fprintf(w, "%s\t%s\t%s\n", r.name, r.kind, r.desc)
			}
			return w.Flush()
		},
	}
	list.Flags().StringVar(&typeFilter, "type", "", "filter by type (agentic|reviewer)")

	return list
}

// newPersonaShowCmd implements `ai persona show <name>`.
func newPersonaShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show <name>",
		Short: "Show resolved persona content + metadata",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := personaPath(args[0])
			data, err := os.ReadFile(path)
			if err != nil {
				if errors.Is(err, os.ErrNotExist) {
					return fmt.Errorf("persona %q not found", args[0])
				}
				return fmt.Errorf("persona show: %w", err)
			}
			fmt.Fprint(cmd.OutOrStdout(), string(data))
			return nil
		},
	}
}

// newPersonaShareCmd remains a stub (§7.9.3 is out of scope for this batch).
func newPersonaShareCmd() *cobra.Command {
	var shareDomain bool
	share := &cobra.Command{
		Use:   "share <name>",
		Short: "File a persona draft upstream (agentic by default; --domain for reviewer)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			notice("persona share:", args[0], "(reviewer:", shareDomain, ")")
			return stub("persona share", "§7.9.3")
		},
	}
	share.Flags().BoolVar(&shareDomain, "domain", false, "share as a reviewer persona (YAML, kind: reviewer)")
	return share
}
