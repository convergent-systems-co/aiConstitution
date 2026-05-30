package cmd

// categories.go — canonical primary-category taxonomy for skill atoms.
//
// This list mirrors the `category` enum in the ai-atoms skill schema
// (schemas/skill-v1.json). Atoms store the slug; the CLI renders the display
// name. Keep the two in sync: a slug here that the schema rejects (or vice
// versa) will validate-pass locally but fail in the catalog build.

// skillCategory pairs a stored slug with its human-readable display name.
type skillCategory struct {
	Slug    string
	Display string
}

// skillCategories is the ordered taxonomy. Order is the presentation order in
// `ai skills categories` and in grouped `ai skills available` output.
var skillCategories = []skillCategory{
	{"coding", "Engineering & Coding"},
	{"dotnet", ".NET / Microsoft Engineering"},
	{"data", "Data, Analytics & Research"},
	{"design", "Design & UX"},
	{"product", "Product & Delivery"},
	{"operations", "Operations & IT"},
	{"finance", "Finance & Accounting"},
	{"legal", "Legal & Compliance"},
	{"sales", "Sales & CRM"},
	{"marketing", "Marketing & Content"},
	{"hr", "People & HR"},
	{"support", "Customer Support"},
	{"knowledge", "Knowledge & Docs"},
	{"governance", "Governance & Meta"},
}

// uncategorizedSlug / Display bucket skills with no (or an unknown) category.
const (
	uncategorizedSlug    = "other"
	uncategorizedDisplay = "Other"
)

// categoryDisplay returns the display name for a category slug, or "Other" for
// an empty/unknown slug.
func categoryDisplay(slug string) string {
	for _, c := range skillCategories {
		if c.Slug == slug {
			return c.Display
		}
	}
	return uncategorizedDisplay
}

// isValidCategory reports whether slug is one of the canonical categories.
func isValidCategory(slug string) bool {
	for _, c := range skillCategories {
		if c.Slug == slug {
			return true
		}
	}
	return false
}

// categorySlugs returns just the slugs, in taxonomy order.
func categorySlugs() []string {
	out := make([]string, len(skillCategories))
	for i, c := range skillCategories {
		out[i] = c.Slug
	}
	return out
}
