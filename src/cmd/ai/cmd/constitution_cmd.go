package cmd

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
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

// backupManifest records the backup metadata.
type backupManifest struct {
	CreatedAt    string `json:"created_at"`
	AIRoot       string `json:"ai_root"`
	ArchivePath  string `json:"archive_path"`
	ClaudeMDPath string `json:"claude_md_path"`
	ClearLinks   bool   `json:"clear_links_applied"`
}

// backupRoot is where backups are stored — outside ~/.ai/ so restoration
// doesn't overwrite them.
func backupRoot(home string) string {
	return filepath.Join(home, ".ai-backups")
}

func newConstitutionCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "constitution",
		Short: "Backup and restore the entire ~/.ai/ directory and tool wiring",
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
		Short: "Archive all of ~/.ai/ to ~/.ai-backups/<UTC>.tar.gz",
		Long: `backup creates a full tar.gz archive of the entire ~/.ai/ directory,
capturing your constitution, memory, skills, hooks, and governance files.

The archive is stored in ~/.ai-backups/ which lives outside ~/.ai/ so
it survives a restore operation.

With --clear-links, tool wiring is removed after the backup so you can
test a different constitution in a clean session. Run 'ai constitution
restore' to extract the archive and re-establish all links.`,
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

	// Create ~/.ai-backups/
	bRoot := backupRoot(home)
	if err := os.MkdirAll(bRoot, 0o750); err != nil {
		return fmt.Errorf("constitution backup: mkdir backup root: %w", err)
	}

	ts := time.Now().UTC().Format("20060102T150405Z")
	archivePath := filepath.Join(bRoot, ts+".tar.gz")

	// Archive all of ~/.ai/
	if err := tarGzDir(aiRoot, archivePath); err != nil {
		return fmt.Errorf("constitution backup: archive: %w", err)
	}

	// Write manifest alongside the archive.
	claudeMD := filepath.Join(home, ".claude", "CLAUDE.md")
	manifest := backupManifest{
		CreatedAt:    time.Now().UTC().Format(time.RFC3339),
		AIRoot:       aiRoot,
		ArchivePath:  archivePath,
		ClaudeMDPath: claudeMD,
		ClearLinks:   clearLinks,
	}
	manifestBytes, _ := json.MarshalIndent(manifest, "", "  ")
	manifestPath := filepath.Join(bRoot, ts+"-manifest.json")
	if err := os.WriteFile(manifestPath, manifestBytes, 0o600); err != nil { //nolint:gosec
		return fmt.Errorf("constitution backup: write manifest: %w", err)
	}

	fi, _ := os.Stat(archivePath)
	size := int64(0)
	if fi != nil {
		size = fi.Size()
	}
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Backup: %s (%d KB)\n", archivePath, size/1024)

	if clearLinks {
		if err := clearConstitutionLinks(home, claudeMD,
			filepath.Join(home, ".copilot", "instructions", "constitution.md")); err != nil {
			return fmt.Errorf("constitution backup --clear-links: %w", err)
		}
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Links cleared. Run 'ai setup' to wire a new constitution.")
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Run 'ai constitution restore' to restore from backup.")
	}

	return nil
}

// tarGzDir creates a gzip-compressed tar archive of src at dst.
// Excludes: .git/, audit/interactions/ (large JSONL logs).
func tarGzDir(src, dst string) error {
	f, err := os.Create(dst) //nolint:gosec
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	gz := gzip.NewWriter(f)
	defer func() { _ = gz.Close() }()
	tw := tar.NewWriter(gz)
	defer func() { _ = tw.Close() }()

	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		// Skip git internals and large audit interaction logs.
		if rel == ".git" || strings.HasPrefix(rel, ".git/") {
			return filepath.SkipDir
		}
		if rel == "audit/interactions" || strings.HasPrefix(rel, "audit/interactions/") {
			return filepath.SkipDir
		}

		// Read file content first so we have the exact size before writing
		// the tar header. This prevents "write too long" when a file grows
		// between stat time and read time (e.g. audit logs, node_modules).
		if info.Mode()&os.ModeSymlink != 0 || info.IsDir() {
			hdr, err := tar.FileInfoHeader(info, "")
			if err != nil {
				return err
			}
			hdr.Name = rel
			return tw.WriteHeader(hdr)
		}
		data, err := os.ReadFile(path) //nolint:gosec
		if err != nil {
			// Skip unreadable files (e.g. sockets, devices).
			return nil //nolint:nilerr
		}
		hdr := &tar.Header{
			Name:    rel,
			Mode:    int64(info.Mode()),
			Size:    int64(len(data)),
			ModTime: info.ModTime(),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		_, err = tw.Write(data)
		return err
	})
}

