package cmd

import "github.com/spf13/cobra"

// newModeCmd implements `ai mode {current,list,clear,show,share}` and
// `ai mode <name>`. See SPEC.md §3.7 + §7.
func newModeCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "mode [name]",
		Short: "Activate a persona or profile (additive; not exclusive)",
		Long: `mode loads a persona or profile on top of the always-loaded
four-file constitution. Personas are additive emphasis, not
replacements — see SPEC.md §7 for the rationale.

Resolution order (SPEC.md §7.8.5):
  1. ~/.config/aiConstitution/profile-drafts/<name>.toml
  2. ~/.ai/governance/profiles/<name>.toml
  3. profile-atoms.com/<name>/latest
  4. ~/.config/aiConstitution/persona-drafts/<name>.md
  5. persona-atoms.com/<name>/latest

Use --persona or --profile for unambiguous selection.

Subcommands:
  current   Print the active mode.
  list      Enumerate available profiles and personas.
  clear     Deactivate the current mode (return to four-file only).
  show      Show resolved content for a name.
  share     File a draft as an upstream atom PR.

See SPEC.md §3.7 + §7.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			notice("mode:", "would resolve and activate", args[0])
			return stub("mode "+args[0], "§3.7 + §7.8.5")
		},
	}

	// current
	c.AddCommand(&cobra.Command{
		Use:   "current",
		Short: "Print the active mode",
		RunE: func(cmd *cobra.Command, _ []string) error {
			notice("mode current:", "would read ~/.config/aiConstitution/mode.json")
			return stub("mode current", "§7.4")
		},
	})

	// list
	c.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "Enumerate profiles + personas, grouped by source",
		RunE: func(cmd *cobra.Command, _ []string) error {
			notice("mode list:", "would walk profiles + personas + drafts + atoms")
			return stub("mode list", "§7.3")
		},
	})

	// clear
	c.AddCommand(&cobra.Command{
		Use:   "clear",
		Short: "Deactivate the current mode (return to four-file only)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			notice("mode clear:", "would delete ~/.config/aiConstitution/mode.json")
			return stub("mode clear", "§7.4")
		},
	})

	// show
	c.AddCommand(&cobra.Command{
		Use:   "show <name>",
		Short: "Show resolved persona/profile content + metadata",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			notice("mode show:", args[0])
			return stub("mode show", "§7.8.5")
		},
	})

	// share
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
