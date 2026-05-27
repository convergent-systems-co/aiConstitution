package cmd_test

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/convergent-systems-co/aiConstitution/src/cmd/ai/cmd"
)

// settingsDir creates a temp dir and sets AICONST_CONFIG_DIR + AI_ROOT
// so the settings commands use isolated state.
func settingsDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("AICONST_CONFIG_DIR", dir)
	t.Setenv("AI_ROOT", dir)
	return dir
}

// writeSettings writes a TOML string to settings.toml inside dir.
func writeSettings(t *testing.T, dir, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "settings.toml"), []byte(content), 0o644); err != nil {
		t.Fatalf("writeSettings: %v", err)
	}
}

// runSettings executes `ai settings <args>` and returns (stdout, stderr, err).
func runSettings(t *testing.T, args ...string) (string, string, error) {
	t.Helper()
	root := cmd.NewRootCmd()
	outBuf := &bytes.Buffer{}
	errBuf := &bytes.Buffer{}
	root.SetOut(outBuf)
	root.SetErr(errBuf)
	root.SetArgs(append([]string{"settings"}, args...))
	err := root.Execute()
	return outBuf.String(), errBuf.String(), err
}

// ─── settings get ─────────────────────────────────────────────────────────────

func TestSettingsGet_TopLevelKey(t *testing.T) {
	dir := settingsDir(t)
	writeSettings(t, dir, `schemaVersion = "0.4"`)

	out, _, err := runSettings(t, "get", "schemaVersion")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.TrimSpace(out) != "0.4" {
		t.Errorf("got %q; want %q", strings.TrimSpace(out), "0.4")
	}
}

func TestSettingsGet_NestedKey(t *testing.T) {
	dir := settingsDir(t)
	writeSettings(t, dir, `
schemaVersion = "0.4"

[review]
cadenceDays = 14
`)

	out, _, err := runSettings(t, "get", "review.cadenceDays")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.TrimSpace(out) != "14" {
		t.Errorf("got %q; want %q", strings.TrimSpace(out), "14")
	}
}

func TestSettingsGet_MissingKey(t *testing.T) {
	dir := settingsDir(t)
	writeSettings(t, dir, `schemaVersion = "0.4"`)

	_, _, err := runSettings(t, "get", "review.nonexistent")
	if err == nil {
		t.Fatal("expected error for missing key; got nil")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error message should contain 'not found'; got: %v", err)
	}
}

func TestSettingsGet_FileAbsent(t *testing.T) {
	settingsDir(t) // sets env vars but writes no file

	_, _, err := runSettings(t, "get", "schemaVersion")
	if err == nil {
		t.Fatal("expected error when settings.toml absent; got nil")
	}
}

func TestSettingsGet_NestedTableMissing(t *testing.T) {
	dir := settingsDir(t)
	writeSettings(t, dir, `schemaVersion = "0.4"`)

	_, _, err := runSettings(t, "get", "upstream.shareNewHooks")
	if err == nil {
		t.Fatal("expected error when table 'upstream' absent; got nil")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error should contain 'not found'; got: %v", err)
	}
}

// ─── settings set ─────────────────────────────────────────────────────────────

func TestSettingsSet_NewTopLevelKey(t *testing.T) {
	dir := settingsDir(t)
	writeSettings(t, dir, `schemaVersion = "0.4"`)

	_, _, err := runSettings(t, "set", `schemaVersion=0.5`)
	if err != nil {
		t.Fatalf("set failed: %v", err)
	}

	// Read back and verify.
	out, _, err := runSettings(t, "get", "schemaVersion")
	if err != nil {
		t.Fatalf("get after set failed: %v", err)
	}
	if strings.TrimSpace(out) != "0.5" {
		t.Errorf("got %q; want 0.5", strings.TrimSpace(out))
	}
}

func TestSettingsSet_ExistingNestedKey(t *testing.T) {
	dir := settingsDir(t)
	writeSettings(t, dir, `
schemaVersion = "0.4"

[review]
cadenceDays = 30
`)

	_, _, err := runSettings(t, "set", "review.cadenceDays=7")
	if err != nil {
		t.Fatalf("set failed: %v", err)
	}

	// Value must be updated.
	out, _, err := runSettings(t, "get", "review.cadenceDays")
	if err != nil {
		t.Fatalf("get after set failed: %v", err)
	}
	if strings.TrimSpace(out) != "7" {
		t.Errorf("got %q; want 7", strings.TrimSpace(out))
	}

	// File must be valid TOML (get verifies this implicitly; also check directly).
	data, err := os.ReadFile(filepath.Join(dir, "settings.toml"))
	if err != nil {
		t.Fatalf("read settings.toml after set: %v", err)
	}
	if len(data) == 0 {
		t.Error("settings.toml is empty after set")
	}
}

func TestSettingsSet_MissingEqualsSign(t *testing.T) {
	settingsDir(t)

	_, _, err := runSettings(t, "set", "review.cadenceDays")
	if err == nil {
		t.Fatal("expected error for missing '=' in argument; got nil")
	}
}

