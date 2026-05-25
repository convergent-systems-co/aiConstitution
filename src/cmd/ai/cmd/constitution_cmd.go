package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/convergent-systems-co/aiConstitution/src/cmd/ai/embed"
	"github.com/convergent-systems-co/aiConstitution/src/internal/constitution"
	"github.com/convergent-systems-co/aiConstitution/src/internal/paths"
	"github.com/spf13/cobra"
)

// backupManifest records what was saved and how to restore it.
type backupManifest struct {
	CreatedAt      string `json:"created_at"`
	AIRoot         string `json:"ai_root"`
	ClaudeMDPath   string `json:"claude_md_path"`
	CopilotSymlink string `json:"copilot_symlink_target"`
	HooksInstalled bool   `json:"hooks_installed"`
	ClearLinks     bool   `json:"clear_links_applied"`
}

func newConstitutionCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "constitution",
		Short: "Backup and restore the active constitution and tool wiring",
	}
	c.AddCommand(newConstitutionBackupCmd())
	c.AddCommand(newConstitutionRestoreCmd())
	return c
}

// ─── backup ──────────────────────────────────────────────────────────────────

func newConstitutionBackupCmd() *cobra.Command {
	var clearLinks bool

	c := &cobra.Command{
		Use:   "backup",
		Short: "Snapshot Constitution.md and tool wiring to ~/.ai/backups/<UTC>/",
		Long: `backup copies the active Constitution.md, ~/.claude/CLAUDE.md, and
records the Copilot symlink and hooks wiring into a timestamped backup.

With --clear-links, the live wiring is removed after backup so you can
test a different constitution without contaminating the active session.
Run 'ai constitution restore' to put everything back.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runConstitutionBackup(cmd, clearLinks)
		},
	}
	c.Flags().BoolVar(&clearLinks, "clear-links", false,
		"remove live wiring after backup (CLAUDE.md @-include, hooks, Copilot symlink)")
	return c
}

func runConstitutionBackup(cmd *cobra.Command, clearLinks bool) error {
	aiRoot := paths.AIRoot()
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("constitution backup: %w", err)
	}

	ts := time.Now().UTC().Format("20060102T150405Z")
	backupDir := filepath.Join(aiRoot, "backups", ts)
	if err := os.MkdirAll(backupDir, 0o750); err != nil {
		return fmt.Errorf("constitution backup: mkdir: %w", err)
	}

	manifest := backupManifest{
		CreatedAt:  time.Now().UTC().Format(time.RFC3339),
		AIRoot:     aiRoot,
		ClearLinks: clearLinks,
	}

	// 1. Back up Constitution.md.
	constitutionSrc := filepath.Join(aiRoot, "Constitution.md")
	if err := copyFileSimple(constitutionSrc, filepath.Join(backupDir, "Constitution.md")); err != nil {
		return fmt.Errorf("constitution backup: copy Constitution.md: %w", err)
	}

	// 2. Back up ~/.claude/CLAUDE.md.
	claudeMD := filepath.Join(home, ".claude", "CLAUDE.md")
	manifest.ClaudeMDPath = claudeMD
	if data, err := os.ReadFile(claudeMD); err == nil { //nolint:gosec
		if err := os.WriteFile(filepath.Join(backupDir, "CLAUDE.md"), data, 0o600); err != nil { //nolint:gosec
			return fmt.Errorf("constitution backup: copy CLAUDE.md: %w", err)
		}
	}

	// 3. Record Copilot symlink target.
	copilotLink := filepath.Join(home, ".copilot", "instructions", "constitution.md")
	if target, err := os.Readlink(copilotLink); err == nil {
		manifest.CopilotSymlink = target
		_ = os.WriteFile(filepath.Join(backupDir, "copilot-target.txt"), []byte(target), 0o600) //nolint:gosec
	}

	// 4. Record that hooks were installed.
	if _, err := os.Stat(filepath.Join(aiRoot, "hooks", "audit.py")); err == nil {
		manifest.HooksInstalled = true
	}

	// Write manifest.
	manifestBytes, _ := json.MarshalIndent(manifest, "", "  ")
	if err := os.WriteFile(filepath.Join(backupDir, "manifest.json"), manifestBytes, 0o600); err != nil { //nolint:gosec
		return fmt.Errorf("constitution backup: write manifest: %w", err)
	}

	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Backup created: %s\n", backupDir)

	if clearLinks {
		if err := clearConstitutionLinks(home, claudeMD, copilotLink); err != nil {
			return fmt.Errorf("constitution backup --clear-links: %w", err)
		}
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Links cleared. Run 'ai setup' to wire a new constitution.")
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Run 'ai constitution restore' to restore the original.")
	}

	return nil
}

// clearConstitutionLinks removes the live tool wiring so a fresh 'ai setup'
// can wire a different constitution without conflicts.
func clearConstitutionLinks(home, claudeMD, copilotLink string) error {
	// Remove the @-include from CLAUDE.md (strip lines starting with @~/.ai/).
	if data, err := os.ReadFile(claudeMD); err == nil { //nolint:gosec
		var kept []string
		for _, line := range strings.Split(string(data), "\n") {
			if !strings.HasPrefix(line, "@~/.ai/") {
				kept = append(kept, line)
			}
		}
		updated := strings.Join(kept, "\n")
		_ = os.WriteFile(claudeMD, []byte(updated), 0o600) //nolint:gosec
	}

	// Remove Copilot symlink.
	_ = os.Remove(copilotLink)

	// Remove hooks from ~/.claude/settings.json by zeroing the hooks block.
	settingsPath := filepath.Join(home, ".claude", "settings.json")
	if data, err := os.ReadFile(settingsPath); err == nil { //nolint:gosec
		var settings map[string]any
		if json.Unmarshal(data, &settings) == nil {
			delete(settings, "hooks")
			if updated, err := json.MarshalIndent(settings, "", "  "); err == nil {
				_ = os.WriteFile(settingsPath, updated, 0o600) //nolint:gosec
			}
		}
	}

	return nil
}

// ─── restore ─────────────────────────────────────────────────────────────────

func newConstitutionRestoreCmd() *cobra.Command {
	var backupID string

	c := &cobra.Command{
		Use:   "restore",
		Short: "Restore Constitution.md and tool wiring from the latest backup",
		Long: `restore reads the most recent (or specified) backup, copies
Constitution.md back into ~/.ai/, rewrites ~/.claude/CLAUDE.md with the
@-include, re-installs hooks into ~/.claude/settings.json, and recreates
the Copilot symlink.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runConstitutionRestore(cmd, backupID)
		},
	}
	c.Flags().StringVar(&backupID, "backup", "",
		"backup ID (timestamp) to restore; defaults to the most recent")
	return c
}

