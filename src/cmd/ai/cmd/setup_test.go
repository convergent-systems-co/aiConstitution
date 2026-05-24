package cmd_test

import (
	"bytes"
	"os"
	"testing"

	"github.com/convergent-systems-co/aiConstitution/src/cmd/ai/cmd"
)

func TestSetupNonInteractiveSucceeds(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("AI_ROOT", dir)
	t.Setenv("AICONST_CONFIG_DIR", dir)

	root := cmd.NewRootCmd()
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"setup", "--non-interactive"})

	err := root.Execute()
	if err != nil {
		t.Logf("setup output: %s", buf)
		t.Errorf("setup --non-interactive returned error: %v", err)
	}
}

func TestSetupNonInteractiveCreatesSettingsToml(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("AI_ROOT", dir)
	t.Setenv("AICONST_CONFIG_DIR", dir)

	root := cmd.NewRootCmd()
	root.SetArgs([]string{"setup", "--non-interactive"})
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	_ = root.Execute()

	if _, err := os.Stat(dir + "/settings.toml"); os.IsNotExist(err) {
		t.Error("settings.toml was not created by setup --non-interactive")
	}
}
