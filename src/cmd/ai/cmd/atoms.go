package cmd

// atoms.go — `ai atoms` subcommand tree: fetch, fork, publish, list.
//
// Implements #261 (fetch: download + extract), #262 (fetch: TOML manifest +
// atoms.json index), #263 (fork), #264 (publish --dry-run), #265 (list).
//
// Design notes:
//   - All filesystem roots are resolved via paths.AIRoot() / paths.ConfigDir()
//     so tests can redirect via AI_ROOT / AICONST_CONFIG_DIR env vars.
//   - HTTP downloads go to os.CreateTemp so there is no persistent temp file
//     on error paths.
//   - Tar.gz extraction enforces a path-prefix check to prevent directory
//     traversal (#262, §7 security rule).
//   - The gc and verify stubs are preserved from the original scaffold.

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/convergent-systems-co/aiConstitution/src/internal/atoms"
	"github.com/convergent-systems-co/aiConstitution/src/internal/paths"
	"github.com/spf13/cobra"
)

// newAtomsCmd implements the `ai atoms` surface. See SPEC.md §7.9 and §7.10.
//
// The generalized atoms surface unifies brand / persona / profile / skill
// fetching behind one resolver. `ai brand fetch`, `ai persona share`,
// `ai profile show`, and `ai skills install` are sugar atop this layer.
func newAtomsCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "atoms",
		Short: "Resolve, fetch, list, and verify atoms across the four registries",
		Long: `atoms is the unified surface for the four Convergent Systems atom
registries:

  brand-atoms.com    — W3C design tokens
  persona-atoms.com  — agentic + reviewer personas
  profile-atoms.com  — profile compositions
  skill-atoms.com    — skill bundles

See SPEC.md §7.9 + §7.10.`,
	}

	c.AddCommand(
		newAtomsFetchCmd(),
		newAtomsForkCmd(),
		newAtomsPublishCmd(),
		newAtomsListCmd(),
		newAtomsGCCmd(),
		newAtomsVerifyCmd(),
	)
	return c
}

// ---- fetch (#261, #262) -------------------------------------------------------

func newAtomsFetchCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "fetch <id-or-url>",
		Short: "Download and extract a constitution atom (tar.gz) to ~/.ai/atoms/",
		Args:  cobra.ExactArgs(1),
		RunE:  runAtomsFetch,
	}
}

