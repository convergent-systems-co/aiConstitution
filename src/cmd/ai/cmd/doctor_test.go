package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestDoctorTerminalNotifierFound verifies that runDoctor prints a [✓]
// marker when terminal-notifier is on PATH (macOS only).
func TestDoctorTerminalNotifierFound(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("terminal-notifier check is macOS-only")
	}

	// Create a fake terminal-notifier binary in a temp directory.
	tmpDir := t.TempDir()
	fakeNotifier := filepath.Join(tmpDir, "terminal-notifier")
	if err := os.WriteFile(fakeNotifier, []byte("#!/bin/sh\n"), 0755); err != nil {
		t.Fatalf("setup fake terminal-notifier: %v", err)
	}

	// Prepend our fake dir to PATH so it is found first.
	origPath := os.Getenv("PATH")
	t.Setenv("PATH", tmpDir+string(os.PathListSeparator)+origPath)

	// Set up HOME with a CLAUDE.md personas block so doctor check 2 passes.
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	claudeDir := filepath.Join(homeDir, ".claude")
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		t.Fatalf("setup claude dir: %v", err)
	}
	block := "<!-- ai:personas — managed by ai cli, do not edit manually -->\n<!-- /ai:personas -->\n"
	if err := os.WriteFile(filepath.Join(claudeDir, "CLAUDE.md"), []byte(block), 0o600); err != nil {
		t.Fatalf("setup CLAUDE.md: %v", err)
	}

	var out bytes.Buffer
	if err := runDoctor(&out, false, ""); err != nil {
		t.Fatalf("runDoctor returned error: %v", err)
	}

	got := out.String()
	if !strings.Contains(got, "[✓]") || !strings.Contains(got, "terminal-notifier") {
		t.Errorf("expected [✓] terminal-notifier line in output; got:\n%s", got)
	}
	if strings.Contains(got, "[⚠]") && strings.Contains(got, "terminal-notifier") {
		t.Errorf("got [⚠] for terminal-notifier but it should be found; output:\n%s", got)
	}
}

// TestDoctorTerminalNotifierMissing verifies that runDoctor prints a [⚠]
// marker and install hint when terminal-notifier is absent from PATH (macOS only).
func TestDoctorTerminalNotifierMissing(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("terminal-notifier check is macOS-only")
	}

	// Set PATH to a non-existent directory so no binaries are found.
	emptyDir := t.TempDir()
	t.Setenv("PATH", emptyDir)

	var out bytes.Buffer
	if err := runDoctor(&out, false, ""); err != nil {
		t.Fatalf("runDoctor returned error: %v", err)
	}

	got := out.String()
	if !strings.Contains(got, "[⚠]") {
		t.Errorf("expected [⚠] in output when terminal-notifier is missing; got:\n%s", got)
	}
	if !strings.Contains(got, "terminal-notifier") {
		t.Errorf("expected 'terminal-notifier' in warning line; got:\n%s", got)
	}
	if !strings.Contains(got, "brew install terminal-notifier") {
		t.Errorf("expected brew install hint in warning line; got:\n%s", got)
	}
}

// TestDoctorTerminalNotifierSkippedOnNonDarwin confirms that the
// terminal-notifier check does not appear in output on non-macOS platforms.
func TestDoctorTerminalNotifierSkippedOnNonDarwin(t *testing.T) {
	if runtime.GOOS == "darwin" {
		t.Skip("this test validates non-darwin behavior")
	}

	var out bytes.Buffer
	if err := runDoctor(&out, false, ""); err != nil {
		t.Fatalf("runDoctor returned error: %v", err)
	}

	got := out.String()
	// On non-darwin the check should not appear at all.
	if strings.Contains(got, "terminal-notifier") {
		t.Errorf("terminal-notifier check appeared on non-darwin platform; got:\n%s", got)
	}
}

