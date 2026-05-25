package cmd

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/convergent-systems-co/aiConstitution/src/internal/paths"
	"github.com/convergent-systems-co/aiConstitution/src/internal/plugins"
	"github.com/spf13/cobra"
)

// pluginsState is the JSON schema for ~/.config/aiConstitution/plugins.json.
// It tracks which installed plugins are currently enabled.
type pluginsState struct {
	Enabled []string `json:"enabled"`
}

// newPluginsCmd implements `ai plugins {list,install,enable,disable,status,update}`.
// See SPEC.md §11.
func newPluginsCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "plugins",
		Short: "Manage Claude plugins that extend the agent's workflow surface",
		Long: `plugins are Claude-specific extensions (e.g., superpowers,
amendment-author, hook-author, atom-publisher, review-panel,
memory-curator) that wrap CLI verbs in guided multi-step workflows.

See SPEC.md §11.`,
	}

	c.AddCommand(
		&cobra.Command{
			Use:   "list",
			Short: "Show available + installed Claude plugins",
			RunE: func(cmd *cobra.Command, _ []string) error {
				notice("plugins list")
				return stub("plugins list", "§11")
			},
		},
		newPluginsInstallCmd(),
		newPluginsEnableCmd(),
		newPluginsDisableCmd(),
		newPluginsStatusCmd(),
		newPluginsUpdateCmd(),
	)
	return c
}

// newPluginsInstallCmd implements `ai plugins install <url-or-path> [--force]`.
//
// Accepts an HTTPS URL (*.tar.gz) or a local path. Downloads or copies
// to a temp location, unpacks, validates manifest.yaml, then installs to
// ~/.ai/plugins/<name>/. Errors if already installed and --force is not set.
func newPluginsInstallCmd() *cobra.Command {
	var force bool

	c := &cobra.Command{
		Use:   "install <url-or-path>",
		Short: "Install a plugin from a URL (*.tar.gz) or local path",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPluginsInstall(cmd, args[0], force)
		},
	}
	c.Flags().BoolVar(&force, "force", false, "reinstall even if already present")
	return c
}

// runPluginsInstall is the install dispatcher. Extracted to keep the
// cobra constructor under cyclomatic-complexity limits.
func runPluginsInstall(cmd *cobra.Command, source string, force bool) error {
	// Fetch the archive to a temp file.
	tmpFile, err := fetchArchive(source)
	if err != nil {
		return err
	}
	defer os.Remove(tmpFile)

	// Unpack to a temp staging directory.
	stageDir, err := os.MkdirTemp("", "ai-plugin-stage-*")
	if err != nil {
		return fmt.Errorf("plugins install: create staging dir: %w", err)
	}
	defer os.RemoveAll(stageDir)

	if err := pluginExtractTarGz(tmpFile, stageDir); err != nil {
		return fmt.Errorf("plugins install: extract archive: %w", err)
	}

	// Locate the plugin name by finding a manifest.yaml in the staging area.
	manifest, manifestDir, err := findManifestInDir(stageDir)
	if err != nil {
		return fmt.Errorf("plugins install: %w", err)
	}

	pluginsDir := paths.PluginsDir()
	destDir := filepath.Join(pluginsDir, manifest.Name)

	if _, err := os.Stat(destDir); err == nil {
		// Plugin directory already exists.
		if !force {
			return fmt.Errorf("plugins install: %q is already installed — use --force to reinstall", manifest.Name)
		}
		if err := os.RemoveAll(destDir); err != nil {
			return fmt.Errorf("plugins install: remove existing: %w", err)
		}
	}

	if err := os.MkdirAll(filepath.Dir(destDir), 0o750); err != nil {
		return fmt.Errorf("plugins install: create plugins dir: %w", err)
	}

	// Move the staged plugin directory into place.
	if err := os.Rename(manifestDir, destDir); err != nil {
		// Rename across filesystems (e.g. tmpfs → home) falls back to copy.
		if err2 := pluginCopyDir(manifestDir, destDir); err2 != nil {
			return fmt.Errorf("plugins install: copy to plugins dir: %w", err2)
		}
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Installed %s v%s\n", manifest.Name, manifest.Version)
	return nil
}

// newPluginsEnableCmd implements `ai plugins enable <name>`.
func newPluginsEnableCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "enable <name>",
		Short: "Enable an installed plugin",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPluginsEnable(args[0])
		},
	}
}

func runPluginsEnable(name string) error {
	// Verify the plugin is installed.
	pluginDir := filepath.Join(paths.PluginsDir(), name)
	if _, err := os.Stat(pluginDir); errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("plugins enable: %q is not installed — run `ai plugins install` first", name)
	}

	state, err := loadPluginsState()
	if err != nil {
		return err
	}

	// Idempotent — do not add duplicates.
	for _, n := range state.Enabled {
		if n == name {
			return nil
		}
	}
	state.Enabled = append(state.Enabled, name)
	return savePluginsState(state)
}

