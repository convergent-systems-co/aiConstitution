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