// clearConstitutionLinks removes the live tool wiring.
func clearConstitutionLinks(home, claudeMD, copilotLink string) error {
	// Strip @~/.ai/ lines from CLAUDE.md.
	if data, err := os.ReadFile(claudeMD); err == nil { //nolint:gosec
		var kept []string
		for _, line := range strings.Split(string(data), "\n") {
			if !strings.HasPrefix(line, "@~/.ai/") {
				kept = append(kept, line)
			}
		}
		_ = os.WriteFile(claudeMD, []byte(strings.Join(kept, "\n")), 0o600) //nolint:gosec
	}

	// Remove Copilot symlink.
	_ = os.Remove(copilotLink)

	// Zero the hooks block in ~/.claude/settings.json.
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
	var backupTS string

	c := &cobra.Command{
		Use:   "restore [backup-id]",
		Short: "Extract the latest ~/.ai-backups/*.tar.gz back to ~/.ai/ and re-wire tools",
		Long: `restore extracts the most recent (or specified) backup archive back
into ~/.ai/, replacing whatever is there now. After extraction it:

  1. Re-installs hooks from the embedded binary
  2. Re-wires hooks into ~/.claude/settings.json
  3. Restores ~/.claude/CLAUDE.md with @-include directive
  4. Recreates the ~/.copilot/instructions/constitution.md symlink
  5. Regenerates Constitution.runtime.md`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Positional arg takes precedence over --backup flag.
			id := backupTS
			if len(args) > 0 {
				id = strings.TrimSuffix(args[0], ".tar.gz")
			}
			return runConstitutionRestore(cmd, id)
		},
	}
	c.Flags().StringVar(&backupTS, "backup", "",
		"timestamp ID to restore (e.g. 20260524T120000Z); defaults to most recent")
	return c
}

func runConstitutionRestore(cmd *cobra.Command, backupTS string) error {
	aiRoot := paths.AIRoot()
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("constitution restore: %w", err)
	}

	bRoot := backupRoot(home)
	archivePath, err := resolveBackupArchive(bRoot, backupTS)
	if err != nil {
		return fmt.Errorf("constitution restore: %w", err)
	}

	out := cmd.OutOrStdout()
	_, _ = fmt.Fprintf(out, "Restoring from: %s\n", archivePath)

	// 1. Clear ~/.ai/ and extract archive.
	if err := os.RemoveAll(aiRoot); err != nil {
		return fmt.Errorf("constitution restore: remove existing ~/.ai/: %w", err)
	}
	if err := os.MkdirAll(aiRoot, 0o750); err != nil {
		return fmt.Errorf("constitution restore: mkdir: %w", err)
	}
	if err := extractTarGzTo(archivePath, aiRoot); err != nil {
		return fmt.Errorf("constitution restore: extract: %w", err)
	}
	_, _ = fmt.Fprintln(out, "[✓] ~/.ai/ restored from archive")

	// 2. Re-install hooks from embedded binary (overwrite what came from backup).
	hooksDir := filepath.Join(aiRoot, "hooks")
	if written, err := embed.ExtractAllHooks(hooksDir, true); err != nil {
		_, _ = fmt.Fprintf(out, "[!] hooks install failed: %v\n", err)
	} else {
		_, _ = fmt.Fprintf(out, "[✓] %d hooks reinstalled\n", len(written))
	}

	// 3. Re-wire hooks into settings.json.
	settingsPath := filepath.Join(home, ".claude", "settings.json")
	if err := rewireHooks(settingsPath, aiRoot); err != nil {
		_, _ = fmt.Fprintf(out, "[!] settings.json wiring: %v\n", err)
	} else {
		_, _ = fmt.Fprintln(out, "[✓] hooks wired into ~/.claude/settings.json")
	}

	// 4. Restore CLAUDE.md @-include.
	claudeMD := filepath.Join(home, ".claude", "CLAUDE.md")
	if err := writeClaudeMDInclude(claudeMD, aiRoot); err != nil {
		_, _ = fmt.Fprintf(out, "[!] CLAUDE.md: %v\n", err)
	} else {
		_, _ = fmt.Fprintln(out, "[✓] ~/.claude/CLAUDE.md @-include written")
	}

	// 5. Recreate Copilot symlink.
	runtimeMD := filepath.Join(aiRoot, "Constitution.runtime.md")
	copilotLink := filepath.Join(home, ".copilot", "instructions", "constitution.md")
	_ = os.MkdirAll(filepath.Dir(copilotLink), 0o750)
	_ = os.Remove(copilotLink)
	if err := os.Symlink(runtimeMD, copilotLink); err != nil {
		_, _ = fmt.Fprintf(out, "[!] Copilot symlink: %v\n", err)
	} else {
		_, _ = fmt.Fprintln(out, "[✓] Copilot symlink recreated")
	}

	// 6. Regenerate Constitution.runtime.md.
	if data, err := os.ReadFile(filepath.Join(aiRoot, "Constitution.md")); err == nil { //nolint:gosec
		if rc, err := constitution.ExtractRuntime(string(data)); err == nil {
			_ = os.WriteFile(runtimeMD, []byte(constitution.FormatRuntime(rc)), 0o600) //nolint:gosec
			_, _ = fmt.Fprintln(out, "[✓] Constitution.runtime.md regenerated")
		}
	}

	_, _ = fmt.Fprintln(out, "\nRestore complete. Start a new Claude Code session to load the restored constitution.")
	return nil
}

