package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// AiAtomsCatalogURL is the canonical catalog endpoint for ai-atoms.com.
// Tests override this via AiAtomsCatalogURLForTest (see export_test.go).
var AiAtomsCatalogURL = "https://ai-atoms.com/exports/catalog.json"

// aiAtomEntry represents a single atom in the ai-atoms.com catalog. Fields are
// populated differently depending on the atom type ("skill" vs "hook").
type aiAtomEntry struct {
	Type        string   `json:"type"`
	ID          string   `json:"id"`
	Version     string   `json:"version"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Lifecycle   string   `json:"lifecycle"`
	DependsOn   []string `json:"depends_on,omitempty"`
	Event       string   `json:"event,omitempty"`
	Language    string   `json:"language,omitempty"`
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
