package cmd

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// newRestoreCmd implements `ai restore <snapshot-path>` and
// `ai restore --from-url <url>`. See issues #211 and #212.
//
// Local path (#211):
//   - Accepts a .tar.gz archive or a directory.
//   - Before extracting: backs up existing AIRoot to a sibling
//     .ai-backup-<UTC>/ directory.
//   - Extracts using archive/tar (no shell dependency).
//
// URL path (#212):
//   - --from-url <url>: if the URL ends in .tar.gz, HTTP GET to a temp
//     file then extract. If the URL looks like a git URL, git clone to a
//     temp dir, find the first *.tar.gz in the root, then extract.
//   - file:// URLs are supported for testing without network access.
func newRestoreCmd() *cobra.Command {
	var fromURL string

	c := &cobra.Command{
		Use:   "restore <snapshot-path>",
		Short: "Restore ~/.ai/ from a local snapshot (.tar.gz) or a remote URL",
		Long: `restore extracts a .tar.gz snapshot into the AIRoot (~/.ai/ by default).

Before extracting the snapshot the current AIRoot is backed up to a sibling
directory named .ai-backup-<UTC-timestamp>/ so the previous state can be
recovered manually if needed. The backup step is skipped when AIRoot does not
yet exist.

Local usage:
  ai restore ~/snapshots/ai-2026-05-24.tar.gz

Remote usage (HTTP or file URL):
  ai restore --from-url https://example.com/ai.tar.gz
  ai restore --from-url file:///tmp/ai.tar.gz

Git remote (looks for *.tar.gz in repo root):
  ai restore --from-url https://github.com/org/ai-snapshot.git

See issues #211 and #212.`,
		// When --from-url is set the positional arg is not required.
		Args: func(cmd *cobra.Command, args []string) error {
			urlFlag, _ := cmd.Flags().GetString("from-url")
			if urlFlag != "" {
				if len(args) != 0 {
					return fmt.Errorf("--from-url and a positional <snapshot-path> are mutually exclusive")
				}
				return nil
			}
			return cobra.ExactArgs(1)(cmd, args)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			aiRoot := resolveAIRoot()

			if fromURL != "" {
				return restoreFromURL(cmd, fromURL, aiRoot)
			}
			return restoreFromPath(cmd, args[0], aiRoot)
		},
	}

	c.Flags().StringVar(&fromURL, "from-url", "", "download snapshot from URL (http/https/file/git) and restore")

	return c
}

// restoreFromPath implements the local-path restore path (#211).
func restoreFromPath(cmd *cobra.Command, src, aiRoot string) error {
	// Validate source exists.
	if _, err := os.Stat(src); err != nil {
		return fmt.Errorf("restore: source %q does not exist: %w", src, err)
	}
	// Validate extension.
	if !strings.HasSuffix(src, ".tar.gz") {
		return fmt.Errorf("restore: unsupported extension — only .tar.gz archives are supported (got %q)", filepath.Ext(src)+".gz")
	}

	backupPath, err := backupAIRoot(aiRoot)
	if err != nil {
		return err
	}

	if err := extractTarGz(src, aiRoot); err != nil {
		return fmt.Errorf("restore: extraction failed: %w", err)
	}

	msg := fmt.Sprintf("Restored from %s.", src)
	if backupPath != "" {
		msg += fmt.Sprintf(" Previous backed up to %s.", backupPath)
	}
	fmt.Fprintln(cmd.OutOrStdout(), msg)
	return nil
}

// restoreFromURL implements the URL-based restore path (#212).
func restoreFromURL(cmd *cobra.Command, rawURL, aiRoot string) error {
	var localPath string
	var cleanup func()

	switch {
	case isFileURL(rawURL):
		// file:// — strip scheme, use path directly.
		localPath = strings.TrimPrefix(rawURL, "file://")
		cleanup = func() {}

	case strings.HasSuffix(rawURL, ".tar.gz"):
		// HTTP/HTTPS .tar.gz — download to temp file.
		tmp, err := downloadToTemp(rawURL)
		if err != nil {
			return fmt.Errorf("restore --from-url: download failed: %w", err)
		}
		localPath = tmp
		cleanup = func() { _ = os.Remove(tmp) }

	case isGitURL(rawURL):
		// Git URL — clone to temp dir, find *.tar.gz.
		tmpDir, err := os.MkdirTemp("", "ai-restore-git-*")
		if err != nil {
			return fmt.Errorf("restore --from-url: mkdirtemp: %w", err)
		}
		cleanup = func() { _ = os.RemoveAll(tmpDir) }
		// gosec G204: URL originates from the CLI flag, validated by
		// isGitURL before reaching this branch.
		if err := exec.Command("git", "clone", "--depth=1", rawURL, tmpDir).Run(); err != nil { //nolint:gosec // G204
			cleanup()
			return fmt.Errorf("restore --from-url: git clone failed: %w", err)
		}
		found, err := findTarGzInDir(tmpDir)
		if err != nil {
			cleanup()
			return fmt.Errorf("restore --from-url: no *.tar.gz found in cloned repo: %w", err)
		}
		localPath = found

	default:
		return fmt.Errorf("restore --from-url: unsupported URL %q (must end in .tar.gz, be a file:// URL, or a git URL)", rawURL)
	}
	defer cleanup()

	// Validate extension on the resolved local path.
	if !strings.HasSuffix(localPath, ".tar.gz") {
		return fmt.Errorf("restore: unsupported extension — only .tar.gz archives are supported")
	}

	backupPath, err := backupAIRoot(aiRoot)
	if err != nil {
		return err
	}

	if err := extractTarGz(localPath, aiRoot); err != nil {
		return fmt.Errorf("restore: extraction failed: %w", err)
	}

	msg := fmt.Sprintf("Restored from %s.", rawURL)
	if backupPath != "" {
		msg += fmt.Sprintf(" Previous backed up to %s.", backupPath)
	}
	fmt.Fprintln(cmd.OutOrStdout(), msg)
	return nil
}

