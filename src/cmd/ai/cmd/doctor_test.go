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
	for _, slug := range []string{"commit", "review"} {
		dir := filepath.Join(root, "skills", slug)
		if err := os.MkdirAll(dir, 0o755); err != nil {
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
	// Must NOT emit WARN when skills are present.
	if strings.Contains(got, "WARN") {
		t.Errorf("unexpected WARN when skills are installed; got:\n%s", got)
	}
}
