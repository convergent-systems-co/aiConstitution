package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// AiAtomsCatalogURL is the canonical catalog endpoint for ai-atoms.com.
// Tests override this via AiAtomsCatalogURLForTest (see export_test.go).
var AiAtomsCatalogURL = "https://ai-atoms.com/exports/catalog.json"

// aiAtomEntry represents a single atom in the ai-atoms.com catalog. Fields are
// populated differently depending on the atom type ("skill" vs "hook").
type aiAtomEntry struct {
	Type                 string   `json:"type"`
	ID                   string   `json:"id"`
	Version              string   `json:"version"`
	Name                 string   `json:"name"`
	Description          string   `json:"description"`
	Lifecycle            string   `json:"lifecycle"`
	DependsOn            []string `json:"depends_on,omitempty"`
	Event                string   `json:"event,omitempty"`
	Language             string   `json:"language,omitempty"`
	SystemPromptFragment string   `json:"system_prompt_fragment,omitempty"`
	Script               string   `json:"script,omitempty"` // hook script content; absent during catalog transition
}

// aiAtomsCatalog is the top-level document returned by the ai-atoms.com catalog
// endpoint.
type aiAtomsCatalog struct {
	Atoms []aiAtomEntry `json:"atoms"`
}

// fetchAiAtomsCatalog fetches and parses the ai-atoms.com catalog.
// Returns (nil, nil) when AiAtomsCatalogURL is empty (test override to skip
// the network call entirely).
func fetchAiAtomsCatalog() ([]aiAtomEntry, error) {
	url := AiAtomsCatalogURL
	if url == "" {
		return nil, nil
	}
	req, err := http.NewRequest(http.MethodGet, url, nil) //nolint:noctx // CLI tool
	if err != nil {
		return nil, fmt.Errorf("ai-atoms: build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ai-atoms: fetch catalog: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ai-atoms: catalog HTTP %d", resp.StatusCode)
	}

	var catalog aiAtomsCatalog
	if err := json.NewDecoder(resp.Body).Decode(&catalog); err != nil {
		return nil, fmt.Errorf("ai-atoms: decode catalog: %w", err)
	}
	return catalog.Atoms, nil
}

// aiAtomEntryToSkillAtom converts a catalog entry to the skillAtom shape used
// by install. The "skill/" prefix is stripped from the ID and from each
// depends_on entry, so dependency installation resolves bare slugs rather than
// the namespaced "skill/<slug>" form (which would 404 against the catalog).
func aiAtomEntryToSkillAtom(e aiAtomEntry) *skillAtom {
	return &skillAtom{
		ID:                   strings.TrimPrefix(e.ID, "skill/"),
		Name:                 e.Name,
		Description:          e.Description,
		Version:              e.Version,
		Lifecycle:            e.Lifecycle,
		DependsOn:            trimSkillPrefixes(e.DependsOn),
		SystemPromptFragment: e.SystemPromptFragment,
	}
}

// trimSkillPrefixes returns deps with the namespacing "skill/" prefix removed
// from each entry, yielding bare slugs that install can resolve directly.
func trimSkillPrefixes(deps []string) []string {
	if len(deps) == 0 {
		return deps
	}
	out := make([]string, len(deps))
	for i, d := range deps {
		out[i] = strings.TrimPrefix(d, "skill/")
	}
	return out
}

// fetchSkillAtomFromCatalog fetches the ai-atoms.com catalog and returns the
// atom matching the given slug. Returns an error if the catalog cannot be
// fetched or if no matching skill is found.
func fetchSkillAtomFromCatalog(slug string) (*skillAtom, error) {
	atoms, err := fetchAiAtomsCatalog()
	if err != nil {
		return nil, fmt.Errorf("skills: fetch catalog: %w", err)
	}
	for _, a := range atoms {
		if a.Type == "skill" && a.ID == "skill/"+slug {
			atom := aiAtomEntryToSkillAtom(a)
			return atom, nil
		}
	}
	return nil, fmt.Errorf("skills: skill %q not found in ai-atoms.com catalog", slug)
}
