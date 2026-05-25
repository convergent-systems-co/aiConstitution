package cmd_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// changeDir temporarily changes the process working directory to dir and
// restores it on test cleanup. Required because init detects stack from cwd.
func changeDir(t *testing.T, dir string) {
	t.Helper()
	original, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir to %s: %v", dir, err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(original); err != nil {
			t.Logf("WARNING: could not restore cwd to %s: %v", original, err)
		}
	})
}

// runInit runs `ai init [args...]` with the given aiRoot.
func runInit(t *testing.T, aiRoot string, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	return runRootCmd(t, aiRoot, append([]string{"init"}, args...)...)
}

// Test_init_project_yaml_go verifies project.yaml is written with Go stack.
func Test_init_project_yaml_go(t *testing.T) {
	dir := t.TempDir()
	// Plant go.mod to trigger Go detection.
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example\ngo 1.22\n"), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}
	changeDir(t, dir)

	aiRoot := t.TempDir()
	_, _, err := runInit(t, aiRoot)
	if err != nil {
		t.Fatalf("init failed: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "project.yaml"))
	if err != nil {
		t.Fatalf("project.yaml not written: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "stack: go") {
		t.Errorf("expected stack: go; got:\n%s", content)
	}
	if !strings.Contains(content, "go test ./...") {
		t.Errorf("expected test_command: go test ./...; got:\n%s", content)
	}
	if !strings.Contains(content, "name: "+filepath.Base(dir)) {
		t.Errorf("expected name: %s; got:\n%s", filepath.Base(dir), content)
	}
}

// Test_init_project_yaml_node verifies Node stack detection.
func Test_init_project_yaml_node(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"name":"test"}`), 0o644); err != nil {
		t.Fatalf("write package.json: %v", err)
	}
	changeDir(t, dir)

	aiRoot := t.TempDir()
	_, _, err := runInit(t, aiRoot)
	if err != nil {
		t.Fatalf("init failed: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "project.yaml"))
	if err != nil {
		t.Fatalf("project.yaml not written: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "stack: node") {
		t.Errorf("expected stack: node; got:\n%s", content)
	}
	if !strings.Contains(content, "npm test") {
		t.Errorf("expected npm test; got:\n%s", content)
	}
}

// Test_init_project_yaml_python verifies Python stack detection (pyproject.toml).
func Test_init_project_yaml_python(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "pyproject.toml"), []byte("[project]\n"), 0o644); err != nil {
		t.Fatalf("write pyproject.toml: %v", err)
	}
	changeDir(t, dir)

	aiRoot := t.TempDir()
	_, _, err := runInit(t, aiRoot)
	if err != nil {
		t.Fatalf("init failed: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "project.yaml"))
	if err != nil {
		t.Fatalf("project.yaml not written: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "stack: python") {
		t.Errorf("expected stack: python; got:\n%s", content)
	}
	if !strings.Contains(content, "pytest") {
		t.Errorf("expected pytest; got:\n%s", content)
	}
}

// Test_init_project_yaml_requirements verifies Python detection via requirements.txt.
func Test_init_project_yaml_requirements(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "requirements.txt"), []byte("flask\n"), 0o644); err != nil {
		t.Fatalf("write requirements.txt: %v", err)
	}
	changeDir(t, dir)

	aiRoot := t.TempDir()
	_, _, err := runInit(t, aiRoot)
	if err != nil {
		t.Fatalf("init failed: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "project.yaml"))
	if err != nil {
		t.Fatalf("project.yaml not written: %v", err)
	}
	if !strings.Contains(string(data), "stack: python") {
		t.Errorf("expected stack: python from requirements.txt; got:\n%s", data)
	}
}

// Test_init_project_yaml_unknown verifies unknown stack produces a TODO comment.
func Test_init_project_yaml_unknown(t *testing.T) {
	dir := t.TempDir()
	changeDir(t, dir)

	aiRoot := t.TempDir()
	_, _, err := runInit(t, aiRoot)
	if err != nil {
		t.Fatalf("init failed: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "project.yaml"))
	if err != nil {
		t.Fatalf("project.yaml not written: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "unknown") {
		t.Errorf("expected stack: unknown; got:\n%s", content)
	}
	if !strings.Contains(content, "TODO") {
		t.Errorf("expected TODO comment for test_command; got:\n%s", content)
	}
}

// Test_init_idempotent verifies that a second `ai init` prints a message and
// exits cleanly without overwriting project.yaml.
func Test_init_idempotent(t *testing.T) {
	dir := t.TempDir()
	changeDir(t, dir)

	// Write a hand-crafted project.yaml.
	original := "name: manual\nstack: manual\n"
	if err := os.WriteFile(filepath.Join(dir, "project.yaml"), []byte(original), 0o644); err != nil {
		t.Fatalf("write project.yaml: %v", err)
	}

	aiRoot := t.TempDir()
	stdout, _, err := runInit(t, aiRoot)
	if err != nil {
		t.Fatalf("init failed on second run: %v", err)
	}
	if !strings.Contains(stdout, "already exists") {
		t.Errorf("expected 'already exists' message; got: %q", stdout)
	}
	// Content must not change.
	data, err := os.ReadFile(filepath.Join(dir, "project.yaml"))
	if err != nil {
		t.Fatalf("read project.yaml: %v", err)
	}
	if string(data) != original {
		t.Errorf("project.yaml was modified; expected %q, got %q", original, string(data))
	}
}

// Test_init_writes_claude_md verifies .claude/CLAUDE.md is written with the
// @-include line.
func Test_init_writes_claude_md(t *testing.T) {
	dir := t.TempDir()
	changeDir(t, dir)

	aiRoot := t.TempDir()
	_, _, err := runInit(t, aiRoot)
	if err != nil {
		t.Fatalf("init failed: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, ".claude", "CLAUDE.md"))
	if err != nil {
		t.Fatalf(".claude/CLAUDE.md not written: %v", err)
	}
	if !strings.Contains(string(data), "@~/.ai/Constitution.md") {
		t.Errorf("expected @~/.ai/Constitution.md in .claude/CLAUDE.md; got:\n%s", data)
	}
}

// Test_init_claude_md_idempotent verifies that an existing .claude/CLAUDE.md
// is not duplicated on re-run.
func Test_init_claude_md_idempotent(t *testing.T) {
	dir := t.TempDir()
	changeDir(t, dir)

	// Pre-create with the @-include already in it.
	if err := os.MkdirAll(filepath.Join(dir, ".claude"), 0o750); err != nil {
		t.Fatalf("mkdir .claude: %v", err)
	}
	existing := "@~/.ai/Constitution.md\n"
	if err := os.WriteFile(filepath.Join(dir, ".claude", "CLAUDE.md"), []byte(existing), 0o644); err != nil {
		t.Fatalf("write CLAUDE.md: %v", err)
	}

	aiRoot := t.TempDir()
	_, _, err := runInit(t, aiRoot)
	if err != nil {
		t.Fatalf("init failed: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, ".claude", "CLAUDE.md"))
	if err != nil {
		t.Fatalf("read CLAUDE.md: %v", err)
	}
	count := strings.Count(string(data), "@~/.ai/Constitution.md")
	if count != 1 {
		t.Errorf("@-include should appear exactly once; appeared %d times in:\n%s", count, data)
	}
}

// Test_init_dry_run verifies that --dry-run prints actions without writing files.
func Test_init_dry_run(t *testing.T) {
	dir := t.TempDir()
	changeDir(t, dir)

	aiRoot := t.TempDir()
	stdout, _, err := runInit(t, aiRoot, "--dry-run")
	if err != nil {
		t.Fatalf("init --dry-run failed: %v", err)
	}

	// Something must have been printed.
	if strings.TrimSpace(stdout) == "" {
		t.Error("expected dry-run output, got empty stdout")
	}
	if !strings.Contains(stdout, "dry-run") && !strings.Contains(stdout, "would") {
		t.Errorf("expected dry-run indication in output; got: %q", stdout)
	}

	// project.yaml must NOT exist.
	if _, err := os.Stat(filepath.Join(dir, "project.yaml")); err == nil {
		t.Error("project.yaml should not have been written in --dry-run mode")
	}
	// .claude/CLAUDE.md must NOT exist.
	if _, err := os.Stat(filepath.Join(dir, ".claude", "CLAUDE.md")); err == nil {
		t.Error(".claude/CLAUDE.md should not have been written in --dry-run mode")
	}
}

// Test_init_writes_copilot_instructions verifies .github/copilot-instructions.md.
func Test_init_writes_copilot_instructions(t *testing.T) {
	dir := t.TempDir()
	changeDir(t, dir)

	aiRoot := t.TempDir()
	_, _, err := runInit(t, aiRoot)
	if err != nil {
		t.Fatalf("init failed: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, ".github", "copilot-instructions.md"))
	if err != nil {
		t.Fatalf(".github/copilot-instructions.md not written: %v", err)
	}
	if !strings.Contains(string(data), "@~/.ai/Constitution.runtime.md") {
		t.Errorf("expected @-include in copilot-instructions.md; got:\n%s", data)
	}
}