// runAtomsFetch is the cobra RunE for `ai atoms fetch <id-or-url>`.
func runAtomsFetch(cmd *cobra.Command, args []string) error {
	rawArg := args[0]

	// Resolve to a URL. If the arg looks like a URL (contains "://") use it
	// directly. Otherwise treat it as "name@version" and return a helpful error
	// since we have no catalog endpoint yet.
	var downloadURL string
	if strings.Contains(rawArg, "://") {
		downloadURL = rawArg
	} else {
		return fmt.Errorf("atom ID %q: catalog resolution not yet supported — pass a full URL (https://... or file://...)", rawArg)
	}

	// Download to a temporary file so we can stream-hash it.
	tmp, err := os.CreateTemp("", "ai-atom-*.tar.gz")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	if err := downloadToFile(downloadURL, tmp); err != nil {
		tmp.Close()
		return fmt.Errorf("fetch %q: %w", downloadURL, err)
	}
	tmp.Close()

	// Compute SHA256 of the downloaded archive.
	downloadedSHA, err := sha256OfFile(tmpName)
	if err != nil {
		return fmt.Errorf("hash downloaded file: %w", err)
	}

	// Extract the archive to a staging directory under ~/.ai/atoms/.
	// We first extract to a temp-named dir so we can read atom.toml before
	// deciding the final destination name.
	atomsRoot := filepath.Join(paths.AIRoot(), "atoms")
	if err := os.MkdirAll(atomsRoot, 0755); err != nil {
		return fmt.Errorf("create atoms root: %w", err)
	}
	stageDir, err := os.MkdirTemp(atomsRoot, ".staging-*")
	if err != nil {
		return fmt.Errorf("create staging dir: %w", err)
	}
	// Clean up staging dir on error.
	staged := false
	defer func() {
		if !staged {
			os.RemoveAll(stageDir)
		}
	}()

	if err := extractTarGz(tmpName, stageDir); err != nil {
		return fmt.Errorf("extract atom archive: %w", err)
	}

	// Find atom.toml. The archive is expected to contain a single top-level
	// directory whose name matches the atom name.
	tomlPath, err := findAtomTOML(stageDir)
	if err != nil {
		return fmt.Errorf("atom.toml not found in archive: %w", err)
	}

	manifest, err := atoms.ParseAtomTOML(tomlPath)
	if err != nil {
		return fmt.Errorf("parse atom.toml: %w", err)
	}

	// Validate name and version fields.
	if manifest.Name == "" {
		return fmt.Errorf("atom.toml missing required field: name")
	}
	if manifest.Version == "" {
		return fmt.Errorf("atom.toml missing required field: version")
	}

	// Verify SHA256 if the manifest provides one.
	if manifest.SHA256 != "" && manifest.SHA256 != downloadedSHA {
		return fmt.Errorf("Hash mismatch — expected %s, got %s. Aborting.", manifest.SHA256, downloadedSHA)
	}

	// Determine the directory that contains atom.toml. When the archive
	// wraps everything in a single top-level directory (the common case),
	// tomlPath is stageDir/<subdir>/atom.toml and we should move <subdir>,
	// not stageDir itself.
	tomlDir := filepath.Dir(tomlPath)
	var moveFrom string
	if tomlDir == stageDir {
		// Flat archive: atom.toml is directly in the staging root.
		moveFrom = stageDir
	} else {
		// Standard archive: atom.toml is in a subdirectory of the staging root.
		moveFrom = tomlDir
	}

	// Move to final destination ~/.ai/atoms/<name>/.
	destDir := filepath.Join(atomsRoot, manifest.Name)
	// Remove any prior installation of this atom at the destination.
	if err := os.RemoveAll(destDir); err != nil {
		return fmt.Errorf("clear existing atom dir: %w", err)
	}
	if err := os.Rename(moveFrom, destDir); err != nil {
		// Rename across filesystems can fail — fall back to copy+delete.
		if cpErr := copyDir(moveFrom, destDir); cpErr != nil {
			return fmt.Errorf("install atom dir: %w (rename failed: %v)", cpErr, err)
		}
		os.RemoveAll(moveFrom)
	}
	// When moveFrom is a subdirectory of stageDir, the parent staging dir
	// remains as an empty directory after the rename. Clean it up explicitly
	// so we do not leave orphaned .staging-* dirs under atomsRoot.
	if moveFrom != stageDir {
		os.RemoveAll(stageDir) // best-effort; errors are non-fatal
	}
	staged = true // moveFrom is gone; stageDir (if different) was just cleaned up.

	// Update atoms.json index.
	indexPath := filepath.Join(paths.ConfigDir(), "atoms.json")
	entry := atoms.AtomsIndexEntry{
		Name:     manifest.Name,
		Version:  manifest.Version,
		Path:     destDir,
		Upstream: manifest.UpstreamURL,
	}
	if err := atoms.UpdateAtomsIndex(indexPath, entry); err != nil {
		return fmt.Errorf("update atoms index: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Fetched %s@%s → %s\n", manifest.Name, manifest.Version, destDir)
	return nil
}

// ---- list (#265) -------------------------------------------------------------

func newAtomsListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List installed atoms",
		Args:  cobra.NoArgs,
		RunE:  runAtomsList,
	}
}

func runAtomsList(cmd *cobra.Command, _ []string) error {
	indexPath := filepath.Join(paths.ConfigDir(), "atoms.json")
	entries, err := atoms.ReadAtomsIndex(indexPath)
	if err != nil {
		return fmt.Errorf("read atoms index: %w", err)
	}

	out := cmd.OutOrStdout()
	if len(entries) == 0 {
		fmt.Fprintln(out, "(no atoms installed)")
		return nil
	}

	// Aligned table via tabwriter.
	tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "NAME\tVERSION\tUPSTREAM\tPATH")
	for _, e := range entries {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", e.Name, e.Version, e.Upstream, e.Path)
	}
	return tw.Flush()
}

// ---- fork (#263) -------------------------------------------------------------

func newAtomsForkCmd() *cobra.Command {
	var asName string
	c := &cobra.Command{
		Use:   "fork <atom-name>",
		Short: "Fork an installed atom to a local copy for customization",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAtomsFork(cmd, args[0], asName)
		},
	}
	c.Flags().StringVar(&asName, "as", "", "local name for the fork (default: <name>-local)")
	return c
}

