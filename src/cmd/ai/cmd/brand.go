package cmd

import "github.com/spf13/cobra"

// newBrandCmd implements `ai brand {fetch,list}`. See SPEC.md §14.4
// and §7.9.5 (cache discipline).
func newBrandCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "brand",
		Short: "Fetch or list brand atoms from brand-atoms.com",
		Long: `brand resolves W3C design tokens from brand-atoms.com.
The canonical brand for Convergent Systems sites is convergent-systems@1.0.0
— see SPEC.md §14.4. Brand atoms cache to
~/.config/aiConstitution/.brand-cache/.

This is a thin alias of `+"`"+`ai atoms fetch --kind=brand`+"`"+`.

See SPEC.md §14.4 + §7.9.5.`,
	}

	// fetch
	var brand string
	fetch := &cobra.Command{
		Use:   "fetch",
		Short: "Fetch a brand atom into the local cache",
		RunE: func(cmd *cobra.Command, _ []string) error {
			notice("brand fetch:", brand)
			return stub("brand fetch", "§14.4 + §7.9.5")
		},
	}
	fetch.Flags().StringVar(&brand, "brand", "convergent-systems", "brand id (default: convergent-systems)")

	// list
	list := &cobra.Command{
		Use:   "list",
		Short: "List brand atoms in the local cache",
		RunE: func(cmd *cobra.Command, _ []string) error {
			notice("brand list:", "would enumerate ~/.config/aiConstitution/.brand-cache/")
			return stub("brand list", "§14.4")
		},
	}

	c.AddCommand(fetch, list)
	return c
}
