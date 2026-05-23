package cmd

import "github.com/spf13/cobra"

// newSkillsCmd implements `ai skills {install,uninstall,upgrade,share}`.
// See SPEC.md §7.10.
func newSkillsCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "skills",
		Short: "Manage skill atoms (tarball bundles: SKILL.md + templates + assets)",
		Long: `skills manages skill atoms from skill-atoms.com. The local layout
holds manifests (TOML pinning atom@version); the content lives in the
~/.config/aiConstitution/.skill-cache/.

See SPEC.md §7.10.`,
	}

	c.AddCommand(
		&cobra.Command{Use: "install <name>[@<version>]", Short: "Resolve from skill-atoms.com; cache; symlink", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
			notice("skills install:", args[0])
			return stub("skills install", "§7.10.2")
		}},
		&cobra.Command{Use: "uninstall <name>", Short: "Remove manifest + symlink (content stays cached)", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
			notice("skills uninstall:", args[0])
			return stub("skills uninstall", "§7.10.2")
		}},
		&cobra.Command{Use: "upgrade <name> [<version>]", Short: "Bump manifest, refetch, re-symlink", Args: cobra.RangeArgs(1, 2), RunE: func(cmd *cobra.Command, args []string) error {
			notice("skills upgrade:", args)
			return stub("skills upgrade", "§7.10.2")
		}},
		&cobra.Command{Use: "upgrade-all", Short: "Bump every installed skill to its latest stable", RunE: func(cmd *cobra.Command, _ []string) error {
			notice("skills upgrade-all")
			return stub("skills upgrade-all", "§7.10.2")
		}},
		&cobra.Command{Use: "share <name>", Short: "File a skill draft upstream as an atom PR", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
			notice("skills share:", args[0])
			return stub("skills share", "§7.10.3")
		}},
	)
	return c
}
