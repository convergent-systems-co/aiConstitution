package cmd_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/BurntSushi/toml"
	"github.com/convergent-systems-co/aiConstitution/src/cmd/ai/cmd"
	"github.com/convergent-systems-co/aiConstitution/src/internal/config"
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

func TestSetupNonInteractiveAppliesSeeds(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("AI_ROOT", dir)
	t.Setenv("AICONST_CONFIG_DIR", dir)
	t.Setenv("AICONST_SEEDS", "defaultMode=writer,shareNewHooks=false,reviewCadenceDays=14,syncIncludeSettings=false")

	root := cmd.NewRootCmd()
	root.SetArgs([]string{"setup", "--non-interactive"})
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	if err := root.Execute(); err != nil {
		t.Fatalf("setup error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "settings.toml"))
	if err != nil {
		t.Fatalf("read settings.toml: %v", err)
	}
	var s config.Settings
	if _, err := toml.Decode(string(data), &s); err != nil {
		t.Fatalf("decode settings.toml: %v", err)
	}
	if s.Focus.DefaultMode != "writer" {
		t.Errorf("Focus.DefaultMode = %q, want %q", s.Focus.DefaultMode, "writer")
	}
	if s.Upstream.ShareNewHooks != false {
		t.Errorf("Upstream.ShareNewHooks = %v, want false", s.Upstream.ShareNewHooks)
	}
	if s.Review.CadenceDays != 14 {
		t.Errorf("Review.CadenceDays = %d, want 14", s.Review.CadenceDays)
	}
	if s.Sync.IncludeSettingsFile != false {
		t.Errorf("Sync.IncludeSettingsFile = %v, want false", s.Sync.IncludeSettingsFile)
	}
}

func TestSetupNonInteractiveWritesToolFiles(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("AI_ROOT", dir)
	t.Setenv("AICONST_CONFIG_DIR", dir)

	root := cmd.NewRootCmd()
	root.SetArgs([]string{"setup", "--non-interactive"})
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	if err := root.Execute(); err != nil {
		t.Fatalf("setup error: %v", err)
	}

	for _, rel := range []string{"CLAUDE.md", ".github/copilot-instructions.md", "AGENTS.md"} {
		p := filepath.Join(dir, rel)
		if _, err := os.Stat(p); err != nil {
			t.Errorf("expected %s to exist: %v", p, err)
		}
	}
}

func TestSetupNonInteractivePreservesExistingToolFiles(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("AI_ROOT", dir)
	t.Setenv("AICONST_CONFIG_DIR", dir)

	custom := []byte("# Custom CLAUDE.md content\n")
	if err := os.WriteFile(filepath.Join(dir, "CLAUDE.md"), custom, 0o600); err != nil {
		t.Fatal(err)
	}

	root := cmd.NewRootCmd()
	root.SetArgs([]string{"setup", "--non-interactive"})
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	if err := root.Execute(); err != nil {
		t.Fatalf("setup error: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(dir, "CLAUDE.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(custom) {
		t.Errorf("CLAUDE.md was overwritten by setup")
	}
}