func runAtomsFork(cmd *cobra.Command, name, asName string) error {
	if asName == "" {
		asName = name + "-local"
	}

	atomsRoot := filepath.Join(paths.AIRoot(), "atoms")
	srcDir := filepath.Join(atomsRoot, name)
	dstDir := filepath.Join(atomsRoot, asName)

	// Source must exist.
	if _, err := os.Stat(srcDir); os.IsNotExist(err) {
		return fmt.Errorf("atom %q not installed — run `ai atoms fetch` first", name)
	}

	// Read the source manifest to capture the upstream_ref value.
	srcTOML := filepath.Join(srcDir, "atom.toml")
	manifest, err := atoms.ParseAtomTOML(srcTOML)
	if err != nil {
		return fmt.Errorf("read source atom.toml: %w", err)
	}

	// Copy the directory.
	if err := os.RemoveAll(dstDir); err != nil {
		return fmt.Errorf("clear existing fork dir: %w", err)
	}
	if err := copyDir(srcDir, dstDir); err != nil {
		return fmt.Errorf("copy atom dir: %w", err)
	}

	// Patch atom.toml in the fork: add upstream_ref, update name.
	forkManifest := manifest
	forkManifest.Name = asName
	forkManifest.UpstreamRef = name + "@" + manifest.Version
	dstTOML := filepath.Join(dstDir, "atom.toml")
	if err := atoms.WriteAtomTOML(dstTOML, forkManifest); err != nil {
		return fmt.Errorf("write forked atom.toml: %w", err)
	}

	// Update atoms.json index.
	indexPath := filepath.Join(paths.ConfigDir(), "atoms.json")
	entry := atoms.AtomsIndexEntry{
		Name:     asName,
		Version:  manifest.Version,
		Path:     dstDir,
		Upstream: manifest.UpstreamURL,
	}
	if err := atoms.UpdateAtomsIndex(indexPath, entry); err != nil {
		return fmt.Errorf("update atoms index: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(),
		"Forked %s → %s. Edit %s and run ai atoms publish.\n",
		name, asName, dstDir)
	return nil
}

// ---- publish (#264) ---------------------------------------------------------

func newAtomsPublishCmd() *cobra.Command {
	var atomName, atomVersion string
	var dryRun bool
	c := &cobra.Command{
		Use:   "publish",
		Short: "Package ~/.ai/ as a constitution atom and (optionally) publish it",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runAtomsPublish(cmd, atomName, atomVersion, dryRun)
		},
	}
	c.Flags().StringVar(&atomName, "name", "", "atom name (required)")
	c.Flags().StringVar(&atomVersion, "version", "", "atom version, e.g. 1.0.0 (required)")
	c.Flags().BoolVar(&dryRun, "dry-run", false, "preview without uploading")
	_ = c.MarkFlagRequired("name")
	_ = c.MarkFlagRequired("version")
	return c
}

func runAtomsPublish(cmd *cobra.Command, name, version string, dryRun bool) error {
	aiRoot := paths.AIRoot()

	// Walk ~/.ai/ excluding audit/, .git/, atoms/. Hash all file contents.
	hasher := sha256.New()
	var fileList []string
	skipDirs := map[string]bool{"audit": true, ".git": true, "atoms": true}

	err := filepath.WalkDir(aiRoot, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, relErr := filepath.Rel(aiRoot, path)
		if relErr != nil {
			return relErr
		}
		// Skip excluded top-level directories.
		topLevel := strings.SplitN(rel, string(filepath.Separator), 2)[0]
		if d.IsDir() && skipDirs[topLevel] {
			return filepath.SkipDir
		}
		if d.IsDir() {
			return nil
		}
		// Hash this file's contents.
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read %q for hashing: %w", path, err)
		}
		hasher.Write(data)
		fileList = append(fileList, rel)
		return nil
	})
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("walk ~/.ai/ for publish: %w", err)
	}

	combinedSHA := hex.EncodeToString(hasher.Sum(nil))
	fileCount := len(fileList)

	// Write / update atom.toml.
	atomDir := filepath.Join(aiRoot, "atoms", name)
	if err := os.MkdirAll(atomDir, 0755); err != nil {
		return fmt.Errorf("create atom dir: %w", err)
	}
	manifest := atoms.AtomManifest{
		Name:    name,
		Version: version,
		SHA256:  combinedSHA,
		Files:   fileList,
	}
	tomlPath := filepath.Join(atomDir, "atom.toml")
	if err := atoms.WriteAtomTOML(tomlPath, manifest); err != nil {
		return fmt.Errorf("write atom.toml: %w", err)
	}

	out := cmd.OutOrStdout()
	if dryRun {
		fmt.Fprintf(out, "Would publish: %s@%s (%d files, SHA256: %s)\n", name, version, fileCount, combinedSHA)
		return nil
	}

	// Full publish is not yet implemented — inform the user.
	fmt.Fprintln(out, "Publishing not yet supported. Use --dry-run to preview.")
	return nil
}

