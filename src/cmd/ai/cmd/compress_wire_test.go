package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestRewireCopilotToCompact_CreatesSymlink verifies that rewireCopilotToCompact
// creates the instructions symlink when ~/.copilot/ exists.
func TestRewireCopilotToCompact_CreatesSymlink(t *testing.T) {
	tmp := t.TempDir()
	aiRoot := filepath.Join(tmp, ".ai")
	home := filepath.Join(tmp, "home")

	// Create compact file.
	if err := os.MkdirAll(aiRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	compact := filepath.Join(aiRoot, "Constitution.compact.md")
	if err := os.WriteFile(compact, []byte("# compact"), 0o600); err != nil {
		t.Fatal(err)
	}

	// Create ~/.copilot/ to signal Copilot is installed.
	if err := os.MkdirAll(filepath.Join(home, ".copilot"), 0o750); err != nil {
		t.Fatal(err)
	}

	if err := rewireCopilotToCompact(home, aiRoot); err != nil {
		t.Fatalf("rewireCopilotToCompact() error = %v", err)
	}

	link := filepath.Join(home, ".copilot", "instructions", "constitution.md")
	if _, err := os.Lstat(link); err != nil {
		t.Errorf("symlink not created at %s: %v", link, err)
	}
}

// TestRewireCopilotToCompact_NoCopilotDir returns error when ~/.copilot/ absent.
func TestRewireCopilotToCompact_NoCopilotDir(t *testing.T) {
	tmp := t.TempDir()
	aiRoot := filepath.Join(tmp, ".ai")
	home := filepath.Join(tmp, "home")
	_ = os.MkdirAll(aiRoot, 0o755)
	_ = os.WriteFile(filepath.Join(aiRoot, "Constitution.compact.md"), []byte("# compact"), 0o600)

	err := rewireCopilotToCompact(home, aiRoot)
	if err == nil {
		t.Error("expected error when ~/.copilot/ absent, got nil")
	}
}

// TestRewireCodexToCompact_CreatesAGENTSMD verifies that rewireCodexToCompact
// creates AGENTS.md when none exists.
func TestRewireCodexToCompact_CreatesAGENTSMD(t *testing.T) {
	tmp := t.TempDir()
	aiRoot := filepath.Join(tmp, ".ai")

	if err := rewireCodexToCompact(tmp, aiRoot); err != nil {
		t.Fatalf("rewireCodexToCompact() error = %v", err)
	}

	data, err := os.ReadFile(filepath.Join(tmp, "AGENTS.md"))
	if err != nil {
		t.Fatalf("AGENTS.md not created: %v", err)
	}
	if !strings.Contains(string(data), "@~/.ai/Constitution.compact.md") {
		t.Errorf("AGENTS.md missing compact include:\n%s", data)
	}
}

// TestRewireCodexToCompact_UpdatesFullToCompact replaces the full include with compact.
func TestRewireCodexToCompact_UpdatesFullToCompact(t *testing.T) {
	tmp := t.TempDir()
	aiRoot := filepath.Join(tmp, ".ai")
	agentsPath := filepath.Join(tmp, "AGENTS.md")

	_ = os.WriteFile(agentsPath, []byte("# Agents\n@~/.ai/Constitution.md\n"), 0o644)

	if err := rewireCodexToCompact(tmp, aiRoot); err != nil {
		t.Fatalf("rewireCodexToCompact() error = %v", err)
	}

	data, _ := os.ReadFile(agentsPath)
	content := string(data)
	if strings.Contains(content, "@~/.ai/Constitution.md") && !strings.Contains(content, "@~/.ai/Constitution.compact.md") {
		t.Errorf("full include not replaced with compact:\n%s", content)
	}
	if !strings.Contains(content, "@~/.ai/Constitution.compact.md") {
		t.Errorf("compact include missing after update:\n%s", content)
	}
}

// TestRewireCodexToCompact_Idempotent verifies no duplicate include on re-run.
func TestRewireCodexToCompact_Idempotent(t *testing.T) {
	tmp := t.TempDir()
	aiRoot := filepath.Join(tmp, ".ai")
	agentsPath := filepath.Join(tmp, "AGENTS.md")

	_ = os.WriteFile(agentsPath, []byte("# Agents\n@~/.ai/Constitution.compact.md\n"), 0o644)

	if err := rewireCodexToCompact(tmp, aiRoot); err != nil {
		t.Fatalf("rewireCodexToCompact() error = %v", err)
	}

	data, _ := os.ReadFile(agentsPath)
	count := strings.Count(string(data), "@~/.ai/Constitution.compact.md")
	if count != 1 {
		t.Errorf("expected exactly 1 compact include, got %d:\n%s", count, data)
	}
}

// TestCheckWiring_AllMissing verifies checkWiring reports all tools as unwired.
func TestCheckWiring_AllMissing(t *testing.T) {
	tmp := t.TempDir()
	home := filepath.Join(tmp, "home")
	cwd := filepath.Join(tmp, "repo")
	aiRoot := filepath.Join(tmp, ".ai")
	_ = os.MkdirAll(home, 0o755)
	_ = os.MkdirAll(cwd, 0o755)

	var buf bytes.Buffer
	checkWiring(&buf, home, cwd, aiRoot)
	out := buf.String()

	for _, tool := range []string{"Claude Code", "Copilot", "Codex"} {
		if !strings.Contains(out, "[ ] "+tool) {
			t.Errorf("expected unwired status for %s, got:\n%s", tool, out)
		}
	}
}

// TestCheckWiring_ClaudeWired verifies checkWiring reports Claude as wired when
// CLAUDE.md contains the compact include.
func TestCheckWiring_ClaudeWired(t *testing.T) {
	tmp := t.TempDir()
	home := filepath.Join(tmp, "home")
	cwd := filepath.Join(tmp, "repo")
	aiRoot := filepath.Join(tmp, ".ai")

	claudeDir := filepath.Join(home, ".claude")
	_ = os.MkdirAll(claudeDir, 0o755)
	_ = os.MkdirAll(cwd, 0o755)
	_ = os.WriteFile(
		filepath.Join(claudeDir, "CLAUDE.md"),
		[]byte("@~/.ai/Constitution.compact.md\n"),
		0o640,
	)

	var buf bytes.Buffer
	checkWiring(&buf, home, cwd, aiRoot)
	out := buf.String()

	if !strings.Contains(out, "[✓] Claude Code") {
		t.Errorf("expected Claude Code wired, got:\n%s", out)
	}
}
