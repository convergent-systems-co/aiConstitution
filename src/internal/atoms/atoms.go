// Package atoms is the resolver for the four Convergent Systems atom
// registries (brand / persona / profile / skill). Per SPEC.md §7.9.2,
// "the shape match means a single resolver implementation in bin/ai
// handles all four registries — only the URL template and the content
// type differ."
//
// Live HTTP fetch against *-atoms.com is DEFERRED to morning work per
// GOALS.md "Out of scope for v0.8." This package ships the type
// system, cache-lookup helpers, and the URL template logic; the
// actual transport is stubbed.
package atoms

// Kind enumerates the atom kinds.
type Kind string

// Atom kind values. Each value corresponds to one URL template under
// the appropriate Convergent Systems atom registry.
const (
	KindAgentic  Kind = "agentic"  // persona-atoms.com/agentic/<name>/<ver>/persona.md
	KindReviewer Kind = "reviewer" // persona-atoms.com/reviewer/<name>/<ver>/reviewer.yaml
	KindProfile  Kind = "profile"  // profile-atoms.com/<name>/<ver>/profile.toml
	KindSkill    Kind = "skill"    // skill-atoms.com/<name>/<ver>/skill.tar.gz
	KindBrand    Kind = "brand"    // brand-atoms.com/brands/<id>/<ver>/brand.json
)

// Ref is a content-addressable atom reference: kind + name + version.
// "version" is a SemVer string; "latest" is allowed only outside
// pinned recipes (per SPEC.md §7.8.1).
type Ref struct {
	Kind    Kind
	Name    string
	Version string
}

// Atom is a resolved atom — its metadata plus the on-disk path to the
// cached content file.
type Atom struct {
	Ref      Ref
	Meta     Meta
	CachePath string // absolute path to the content file
}

// Meta is the .meta.json shape per SPEC.md §7.9.1.
type Meta struct {
	Name           string   `json:"name"`
	Version        string   `json:"version"`
	Kind           Kind     `json:"kind"`
	DisplayName    string   `json:"displayName,omitempty"`
	Description    string   `json:"description,omitempty"`
	Domains        []string `json:"domains,omitempty"`        // reviewer only
	ContentSha256  string   `json:"contentSha256"`
	Authored       string   `json:"authored,omitempty"`
	PublishedAt    string   `json:"publishedAt,omitempty"`
	CompatibleWith []Compat `json:"compatibleWith,omitempty"`
	License        string   `json:"license,omitempty"`
}

// Compat is one entry in the compatibleWith[] list (SPEC.md §7.9.1).
type Compat struct {
	Atom     string `json:"atom"`
	Kind     Kind   `json:"kind"`
	Versions string `json:"versions"` // SemVer range
}

// Resolve loads an atom from the local cache. If absent and network
// fetching is enabled, fetches from the appropriate registry. On
// content-hash mismatch, quarantines the cache entry and refetches.
//
// TBD for v0.8 — only the type system is in place; the HTTP transport
// and cache-write logic are morning work.
func Resolve(_ Ref) (Atom, error) {
	return Atom{}, nil
}

// URL returns the canonical registry URL for a Ref. Uses the
// per-registry templates from SPEC.md §7.9.1 / §7.9.2.
func URL(r Ref, registries Registries) string {
	switch r.Kind {
	case KindAgentic:
		return joinURL(registries.Persona, "agentic", r.Name, r.Version, "persona.md")
	case KindReviewer:
		return joinURL(registries.Persona, "reviewer", r.Name, r.Version, "reviewer.yaml")
	case KindProfile:
		return joinURL(registries.Profile, r.Name, r.Version, "profile.toml")
	case KindSkill:
		return joinURL(registries.Skill, r.Name, r.Version, "skill.tar.gz")
	case KindBrand:
		// brand-atoms.com keeps a slightly different path shape (the
		// brand id is the "name"); honored here.
		return joinURL(registries.Brand, "dist/brands", r.Name, r.Version, "json/brand.json")
	}
	return ""
}

// Registries are the four registry roots, overridable via
// settings.toml [atoms].
type Registries struct {
	Persona string
	Profile string
	Skill   string
	Brand   string
}

// DefaultRegistries returns the canonical Convergent Systems URLs.
func DefaultRegistries() Registries {
	return Registries{
		Persona: "https://persona-atoms.com",
		Profile: "https://profile-atoms.com",
		Skill:   "https://skill-atoms.com",
		Brand:   "https://brand-atoms.com",
	}
}

// joinURL is a path-aware URL join that tolerates trailing slashes.
func joinURL(base string, parts ...string) string {
	out := trimTrailingSlash(base)
	for _, p := range parts {
		out += "/" + trimSlashes(p)
	}
	return out
}

func trimTrailingSlash(s string) string {
	for len(s) > 0 && s[len(s)-1] == '/' {
		s = s[:len(s)-1]
	}
	return s
}

func trimSlashes(s string) string {
	for len(s) > 0 && s[0] == '/' {
		s = s[1:]
	}
	return trimTrailingSlash(s)
}
