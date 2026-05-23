package cmd

import "github.com/spf13/cobra"

// newPersonaCmd implements `ai persona {list,show,share}`.
// See SPEC.md §3 (v0.6 additions) and §7.9.
func newPersonaCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "persona",
		Short: "Inspect persona atoms (agentic + reviewer)",
		Long: `persona surfaces the two kinds of persona atoms:

  agentic   (kind: "agentic")  — loaded by `+"`"+`ai mode <name>`+"`"+`
  reviewer  (kind: "reviewer") — invoked by /spawn review panels

Both kinds resolve via persona-atoms.com, cached locally at
~/.config/aiConstitution/.persona-cache/<kind>/<name>/<version>/.

See SPEC.md §3, §7.9.`,
	}

	// list
	var listKind, listDomain string
	list := &cobra.Command{
		Use:   "list",
		Short: "List persona atoms, grouped by kind",
		RunE: func(cmd *cobra.Command, _ []string) error {
			notice("persona list:", "kind:", listKind, "domain:", listDomain)
			return stub("persona list", "§3 + §7.9.1")
		},
	}
	list.Flags().StringVar(&listKind, "kind", "", "filter by kind (agentic|reviewer)")
	list.Flags().StringVar(&listDomain, "domain", "", "filter reviewer personas by domain (engineering|security|architecture|documentation|finops|data)")

	// show
	c.AddCommand(&cobra.Command{
		Use:   "show <name>",
		Short: "Show resolved persona content + metadata",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			notice("persona show:", args[0])
			return stub("persona show", "§7.9.1")
		},
	})

	// share
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

	c.AddCommand(list, share)
	return c
}
