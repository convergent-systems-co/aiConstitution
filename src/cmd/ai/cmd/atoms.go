package cmd

import "github.com/spf13/cobra"

// newAtomsCmd implements `ai atoms fetch`. See SPEC.md §7.9 and §7.10.
//
// The generalized atoms surface unifies brand / persona / profile / skill
// fetching behind one resolver. `ai brand fetch`, `ai persona share`,
// `ai profile show`, and `ai skills install` are sugar atop this layer.
func newAtomsCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "atoms",
		Short: "Resolve, fetch, list, and verify atoms across the four registries",
		Long: `atoms is the unified surface for the four Convergent Systems atom
registries:

  brand-atoms.com    — W3C design tokens
  persona-atoms.com  — agentic + reviewer personas
  profile-atoms.com  — profile compositions
  skill-atoms.com    — skill bundles

See SPEC.md §7.9 + §7.10.`,
	}

	// fetch
	var kind, name, version string
	fetch := &cobra.Command{
		Use:   "fetch",
		Short: "Fetch an atom into the local cache (content-addressed by version)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			notice("atoms fetch:", kind, name+"@"+version)
			return stub("atoms fetch", "§7.9 + §7.10")
		},
	}
	fetch.Flags().StringVar(&kind, "kind", "", "agentic|reviewer|profile|skill|brand")
	fetch.Flags().StringVar(&name, "name", "", "atom name")
	fetch.Flags().StringVar(&version, "version", "latest", "SemVer pin (default: latest stable)")
	_ = fetch.MarkFlagRequired("kind")
	_ = fetch.MarkFlagRequired("name")

	// list
	var listKind string
	list := &cobra.Command{
		Use:   "list",
		Short: "List atoms in the local cache by kind",
		RunE: func(cmd *cobra.Command, _ []string) error {
			notice("atoms list:", listKind)
			return stub("atoms list", "§7.9.5")
		},
	}
	list.Flags().StringVar(&listKind, "kind", "", "filter (agentic|reviewer|profile|skill|brand)")

	// gc
	gc := &cobra.Command{
		Use:   "gc",
		Short: "Garbage-collect unreferenced atom cache entries (respects per-kind TTLs)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			notice("atoms gc:", "would walk caches and delete entries past gcUnusedDays AND unreferenced.")
			return stub("atoms gc", "§7.9.5")
		},
	}

	// verify
	verify := &cobra.Command{
		Use:   "verify",
		Short: "Verify SHA-256 content hashes of every cached atom",
		RunE: func(cmd *cobra.Command, _ []string) error {
			notice("atoms verify:", "would re-hash every cache entry and compare to meta.json.contentSha256.")
			return stub("atoms verify", "§7.9.5")
		},
	}

	c.AddCommand(fetch, list, gc, verify)
	return c
}
