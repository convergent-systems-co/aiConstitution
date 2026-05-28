package cmd_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/convergent-systems-co/aiConstitution/src/cmd/ai/cmd"
)

// TestHooksPropose_PythonDefault verifies that calling runHooksPropose with no
// lang flag (defaulting to "python") creates a <name>.py scaffold containing
// the shebang and the placeholder description.
func TestHooksPropose_PythonDefault(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	var out bytes.Buffer

	if err := cmd.RunHooksPropose("my-hook", "", "python", dir, &out); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := filepath.Join(dir, "hooks", "my-hook.py")
	data, err := os.ReadFile(expected)
	if err != nil {
		t.Fatalf("scaffold file not created at %s: %v", expected, err)
	}
	content := string(data)

	if !strings.Contains(content, "#!/usr/bin/env python3") {
		t.Errorf("missing python shebang; got:\n%s", content)
	}
	if !strings.Contains(content, "<description of what this hook checks>") {
		t.Errorf("missing placeholder description; got:\n%s", content)
	}
	if !strings.Contains(content, "my-hook") {
		t.Errorf("hook name not embedded in scaffold; got:\n%s", content)
	}

	// Output should mention the created path and the next step.
	outStr := out.String()
	if !strings.Contains(outStr, "Created:") {
		t.Errorf("stdout missing 'Created:'; got: %q", outStr)
	}
	if !strings.Contains(outStr, "Next:") {
		t.Errorf("stdout missing 'Next:'; got: %q", outStr)
	}
}

// TestHooksPropose_ShellLang verifies that --lang sh creates a <name>.sh
// scaffold with the bash shebang.
func TestHooksPropose_ShellLang(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	var out bytes.Buffer

	if err := cmd.RunHooksPropose("shell-hook", "", "sh", dir, &out); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := filepath.Join(dir, "hooks", "shell-hook.sh")
	data, err := os.ReadFile(expected)
	if err != nil {
		t.Fatalf("scaffold file not created at %s: %v", expected, err)
	}
	content := string(data)

	if !strings.Contains(content, "#!/usr/bin/env bash") {
		t.Errorf("missing bash shebang; got:\n%s", content)
	}
	if !strings.Contains(content, "shell-hook") {
		t.Errorf("hook name not embedded in scaffold; got:\n%s", content)
	}
}

// TestHooksPropose_UnsupportedLang verifies that --lang go (or --lang node)
// returns an error and does not create any file.
func TestHooksPropose_UnsupportedLang(t *testing.T) {
	t.Parallel()
	for _, lang := range []string{"go", "node"} {
		lang := lang
		t.Run(lang, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			var out bytes.Buffer

			err := cmd.RunHooksPropose("bad-hook", "", lang, dir, &out)
			if err == nil {
				t.Fatalf("expected error for lang=%q but got nil", lang)
			}
			if !strings.Contains(err.Error(), "unsupported lang") {
				t.Errorf("error should mention 'unsupported lang'; got: %v", err)
			}
			if !strings.Contains(err.Error(), lang) {
				t.Errorf("error should include the lang name %q; got: %v", lang, err)
			}
		})
	}
}

// TestHooksPropose_AlreadyExists verifies that if the target file already
// exists, runHooksPropose returns an error and does not overwrite.
func TestHooksPropose_AlreadyExists(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	hooksDir := filepath.Join(dir, "hooks")
	if err := os.MkdirAll(hooksDir, 0o750); err != nil {
		t.Fatal(err)
	}
	existingPath := filepath.Join(hooksDir, "existing-hook.py")
	original := "original content"
	if err := os.WriteFile(existingPath, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	err := cmd.RunHooksPropose("existing-hook", "", "python", dir, &out)
	if err == nil {
		t.Fatal("expected error when file already exists, got nil")
	}
	if !strings.Contains(err.Error(), "hook already exists") {
		t.Errorf("error should mention 'hook already exists'; got: %v", err)
	}
	if !strings.Contains(err.Error(), existingPath) {
		t.Errorf("error should contain the existing path %q; got: %v", existingPath, err)
	}

	// File should be unchanged.
	data, _ := os.ReadFile(existingPath)
	if string(data) != original {
		t.Errorf("existing file was overwritten; got: %q", string(data))
	}
}

// TestHooksPropose_FromViolation verifies that when --from-violation is given,
// the "What happened" field from the violation file seeds the scaffold description.
func TestHooksPropose_FromViolation(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Write a minimal violation file.
	violationContent := `# Violation — 2026-01-01T00:00:00Z

- **Section / Rule violated:** §3.1 — prime directive
- **What happened:** The assistant leaked a secret token into the audit log.
- **How noticed:** self-detected
- **Remediation:** Redacted and added a check.
- **Proposed amendment (if any):** none
`
	violationPath := filepath.Join(dir, "violation.md")
	if err := os.WriteFile(violationPath, []byte(violationContent), 0o644); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	if err := cmd.RunHooksPropose("from-violation-hook", violationPath, "python", dir, &out); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	scaffoldPath := filepath.Join(dir, "hooks", "from-violation-hook.py")
	data, err := os.ReadFile(scaffoldPath)
	if err != nil {
		t.Fatalf("scaffold not created: %v", err)
	}
	content := string(data)

	if !strings.Contains(content, "leaked a secret token into the audit log") {
		t.Errorf("violation 'What happened' text not seeded into scaffold; got:\n%s", content)
	}
	// Should NOT contain the generic placeholder when a real description is given.
	if strings.Contains(content, "<description of what this hook checks>") {
		t.Errorf("scaffold still has placeholder text despite --from-violation; got:\n%s", content)
	}
}

// TestHooksPropose_FromViolation_NotFound verifies that a non-existent violation
// file path returns an error.
func TestHooksPropose_FromViolation_NotFound(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	var out bytes.Buffer

	err := cmd.RunHooksPropose("my-hook", filepath.Join(dir, "nonexistent.md"), "python", dir, &out)
	if err == nil {
		t.Fatal("expected error for missing violation file, got nil")
	}
}
