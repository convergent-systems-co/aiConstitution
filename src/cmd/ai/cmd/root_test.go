package cmd_test

import (
	"testing"

	cmd "github.com/convergent-systems-co/aiConstitution/src/cmd/ai/cmd"
)

// TestRootCmdNoDuplicateSubcommands ensures no two direct children of the root
// command share a name. Cobra does not deduplicate on AddCommand, so a duplicate
// registration silently adds a second child — only the first one is reachable.
func TestRootCmdNoDuplicateSubcommands(t *testing.T) {
	t.Parallel()
	seen := map[string]bool{}
	for _, c := range cmd.NewRootCmd().Commands() {
		name := c.Name()
		if seen[name] {
			t.Errorf("duplicate subcommand registered: %q", name)
		}
		seen[name] = true
	}
}
