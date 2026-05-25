// manifest.go — AtomManifest and atoms.json index helpers.
//
// An "atom" is a constitution bundle: a tar.gz archive containing an
// atom.toml manifest plus arbitrary content files. This file defines:
//
//   - AtomManifest — the parsed shape of atom.toml
//   - ParseAtomTOML / WriteAtomTOML — TOML I/O
//   - AtomsIndexEntry / ReadAtomsIndex / UpdateAtomsIndex — atoms.json I/O
//
// atom.toml fields:
//
//	name         string   — atom name (kebab-case)
//	version      string   — SemVer string
//	sha256       string   — hex-encoded SHA-256 of the downloaded tar.gz
//	upstream_url string   — canonical download URL
//	upstream_ref string   — "<original-name>@<version>" (forks only)
//	files        []string — relative file paths included (publish only)

package atoms

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// AtomManifest is the parsed representation of atom.toml.
type AtomManifest struct {
	Name        string   `toml:"name"`
	Version     string   `toml:"version"`
	SHA256      string   `toml:"sha256"`
	UpstreamURL string   `toml:"upstream_url"`
	UpstreamRef string   `toml:"upstream_ref,omitempty"`
	Files       []string `toml:"files,omitempty"`
}

// ParseAtomTOML reads and parses the atom.toml file at path.
func ParseAtomTOML(path string) (AtomManifest, error) {
	var m AtomManifest
	if _, err := toml.DecodeFile(path, &m); err != nil {
		return AtomManifest{}, fmt.Errorf("parse atom.toml at %q: %w", path, err)
	}
	return m, nil
}

// WriteAtomTOML encodes m as TOML and writes it to path, creating
// parent directories as needed.
func WriteAtomTOML(path string, m AtomManifest) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("mkdir for atom.toml: %w", err)
	}
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create atom.toml: %w", err)
	}
	defer f.Close()
	enc := toml.NewEncoder(f)
	if err := enc.Encode(m); err != nil {
		return fmt.Errorf("encode atom.toml: %w", err)
	}
	return nil
}

// AtomsIndexEntry is one record in ~/.config/aiConstitution/atoms.json.
// The JSON field names are intentionally lowercase-snake to match the
// atom.toml vocabulary.
type AtomsIndexEntry struct {
	Name     string `json:"name"`
	Version  string `json:"version"`
	Path     string `json:"path"`
	Upstream string `json:"upstream"`
}

// ReadAtomsIndex reads atoms.json from indexPath. Returns an empty
// slice (not an error) when the file does not exist.
func ReadAtomsIndex(indexPath string) ([]AtomsIndexEntry, error) {
	data, err := os.ReadFile(indexPath)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read atoms.json: %w", err)
	}
	var entries []AtomsIndexEntry
	if jsonErr := json.Unmarshal(data, &entries); jsonErr != nil {
		return nil, fmt.Errorf("parse atoms.json: %w", jsonErr)
	}
	return entries, nil
}

// UpdateAtomsIndex upserts entry into the atoms.json at indexPath.
// An existing entry with the same name is replaced; a new entry is
// appended. Parent directories are created if absent.
func UpdateAtomsIndex(indexPath string, entry AtomsIndexEntry) error {
	entries, err := ReadAtomsIndex(indexPath)
	if err != nil {
		return err
	}

	// Upsert by name.
	found := false
	for i, e := range entries {
		if e.Name == entry.Name {
			entries[i] = entry
			found = true
			break
		}
	}
	if !found {
		entries = append(entries, entry)
	}

	if err := os.MkdirAll(filepath.Dir(indexPath), 0755); err != nil {
		return fmt.Errorf("mkdir for atoms.json: %w", err)
	}
	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal atoms.json: %w", err)
	}
	data = append(data, '\n')
	if writeErr := os.WriteFile(indexPath, data, 0644); writeErr != nil {
		return fmt.Errorf("write atoms.json: %w", writeErr)
	}
	return nil
}
