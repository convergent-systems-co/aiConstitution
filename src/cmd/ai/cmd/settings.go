package cmd

import "github.com/spf13/cobra"

// newSettingsCmd implements `ai settings {get,set,edit,reset}`.
// See SPEC.md §3.11 + §13.
func newSettingsCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "settings",
		Short: "Read or write user preferences at ~/.config/aiConstitution/settings.toml",
		Long: `settings manages the canonical TOML at
~/.config/aiConstitution/settings.toml (XDG-compliant on Linux; same
path on macOS for cross-platform predictability).

Precedence (highest first): environment variable → settings.toml →
shipped defaults compiled into the binary.

See SPEC.md §3.11 + §13.`,
	}

	c.AddCommand(
		&cobra.Command{Use: "get <key>", Short: "Read a setting", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
			notice("settings get:", args[0])
			return stub("settings get", "§13")
		}},
		&cobra.Command{Use: "set <key>=<value>", Short: "Write a setting (validated)", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
			notice("settings set:", args[0])
			return stub("settings set", "§13.2")
		}},
		&cobra.Command{Use: "edit", Short: "Open settings.toml in $EDITOR (validates on save)", RunE: func(cmd *cobra.Command, _ []string) error {
			notice("settings edit")
			return stub("settings edit", "§13.2")
		}},
		func() *cobra.Command {
			var acceptDefaults bool
			r := &cobra.Command{
				Use:   "reset",
				Short: "Restore defaults",
				RunE: func(cmd *cobra.Command, _ []string) error {
					notice("settings reset: accept-defaults=", acceptDefaults)
					return stub("settings reset", "§13.1")
				},
			}
			r.Flags().BoolVar(&acceptDefaults, "accept-defaults", false, "non-interactive accept of the canonical defaults")
			return r
		}(),
	)
	return c
}
