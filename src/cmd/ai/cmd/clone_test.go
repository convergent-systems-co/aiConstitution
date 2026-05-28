package cmd_test

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	cmd "github.com/convergent-systems-co/aiConstitution/src/cmd/ai/cmd"
	"github.com/convergent-systems-co/aiConstitution/src/cmd/ai/internal/identity"
)

// TestClone_IdentityApplied verifies that applyIdentityRouting writes the
// correct git user.name and user.email into a freshly-initialised repo
// when a matching project entry exists in projects.json.
func TestClone_IdentityApplied(t *testing.T) {
	// Create a temp dir that acts as the aiConstitution config dir.
	configDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(configDir, "metadata"), 0o750); err != nil {
		t.Fatal(err)
	}

	// Write a projects.json with a pattern that matches our test URL.
	cfg := identity.ProjectsConfig{
		Version: "1",
		Projects: []identity.Project{
			{
				Name:        "work",
				URLPatterns: []string{"github.com/myorg/*"},
				GitName:     "Work User",
				GitEmail:    "work@corp.example",
			},
		},
	}
	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "metadata", "projects.json"), data, 0o600); err != nil {
		t.Fatal(err)
	}

	// Point the config dir override at our temp config.
	t.Setenv("AI_CONFIG_DIR", configDir)

	// Create a temp dir to simulate the cloned repo (git init).
	repoDir := t.TempDir()
	if out, err := exec.Command("git", "init", repoDir).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}

	var buf bytes.Buffer
	err = cmd.ApplyIdentityRoutingForTest(&buf, "https://github.com/myorg/repo.git", repoDir, "")
	if err != nil {
		t.Fatalf("applyIdentityRouting returned error: %v", err)
	}

	// Verify output message.
	if !strings.Contains(buf.String(), "work@corp.example") {
		t.Errorf("output %q does not mention the expected email", buf.String())
	}

	// Verify git config was written.
	emailOut, err := exec.Command("git", "-C", repoDir, "config", "user.email").Output()
	if err != nil {
		t.Fatalf("git config user.email: %v", err)
	}
	if got := strings.TrimSpace(string(emailOut)); got != "work@corp.example" {
		t.Errorf("user.email = %q; want %q", got, "work@corp.example")
	}

	nameOut, err := exec.Command("git", "-C", repoDir, "config", "user.name").Output()
	if err != nil {
		t.Fatalf("git config user.name: %v", err)
	}
	if got := strings.TrimSpace(string(nameOut)); got != "Work User" {
		t.Errorf("user.name = %q; want %q", got, "Work User")
	}
}

// TestClone_IdentityApplied_ForceName verifies that --identity=<name>
// bypasses URL matching and applies the named identity directly.
func TestClone_IdentityApplied_ForceName(t *testing.T) {
	configDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(configDir, "metadata"), 0o750); err != nil {
		t.Fatal(err)
	}

	cfg := identity.ProjectsConfig{
		Version: "1",
		Projects: []identity.Project{
			{
				Name:        "personal",
				URLPatterns: []string{"github.com/personal-org/*"},
				GitName:     "Personal User",
				GitEmail:    "me@personal.example",
			},
		},
	}
	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "metadata", "projects.json"), data, 0o600); err != nil {
		t.Fatal(err)
	}

	t.Setenv("AI_CONFIG_DIR", configDir)

	repoDir := t.TempDir()
	if out, err := exec.Command("git", "init", repoDir).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}

	var buf bytes.Buffer
	// URL does NOT match the pattern, but forceName overrides pattern matching.
	err = cmd.ApplyIdentityRoutingForTest(&buf, "https://github.com/other-org/repo.git", repoDir, "personal")
	if err != nil {
		t.Fatalf("applyIdentityRouting returned error: %v", err)
	}

	emailOut, err := exec.Command("git", "-C", repoDir, "config", "user.email").Output()
	if err != nil {
		t.Fatalf("git config user.email: %v", err)
	}
	if got := strings.TrimSpace(string(emailOut)); got != "me@personal.example" {
		t.Errorf("user.email = %q; want %q", got, "me@personal.example")
	}
}

