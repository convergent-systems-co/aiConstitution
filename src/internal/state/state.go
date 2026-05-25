// Package state manages the per-machine JSON state files at
// ~/.config/aiConstitution/:
//
//   mode.json   — active mode (SPEC.md §7.8.3)
//   state.json  — consolidated machine state: last-seen-version,
//                 last-review-timestamp, last-drill-timestamp,
//                 snooze targets (SPEC.md §7.6)
//
// Both files are JSON (not TOML) because they are written by the
// binary, read by the binary, never hand-edited. JSON's stricter
// syntax and lack of formatting affordances (comments, multiline
// strings, inline tables) are virtues for machine-only state.
package state

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/convergent-systems-co/aiConstitution/src/internal/paths"
)

// Mode is the on-disk shape of mode.json. Per SPEC.md §7.8.3.
type Mode struct {
	Type          string         `json:"type"` // "profile" | "persona"
	Name          string         `json:"name"`
	Version       string         `json:"version"`
	Source        string         `json:"source"` // "atom" | "user-draft" | "shipped"
	ActivatedAt   time.Time      `json:"activatedAt"`
	ActivatedVia  string         `json:"activatedVia"` // "cli" | "settings-default" | "restore"
	ComposedAtoms []ComposedAtom `json:"composedAtoms"`
	SourcePath    string         `json:"sourcePath"`
	Exclusive     bool           `json:"exclusive,omitempty"`
}

// ComposedAtom is one entry in a profile's composes[] list, resolved
// to a concrete version.
type ComposedAtom struct {
	Atom    string `json:"atom"`
	Version string `json:"version"`
	Source  string `json:"source"` // "atom" | "user-draft"
}

// State is the on-disk shape of state.json. Per SPEC.md §7.6 "second
// machine-state file" — consolidated marker file.
type State struct {
	LastSeenVersion     string    `json:"lastSeenVersion"`
	LastReviewTimestamp time.Time `json:"lastReviewTimestamp,omitempty"`
	LastDrillTimestamp  time.Time `json:"lastDrillTimestamp,omitempty"`
	LastReviewSnooze    time.Time `json:"lastReviewSnooze,omitempty"`
	UpdatePending       bool      `json:"updatePending,omitempty"`
	UpdateSource        string    `json:"updateSource,omitempty"` // "brew" | "scoop" | "winget" | "git"
}

// LoadMode reads ~/.config/aiConstitution/mode.json. Returns
// (Mode{}, nil) if the file does not yet exist (the "no mode" case).
// Any other read or unmarshal error is surfaced to the caller.
func LoadMode() (Mode, error) {
	data, err := os.ReadFile(filepath.Clean(paths.ModeJSON()))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Mode{}, nil
		}
		return Mode{}, err
	}
	var m Mode
	if err := json.Unmarshal(data, &m); err != nil {
		return Mode{}, err
	}
	return m, nil
}

// SaveMode writes mode.json atomically (write-to-temp then rename) with
// 0o600 permissions. The parent ConfigDir is created if missing.
func SaveMode(m Mode) error {
	path := paths.ModeJSON()
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".mode-*.json")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	// Best-effort cleanup of the temp file if rename never landed.
	defer func() { _ = os.Remove(tmpName) }()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpName, 0o600); err != nil { //nolint:gosec // G302: user config file
		return err
	}
	return os.Rename(tmpName, path)
}

// ClearMode deletes mode.json. Used by `ai mode clear` so that
// `ai mode current` clearly observes the absence rather than an
// empty-object marker.
func ClearMode() error {
	err := os.Remove(paths.ModeJSON())
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

// LoadState reads state.json. TBD for v0.8.
func LoadState() (State, error) {
	return State{}, nil
}

// SaveState writes state.json. TBD for v0.8.
func SaveState(_ State) error {
	return nil
}
