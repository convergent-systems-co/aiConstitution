package cmd

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCloneAndHooksRepoInstallUseSameHookSet(t *testing.T) {
	cloneRepo := initGitRepo(t)
	hooksRepo := initGitRepo(t)

	var cloneOut bytes.Buffer
	if err := installPrecommitHook(&cloneOut, cloneRepo); err != nil {
		t.Fatalf("install clone hooks: %v", err)
	}
	var hooksOut bytes.Buffer
	if err := installRepoPrecommit(hooksRepo, &hooksOut); err != nil {
		t.Fatalf("install repo hooks: %v", err)
	}

	cloneHooks := readHookSet(t, cloneRepo)
	repoHooks := readHookSet(t, hooksRepo)
	if len(cloneHooks) != len(repoHooks) {
		t.Fatalf("hook count mismatch: clone=%d repo=%d", len(cloneHooks), len(repoHooks))
	}
	for name, cloneBody := range cloneHooks {
		repoBody, ok := repoHooks[name]
		if !ok {
			t.Fatalf("repo install missing hook %s", name)
		}
		if normalizeHookSourceComment(cloneBody) != normalizeHookSourceComment(repoBody) {
			t.Fatalf("hook %s differs between clone and hooks install --repo", name)
		}
	}
}

func TestRepoManagedHooksInstallFullCrossPlatformSet(t *testing.T) {
	repo := initGitRepo(t)
	var out bytes.Buffer
	if err := installRepoManagedGitHooks(repo, "test", &out); err != nil {
		t.Fatalf("install hooks: %v", err)
	}

	for _, hook := range repoManagedGitHookTemplates {
		path := filepath.Join(repo, ".git", "hooks", hook.name)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", hook.name, err)
		}
		body := string(data)
		if hook.name == "pre-commit" || hook.name == "pre-push" {
			if !bytes.Contains(data, []byte("ai guard --git-hook")) {
				t.Fatalf("%s should call ai guard --git-hook:\n%s", hook.name, body)
			}
		}
		if hook.name == "commit-msg" && !bytes.Contains(data, []byte("Co-authored-by")) {
			t.Fatalf("commit-msg should enforce AI-authored trailers:\n%s", body)
		}
	}
}

func initGitRepo(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	if out, err := exec.Command("git", "init", repo).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}
	return repo
}

func readHookSet(t *testing.T, repo string) map[string]string {
	t.Helper()
	hooks := make(map[string]string)
	for _, hook := range repoManagedGitHookTemplates {
		path := filepath.Join(repo, ".git", "hooks", hook.name)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", hook.name, err)
		}
		hooks[hook.name] = string(data)
	}
	return hooks
}

func normalizeHookSourceComment(body string) string {
	lines := strings.Split(body, "\n")
	kept := lines[:0]
	for _, line := range lines {
		if strings.HasPrefix(line, "# Installed by `") {
			continue
		}
		kept = append(kept, line)
	}
	return strings.Join(kept, "\n")
}
