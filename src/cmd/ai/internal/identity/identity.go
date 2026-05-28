package identity

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// Project holds the per-project git identity configuration read from
// metadata/projects.json.
type Project struct {
	Name        string   `json:"name"`
	URLPatterns []string `json:"url_patterns"`
	GitName     string   `json:"git_name"`
	GitEmail    string   `json:"git_email"`
	SigningKey   string   `json:"signing_key"`
}

// ProjectsConfig is the top-level structure of metadata/projects.json.
type ProjectsConfig struct {
	Version  string    `json:"version"`
	Projects []Project `json:"projects"`
}

// Load reads projects.json from configDir/metadata/projects.json.
// Returns nil, nil if the file does not exist.
func Load(configDir string) (*ProjectsConfig, error) {
	path := filepath.Join(configDir, "metadata", "projects.json")
	data, err := os.ReadFile(path) //nolint:gosec
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var cfg ProjectsConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// NormalizeURL strips git scheme prefixes and normalizes a clone URL to a
// canonical host/org/repo form suitable for pattern matching:
//
//	git@github.com:org/repo.git  → github.com/org/repo
//	https://github.com/org/repo.git → github.com/org/repo
func NormalizeURL(rawURL string) string {
	u := rawURL
	// Strip .git suffix.
	u = strings.TrimSuffix(u, ".git")
	// git@host:path → host/path
	if idx := strings.Index(u, "@"); idx >= 0 && !strings.HasPrefix(u, "http") {
		u = u[idx+1:]
		u = strings.Replace(u, ":", "/", 1)
	}
	// Strip https:// or http://.
	u = strings.TrimPrefix(strings.TrimPrefix(u, "https://"), "http://")
	return u
}

// Match returns the first Project whose url_patterns match the clone URL,
// or nil if none match. Pattern matching uses filepath.Match (glob),
// applied against the normalized form of cloneURL. Note that
// filepath.Match does not support ** — use a single * per path segment.
func Match(projects []Project, cloneURL string) *Project {
	normalized := NormalizeURL(cloneURL)
	for i := range projects {
		for _, pat := range projects[i].URLPatterns {
			matched, err := filepath.Match(pat, normalized)
			if err == nil && matched {
				return &projects[i]
			}
		}
	}
	return nil
}

// FindByName returns the project with the given name, or nil.
func FindByName(projects []Project, name string) *Project {
	for i := range projects {
		if projects[i].Name == name {
			return &projects[i]
		}
	}
	return nil
}
