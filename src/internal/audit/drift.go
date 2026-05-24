package audit

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// DriftContent describes a single drift observation.
type DriftContent struct {
	Rule             string // e.g. "§3.U17"
	Trigger          string // "near-miss" | "pattern" | "cluster" | "behavioral"
	Evidence         string // what was observed
	SessionsAffected string // count or date range
	ProposedAction   string // proposed remediation
}

// WriteDrift writes a drift record to <root>/audit/drift/<UTC>-<slug>.md.
func WriteDrift(root, slug string, dc DriftContent) error {
	dir := filepath.Join(root, "audit", "drift")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("audit.WriteDrift: mkdir: %w", err)
	}
	ts := time.Now().UTC().Format("20060102T150405Z")
	filename := fmt.Sprintf("%s-%s.md", ts, slug)
	content := fmt.Sprintf("# Drift — %s\n\n- **Rule:** %s\n- **Trigger:** %s\n- **Evidence:** %s\n- **Sessions affected:** %s\n- **Proposed action:** %s\n",
		time.Now().UTC().Format("2006-01-02T15:04:05Z"),
		dc.Rule, dc.Trigger, dc.Evidence, dc.SessionsAffected, dc.ProposedAction,
	)
	return os.WriteFile(filepath.Join(dir, filename), []byte(content), 0o600) //nolint:gosec
}