// newPluginsDisableCmd implements `ai plugins disable <name>`.
func newPluginsDisableCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "disable <name>",
		Short: "Disable a plugin without uninstalling",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPluginsDisable(args[0])
		},
	}
}

func runPluginsDisable(name string) error {
	state, err := loadPluginsState()
	if err != nil {
		return err
	}

	newEnabled := make([]string, 0, len(state.Enabled))
	found := false
	for _, n := range state.Enabled {
		if n == name {
			found = true
			continue
		}
		newEnabled = append(newEnabled, n)
	}
	if !found {
		return fmt.Errorf("plugins disable: %q is not in the enabled list", name)
	}

	state.Enabled = newEnabled
	return savePluginsState(state)
}

// newPluginsStatusCmd implements `ai plugins status`.
func newPluginsStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "List all installed plugins with enabled/disabled marker",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runPluginsStatus(cmd)
		},
	}
}

func runPluginsStatus(cmd *cobra.Command) error {
	pluginsDir := paths.PluginsDir()
	entries, err := os.ReadDir(pluginsDir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			fmt.Fprintln(cmd.OutOrStdout(), "(no plugins installed)")
			return nil
		}
		return fmt.Errorf("plugins status: read plugins dir: %w", err)
	}

	// Filter to directories that have a manifest.yaml.
	var installed []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		manifestPath := filepath.Join(pluginsDir, e.Name(), "manifest.yaml")
		if _, err := os.Stat(manifestPath); err == nil {
			installed = append(installed, e.Name())
		}
	}

	if len(installed) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "(no plugins installed)")
		return nil
	}

	state, err := loadPluginsState()
	if err != nil {
		return err
	}
	enabledSet := make(map[string]bool, len(state.Enabled))
	for _, n := range state.Enabled {
		enabledSet[n] = true
	}

	for _, name := range installed {
		manifestPath := filepath.Join(pluginsDir, name, "manifest.yaml")
		m, err := plugins.ParseManifest(manifestPath)
		if err != nil {
			// Manifest is unreadable; surface the anomaly but continue.
			fmt.Fprintf(cmd.OutOrStdout(), "  %-30s  [invalid manifest: %v]\n", name, err)
			continue
		}
		status := "disabled"
		if enabledSet[name] {
			status = "enabled"
		}
		fmt.Fprintf(cmd.OutOrStdout(), "  %-30s  v%-10s  %s\n", m.Name, m.Version, status)
	}
	return nil
}

// newPluginsUpdateCmd implements `ai plugins update <name>`.
func newPluginsUpdateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "update <name>",
		Short: "Update an installed plugin from its source URL",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPluginsUpdate(cmd, args[0])
		},
	}
}

func runPluginsUpdate(cmd *cobra.Command, name string) error {
	pluginDir := filepath.Join(paths.PluginsDir(), name)
	manifestPath := filepath.Join(pluginDir, "manifest.yaml")

	if _, err := os.Stat(pluginDir); errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("plugins update: %q is not installed", name)
	}

	m, err := plugins.ParseManifest(manifestPath)
	if err != nil {
		return fmt.Errorf("plugins update: read current manifest: %w", err)
	}
	if m.Source == "" {
		return fmt.Errorf("plugins update: %q has no source URL in manifest — cannot update automatically", name)
	}

	oldVersion := m.Version

	// Re-install using --force, which re-downloads and replaces the dir.
	if err := runPluginsInstall(cmd, m.Source, true); err != nil {
		return fmt.Errorf("plugins update: reinstall: %w", err)
	}

	// Read the new manifest to get the updated version.
	newManifest, err := plugins.ParseManifest(manifestPath)
	if err != nil {
		// Install succeeded but we can't read the new version; non-fatal.
		fmt.Fprintf(cmd.OutOrStdout(), "Updated %s: %s → (unknown)\n", name, oldVersion)
		return nil
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Updated %s: %s → %s\n", name, oldVersion, newManifest.Version)
	return nil
}

// --- helpers ----------------------------------------------------------------