func TestDoctorPersonasBlockMissing(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	claudeDir := filepath.Join(dir, ".claude")
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(claudeDir, "CLAUDE.md"), []byte("# Instructions\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	_ = runDoctor(&out, false, "")
	if !strings.Contains(out.String(), "personas block missing") {
		t.Errorf("expected personas block warning, got:\n%s", out.String())
	}
}

func TestDoctorPersonasBlockPresent(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	claudeDir := filepath.Join(dir, ".claude")
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := "<!-- ai:personas — managed by ai cli, do not edit manually -->\n<!-- /ai:personas -->\n"
	if err := os.WriteFile(filepath.Join(claudeDir, "CLAUDE.md"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	_ = runDoctor(&out, false, "")
	if !strings.Contains(out.String(), "[✓] CLAUDE.md personas block") {
		t.Errorf("expected personas block OK, got:\n%s", out.String())
	}
}

// ---------------------------------------------------------------------------
// #367 — ai doctor skills check
// ---------------------------------------------------------------------------

// setAIRoot temporarily overrides AI_ROOT for the duration of the test,
// which controls where skillsManifestDir() resolves to.
func setAIRoot(t *testing.T, root string) {
	t.Helper()
	t.Setenv("AI_ROOT", root)
}

func TestDoctorSkillsCheck_NoSkills(t *testing.T) {
	root := t.TempDir()
	setAIRoot(t, root)
	// No skills/ subdirectory — simulates fresh install with no skills.

	var out bytes.Buffer
	if err := checkInstalledSkills(&out); err != nil {
		t.Fatalf("checkInstalledSkills returned unexpected error: %v", err)
	}

	got := out.String()
	if !strings.Contains(got, "WARN") {
		t.Errorf("expected WARN when no skills installed; got:\n%s", got)
	}
	if !strings.Contains(got, "ai skills available") {
		t.Errorf("expected hint 'ai skills available' in output; got:\n%s", got)
	}
	if !strings.Contains(got, "ai skills install") {
		t.Errorf("expected hint 'ai skills install' in output; got:\n%s", got)
	}
}

func TestDoctorSkillsCheck_WithSkills(t *testing.T) {
	root := t.TempDir()
	setAIRoot(t, root)

	// Create two fake skill directories.
	slugs := []string{"commit", "review"}
	for _, slug := range slugs {
		dir := filepath.Join(root, "skills", slug)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	// Create a Claude skills dir with symlinks already present so the
	// unlinked-skills check does not produce a WARN in this count-focused test.
	claudeDir := t.TempDir()
	t.Setenv("CLAUDE_SKILLS_DIR", claudeDir)
	for _, slug := range slugs {
		skillDir := filepath.Join(root, "skills", slug)
		if err := os.Symlink(skillDir, filepath.Join(claudeDir, slug)); err != nil {
			t.Fatal(err)
		}
	}

	var out bytes.Buffer
	if err := checkInstalledSkills(&out); err != nil {
		t.Fatalf("checkInstalledSkills returned unexpected error: %v", err)
	}

	got := out.String()
	if !strings.Contains(got, "OK") {
		t.Errorf("expected OK when skills are installed; got:\n%s", got)
	}
	if !strings.Contains(got, "2") {
		t.Errorf("expected count '2' in output; got:\n%s", got)
	}
	// Must NOT emit WARN when skills are installed and linked.
	if strings.Contains(got, "WARN") {
		t.Errorf("unexpected WARN when skills are installed and linked; got:\n%s", got)
	}
}

// ---------------------------------------------------------------------------
// #371 — doctor detects unlinked skills
// ---------------------------------------------------------------------------

func TestDoctorDetectsUnlinkedSkills(t *testing.T) {
	root := t.TempDir()
	setAIRoot(t, root)

	// Create two skill dirs (no SKILL.md needed for count check).
	for _, slug := range []string{"alpha", "beta"} {
		if err := os.MkdirAll(filepath.Join(root, "skills", slug), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	// Create a Claude skills dir that has NO symlinks yet.
	claudeDir := t.TempDir()
	t.Setenv("CLAUDE_SKILLS_DIR", claudeDir)

	var out bytes.Buffer
	if err := checkInstalledSkills(&out); err != nil {
		t.Fatalf("checkInstalledSkills returned unexpected error: %v", err)
	}

	got := out.String()
	// Should still show OK for installed count...
	if !strings.Contains(got, "OK") {
		t.Errorf("expected OK for installed count; got:\n%s", got)
	}
	// ...but also warn that symlinks are missing.
	if !strings.Contains(got, "WARN") {
		t.Errorf("expected WARN about unlinked skills; got:\n%s", got)
	}
	if !strings.Contains(got, "ai skills link") {
		t.Errorf("expected hint 'ai skills link' in output; got:\n%s", got)
	}
}

func TestDoctorLinkedSkills_NoWarn(t *testing.T) {
	root := t.TempDir()
	setAIRoot(t, root)

	slugs := []string{"alpha", "beta"}
	claudeDir := t.TempDir()
	t.Setenv("CLAUDE_SKILLS_DIR", claudeDir)

	// Create skill dirs and corresponding symlinks in the Claude dir.
	for _, slug := range slugs {
		skillDir := filepath.Join(root, "skills", slug)
		if err := os.MkdirAll(skillDir, 0o755); err != nil {
			t.Fatal(err)
		}
		// Create a proper symlink so the check sees them as linked.
		if err := os.Symlink(skillDir, filepath.Join(claudeDir, slug)); err != nil {
			t.Fatal(err)
		}
	}

	var out bytes.Buffer
	if err := checkInstalledSkills(&out); err != nil {
		t.Fatalf("checkInstalledSkills returned unexpected error: %v", err)
	}

	got := out.String()
	if !strings.Contains(got, "OK") {
		t.Errorf("expected OK when all skills linked; got:\n%s", got)
	}
	// Must NOT warn about unlinked skills when all are linked.
	if strings.Contains(got, "ai skills link") {
		t.Errorf("unexpected 'ai skills link' hint when all skills are linked; got:\n%s", got)
	}
}

// ---------------------------------------------------------------------------
// #391 — checkHookWiring
// ---------------------------------------------------------------------------

// writeSettingsJSON writes a minimal settings.json that wires the given hook
// basenames via a PreToolUse event.
func writeSettingsJSON(t *testing.T, path string, hookedBasenames []string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir settings dir: %v", err)
	}
	// Build the hooks array entries.
	hookEntries := ""
	for i, name := range hookedBasenames {
		if i > 0 {
			hookEntries += ","
		}
		hookEntries += `{"command": "/Users/x/.ai/hooks/` + name + `"}`
	}
	content := `{
  "hooks": {
    "PreToolUse": [
      {"hooks": [` + hookEntries + `]}
    ]
  }
}`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write settings.json: %v", err)
	}
}

func TestCheckHookWiring_AllWired(t *testing.T) {
	aiRoot := t.TempDir()
	home := t.TempDir()

	// Install all 5 required hooks.
	hooksDir := filepath.Join(aiRoot, "hooks")
	if err := os.MkdirAll(hooksDir, 0o755); err != nil {
		t.Fatal(err)
	}
	required := []string{"audit.py", "branch-guard.py", "secret-block.py", "worktree-guard.py", "checkpoint-tick.py"}
	for _, h := range required {
		_ = os.WriteFile(filepath.Join(hooksDir, h), []byte("# hook"), 0o644)
	}

	// Wire all 5 in settings.json.
	settingsPath := filepath.Join(home, ".claude", "settings.json")
	writeSettingsJSON(t, settingsPath, required)

	var out bytes.Buffer
	checkHookWiring(&out, aiRoot, home)

	got := out.String()
	if !strings.Contains(got, "[✓] Hook wiring complete") {
		t.Errorf("expected all-wired success line; got:\n%s", got)
	}
	if strings.Contains(got, "[⚠]") {
		t.Errorf("unexpected warning when all hooks are wired; got:\n%s", got)
	}
}

func TestCheckHookWiring_PartiallyWired(t *testing.T) {
	aiRoot := t.TempDir()
	home := t.TempDir()

	// Install audit.py and branch-guard.py.
	hooksDir := filepath.Join(aiRoot, "hooks")
	if err := os.MkdirAll(hooksDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, h := range []string{"audit.py", "branch-guard.py"} {
		_ = os.WriteFile(filepath.Join(hooksDir, h), []byte("# hook"), 0o644)
	}

	// Wire only audit.py; branch-guard.py is installed but not wired.
	settingsPath := filepath.Join(home, ".claude", "settings.json")
	writeSettingsJSON(t, settingsPath, []string{"audit.py"})

	var out bytes.Buffer
	checkHookWiring(&out, aiRoot, home)

	got := out.String()
	if !strings.Contains(got, "[⚠]") || !strings.Contains(got, "branch-guard.py") {
		t.Errorf("expected warning about branch-guard.py not wired; got:\n%s", got)
	}
	if strings.Contains(got, "audit.py installed but not wired") {
		t.Errorf("audit.py is wired, should not appear in warnings; got:\n%s", got)
	}
}

func TestCheckHookWiring_NotInstalled(t *testing.T) {
	aiRoot := t.TempDir()
	home := t.TempDir()

	// hooksDir exists but is empty (nothing installed).
	hooksDir := filepath.Join(aiRoot, "hooks")
	if err := os.MkdirAll(hooksDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Wire nothing (empty hooks array).
	settingsPath := filepath.Join(home, ".claude", "settings.json")
	writeSettingsJSON(t, settingsPath, nil)

	var out bytes.Buffer
	checkHookWiring(&out, aiRoot, home)

	got := out.String()
	// No hooks installed → no "not wired" warnings (not installed is a separate concern).
	if strings.Contains(got, "installed but not wired") {
		t.Errorf("got unexpected 'installed but not wired' warning when nothing is installed; got:\n%s", got)
	}
	// Should report all-OK since there's nothing installed to check wiring for.
	if !strings.Contains(got, "[✓] Hook wiring complete") {
		t.Errorf("expected wiring-complete when nothing is installed; got:\n%s", got)
	}
}

func TestCheckHookWiring_NoSettings(t *testing.T) {
	aiRoot := t.TempDir()
	home := t.TempDir()

	// Install all required hooks but no settings.json.
	hooksDir := filepath.Join(aiRoot, "hooks")
	if err := os.MkdirAll(hooksDir, 0o755); err != nil {
		t.Fatal(err)
	}
	required := []string{"audit.py", "branch-guard.py", "secret-block.py", "worktree-guard.py", "checkpoint-tick.py"}
	for _, h := range required {
		_ = os.WriteFile(filepath.Join(hooksDir, h), []byte("# hook"), 0o644)
	}
	// settings.json is intentionally absent.

	var out bytes.Buffer
	checkHookWiring(&out, aiRoot, home)

	got := out.String()
	// All installed hooks should appear as not wired.
	for _, h := range required {
		if !strings.Contains(got, h) {
			t.Errorf("expected warning mentioning %s when settings.json absent; got:\n%s", h, got)
		}
	}
}
