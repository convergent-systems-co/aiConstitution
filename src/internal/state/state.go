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

import "time"

// Mode is the on-disk shape of mode.json. Per SPEC.md §7.8.3.
type Mode struct {
	Type           string         `json:"type"`            // "profile" | "persona"
	Name           string         `json:"name"`
	Version        string         `json:"version"`
	Source         string         `json:"source"`          // "atom" | "user-draft" | "shipped"
	ActivatedAt    time.Time      `json:"activatedAt"`
	ActivatedVia   string         `json:"activatedVia"`    // "cli" | "settings-default" | "restore"
	ComposedAtoms  []ComposedAtom `json:"composedAtoms"`
	SourcePath     string         `json:"sourcePath"`
	Exclusive      bool           `json:"exclusive,omitempty"`
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
	LastSeenVersion       string    `json:"lastSeenVersion"`
	LastReviewTimestamp   time.Time `json:"lastReviewTimestamp,omitempty"`
	LastDrillTimestamp    time.Time `json:"lastDrillTimestamp,omitempty"`
	LastReviewSnooze      time.Time `json:"lastReviewSnooze,omitempty"`
	UpdatePending         bool      `json:"updatePending,omitempty"`
	UpdateSource          string    `json:"updateSource,omitempty"` // "brew" | "scoop" | "winget" | "git"
}

// LoadMode reads ~/.config/aiConstitution/mode.json. Returns
// (Mode{}, nil) if the file doesn't exist (mode == "none").
//
// TBD for v0.8.
func LoadMode() (Mode, error) {
	return Mode{}, nil
}

// SaveMode writes mode.json. TBD for v0.8.
func SaveMode(_ Mode) error {
	return nil
}

// ClearMode deletes mode.json. TBD for v0.8.
func ClearMode() error {
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