// ---- gc / verify stubs (preserved from scaffold) ----------------------------

func newAtomsGCCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "gc",
		Short: "Garbage-collect unreferenced atom cache entries (respects per-kind TTLs)",
		RunE: func(_ *cobra.Command, _ []string) error {
			notice("atoms gc:", "would walk caches and delete entries past gcUnusedDays AND unreferenced.")
			return stub("atoms gc", "§7.9.5")
		},
	}
}

func newAtomsVerifyCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "verify",
		Short: "Verify SHA-256 content hashes of every cached atom",
		RunE: func(_ *cobra.Command, _ []string) error {
			notice("atoms verify:", "would re-hash every cache entry and compare to meta.json.contentSha256.")
			return stub("atoms verify", "§7.9.5")
		},
	}
}

// ---- transport helpers -------------------------------------------------------

// downloadToFile streams the response body of url into dst.
// Returns an error if the HTTP status is not 2xx.
func downloadToFile(url string, dst *os.File) error {
	resp, err := http.Get(url) //nolint:noctx // CLI tool; context threading out of scope for MVP
	if err != nil {
		return fmt.Errorf("HTTP GET: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("HTTP %d from %s", resp.StatusCode, url)
	}
	if _, err := io.Copy(dst, resp.Body); err != nil {
		return fmt.Errorf("stream response body: %w", err)
	}
	return nil
}

// sha256OfFile returns the hex-encoded SHA256 of the file at path.
func sha256OfFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// extractTarGz extracts the tar.gz at srcPath into destDir.
// Entries whose resolved path does not start with destDir are rejected
// (directory traversal protection per Code.md §7).
func extractTarGz(srcPath, destDir string) error {
	f, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("open gzip: %w", err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("read tar entry: %w", err)
		}

		// Reject absolute paths and path traversal.
		clean := filepath.Clean(hdr.Name)
		if filepath.IsAbs(clean) || strings.HasPrefix(clean, "..") {
			return fmt.Errorf("tar entry %q is outside the extraction root", hdr.Name)
		}

		target := filepath.Join(destDir, clean)
		// Verify the resolved path is still under destDir.
		if !strings.HasPrefix(target, filepath.Clean(destDir)+string(os.PathSeparator)) &&
			target != filepath.Clean(destDir) {
			return fmt.Errorf("tar entry %q would escape extraction root", hdr.Name)
		}

		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0755); err != nil {
				return fmt.Errorf("mkdir %q: %w", target, err)
			}
		case tar.TypeReg, tar.TypeRegA:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return fmt.Errorf("mkdir parent of %q: %w", target, err)
			}
			out, err := os.Create(target)
			if err != nil {
				return fmt.Errorf("create %q: %w", target, err)
			}
			if _, err := io.Copy(out, tr); err != nil {
				out.Close()
				return fmt.Errorf("write %q: %w", target, err)
			}
			out.Close()
		}
	}
	return nil
}

// findAtomTOML searches destDir (one level deep) for atom.toml and
// returns its absolute path. The archive is expected to contain a
// single top-level directory.
func findAtomTOML(destDir string) (string, error) {
	entries, err := os.ReadDir(destDir)
	if err != nil {
		return "", err
	}
	// Search directly in destDir first (flat archives).
	direct := filepath.Join(destDir, "atom.toml")
	if _, err := os.Stat(direct); err == nil {
		return direct, nil
	}
	// Search one level deep (standard: single top-level dir).
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		candidate := filepath.Join(destDir, e.Name(), "atom.toml")
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("no atom.toml found in extracted archive under %q", destDir)
}

// copyDir copies the directory tree at src to dst, creating dst if needed.
func copyDir(src, dst string) error {
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
			return os.MkdirAll(target, 0755)
		}
		return copyFile(path, target)
	})
}

// copyFile copies the file at src to dst.
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}
