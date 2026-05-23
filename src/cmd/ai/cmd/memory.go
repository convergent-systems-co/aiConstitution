package cmd

import "github.com/spf13/cobra"

// newMemoryCmd implements `ai memory {list,codify,retire}`.
// See SPEC.md §3 and §6.
func newMemoryCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "memory",
		Short: "Inspect and curate ~/.ai/memory/",
		Long: `memory operates on the cross-tool memory layer at ~/.ai/memory/.

Subcommands:
  list      Enumerate memories (optionally filtered by type).
  codify    Promote a memory to a constitutional amendment.
  retire    Remove a memory (typically after codification).

See SPEC.md §3, §6, and Common.md §5.1.`,
	}

	// list
	var listType string
	list := &cobra.Command{
		Use:   "list",
		Short: "List memories",
		RunE: func(cmd *cobra.Command, _ []string) error {
			notice("memory list:", "type filter:", listType)
			return stub("memory list", "§3 + Common.md §5.1")
		},
	}
	list.Flags().StringVar(&listType, "type", "", "filter by type (feedback|reference|project|user)")

	// codify
	codify := &cobra.Command{
		Use:   "codify <slug>",
		Short: "Promote a memory to a constitutional amendment",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			notice("memory codify:", args[0])
			return stub("memory codify", "§6")
		},
	}

	// retire
	retire := &cobra.Command{
		Use:   "retire <slug>",
		Short: "Retire (remove) a memory entry",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			notice("memory retire:", args[0])
			return stub("memory retire", "§6")
		},
	}

	c.AddCommand(list, codify, retire)
	return c
}