// writeClaudeMDInclude writes (or idempotently updates) ~/.claude/CLAUDE.md
// to contain the @~/.ai/Constitution.md @-include.
func writeClaudeMDInclude(claudeMD, aiRoot string) error {
	include := "@~/.ai/Constitution.md\n"
	data, err := os.ReadFile(claudeMD) //nolint:gosec
	if err == nil && strings.Contains(string(data), "@~/.ai/Constitution.md") {
		return nil // already present
	}
	_ = os.MkdirAll(filepath.Dir(claudeMD), 0o750)
	existing := ""
	if err == nil {
		existing = string(data)
	}
	_ = aiRoot
	return os.WriteFile(claudeMD, []byte(include+existing), 0o600) //nolint:gosec
}

// resolveBackupArchive finds the archive to restore.
func resolveBackupArchive(bRoot, backupTS string) (string, error) {
	if backupTS != "" {
		p := filepath.Join(bRoot, backupTS+".tar.gz")
		if _, err := os.Stat(p); err != nil {
			return "", fmt.Errorf("backup %q not found in %s", backupTS, bRoot)
		}
		return p, nil
	}
	entries, err := os.ReadDir(bRoot)
	if err != nil || len(entries) == 0 {
		return "", fmt.Errorf("no backups found in %s — run 'ai constitution backup' first", bRoot)
	}
	var archives []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".tar.gz") {
			archives = append(archives, e.Name())
		}
	}
	if len(archives) == 0 {
		return "", fmt.Errorf("no .tar.gz backups found in %s", bRoot)
	}
	sort.Strings(archives)
	return filepath.Join(bRoot, archives[len(archives)-1]), nil
}

// extractTarGzTo extracts a tar.gz archive into dst.
func extractTarGzTo(archivePath, dst string) error {
	f, err := os.Open(archivePath) //nolint:gosec
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer func() { _ = gz.Close() }()
	tr := tar.NewReader(gz)

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		// Path traversal guard.
		target := filepath.Join(dst, filepath.Clean("/"+hdr.Name)) //nolint:gosec
		if !strings.HasPrefix(target, filepath.Clean(dst)+string(os.PathSeparator)) {
			continue
		}
		if hdr.Typeflag == tar.TypeDir {
			if err := os.MkdirAll(target, 0o750); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
			return err
		}
		out, err := os.Create(target) //nolint:gosec
		if err != nil {
			return err
		}
		if _, err := io.Copy(out, tr); err != nil { //nolint:gosec
			_ = out.Close()
			return err
		}
		_ = out.Close()
	}
	return nil
}

// rewireHooks writes the canonical hooks block into ~/.claude/settings.json.
func rewireHooks(settingsPath, aiRoot string) error {
	hooksDir := filepath.Join(aiRoot, "hooks")
	hookEntry := func(name string) map[string]any {
		return map[string]any{"type": "command", "command": filepath.Join(hooksDir, name)}
	}
	hooksBlock := map[string]any{
		"SessionStart":     []any{hookEntry("audit.py")},
		"UserPromptSubmit": []any{hookEntry("audit.py")},
		"PreToolUse":       []any{hookEntry("branch-guard.py"), hookEntry("secret-block.py"), hookEntry("worktree-guard.py")},
		"PostToolUse":      []any{hookEntry("audit.py")},
		"Stop":             []any{hookEntry("audit.py"), hookEntry("checkpoint-tick.py")},
		"SessionEnd":       []any{hookEntry("audit.py")},
		"SubagentStop":     []any{hookEntry("audit.py")},
		"PreCompact":       []any{hookEntry("audit.py")},
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

// copyFileSimple copies src to dst.
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
