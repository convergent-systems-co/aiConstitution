package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/convergent-systems-co/aiConstitution/src/cmd/ai/internal/buildinfo"
	"github.com/convergent-systems-co/aiConstitution/src/internal/paths"
	"github.com/spf13/cobra"
)

type upgradeOptions struct {
	DryRun      bool
	SkipSelf    bool
	StrictSelf  bool
	SkipHooks   bool
	SkipSkills  bool
	SkipPlugins bool
	SkipCodex   bool
}

type upgradeCommand struct {
	Name string
	Args []string
}

var runUpgradeExternal = func(name string, args ...string) error {
	c := exec.Command(name, args...) //nolint:gosec // command is selected by trusted upgrade detection
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return c.Run()
}

// osExecutable is the indirection point used by detectSelfUpgradeCommand so
// tests can drive the heuristic with a constructed path without spawning a
// child process.
var osExecutable = os.Executable

func newUpgradeCmd() *cobra.Command {
	var opts upgradeOptions
	c := &cobra.Command{
		Use:   "upgrade",
		Short: "Safely upgrade ai and reconcile local governance deployment targets",
		Long: `upgrade is the safe end-to-end upgrade path.

It snapshots ~/.ai first, detects how the ai binary should be upgraded,
optionally upgrades the binary, then reconciles derived artifacts and
deployment targets without re-running setup or overwriting Constitution.md.

Reconciliation includes compact/runtime generation, hook infrastructure,
skills links, plugin links, and Codex AGENTS.md wiring.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runUpgrade(cmd, opts)
		},
	}
	c.Flags().BoolVar(&opts.DryRun, "dry-run", false, "print the upgrade plan without changing files")
	c.Flags().BoolVar(&opts.SkipSelf, "skip-self", false, "skip package-manager/self binary upgrade")
	c.Flags().BoolVar(&opts.StrictSelf, "strict-self", false, "fail if the self-upgrade command fails")
	c.Flags().BoolVar(&opts.SkipHooks, "skip-hooks", false, "skip hook and wrapper reconciliation")
	c.Flags().BoolVar(&opts.SkipSkills, "skip-skills", false, "skip skill deployment links")
	c.Flags().BoolVar(&opts.SkipPlugins, "skip-plugins", false, "skip plugin deployment links")
	c.Flags().BoolVar(&opts.SkipCodex, "skip-codex", false, "skip Codex AGENTS.md wiring")
	return c
}

func runUpgrade(cmd *cobra.Command, opts upgradeOptions) error {
	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "ai upgrade: current %s\n", buildinfo.Raw())

	latest, err := githubLatestRelease()
	if err != nil {
		fmt.Fprintf(out, "warning: could not check latest release: %v\n", err)
	} else if latest != "" {
		fmt.Fprintf(out, "ai upgrade: latest %s\n", latest)
	}

	selfCmd := detectSelfUpgradeCommand()
	if opts.DryRun {
		printUpgradePlan(out, opts, selfCmd)
		return nil
	}

	backupPath, err := writeUpgradeBackup()
	if err != nil {
		return fmt.Errorf("upgrade: backup failed: %w", err)
	}
	fmt.Fprintf(out, "Backup written: %s\n", backupPath)

	if !opts.SkipSelf {
		if selfCmd.Name == "" {
			fmt.Fprintln(out, "Self-upgrade: no package manager detected; skipping")
		} else {
			fmt.Fprintf(out, "Self-upgrade: %s %s\n", selfCmd.Name, strings.Join(selfCmd.Args, " "))
			if err := runUpgradeExternal(selfCmd.Name, selfCmd.Args...); err != nil {
				if opts.StrictSelf {
					return fmt.Errorf("upgrade: self-upgrade failed: %w", err)
				}
				fmt.Fprintf(out, "warning: self-upgrade failed: %v\n", err)
				fmt.Fprintln(out, "Continuing with local reconciliation. Re-run with --strict-self to fail here.")
			}
		}
	}

	return reconcileAfterUpgrade(cmd, opts)
}

func printUpgradePlan(out anyWriter, opts upgradeOptions, selfCmd upgradeCommand) {
	fmt.Fprintln(out, "Plan:")
	fmt.Fprintln(out, "  1. Backup ~/.ai without audit/interactions")
	if opts.SkipSelf {
		fmt.Fprintln(out, "  2. Skip self-upgrade")
	} else if selfCmd.Name == "" {
		fmt.Fprintln(out, "  2. Self-upgrade skipped: no package manager detected")
	} else {
		fmt.Fprintf(out, "  2. Self-upgrade with: %s %s\n", selfCmd.Name, strings.Join(selfCmd.Args, " "))
	}
	fmt.Fprintln(out, "  3. Regenerate Constitution.compact.md and Constitution.runtime.md")
	if !opts.SkipHooks {
		fmt.Fprintln(out, "  4. Reconcile hooks and command wrappers")
	}
	if !opts.SkipSkills {
		fmt.Fprintln(out, "  5. Link skills to Claude, Copilot, and Codex")
	}
	if !opts.SkipPlugins {
		fmt.Fprintln(out, "  6. Link plugins to Claude, Copilot, and Codex")
	}
	if !opts.SkipCodex {
		fmt.Fprintln(out, "  7. Wire Codex AGENTS.md to the compact Constitution")
	}
}

type anyWriter interface {
	Write([]byte) (int, error)
}

func writeUpgradeBackup() (string, error) {
	destDir := filepath.Join(paths.ConfigDir(), "backups")
	if err := os.MkdirAll(destDir, 0o750); err != nil {
		return "", err
	}
	name := "upgrade-" + time.Now().UTC().Format("20060102-150405") + ".tar.gz"
	archive := filepath.Join(destDir, name)
	if err := writeBackupArchive(paths.AIRoot(), archive); err != nil {
		return "", err
	}
	return archive, nil
}

func detectSelfUpgradeCommand() upgradeCommand {
	if override := strings.TrimSpace(os.Getenv("AICONST_UPGRADE_COMMAND")); override != "" {
		parts := strings.Fields(override)
		if len(parts) > 0 {
			return upgradeCommand{Name: parts[0], Args: parts[1:]}
		}
	}

	exe, _ := osExecutable()
	exe = filepath.ToSlash(exe)
	if runtime.GOOS == "darwin" && strings.Contains(exe, "/Cellar/ai/") {
		if _, err := exec.LookPath("brew"); err == nil {
			return upgradeCommand{Name: "brew", Args: []string{"upgrade", "ai"}}
		}
	}
	return upgradeCommand{}
}

func reconcileAfterUpgrade(cmd *cobra.Command, opts upgradeOptions) error {
	out := cmd.OutOrStdout()
	if err := runCompress(cmd, false, ""); err != nil {
		return fmt.Errorf("upgrade: regenerate compact constitution: %w", err)
	}
	if _, err := runGenerateRuntimeForRoot(paths.AIRoot()); err != nil {
		return fmt.Errorf("upgrade: regenerate runtime constitution: %w", err)
	}
	fmt.Fprintln(out, "Regenerated Constitution.compact.md and Constitution.runtime.md")

	if !opts.SkipHooks {
		if err := runHooksInstall("", "command-wrappers", false, false); err != nil {
			fmt.Fprintf(out, "warning: command-wrapper reconciliation failed: %v\n", err)
		}
		if err := runHooksInstall("", "", true, false); err != nil {
			fmt.Fprintf(out, "warning: hook reconciliation failed: %v\n", err)
		}
	}
	if !opts.SkipSkills {
		if err := runSkillsLink(cmd, skillLinkTargets{}); err != nil {
			return fmt.Errorf("upgrade: link skills: %w", err)
		}
	}
	if !opts.SkipPlugins {
		if err := runPluginsLink(cmd, pluginLinkTargets{}); err != nil {
			return fmt.Errorf("upgrade: link plugins: %w", err)
		}
	}
	if !opts.SkipCodex {
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("upgrade: get cwd: %w", err)
		}
		if err := runIntegrateCodex(cwd); err != nil {
			return fmt.Errorf("upgrade: wire Codex: %w", err)
		}
	}
	fmt.Fprintln(out, "Upgrade reconciliation complete.")
	return nil
}