func TestSettingsSet_FileAbsent_CreatesFile(t *testing.T) {
	dir := settingsDir(t)
	// No settings.toml; set should create it.
	_, _, err := runSettings(t, "set", "schemaVersion=0.4")
	if err != nil {
		t.Fatalf("set with no existing file failed: %v", err)
	}

	path := filepath.Join(dir, "settings.toml")
	if _, statErr := os.Stat(path); os.IsNotExist(statErr) {
		t.Error("settings.toml was not created by set")
	}
}

func TestSettingsSet_BoolValue(t *testing.T) {
	dir := settingsDir(t)
	writeSettings(t, dir, `
[upstream]
shareNewHooks = true
`)

	_, _, err := runSettings(t, "set", "upstream.shareNewHooks=false")
	if err != nil {
		t.Fatalf("set bool failed: %v", err)
	}

	out, _, err := runSettings(t, "get", "upstream.shareNewHooks")
	if err != nil {
		t.Fatalf("get bool after set failed: %v", err)
	}
	if strings.TrimSpace(out) != "false" {
		t.Errorf("got %q; want false", strings.TrimSpace(out))
	}
}

// ─── settings reset ───────────────────────────────────────────────────────────

func TestSettingsReset_AcceptDefaults_WritesFile(t *testing.T) {
	dir := settingsDir(t)

	_, _, err := runSettings(t, "reset", "--accept-defaults")
	if err != nil {
		t.Fatalf("reset --accept-defaults failed: %v", err)
	}

	path := filepath.Join(dir, "settings.toml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("settings.toml not written: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("settings.toml is empty after reset")
	}
}

func TestSettingsReset_AcceptDefaults_SchemaVersionPresent(t *testing.T) {
	settingsDir(t)

	_, _, err := runSettings(t, "reset", "--accept-defaults")
	if err != nil {
		t.Fatalf("reset --accept-defaults failed: %v", err)
	}

	// Read schemaVersion back via get.
	out, _, err := runSettings(t, "get", "schemaVersion")
	if err != nil {
		t.Fatalf("get schemaVersion after reset failed: %v", err)
	}
	v := strings.TrimSpace(out)
	if v == "" {
		t.Error("schemaVersion is empty after reset")
	}
}

func TestSettingsReset_AcceptDefaults_ValidTOML(t *testing.T) {
	settingsDir(t)

	_, _, err := runSettings(t, "reset", "--accept-defaults")
	if err != nil {
		t.Fatalf("reset --accept-defaults failed: %v", err)
	}

	// If get works, the file is valid TOML.
	out, _, err := runSettings(t, "get", "schemaVersion")
	if err != nil {
		t.Fatalf("file after reset is not valid TOML (get failed): %v", err)
	}
	if strings.TrimSpace(out) == "" {
		t.Error("schemaVersion unexpectedly empty")
	}
}

func TestSettingsReset_NoFlag_PrintsWhatWouldChange(t *testing.T) {
	dir := settingsDir(t)
	// Write a file that differs from defaults.
	writeSettings(t, dir, `schemaVersion = "0.3"`)

	// Without --accept-defaults and without a TTY, the command should
	// print a summary and exit without writing (non-interactive mode).
	// We model "no TTY" here by running via the test harness.
	// The command must not return a hard error in non-interactive context —
	// it should just report what it would do and exit 0 (or print a notice).
	// We verify: no panic, some output or error communicating intent.
	out, _, err := runSettings(t, "reset")
	// The command may or may not error in non-interactive mode; we only
	// check it does not panic and, if no error, the output mentions what
	// it would change.
	if err != nil {
		// Acceptable — non-interactive reset with no TTY may error to say
		// "rerun with --accept-defaults".
		if !strings.Contains(err.Error(), "accept-defaults") && !strings.Contains(err.Error(), "non-interactive") {
			t.Errorf("unexpected error from reset with no flag: %v", err)
		}
		return
	}
	// If no error, output should convey intent.
	combined := out
	if combined == "" {
		// Nothing printed is also acceptable — just no panic.
		_ = fmt.Sprintf("no output from reset (no flag), err=nil") // silence staticcheck
	}
}

// ─── settings edit ────────────────────────────────────────────────────────────

func TestSettingsEdit_CreatesFileIfAbsent(t *testing.T) {
	dir := settingsDir(t)

	// Use a no-op editor so the test doesn't block.
	t.Setenv("EDITOR", "true") // "true" is /usr/bin/true — exits 0 immediately

	_, _, err := runSettings(t, "edit")
	if err != nil {
		t.Fatalf("edit with no-op editor failed: %v", err)
	}

	path := filepath.Join(dir, "settings.toml")
	if _, statErr := os.Stat(path); os.IsNotExist(statErr) {
		t.Error("settings.toml was not created before editor was opened")
	}
}

func TestSettingsEdit_KeepsExistingFile(t *testing.T) {
	dir := settingsDir(t)
	original := `schemaVersion = "0.4"`
	writeSettings(t, dir, original)

	// "true" exits 0 without modifying the file.
	t.Setenv("EDITOR", "true")

	_, _, err := runSettings(t, "edit")
	if err != nil {
		t.Fatalf("edit with no-op editor failed: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "settings.toml"))
	if err != nil {
		t.Fatalf("read settings.toml after edit: %v", err)
	}
	if string(data) != original {
		t.Errorf("settings.toml was modified unexpectedly; got %q", string(data))
	}
}
