package cmd

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/convergent-systems-co/aiConstitution/src/internal/paths"

	"github.com/spf13/cobra"
)

// newBackupCmd implements `ai backup`. Mentioned in SPEC.md §11.2 as
// part of the existing/stays-in-CLI set, and invoked transactionally
// by migrations (e.g., §7.9.7 v0.4 → v0.5 atoms migration).
//
// The archive excludes .git/ (recoverable from git remote) and
// audit/interactions/ (local-only per Common.md §5.2 — interaction logs
// can carry sensitive prompt content and are never synced).
func newBackupCmd() *cobra.Command {
	var dest string
	c := &cobra.Command{
		Use:   "backup",
		Short: "Snapshot the canonical tree to a local archive (used by migrations)",
		Long: `backup writes a tarball snapshot of ~/.ai/ (excluding
.git/ and audit/interactions/) to the configured backup directory.
Migrations run this first so a failed migration can be rolled back.

See SPEC.md §11.2 + §7.9.7.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			destDir := dest
			if destDir == "" {
				destDir = filepath.Join(paths.ConfigDir(), "backups")
			}
			if err := os.MkdirAll(destDir, 0o750); err != nil {
				return fmt.Errorf("backup: mkdir %s: %w", destDir, err)
			}
			name := "backup-" + time.Now().UTC().Format("20060102-150405") + ".tar.gz"
			archive := filepath.Join(destDir, name)
			if err := writeBackupArchive(paths.AIRoot(), archive); err != nil {
				return fmt.Errorf("backup: %w", err)
			}
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), archive)
			return nil
		},
	}
	c.Flags().StringVar(&dest, "dest", "", "destination directory (default: ~/.config/aiConstitution/backups/)")
	return c
}

// writeBackupArchive walks aiRoot and writes a gzipped tar to dst. The
// archive omits .git/ and audit/interactions/ subtrees per the rule in
// Common.md §5.2 (interaction logs are local-only) and the convention
// that backup is for governance content, not VCS state.
func writeBackupArchive(aiRoot, dst string) error {
	// 0o600: the archive may contain user-only governance content;
	// the file must not be group/world readable.
	out, err := os.OpenFile(filepath.Clean(dst), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	defer func() { _ = out.Close() }()

	gz, err := gzip.NewWriterLevel(out, gzip.BestCompression)
	if err != nil {
		return err
	}
	defer func() { _ = gz.Close() }()

	tw := tar.NewWriter(gz)
	defer func() { _ = tw.Close() }()

	return filepath.WalkDir(aiRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, relErr := filepath.Rel(aiRoot, path)
		if relErr != nil {
			return relErr
		}
		if rel == "." {
			return nil
		}
		if shouldSkipBackupEntry(rel) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		return writeBackupEntry(tw, path, rel, d)
	})
}

// shouldSkipBackupEntry returns true for paths the backup deliberately
// omits. The rule is path-prefix based on the AI-root-relative path
// (forward-slash normalized) so .git/ and audit/interactions/ are
// detected on both Unix and Windows.
func shouldSkipBackupEntry(rel string) bool {
	normalized := filepath.ToSlash(rel)
	if normalized == ".git" || strings.HasPrefix(normalized, ".git/") {
		return true
	}
	if normalized == "audit/interactions" || strings.HasPrefix(normalized, "audit/interactions/") {
		return true
	}
	return false
}

// writeBackupEntry emits one file or directory header to the tar
// writer. Symlinks and special files are skipped (the canonical tree
// is plain files only; if symlinks appear later they need explicit
// handling).
func writeBackupEntry(tw *tar.Writer, path, rel string, d fs.DirEntry) error {
	info, err := d.Info()
	if err != nil {
		return err
	}
	hdr, err := tar.FileInfoHeader(info, "")
	if err != nil {
		return err
	}
	hdr.Name = filepath.ToSlash(rel)
	if d.IsDir() {
		hdr.Name += "/"
	}
	if err := tw.WriteHeader(hdr); err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return nil
	}
	f, err := os.Open(filepath.Clean(path))
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	_, copyErr := io.Copy(tw, f)
	return copyErr
}