// fetchArchive downloads or copies a plugin archive (*.tar.gz) to a temp file.
// Source may be an HTTPS URL or a local filesystem path.
func fetchArchive(source string) (tmpPath string, err error) {
	tmp, err := os.CreateTemp("", "ai-plugin-*.tar.gz")
	if err != nil {
		return "", fmt.Errorf("plugins: create temp file: %w", err)
	}
	defer func() {
		if err != nil {
			os.Remove(tmp.Name())
		}
	}()
	defer tmp.Close()

	if strings.HasPrefix(source, "https://") || strings.HasPrefix(source, "http://") {
		resp, err := http.Get(source) //nolint:noctx // simple CLI fetch
		if err != nil {
			return "", fmt.Errorf("plugins: download %q: %w", source, err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return "", fmt.Errorf("plugins: download %q: HTTP %d", source, resp.StatusCode)
		}
		if _, err := io.Copy(tmp, resp.Body); err != nil {
			return "", fmt.Errorf("plugins: download %q: %w", source, err)
		}
	} else {
		// Local path: copy to temp so the caller can delete uniformly.
		src, err := os.Open(source)
		if err != nil {
			return "", fmt.Errorf("plugins: open %q: %w", source, err)
		}
		defer src.Close()
		if _, err := io.Copy(tmp, src); err != nil {
			return "", fmt.Errorf("plugins: copy %q: %w", source, err)
		}
	}

	return tmp.Name(), nil
}

// extractTarGz unpacks a .tar.gz archive into destDir.
// Rejects entries whose paths contain ".." (path traversal guard).
func pluginExtractTarGz(archivePath, destDir string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("open archive: %w", err)
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("gzip reader: %w", err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return fmt.Errorf("read tar: %w", err)
		}

		// Path traversal guard: reject any entry that would escape destDir.
		clean := filepath.Clean(hdr.Name)
		if strings.HasPrefix(clean, "..") || strings.Contains(clean, "../") {
			return fmt.Errorf("archive contains unsafe path: %q", hdr.Name)
		}

		target := filepath.Join(destDir, clean)

		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o750); err != nil {
				return fmt.Errorf("create dir %q: %w", target, err)
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
				return fmt.Errorf("create parent dir %q: %w", target, err)
			}
			out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(hdr.Mode)&0o755) //nolint:gosec // G304: tar extract
			if err != nil {
				return fmt.Errorf("create file %q: %w", target, err)
			}
			if _, err := io.Copy(out, tr); err != nil { //nolint:gosec // G110: controlled tar extraction
				out.Close()
				return fmt.Errorf("write file %q: %w", target, err)
			}
			out.Close()
		}
	}
	return nil
}

// findManifestInDir locates the first manifest.yaml in the staging dir,
// returning the parsed manifest and the directory that contains it.
func findManifestInDir(stageDir string) (*plugins.PluginManifest, string, error) {
	var found *plugins.PluginManifest
	var foundDir string

	err := filepath.WalkDir(stageDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || found != nil {
			return err
		}
		if d.IsDir() || d.Name() != "manifest.yaml" {
			return nil
		}
		m, parseErr := plugins.ParseManifest(path)
		if parseErr != nil {
			return parseErr
		}
		found = m
		foundDir = filepath.Dir(path)
		return nil
	})
	if err != nil {
		return nil, "", fmt.Errorf("scan archive for manifest.yaml: %w", err)
	}
	if found == nil {
		return nil, "", errors.New("archive does not contain a manifest.yaml")
	}
	return found, foundDir, nil
}

// loadPluginsState reads ~/.config/aiConstitution/plugins.json.
// Returns an empty state if the file does not exist.
func loadPluginsState() (pluginsState, error) {
	stateFile := paths.PluginsStateFile()
	data, err := os.ReadFile(stateFile)
	if errors.Is(err, fs.ErrNotExist) {
		return pluginsState{Enabled: []string{}}, nil
	}
	if err != nil {
		return pluginsState{}, fmt.Errorf("plugins: read state file: %w", err)
	}

	var state pluginsState
	if err := json.Unmarshal(data, &state); err != nil {
		return pluginsState{}, fmt.Errorf("plugins: parse state file: %w", err)
	}
	if state.Enabled == nil {
		state.Enabled = []string{}
	}
	return state, nil
}

// savePluginsState writes plugins state to ~/.config/aiConstitution/plugins.json
// atomically (temp file + rename).
func savePluginsState(state pluginsState) error {
	stateFile := paths.PluginsStateFile()
	if err := os.MkdirAll(filepath.Dir(stateFile), 0o750); err != nil {
		return fmt.Errorf("plugins: create config dir: %w", err)
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("plugins: marshal state: %w", err)
	}

	// Atomic write: temp file in same directory, then rename.
	tmp, err := os.CreateTemp(filepath.Dir(stateFile), ".plugins-*.json")
	if err != nil {
		return fmt.Errorf("plugins: create temp state file: %w", err)
	}
	tmpName := tmp.Name()
	defer func() {
		// Clean up on failure.
		if _, err := os.Stat(tmpName); err == nil {
			os.Remove(tmpName)
		}
	}()

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("plugins: write state: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("plugins: close state file: %w", err)
	}
	if err := os.Rename(tmpName, stateFile); err != nil {
		return fmt.Errorf("plugins: commit state file: %w", err)
	}
	return nil
}

// copyDir recursively copies src to dst for cross-device rename fallback.
func pluginCopyDir(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o750)
		}
		return pluginCopyFile(path, target)
	})
}

// copyFile copies a single file from src to dst, preserving mode bits.
func pluginCopyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	info, err := srcFile.Stat()
	if err != nil {
		return err
	}

	dstFile, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode()) //nolint:gosec // G304: known-safe copy
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	return err
}