// TestClone_IdentityApplied_SigningKey verifies that a signing key is
// written to git config when present in the project definition.
func TestClone_IdentityApplied_SigningKey(t *testing.T) {
	configDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(configDir, "metadata"), 0o750); err != nil {
		t.Fatal(err)
	}

	cfg := identity.ProjectsConfig{
		Version: "1",
		Projects: []identity.Project{
			{
				Name:        "signed",
				URLPatterns: []string{"github.com/secure-org/*"},
				GitName:     "Secure User",
				GitEmail:    "secure@example.com",
				SigningKey:   "DEADBEEF",
			},
		},
	}
	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "metadata", "projects.json"), data, 0o600); err != nil {
		t.Fatal(err)
	}

	t.Setenv("AI_CONFIG_DIR", configDir)

	repoDir := t.TempDir()
	if out, err := exec.Command("git", "init", repoDir).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}

	var buf bytes.Buffer
	if err := cmd.ApplyIdentityRoutingForTest(&buf, "https://github.com/secure-org/repo.git", repoDir, ""); err != nil {
		t.Fatalf("applyIdentityRouting: %v", err)
	}

	signingOut, err := exec.Command("git", "-C", repoDir, "config", "user.signingkey").Output()
	if err != nil {
		t.Fatalf("git config user.signingkey: %v", err)
	}
	if got := strings.TrimSpace(string(signingOut)); got != "DEADBEEF" {
		t.Errorf("user.signingkey = %q; want %q", got, "DEADBEEF")
	}
}

// TestClone_IdentityApplied_NoMatch verifies that a URL with no matching
// project is a silent no-op (no error, no output, no git config change).
func TestClone_IdentityApplied_NoMatch(t *testing.T) {
	configDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(configDir, "metadata"), 0o750); err != nil {
		t.Fatal(err)
	}

	cfg := identity.ProjectsConfig{
		Version:  "1",
		Projects: []identity.Project{{Name: "work", URLPatterns: []string{"github.com/work-org/*"}, GitEmail: "w@w.example"}},
	}
	data, _ := json.Marshal(cfg)
	if err := os.WriteFile(filepath.Join(configDir, "metadata", "projects.json"), data, 0o600); err != nil {
		t.Fatal(err)
	}

	t.Setenv("AI_CONFIG_DIR", configDir)

	repoDir := t.TempDir()
	if out, err := exec.Command("git", "init", repoDir).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}

	var buf bytes.Buffer
	if err := cmd.ApplyIdentityRoutingForTest(&buf, "https://github.com/other-org/repo.git", repoDir, ""); err != nil {
		t.Errorf("expected nil error for no-match case; got %v", err)
	}
	if buf.Len() > 0 {
		t.Errorf("expected no output for no-match case; got %q", buf.String())
	}
}

// TestClone_IdentityApplied_NoProjectsJSON verifies that a missing
// projects.json is a silent no-op.
func TestClone_IdentityApplied_NoProjectsJSON(t *testing.T) {
	configDir := t.TempDir() // no metadata/projects.json written
	t.Setenv("AI_CONFIG_DIR", configDir)

	repoDir := t.TempDir()
	if out, err := exec.Command("git", "init", repoDir).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}

	var buf bytes.Buffer
	if err := cmd.ApplyIdentityRoutingForTest(&buf, "https://github.com/myorg/repo.git", repoDir, ""); err != nil {
		t.Errorf("expected nil error when no projects.json; got %v", err)
	}
}

// TestClone_IdentityApplied_ForceNameNotFound verifies that forcing an
// unknown identity name returns an error.
func TestClone_IdentityApplied_ForceNameNotFound(t *testing.T) {
	configDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(configDir, "metadata"), 0o750); err != nil {
		t.Fatal(err)
	}

	cfg := identity.ProjectsConfig{
		Version:  "1",
		Projects: []identity.Project{{Name: "alpha", GitEmail: "a@a.example"}},
	}
	data, _ := json.Marshal(cfg)
	if err := os.WriteFile(filepath.Join(configDir, "metadata", "projects.json"), data, 0o600); err != nil {
		t.Fatal(err)
	}

	t.Setenv("AI_CONFIG_DIR", configDir)

	repoDir := t.TempDir()
	if out, err := exec.Command("git", "init", repoDir).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}

	var buf bytes.Buffer
	err := cmd.ApplyIdentityRoutingForTest(&buf, "https://github.com/myorg/repo.git", repoDir, "nonexistent")
	if err == nil {
		t.Error("expected error for unknown identity name; got nil")
	}
	if !strings.Contains(err.Error(), "nonexistent") {
		t.Errorf("error %q does not mention the unknown name", err.Error())
	}
}
