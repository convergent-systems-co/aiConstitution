package cmd_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/convergent-systems-co/aiConstitution/src/cmd/ai/cmd"
)

// -- helpers ------------------------------------------------------------------

func runHooksCmd(t *testing.T, args ...string) (string, error) {
	t.Helper()
	root := cmd.NewRootCmd()
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs(append([]string{"hooks"}, args...))
	err := root.Execute()
	return buf.String(), err
}

// -- validate tests (#200, #201) ----------------------------------------------

// TestValidatePassOnValidHook verifies that a well-formed .py hook gets [✓].
func TestValidatePassOnValidHook(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("AI_HOOKS_VALIDATE_DIR", dir) // hooks validate will read this env var

	content := `#!/usr/bin/env python3
"""Minimal valid hook."""
import sys
if __name__ == "__main__":
    sys.exit(0)
`
	if err := os.WriteFile(filepath.Join(dir, "valid-hook.py"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := runHooksCmd(t, "validate", "--dir="+dir)
	if err != nil {
		t.Fatalf("validate returned error on valid hook: %v\noutput: %s", err, out)
	}
	if !strings.Contains(out, "[✓]") {
		t.Errorf("expected [✓] in output, got:\n%s", out)
	}
}

// TestValidateExitOneOnSyntaxError verifies exit 1 and [✗] for a bad .py.
func TestValidateExitOneOnSyntaxError(t *testing.T) {
	dir := t.TempDir()

	content := `#!/usr/bin/env python3
def broken(
    # missing closing paren — syntax error
`
	if err := os.WriteFile(filepath.Join(dir, "broken.py"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := runHooksCmd(t, "validate", "--dir="+dir)
	if err == nil {
		t.Errorf("validate should return error when a hook has a syntax error\noutput: %s", out)
	}
	if !strings.Contains(out, "[✗]") {
		t.Errorf("expected [✗] in output, got:\n%s", out)
	}
}

// TestValidateBareExceptWarning verifies [⚠] (not [✗]) for bare except:.
func TestValidateBareExceptWarning(t *testing.T) {
	dir := t.TempDir()

	content := `#!/usr/bin/env python3
"""Hook with a bare except — should be warned, not failed."""
try:
    pass
except:
    pass
`
	if err := os.WriteFile(filepath.Join(dir, "bare-except.py"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := runHooksCmd(t, "validate", "--dir="+dir)
	if err != nil {
		t.Fatalf("validate should not exit 1 for bare-except warning: %v\noutput: %s", err, out)
	}
	if !strings.Contains(out, "[⚠]") {
		t.Errorf("expected [⚠] in output for bare except, got:\n%s", out)
	}
}

// TestValidateMissingShebang verifies [✗] when the first line is not a shebang.
func TestValidateMissingShebang(t *testing.T) {
	dir := t.TempDir()

	content := `"""Hook missing a shebang line."""
import sys
sys.exit(0)
`
	if err := os.WriteFile(filepath.Join(dir, "no-shebang.py"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := runHooksCmd(t, "validate", "--dir="+dir)
	if err == nil {
		t.Errorf("validate should return error when shebang is missing\noutput: %s", out)
	}
	if !strings.Contains(out, "[✗]") {
		t.Errorf("expected [✗] in output for missing shebang, got:\n%s", out)
	}
}

// TestValidateShellHook verifies that .sh files are checked via bash -n.
func TestValidateShellHook(t *testing.T) {
	dir := t.TempDir()

	valid := "#!/usr/bin/env bash\necho hello\n"
	if err := os.WriteFile(filepath.Join(dir, "good.sh"), []byte(valid), 0o644); err != nil {
		t.Fatal(err)
	}
	broken := "#!/usr/bin/env bash\nif [[ ]\n"
	if err := os.WriteFile(filepath.Join(dir, "bad.sh"), []byte(broken), 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := runHooksCmd(t, "validate", "--dir="+dir)
	if err == nil {
		t.Errorf("validate should return error when .sh has syntax error\noutput: %s", out)
	}
	if !strings.Contains(out, "[✗]") {
		t.Errorf("expected [✗] for broken.sh, got:\n%s", out)
	}
}

// TestValidateNoHooksIsOk verifies exit 0 on an empty directory.
func TestValidateNoHooksIsOk(t *testing.T) {
	dir := t.TempDir()
	out, err := runHooksCmd(t, "validate", "--dir="+dir)
	if err != nil {
		t.Fatalf("validate should succeed on empty dir: %v\noutput: %s", err, out)
	}
}

// -- evaluate tests (#202) ----------------------------------------------------

// TestEvaluateDoesNotReturnStubError verifies that `ai hooks evaluate` is
// no longer a stub: it must either succeed (exit 0) or return a real error
// (not the errNotImplementedHint).
func TestEvaluateDoesNotReturnStubError(t *testing.T) {
	out, err := runHooksCmd(t, "evaluate")
	if err != nil {
		// Must NOT be a stub error.
		if strings.Contains(err.Error(), "not yet implemented") {
			t.Errorf("evaluate is still a stub: %v\noutput: %s", err, out)
		}
		// A real error (e.g. python3 not found) is acceptable — the stub
		// is the contract violation.
	}
}

// TestEvaluateProducesPerHookOutput verifies that evaluate emits at least one
// [✓] or [✗] line, confirming it iterates over embedded hooks.
func TestEvaluateProducesPerHookOutput(t *testing.T) {
	out, _ := runHooksCmd(t, "evaluate")
	if !strings.Contains(out, "[✓]") && !strings.Contains(out, "[✗]") {
		t.Errorf("evaluate must emit [✓] or [✗] for at least one hook, got:\n%s", out)
	}
}
