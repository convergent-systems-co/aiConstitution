// setup_integration_test.go — §4.1 integration tests for ai setup.
// Uses sandbox() from harness_test.go; MUST NOT call t.Parallel().
package cmd_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	cmd "github.com/convergent-systems-co/aiConstitution/src/cmd/ai/cmd"
)

// runSetup executes "ai setup --non-interactive --no-hooks" with the given
// AICONST_SEEDS value in the current sandbox environment.
// Callers must call sandbox(t) before runSetup.
func runSetup(t *testing.T, seeds string) string {
	t.Helper()
	t.Setenv("AICONST_SEEDS", seeds)
	root := cmd.NewRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"setup", "--non-interactive", "--no-hooks"})
	if err := root.Execute(); err != nil {
		t.Fatalf("ai setup --non-interactive --no-hooks: %v\noutput:\n%s", err, buf.String())
	}
	return buf.String()
}

// TestSetup_WritesConstitution verifies that setup --non-interactive writes
// Constitution.md to AI_ROOT containing the principal name and no unrendered
// template markers.
func TestSetup_WritesConstitution(t *testing.T) {
	s := sandbox(t)

	runSetup(t, "Q01=Test User")

	constitutionPath := filepath.Join(s.AIRoot, "Constitution.md")
	data, err := os.ReadFile(constitutionPath)
	if err != nil {
		t.Fatalf("Constitution.md not written to %s: %v", constitutionPath, err)
	}
	content := string(data)

	if strings.Contains(content, "{{") {
		for i, line := range strings.Split(content, "\n") {
			if strings.Contains(line, "{{") {
				t.Errorf("unrendered template marker at line %d: %q", i+1, line)
				break
			}
		}
	}
	if !strings.Contains(content, "Test User") {
		t.Error("Constitution.md does not contain the principal name 'Test User'")
	}
	if len(data) < 1000 {
		t.Errorf("Constitution.md suspiciously small (%d bytes); template may have partially rendered", len(data))
	}
}

// TestSetup_IdempotentNoClobber verifies that running setup twice on the same
// AI_ROOT does not silently overwrite Constitution.md. The second run must
// either produce a backup file or leave the original unchanged.
func TestSetup_IdempotentNoClobber(t *testing.T) {
	s := sandbox(t)

	// First run.
	runSetup(t, "Q01=First User")

	original, err := os.ReadFile(filepath.Join(s.AIRoot, "Constitution.md"))
	if err != nil {
		t.Fatalf("first run: Constitution.md not written: %v", err)
	}

	// Second run with a different principal so we can detect clobber.
	runSetup(t, "Q01=Second User")

	afterSecond, err := os.ReadFile(filepath.Join(s.AIRoot, "Constitution.md"))
	if err != nil {
		t.Fatalf("second run: Constitution.md missing: %v", err)
	}

	// Acceptable: Constitution.md now has "Second User" AND a backup exists,
	// OR Constitution.md still has "First User".
	// Unacceptable: "Second User" present with no backup anywhere.
	hasBackup := false
	constitutionPath := filepath.Join(s.AIRoot, "Constitution.md")
	_ = filepath.WalkDir(s.AIRoot, func(path string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() &&
			strings.Contains(d.Name(), "Constitution") &&
			path != constitutionPath {
			hasBackup = true
		}
		return nil
	})

	if strings.Contains(string(afterSecond), "Second User") && !hasBackup {
		t.Errorf("second run clobbered Constitution.md with no backup\n"+
			"original (%d bytes) started: %q\nafter second run: %q\nbackup found: %v",
			len(original),
			string(original)[:clamp(200, len(original))],
			string(afterSecond)[:clamp(200, len(afterSecond))],
			hasBackup)
	}
}

func clamp(max, n int) int {
	if n < max {
		return n
	}
	return max
}
