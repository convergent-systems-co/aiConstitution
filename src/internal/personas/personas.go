// Package personas loads and parses persona YAML files from the
// ~/.ai/personas/ directory tree. It handles both agentic personas
// (used by `ai mode` and the spawn pipeline) and reviewer personas
// (used by `ai review --pr` panel scoring). See SPEC.md §8 and Epic #26.
package personas

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"go.yaml.in/yaml/v3"
)

// PersonaFile is the parsed representation of a persona YAML file.
// Both agentic and reviewer persona types share this struct; reviewer-only
// fields (Role, PanelWeight) are zero-valued for agentic personas.
type PersonaFile struct {
	// Name is the canonical identifier (e.g. "devops-engineer").
	Name string `yaml:"name"`

	// Type is either "agentic" or "reviewer".
	Type string `yaml:"type"`

	// Description is a human-readable summary of what this persona does.
	Description string `yaml:"description"`

	// Domains is the normalized domain list. The YAML source may supply
	// either a scalar string (`domain: cli`) or a sequence
	// (`domains: [cli, hooks]`). Both parse to []string via the
	// custom domainList unmarshaler. See #252.
	Domains domainList `yaml:"domains"`

	// Capabilities lists the tools or capabilities this persona may invoke.
	// Agentic personas only.
	Capabilities []string `yaml:"capabilities"`

	// Role is the reviewer panel role (e.g. "security"). Reviewer personas only.
	Role string `yaml:"role"`

	// PanelWeight is the fractional weight this reviewer contributes to the
	// aggregate panel score (0.0–1.0). Reviewer personas only.
	PanelWeight float64 `yaml:"panel_weight"`
}

// domainList is a []string that can be populated from either a YAML scalar
// (`domain: cli`) or a YAML sequence (`domains: [cli, hooks]`).
//
// The YAML key is "domains" in the struct tag; the loader also checks the
// legacy "domain" key before calling UnmarshalYAML. See normalizeDomain.
type domainList []string

// UnmarshalYAML implements yaml.Unmarshaler for domainList. It accepts
// both a scalar string and a sequence of strings, normalising both to
// []string.
func (d *domainList) UnmarshalYAML(value *yaml.Node) error {
	switch value.Kind {
	case yaml.ScalarNode:
		// `domains: cli` — single string
		*d = domainList{value.Value}
		return nil
	case yaml.SequenceNode:
		// `domains: [cli, hooks]`
		var list []string
		if err := value.Decode(&list); err != nil {
			return err
		}
		*d = domainList(list)
		return nil
	default:
		return fmt.Errorf("personas: unsupported YAML node kind %v for domain list", value.Kind)
	}
}

// rawPersonaFile mirrors PersonaFile but uses domainList for both the
// `domain` (singular) and `domains` (plural) keys so we can merge them
// after the initial unmarshal.
type rawPersonaFile struct {
	Name         string     `yaml:"name"`
	Type         string     `yaml:"type"`
	Description  string     `yaml:"description"`
	Domain       domainList `yaml:"domain"`  // legacy scalar key
	Domains      domainList `yaml:"domains"` // preferred array key
	Capabilities []string   `yaml:"capabilities"`
	Role         string     `yaml:"role"`
	PanelWeight  float64    `yaml:"panel_weight"`
}

// toPersonaFile merges the raw struct (which has both `domain` and `domains`
// fields) into a PersonaFile whose Domains field contains the union of both.
func (r rawPersonaFile) toPersonaFile() PersonaFile {
	domains := append([]string(nil), []string(r.Domain)...)
	for _, d := range r.Domains {
		// Avoid duplicates when both keys happen to list the same value.
		duplicate := false
		for _, existing := range domains {
			if existing == d {
				duplicate = true
				break
			}
		}
		if !duplicate {
			domains = append(domains, d)
		}
	}
	return PersonaFile{
		Name:         r.Name,
		Type:         r.Type,
		Description:  r.Description,
		Domains:      domainList(domains),
		Capabilities: r.Capabilities,
		Role:         r.Role,
		PanelWeight:  r.PanelWeight,
	}
}

// LoadPersonas reads all *.yaml files under dir and returns the successfully
// parsed PersonaFile values. Files that fail to parse are logged at warning
// level and skipped; no top-level error is returned for individual file
// failures. An error is returned only if dir itself cannot be read.
func LoadPersonas(dir string) ([]PersonaFile, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("personas.LoadPersonas: read dir %q: %w", dir, err)
	}

	var result []PersonaFile
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".yaml") && !strings.HasSuffix(name, ".yml") {
			continue
		}

		path := filepath.Join(dir, name)
		data, err := os.ReadFile(path)
		if err != nil {
			log.Printf("[personas] warning: read %q: %v — skipping", path, err)
			continue
		}

		var raw rawPersonaFile
		if err := yaml.Unmarshal(data, &raw); err != nil {
			log.Printf("[personas] warning: parse %q: %v — skipping", path, err)
			continue
		}

		result = append(result, raw.toPersonaFile())
	}
	return result, nil
}
