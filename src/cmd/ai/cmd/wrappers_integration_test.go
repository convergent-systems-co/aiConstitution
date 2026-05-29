// wrappers_integration_test.go — §4.3 integration tests for command-wrapper install.
// Uses sandbox() from harness_test.go; MUST NOT call t.Parallel().
package cmd_test

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	cmd "github.com/convergent-systems-co/aiConstitution/src/cmd/ai/cmd"
)

// TestHooksInstallCommandWrappers_ExtractsToAIBin verifies that
// "ai hooks install command-wrappers" extracts the platform-appropriate
// shims to AI_ROOT/bin/ with executable permissions and ai-wrap delegation.
func TestHooksInstallCommandWrappers_ExtractsToAIBin(t *testing.T) {
	s := sandbox(t)

	root := cmd.NewRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"hooks", "install", "command-wrappers"})
	if err := root.Execute(); err != nil {
		t.Fatalf("hooks install command-wrappers: %v\n%s", err, buf.String())
	}

	binDir := filepath.Join(s.AIRoot, "bin")

	// On POSIX: expect bare "git" and "gh" (not .cmd or .ps1).
	// On Windows: expect "git.cmd" and "gh.cmd" (or .ps1).
	var wantNames []string
	var notwantSuffix string
	if runtime.GOOS == "windows" {
		wantNames = []string{"git.cmd", "git.ps1", "gh.cmd", "gh.ps1"}
	} else {
		wantNames = []string{"git", "gh"}
		notwantSuffix = ".cmd"
	}

	for _, name := range wantNames {
		p := filepath.Join(binDir, name)
		info, err := os.Stat(p)
		if err != nil {
			t.Errorf("%s not extracted to %s: %v", name, binDir, err)
			continue
		}
		// On POSIX, wrappers must be executable.
		if runtime.GOOS != "windows" && info.Mode()&0o100 == 0 {
			t.Errorf("%s is not executable (mode=%o)", name, info.Mode())
		}
		// Content must delegate to "ai wrap".
		data, err := os.ReadFile(p)
		if err != nil {
			t.Errorf("cannot read %s: %v", p, err)
			continue
		}
		if !strings.Contains(string(data), "ai wrap") {
			t.Errorf("%s does not contain 'ai wrap':\n%s", name, data)
		}
	}

	// Platform-inappropriate forms must NOT be extracted on POSIX.
	if notwantSuffix != "" {
		for _, base := range []string{"git", "gh"} {
			bad := filepath.Join(binDir, base+notwantSuffix)
			if _, err := os.Stat(bad); err == nil {
				t.Errorf("%s should not be extracted on %s", bad, runtime.GOOS)
			}
		}
	}
}

// TestHooksInstallCommandWrappers_Idempotent verifies that running
// "ai hooks install command-wrappers" twice does not error or change file count.
func TestHooksInstallCommandWrappers_Idempotent(t *testing.T) {
	s := sandbox(t)
	binDir := filepath.Join(s.AIRoot, "bin")

	runInstall := func(label string) {
		t.Helper()
		root := cmd.NewRootCmd()
		var buf bytes.Buffer
		root.SetOut(&buf)
		root.SetErr(&buf)
		root.SetArgs([]string{"hooks", "install", "command-wrappers"})
		if err := root.Execute(); err != nil {
			t.Fatalf("[%s] hooks install command-wrappers: %v\n%s", label, err, buf.String())
		}
	}

	runInstall("first")
	entries1, err := os.ReadDir(binDir)
	if err != nil {
		t.Fatalf("bin/ missing after first install: %v", err)
	}

	runInstall("second")
	entries2, err := os.ReadDir(binDir)
	if err != nil {
		t.Fatalf("bin/ missing after second install: %v", err)
	}

	if len(entries1) != len(entries2) {
		t.Errorf("second install changed bin/ file count: first=%d, second=%d",
			len(entries1), len(entries2))
	}
}
