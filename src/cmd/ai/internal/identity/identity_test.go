package identity_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/convergent-systems-co/aiConstitution/src/cmd/ai/internal/identity"
)

// TestNormalizeURL verifies that various clone URL forms are collapsed
// to the canonical host/org/repo form used for pattern matching.
func TestNormalizeURL(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"https://github.com/myorg/repo.git", "github.com/myorg/repo"},
		{"https://github.com/myorg/repo", "github.com/myorg/repo"},
		{"http://github.com/myorg/repo.git", "github.com/myorg/repo"},
		{"git@github.com:myorg/repo.git", "github.com/myorg/repo"},
		{"git@github.com:myorg/repo", "github.com/myorg/repo"},
		{"git@gitlab.com:group/subgroup/repo.git", "gitlab.com/group/subgroup/repo"},
		{"https://bitbucket.org/team/project.git", "bitbucket.org/team/project"},
	}
	for _, tc := range cases {
		got := identity.NormalizeURL(tc.in)
		if got != tc.want {
			t.Errorf("NormalizeURL(%q) = %q; want %q", tc.in, got, tc.want)
		}
	}
}

// TestMatch_GlobPattern verifies that a glob pattern like
// github.com/myorg/* matches an https clone URL for a repo in that org.
func TestMatch_GlobPattern(t *testing.T) {
	projects := []identity.Project{
		{
			Name:        "work",
			URLPatterns: []string{"github.com/myorg/*"},
			GitName:     "Work User",
			GitEmail:    "work@example.com",
		},
	}
	p := identity.Match(projects, "https://github.com/myorg/repo.git")
	if p == nil {
		t.Fatal("Match returned nil; want non-nil")
	}
	if p.Name != "work" {
		t.Errorf("Match.Name = %q; want %q", p.Name, "work")
	}
}

// TestMatch_GitAt verifies that a git@ URL is normalized and matched
// correctly against a glob pattern.
func TestMatch_GitAt(t *testing.T) {
	projects := []identity.Project{
		{
			Name:        "personal",
			URLPatterns: []string{"github.com/myorg/*"},
			GitEmail:    "personal@example.com",
		},
	}
	p := identity.Match(projects, "git@github.com:myorg/repo.git")
	if p == nil {
		t.Fatal("Match returned nil for git@ URL; want non-nil")
	}
}

// TestMatch_NoMatch verifies that a URL with no matching project returns nil.
func TestMatch_NoMatch(t *testing.T) {
	projects := []identity.Project{
		{
			Name:        "work",
			URLPatterns: []string{"github.com/work-org/*"},
			GitEmail:    "work@example.com",
		},
	}
	p := identity.Match(projects, "https://github.com/other-org/repo.git")
	if p != nil {
		t.Errorf("Match returned %+v; want nil", p)
	}
}

// TestMatch_FirstWins verifies that when two projects have patterns
// matching the same URL, the first project in the list is returned.
func TestMatch_FirstWins(t *testing.T) {
	projects := []identity.Project{
		{
			Name:        "first",
			URLPatterns: []string{"github.com/myorg/*"},
			GitEmail:    "first@example.com",
		},
		{
			Name:        "second",
			URLPatterns: []string{"github.com/myorg/*"},
			GitEmail:    "second@example.com",
		},
	}
	p := identity.Match(projects, "https://github.com/myorg/repo.git")
	if p == nil {
		t.Fatal("Match returned nil; want non-nil")
	}
	if p.Name != "first" {
		t.Errorf("Match.Name = %q; want %q (first wins)", p.Name, "first")
	}
}

// TestFindByName verifies the name-based lookup for both found and
// not-found cases.
func TestFindByName(t *testing.T) {
	projects := []identity.Project{
		{Name: "alpha", GitEmail: "alpha@example.com"},
		{Name: "beta", GitEmail: "beta@example.com"},
	}

	t.Run("found", func(t *testing.T) {
		p := identity.FindByName(projects, "beta")
		if p == nil {
			t.Fatal("FindByName returned nil; want non-nil")
		}
		if p.GitEmail != "beta@example.com" {
			t.Errorf("GitEmail = %q; want %q", p.GitEmail, "beta@example.com")
		}
	})

	t.Run("not_found", func(t *testing.T) {
		p := identity.FindByName(projects, "gamma")
		if p != nil {
			t.Errorf("FindByName returned %+v; want nil", p)
		}
	})
}

// TestLoad_FileNotFound verifies that Load returns (nil, nil) when the
// projects.json file does not exist — a no-op, not an error.
func TestLoad_FileNotFound(t *testing.T) {
	tmp := t.TempDir()
	cfg, err := identity.Load(tmp)
	if err != nil {
		t.Fatalf("Load returned error %v; want nil", err)
	}
	if cfg != nil {
		t.Errorf("Load returned non-nil config; want nil when file absent")
	}
}

// TestLoad_ValidJSON verifies that Load parses a well-formed
// projects.json correctly.
func TestLoad_ValidJSON(t *testing.T) {
	tmp := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmp, "metadata"), 0o750); err != nil {
		t.Fatal(err)
	}

	want := identity.ProjectsConfig{
		Version: "1",
		Projects: []identity.Project{
			{
				Name:        "work",
				URLPatterns: []string{"github.com/work-org/*"},
				GitName:     "Work User",
				GitEmail:    "work@corp.example",
				SigningKey:   "ABCD1234",
			},
		},
	}
	data, err := json.Marshal(want)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "metadata", "projects.json"), data, 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := identity.Load(tmp)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if got == nil {
		t.Fatal("Load returned nil; want parsed config")
	}
	if got.Version != want.Version {
		t.Errorf("Version = %q; want %q", got.Version, want.Version)
	}
	if len(got.Projects) != 1 {
		t.Fatalf("len(Projects) = %d; want 1", len(got.Projects))
	}
	p := got.Projects[0]
	if p.Name != "work" || p.GitEmail != "work@corp.example" || p.SigningKey != "ABCD1234" {
		t.Errorf("project mismatch: %+v", p)
	}
}
