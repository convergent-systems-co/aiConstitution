package cmd_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// setupAuditFixture wires AI_ROOT to a tmp dir with audit/violations,
// audit/overrides, and audit/interactions populated with the named files.
func setupAuditFixture(t *testing.T, violations, overrides, interactions map[string]string) string {
	t.Helper()
	aiRoot := t.TempDir()
	t.Setenv("AI_ROOT", aiRoot)
	write := func(sub string, files map[string]string) {
		dir := filepath.Join(aiRoot, "audit", sub)
		if err := os.MkdirAll(dir, 0o750); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
		for name, body := range files {
			if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
				t.Fatalf("write %s/%s: %v", sub, name, err)
			}
		}
	}
	write("violations", violations)
	write("overrides", overrides)
	write("interactions", interactions)
	return aiRoot
}

func TestAuditList_PrintsViolationsAndOverridesNewestFirst(t *testing.T) {
	setupAuditFixture(t,
		map[string]string{
			"20260101T120000Z-old.md":     "# Violation\n",
			"20260524T120000Z-recent.md": "# Violation\n",
		},
		map[string]string{
			"20260301T080000Z-override.md": "# Override\n",
		},
		nil,
	)

	out, err := runRootCmd(t, "audit", "list")
	if err != nil {
		t.Fatalf("audit list: %v\noutput:\n%s", err, out)
	}

	for _, want := range []string{"20260101T120000Z-old.md", "20260524T120000Z-recent.md", "20260301T080000Z-override.md"} {
		if !strings.Contains(out, want) {
			t.Errorf("want list output to include %q, got:\n%s", want, out)
		}
	}
	// Newest-first: the 2026-05 entry must appear before the 2026-01 entry.
	idxRecent := strings.Index(out, "20260524T120000Z-recent.md")
	idxOld := strings.Index(out, "20260101T120000Z-old.md")
	if idxRecent < 0 || idxOld < 0 || idxRecent > idxOld {
		t.Errorf("want newest-first ordering, recent at %d, old at %d; output:\n%s", idxRecent, idxOld, out)
	}
}

func TestAuditShow_PrintsViolationFile(t *testing.T) {
	body := "# Violation — 20260524T120000Z\n\nrule: test\n"
	setupAuditFixture(t,
		map[string]string{"20260524T120000Z-thing.md": body},
		nil, nil,
	)
	out, err := runRootCmd(t, "audit", "show", "20260524T120000Z-thing.md")
	if err != nil {
		t.Fatalf("audit show: %v\noutput:\n%s", err, out)
	}
	if !strings.Contains(out, "rule: test") {
		t.Errorf("want file body in output; got:\n%s", out)
	}
}

func TestAuditShow_PrintsOverrideFile(t *testing.T) {
	body := "# Override — 20260301T080000Z\n\nfoo\n"
	setupAuditFixture(t,
		nil,
		map[string]string{"20260301T080000Z-x.md": body},
		nil,
	)
	out, err := runRootCmd(t, "audit", "show", "20260301T080000Z-x.md")
	if err != nil {
		t.Fatalf("audit show: %v\noutput:\n%s", err, out)
	}
	if !strings.Contains(out, "foo") {
		t.Errorf("want file body in output; got:\n%s", out)
	}
}

func TestAuditShow_ErrorWhenMissing(t *testing.T) {
	setupAuditFixture(t, nil, nil, nil)
	_, err := runRootCmd(t, "audit", "show", "missing.md")
	if err == nil {
		t.Errorf("want error for missing audit file, got nil")
	}
}

func TestAuditRotate_DropsLogsOlderThan30Days(t *testing.T) {
	// Generate canonical month tokens relative to today so the test
	// remains stable across calendar shifts.
	now := time.Now().UTC()
	currentMonth := now.Format("2006-01")
	// Pick a month well-outside the 30-day window.
	old := now.AddDate(0, -6, 0).Format("2006-01")

	aiRoot := setupAuditFixture(t, nil, nil, map[string]string{
		currentMonth + ".jsonl": "{\"kind\":\"request\"}\n",
		old + ".jsonl":          "{\"kind\":\"request\"}\n",
	})

	out, err := runRootCmd(t, "audit", "rotate")
	if err != nil {
		t.Fatalf("audit rotate: %v\noutput:\n%s", err, out)
	}

	intDir := filepath.Join(aiRoot, "audit", "interactions")
	if _, err := os.Stat(filepath.Join(intDir, old+".jsonl")); !os.IsNotExist(err) {
		t.Errorf("old log %s should be deleted; stat err=%v", old+".jsonl", err)
	}
	if _, err := os.Stat(filepath.Join(intDir, currentMonth+".jsonl")); err != nil {
		t.Errorf("current log %s should be kept; stat err=%v", currentMonth+".jsonl", err)
	}
}
