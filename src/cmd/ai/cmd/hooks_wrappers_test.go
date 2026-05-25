package cmd_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/convergent-systems-co/aiConstitution/src/cmd/ai/cmd"
)

func TestHooksInstallCommandWrappersExtractsFiles(t *testing.T) {
	aiRoot := t.TempDir()
	t.Setenv("AI_ROOT", aiRoot)

	root := cmd.NewRootCmd()
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"hooks", "install", "command-wrappers"})
	if err := root.Execute(); err != nil {
		t.Fatalf("hooks install command-wrappers error: %v\noutput:%s", err, buf)
	}

	binDir := filepath.Join(aiRoot, "bin")
	for _, name := range []string{"git", "gh"} {
		p := filepath.Join(binDir, name)
		info, err := os.Stat(p)
		if err != nil {
			t.Errorf("expected wrapper %s: %v", p, err)
			continue
		}
		// 0o755 expected (executable).
		if info.Mode().Perm()&0o100 == 0 {
			t.Errorf("wrapper %s is not executable (mode=%v)", p, info.Mode())
		}
	}
}
