package cmd_test

import (
	"bytes"
	"fmt"
	"os"
	"testing"

	"github.com/convergent-systems-co/aiConstitution/src/cmd/ai/cmd"
)

// runRootCmd builds a fresh cobra root, sets AI_ROOT to aiRoot, then runs
// args against it. Returns stdout, stderr, and any execution error.
// The error returned is the cobra command error (RunE result), not an
// infrastructure error — callers test it directly.
func runRootCmd(t *testing.T, aiRoot string, args ...string) (stdout, stderr string, err error) {
	t.Helper()

	// Point AI_ROOT at the test temp dir so paths.AIRoot() resolves there.
	orig := os.Getenv("AI_ROOT")
	if err := os.Setenv("AI_ROOT", aiRoot); err != nil {
		t.Fatalf("setenv AI_ROOT: %v", err)
	}
	t.Cleanup(func() {
		if orig == "" {
			_ = os.Unsetenv("AI_ROOT")
		} else {
			_ = os.Setenv("AI_ROOT", orig)
		}
	})

	root := cmd.NewRootCmd()
	root.SilenceErrors = true
	root.SilenceUsage = true

	var outBuf, errBuf bytes.Buffer
	root.SetOut(&outBuf)
	root.SetErr(&errBuf)
	root.SetArgs(args)

	execErr := root.Execute()
	outStr := outBuf.String()
	errStr := errBuf.String()

	if execErr != nil {
		// Wrap the error to carry stdout/stderr context for debugging.
		return outStr, errStr, fmt.Errorf("%w (stdout=%q stderr=%q)", execErr, outStr, errStr)
	}
	return outStr, errStr, nil
}
