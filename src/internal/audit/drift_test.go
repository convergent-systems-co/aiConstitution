package audit_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/convergent-systems-co/aiConstitution/src/internal/audit"
)

func TestWriteDrift_CreatesFile(t *testing.T) {
	root := t.TempDir()
	dc := audit.DriftContent{
		Rule:             "§3.U17",
		Trigger:          "near-miss",
		Evidence:         "Blast radius hit 98 files; limit is 100.",
		SessionsAffected: "1",
		ProposedAction:   "strengthen enforcement",
	}
	if err := audit.WriteDrift(root, "worktree-blast", dc); err != nil {
		t.Fatalf("WriteDrift() error: %v", err)
	}
	entries, err := os.ReadDir(filepath.Join(root, "audit", "drift"))
	if err != nil {
		t.Fatalf("drift dir not created: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 drift file, got %d", len(entries))
	}
	data, _ := os.ReadFile(filepath.Join(root, "audit", "drift", entries[0].Name()))
	body := string(data)
	for _, want := range []string{"§3.U17", "near-miss", "Blast radius hit 98"} {
		if !strings.Contains(body, want) {
			t.Errorf("drift file missing %q:\n%s", want, body)
		}
	}
}

func TestWriteDrift_FileNameHasTimestampAndSlug(t *testing.T) {
	root := t.TempDir()
	before := time.Now().UTC()
	_ = audit.WriteDrift(root, "my-slug", audit.DriftContent{Rule: "§2.1", Trigger: "pattern"})
	entries, _ := os.ReadDir(filepath.Join(root, "audit", "drift"))
	if len(entries) == 0 {
		t.Fatal("no drift files created")
	}
	name := entries[0].Name()
	year := before.Format("2006")
	if !strings.Contains(name, year) {
		t.Errorf("expected year %s in filename %q", year, name)
	}
	if !strings.Contains(name, "my-slug") {
		t.Errorf("expected slug 'my-slug' in filename %q", name)
	}
}