func runConstitutionRestore(cmd *cobra.Command, backupID string) error {
	aiRoot := paths.AIRoot()
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("constitution restore: %w", err)
	}

	// Locate backup directory.
	backupsRoot := filepath.Join(aiRoot, "backups")
	backupDir, err := resolveBackupDir(backupsRoot, backupID)
	if err != nil {
		return fmt.Errorf("constitution restore: %w", err)
	}

	// Read manifest.
	var manifest backupManifest
	manifestBytes, err := os.ReadFile(filepath.Join(backupDir, "manifest.json")) //nolint:gosec
	if err != nil {
		return fmt.Errorf("constitution restore: read manifest: %w", err)
	}
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		return fmt.Errorf("constitution restore: parse manifest: %w", err)
	}

	out := cmd.OutOrStdout()

	// 1. Restore Constitution.md.
	if err := copyFileSimple(
		filepath.Join(backupDir, "Constitution.md"),
		filepath.Join(aiRoot, "Constitution.md"),
	); err != nil {
		return fmt.Errorf("constitution restore: Constitution.md: %w", err)
	}
	_, _ = fmt.Fprintln(out, "[✓] Constitution.md restored")

	// 2. Restore CLAUDE.md.
	claudeMD := filepath.Join(home, ".claude", "CLAUDE.md")
	if data, err := os.ReadFile(filepath.Join(backupDir, "CLAUDE.md")); err == nil { //nolint:gosec
		if err := os.MkdirAll(filepath.Dir(claudeMD), 0o750); err == nil {
			_ = os.WriteFile(claudeMD, data, 0o600) //nolint:gosec
			_, _ = fmt.Fprintln(out, "[✓] ~/.claude/CLAUDE.md restored")
		}
	} else {
		_, _ = fmt.Fprintln(out, "[!] CLAUDE.md backup not found — skipping")
	}

	// 3. Re-install hooks.
	hooksDir := filepath.Join(aiRoot, "hooks")
	if written, err := embed.ExtractAllHooks(hooksDir, true); err != nil {
		_, _ = fmt.Fprintf(out, "[!] hooks install failed: %v\n", err)
	} else {
		_, _ = fmt.Fprintf(out, "[✓] %d hooks reinstalled to %s\n", len(written), hooksDir)
	}

	// 4. Re-wire hooks into settings.json.
	settingsPath := filepath.Join(home, ".claude", "settings.json")
	if err := rewireHooks(settingsPath, aiRoot); err != nil {
		_, _ = fmt.Fprintf(out, "[!] settings.json wiring failed: %v\n", err)
	} else {
		_, _ = fmt.Fprintln(out, "[✓] hooks wired into ~/.claude/settings.json")
	}

	// 5. Restore Copilot symlink.
	copilotLink := filepath.Join(home, ".copilot", "instructions", "constitution.md")
	runtimeMD := filepath.Join(aiRoot, "Constitution.runtime.md")
	_ = os.MkdirAll(filepath.Dir(copilotLink), 0o750)
	_ = os.Remove(copilotLink)
	if err := os.Symlink(runtimeMD, copilotLink); err != nil {
		_, _ = fmt.Fprintf(out, "[!] Copilot symlink failed: %v\n", err)
	} else {
		_, _ = fmt.Fprintln(out, "[✓] Copilot symlink restored")
	}

	// 6. Regenerate runtime.
	constitutionContent, err := os.ReadFile(filepath.Join(aiRoot, "Constitution.md")) //nolint:gosec
	if err == nil {
		if rc, err := constitution.ExtractRuntime(string(constitutionContent)); err == nil {
			runtimeOut := constitution.FormatRuntime(rc)
			_ = os.WriteFile(runtimeMD, []byte(runtimeOut), 0o600) //nolint:gosec
			_, _ = fmt.Fprintln(out, "[✓] Constitution.runtime.md regenerated")
		}
	}

	_, _ = fmt.Fprintf(out, "\nRestore complete from backup: %s\n", filepath.Base(backupDir))
	_, _ = fmt.Fprintln(out, "Start a new Claude Code session to load the restored constitution.")
	return nil
}

