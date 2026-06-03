package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRepoHookTemplatesUseInstalledGuardCommand(t *testing.T) {
	repoRoot := findRepoRoot(t)
	hooks := []string{
		"pre-commit",
		"pre-push",
		"pre-commit.ps1",
		"pre-push.ps1",
	}
	for _, hook := range hooks {
		path := filepath.Join(repoRoot, "scripts", "git-hooks", hook)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", hook, err)
		}
		body := string(data)
		if strings.Contains(body, "go run") {
			t.Fatalf("%s must not depend on go run:\n%s", hook, body)
		}
		if !strings.Contains(body, "ai guard --git-hook") {
			t.Fatalf("%s should call ai guard --git-hook:\n%s", hook, body)
		}
	}
}

func findRepoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find repo root")
		}
		dir = parent
	}
}
