package state_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/convergent-systems-co/aiConstitution/src/internal/paths"
	"github.com/convergent-systems-co/aiConstitution/src/internal/state"
)

func TestLoadModeReturnsZeroWhenFileMissing(t *testing.T) {
	cfg := t.TempDir()
	t.Setenv("AICONST_CONFIG_DIR", cfg)
	paths.SetOverrides("", "")

	m, err := state.LoadMode()
	if err != nil {
		t.Fatalf("LoadMode: %v", err)
	}
	if m.Name != "" {
		t.Errorf("want zero Mode, got Name=%q", m.Name)
	}
}

func TestSaveModeThenLoadModeRoundTrips(t *testing.T) {
	cfg := t.TempDir()
	t.Setenv("AICONST_CONFIG_DIR", cfg)
	paths.SetOverrides("", "")

	now := time.Now().UTC().Truncate(time.Second)
	want := state.Mode{
		Type:         "persona",
		Name:         "debugger",
		Source:       "shipped",
		ActivatedAt:  now,
		ActivatedVia: "cli",
	}
	if err := state.SaveMode(want); err != nil {
		t.Fatalf("SaveMode: %v", err)
	}

	if _, err := os.Stat(filepath.Join(cfg, "mode.json")); err != nil {
		t.Fatalf("mode.json not written: %v", err)
	}

	got, err := state.LoadMode()
	if err != nil {
		t.Fatalf("LoadMode: %v", err)
	}
	if got.Name != want.Name || got.Type != want.Type || got.Source != want.Source {
		t.Errorf("round-trip mismatch: got=%+v want=%+v", got, want)
	}
	if !got.ActivatedAt.Equal(want.ActivatedAt) {
		t.Errorf("ActivatedAt mismatch: got=%v want=%v", got.ActivatedAt, want.ActivatedAt)
	}
}

func TestSaveModeFileIsRestrictivelyPermissioned(t *testing.T) {
	cfg := t.TempDir()
	t.Setenv("AICONST_CONFIG_DIR", cfg)
	paths.SetOverrides("", "")

	if err := state.SaveMode(state.Mode{Name: "x"}); err != nil {
		t.Fatalf("SaveMode: %v", err)
	}
	st, err := os.Stat(filepath.Join(cfg, "mode.json"))
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	// 0o600 is the spec — user config file, not group/world readable.
	if perm := st.Mode().Perm(); perm != 0o600 {
		t.Errorf("mode.json perms = %o, want 0600", perm)
	}
}