// resolveBackupDir finds the target backup directory.
func resolveBackupDir(backupsRoot, backupID string) (string, error) {
	if backupID != "" {
		dir := filepath.Join(backupsRoot, backupID)
		if _, err := os.Stat(dir); err != nil {
			return "", fmt.Errorf("backup %q not found in %s", backupID, backupsRoot)
		}
		return dir, nil
	}
	// Latest backup = lexicographically last entry (UTC timestamps sort correctly).
	entries, err := os.ReadDir(backupsRoot)
	if err != nil || len(entries) == 0 {
		return "", fmt.Errorf("no backups found in %s — run 'ai constitution backup' first", backupsRoot)
	}
	var dirs []string
	for _, e := range entries {
		if e.IsDir() {
			dirs = append(dirs, e.Name())
		}
	}
	if len(dirs) == 0 {
		return "", fmt.Errorf("no backups found in %s", backupsRoot)
	}
	sort.Strings(dirs)
	return filepath.Join(backupsRoot, dirs[len(dirs)-1]), nil
}

// rewireHooks writes the canonical hooks block into settings.json.
func rewireHooks(settingsPath, aiRoot string) error {
	hooksDir := filepath.Join(aiRoot, "hooks")
	hookEntry := func(name string) map[string]any {
		return map[string]any{"type": "command", "command": filepath.Join(hooksDir, name)}
	}
	hooksBlock := map[string]any{
		"SessionStart":    []any{hookEntry("audit.py")},
		"UserPromptSubmit": []any{hookEntry("audit.py")},
		"PreToolUse":      []any{hookEntry("branch-guard.py"), hookEntry("secret-block.py"), hookEntry("worktree-guard.py")},
		"PostToolUse":     []any{hookEntry("audit.py")},
		"Stop":            []any{hookEntry("audit.py"), hookEntry("checkpoint-tick.py")},
		"SessionEnd":      []any{hookEntry("audit.py")},
		"SubagentStop":    []any{hookEntry("audit.py")},
		"PreCompact":      []any{hookEntry("audit.py")},
	}

	var settings map[string]any
	if data, err := os.ReadFile(settingsPath); err == nil { //nolint:gosec
		_ = json.Unmarshal(data, &settings)
	}
	if settings == nil {
		settings = make(map[string]any)
	}
	settings["hooks"] = hooksBlock

	updated, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(settingsPath, updated, 0o600) //nolint:gosec
}

// copyFileSimple copies src to dst, creating dst's directory if needed.
func copyFileSimple(src, dst string) error {
	data, err := os.ReadFile(src) //nolint:gosec
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o750); err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o600) //nolint:gosec
}