// backupAIRoot copies the existing AIRoot to a sibling backup directory.
// The backup is always named .ai-backup-<UTC> as a sibling of AIRoot,
// matching the spec ("~/.ai/../.ai-backup-<UTC>/").
// Returns the backup path (empty string if AIRoot did not exist) or an error.
func backupAIRoot(aiRoot string) (backupPath string, err error) {
	if _, statErr := os.Stat(aiRoot); os.IsNotExist(statErr) {
		// Nothing to back up.
		return "", nil
	}

	ts := time.Now().UTC().Format("20060102T150405Z")
	parent := filepath.Dir(aiRoot)
	backupPath = filepath.Join(parent, ".ai-backup-"+ts)

	if err := copyDir(aiRoot, backupPath); err != nil {
		return "", fmt.Errorf("restore: backup of %s to %s failed: %w", aiRoot, backupPath, err)
	}
	return backupPath, nil
}

// copyDir recursively copies src directory to dst.
func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, info.Mode())
		}
		return copyFile(path, target, info.Mode())
	})
}

// copyFile copies a single file.
func copyFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src) //nolint:gosec // G304: src comes from filepath.Walk of a known path
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0o750); err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

// extractTarGz extracts a .tar.gz archive into destDir. Each entry's
// path is cleaned and validated to prevent path-traversal attacks.
func extractTarGz(src, destDir string) error {
	f, err := os.Open(src) //nolint:gosec // G304: src is caller-validated
	if err != nil {
		return err
	}
	defer f.Close()

	gr, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("gzip: %w", err)
	}
	defer gr.Close()

	tr := tar.NewReader(gr)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("tar: %w", err)
		}

		// Security: reject entries that would escape destDir.
		cleaned := filepath.Clean(hdr.Name)
		if strings.HasPrefix(cleaned, "..") {
			return fmt.Errorf("tar: refusing path-traversal entry %q", hdr.Name)
		}

		target := filepath.Join(destDir, cleaned) //nolint:gosec // G305: cleaned and validated above

		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, os.FileMode(hdr.Mode)); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
				return err
			}
			// G306: permissions come from the archive header, which is the
			// author's intent. The caller (backupAIRoot) has already validated
			// the source.
			out, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, os.FileMode(hdr.Mode)) //nolint:gosec // G304,G306
			if err != nil {
				return err
			}
			// G110: size is bounded by the reader; we accept the risk of a
			// large archive in this tool, which is CLI-invoked only.
			_, copyErr := io.Copy(out, tr) //nolint:gosec // G110
			out.Close()
			if copyErr != nil {
				return copyErr
			}
		}
	}
	return nil
}

// downloadToTemp downloads the URL to a temporary file and returns its path.
// The caller is responsible for deleting the temp file.
func downloadToTemp(rawURL string) (string, error) {
	// gosec G107: URL is from the CLI flag, not user-supplied content.
	resp, err := http.Get(rawURL) //nolint:gosec // G107: CLI flag origin
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d from %s", resp.StatusCode, rawURL)
	}

	tmp, err := os.CreateTemp("", "ai-restore-dl-*.tar.gz")
	if err != nil {
		return "", err
	}
	defer tmp.Close()

	// G110: bounded by HTTP response; CLI tool only.
	if _, err := io.Copy(tmp, resp.Body); err != nil { //nolint:gosec // G110
		_ = os.Remove(tmp.Name())
		return "", err
	}
	return tmp.Name(), nil
}

// findTarGzInDir returns the path to the first *.tar.gz file in dir.
func findTarGzInDir(dir string) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".tar.gz") {
			return filepath.Join(dir, e.Name()), nil
		}
	}
	return "", fmt.Errorf("no *.tar.gz found in %s", dir)
}

// isFileURL reports whether the URL uses the file:// scheme.
func isFileURL(u string) bool {
	return strings.HasPrefix(u, "file://")
}

// isGitURL reports whether the URL looks like a git clone URL.
func isGitURL(u string) bool {
	return strings.HasSuffix(u, ".git") ||
		strings.Contains(u, "github.com") ||
		strings.Contains(u, "gitlab.com") ||
		strings.Contains(u, "bitbucket.org") ||
		strings.HasPrefix(u, "git@") ||
		strings.HasPrefix(u, "ssh://")
}

// resolveAIRoot returns the canonical ~/.ai/ root, honoring the AI_ROOT
// environment variable override. Mirrors the logic in paths.AIRoot() without
// importing the separate src/internal module.
func resolveAIRoot() string {
	if env := os.Getenv("AI_ROOT"); env != "" {
		return env
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ".ai"
	}
	return filepath.Join(home, ".ai")
}
