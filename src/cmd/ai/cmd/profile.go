package cmd

import "github.com/spf13/cobra"

// newProfileCmd implements `ai profile {list,show,new,edit,remove,share}`.
// See SPEC.md §3.8 + §7.8.
func newProfileCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "profile",
		Short: "Manage profiles (compositions of atomic personas)",
		Long: `profile manages the TOML recipes at
~/.ai/governance/profiles/ and ~/.config/aiConstitution/profile-drafts/
that pin atom@version references.

See SPEC.md §3.8 + §7.8.`,
	}

	c.AddCommand(
		&cobra.Command{Use: "list", Short: "List profiles", RunE: func(cmd *cobra.Command, _ []string) error {
			notice("profile list:", "would enumerate ~/.ai/governance/profiles/")
			return stub("profile list", "§3.8 + §7.8.2")
		}},
		&cobra.Command{Use: "show <name>", Short: "Show profile TOML + resolved persona content", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
			notice("profile show:", args[0])
			return stub("profile show", "§7.8")
		}},
		&cobra.Command{Use: "new <name>", Short: "Interactive composer for a new profile", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
			notice("profile new:", args[0])
			return stub("profile new", "§7.8")
		}},
		&cobra.Command{Use: "edit <name>", Short: "Open profile TOML in $EDITOR (validates on save)", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
			notice("profile edit:", args[0])
			return stub("profile edit", "§7.8")
		}},
		&cobra.Command{Use: "remove <name>", Short: "Remove a profile (refuses if active or default)", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
			notice("profile remove:", args[0])
			return stub("profile remove", "§7.8")
		}},
		&cobra.Command{Use: "share <name>", Short: "File the profile upstream as an atom PR", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
			notice("profile share:", args[0])
			return stub("profile share", "§7.9.3")
		}},
	)
	return c
}
