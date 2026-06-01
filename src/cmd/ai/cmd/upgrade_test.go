package cmd

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUpgradeDryRunPrintsPlanAndDoesNotBackup(t *testing.T) {
	aiRoot := t.TempDir()
	cfg := t.TempDir()
	t.Setenv("AI_ROOT", aiRoot)
	t.Setenv("AICONST_CONFIG_DIR", cfg)
	t.Setenv("AICONST_UPGRADE_COMMAND", "brew upgrade ai")
	withFakeGitHubRelease(t, "v9.9.9", nil)

	var out bytes.Buffer
	c := NewRootCmd()
	c.SetOut(&out)
	c.SetErr(&out)
	c.SetArgs([]string{"upgrade", "--dry-run"})
	if err := c.Execute(); err != nil {
		t.Fatalf("upgrade --dry-run: %v\n%s", err, out.String())
	}

	got := out.String()
	for _, want := range []string{"Plan:", "Backup ~/.ai", "brew upgrade ai", "Wire Codex AGENTS.md"} {
		if !strings.Contains(got, want) {
			t.Fatalf("upgrade plan missing %q:\n%s", want, got)
		}
	}
	if _, err := os.Stat(filepath.Join(cfg, "backups")); !os.IsNotExist(err) {
		t.Fatalf("dry-run should not create backups dir; stat err=%v", err)
	}
}

func TestUpgradeSkipSelfDoesNotRunExternalCommand(t *testing.T) {
	aiRoot := t.TempDir()
	cfg := t.TempDir()
	t.Setenv("AI_ROOT", aiRoot)
	t.Setenv("AICONST_CONFIG_DIR", cfg)
	withFakeGitHubRelease(t, "", nil)
	writeMinimalUpgradeConstitution(t, aiRoot)
	before, _ := os.ReadFile(filepath.Join(aiRoot, "Constitution.md"))

	ranExternal := false
	origRun := runUpgradeExternal
	t.Cleanup(func() { runUpgradeExternal = origRun })
	runUpgradeExternal = func(string, ...string) error {
		ranExternal = true
		return nil
	}

	var out bytes.Buffer
	c := NewRootCmd()
	c.SetOut(&out)
	c.SetErr(&out)
	c.SetArgs([]string{"upgrade", "--skip-self", "--skip-hooks", "--skip-skills", "--skip-plugins", "--skip-codex"})
	if err := c.Execute(); err != nil {
		t.Fatalf("upgrade: %v\n%s", err, out.String())
	}
	if ranExternal {
		t.Fatal("external self-upgrade command ran despite --skip-self")
	}
	after, _ := os.ReadFile(filepath.Join(aiRoot, "Constitution.md"))
	if string(after) != string(before) {
		t.Fatalf("upgrade rewrote Constitution.md\nbefore:\n%s\nafter:\n%s", before, after)
	}
	if _, err := os.Stat(filepath.Join(aiRoot, "Constitution.compact.md")); err != nil {
		t.Fatalf("compact constitution not generated: %v", err)
	}
	if _, err := os.Stat(filepath.Join(aiRoot, "Constitution.runtime.md")); err != nil {
		t.Fatalf("runtime constitution not generated: %v", err)
	}
	entries, err := os.ReadDir(filepath.Join(cfg, "backups"))
	if err != nil || len(entries) == 0 {
		t.Fatalf("expected upgrade backup; entries=%d err=%v", len(entries), err)
	}
}

func TestUpgradeSelfFailureContinuesByDefault(t *testing.T) {
	aiRoot := t.TempDir()
	cfg := t.TempDir()
	t.Setenv("AI_ROOT", aiRoot)
	t.Setenv("AICONST_CONFIG_DIR", cfg)
	t.Setenv("AICONST_UPGRADE_COMMAND", "fake-upgrade ai")
	withFakeGitHubRelease(t, "", nil)
	writeMinimalUpgradeConstitution(t, aiRoot)

	origRun := runUpgradeExternal
	t.Cleanup(func() { runUpgradeExternal = origRun })
	runUpgradeExternal = func(string, ...string) error {
		return errors.New("boom")
	}

	var out bytes.Buffer
	c := NewRootCmd()
	c.SetOut(&out)
	c.SetErr(&out)
	c.SetArgs([]string{"upgrade", "--skip-hooks", "--skip-skills", "--skip-plugins", "--skip-codex"})
	if err := c.Execute(); err != nil {
		t.Fatalf("upgrade should continue after self-upgrade failure by default: %v\n%s", err, out.String())
	}
	if !strings.Contains(out.String(), "warning: self-upgrade failed") {
		t.Fatalf("expected warning about failed self-upgrade:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "Upgrade reconciliation complete") {
		t.Fatalf("expected reconciliation to complete:\n%s", out.String())
	}
}

func TestUpgradeStrictSelfFailsOnSelfFailure(t *testing.T) {
	aiRoot := t.TempDir()
	cfg := t.TempDir()
	t.Setenv("AI_ROOT", aiRoot)
	t.Setenv("AICONST_CONFIG_DIR", cfg)
	t.Setenv("AICONST_UPGRADE_COMMAND", "fake-upgrade ai")
	withFakeGitHubRelease(t, "", nil)
	writeMinimalUpgradeConstitution(t, aiRoot)

	origRun := runUpgradeExternal
	t.Cleanup(func() { runUpgradeExternal = origRun })
	runUpgradeExternal = func(string, ...string) error {
		return errors.New("boom")
	}

	var out bytes.Buffer
	c := NewRootCmd()
	c.SetOut(&out)
	c.SetErr(&out)
	c.SetArgs([]string{"upgrade", "--strict-self", "--skip-hooks", "--skip-skills", "--skip-plugins", "--skip-codex"})
	err := c.Execute()
	if err == nil || !strings.Contains(err.Error(), "self-upgrade failed") {
		t.Fatalf("expected strict self-upgrade failure, got err=%v out=%s", err, out.String())
	}
}

func writeMinimalUpgradeConstitution(t *testing.T, aiRoot string) {
	t.Helper()
	const constitution = `# AI Constitution

## §2 Behavioral Standards

Be careful.

## §3 Universal Rules

### §3.1 Prime Directives

**3.1 Preserve user governance.** Do not wipe the Constitution.

### §3.2 Autonomy Gates

Ask before destructive changes.

## 1. Common Rules

**1.1 Preserve.** Keep the user's Constitution intact.
`
	if err := os.WriteFile(filepath.Join(aiRoot, "Constitution.md"), []byte(constitution), 0o600); err != nil {
		t.Fatal(err)
	}
}
