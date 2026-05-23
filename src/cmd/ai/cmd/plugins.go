package cmd

import "github.com/spf13/cobra"

// newPluginsCmd implements `ai plugins {list,enable,disable,status,update}`.
// See SPEC.md §11.
func newPluginsCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "plugins",
		Short: "Manage Claude plugins that extend the agent's workflow surface",
		Long: `plugins are Claude-specific extensions (e.g., superpowers,
amendment-author, hook-author, atom-publisher, review-panel,
memory-curator) that wrap CLI verbs in guided multi-step workflows.

See SPEC.md §11.`,
	}

	c.AddCommand(
		&cobra.Command{Use: "list", Short: "Show available + installed Claude plugins", RunE: func(cmd *cobra.Command, _ []string) error {
			notice("plugins list")
			return stub("plugins list", "§11")
		}},
		&cobra.Command{Use: "enable <name>", Short: "Enable a Claude plugin", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
			notice("plugins enable:", args[0])
			return stub("plugins enable", "§11.1")
		}},
		&cobra.Command{Use: "disable <name>", Short: "Disable a plugin without uninstalling", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
			notice("plugins disable:", args[0])
			return stub("plugins disable", "§11.6")
		}},
		&cobra.Command{Use: "status", Short: "Per-plugin: installed? enabled? version?", RunE: func(cmd *cobra.Command, _ []string) error {
			notice("plugins status")
			return stub("plugins status", "§11.6")
		}},
		&cobra.Command{Use: "update <name>", Short: "Update an installed plugin", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
			notice("plugins update:", args[0])
			return stub("plugins update", "§11.6")
		}},
	)
	return c
}
