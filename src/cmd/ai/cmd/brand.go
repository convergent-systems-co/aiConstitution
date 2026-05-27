package cmd

// brand.go — `ai brand {fetch,list}`.
//
// Implements #353 (brand list: GitHub Contents API directory walk;
// brand fetch: download brand files to cache, apply brand.toml settings).
//
// Design notes:
//   - Registry URL defaults to the GitHub Contents API for brand-atoms.
//     Tests override via AICONST_BRAND_REGISTRY_URL so no real network
//     traffic is needed.
//   - HTTP calls go through the package-level brandHTTPGet seam so tests
//     can inject a fake client.
//   - Cache dir: ~/.ai/atoms/cache/brand-atoms/<slug>/ per §7.9.5.
//   - settings.toml is only written when brand.toml is present and
//     contains at least one recognized key (name, voice, tone).

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/BurntSushi/toml"
	internalconfig "github.com/convergent-systems-co/aiConstitution/src/internal/config"
	"github.com/convergent-systems-co/aiConstitution/src/internal/paths"
	"github.com/spf13/cobra"
)

// brandRegistryBaseURL returns the base URL for brand-atoms Contents API
// requests. Reads AICONST_BRAND_REGISTRY_URL first (test seam), then falls
// back to the canonical GitHub Contents API endpoint.
func brandRegistryBaseURL() string {
	if env := os.Getenv("AICONST_BRAND_REGISTRY_URL"); env != "" {
		return env
	}
	return "https://api.github.com/repos/convergent-systems-co/brand-atoms/contents"
}

// brandHTTPGet is the package-level HTTP GET seam. Tests may replace it
// with a fake that records calls. The default implementation uses the
// standard library http.Get.
var brandHTTPGet = func(url string) (*http.Response, error) {
	return http.Get(url) //nolint:noctx // CLI tool; context threading out of scope for MVP
}

// githubContentsEntry is the minimal shape of one entry returned by the
// GitHub Contents API (/repos/.../contents/<path>).
type githubContentsEntry struct {
	Name        string `json:"name"`
	Type        string `json:"type"` // "file" or "dir"
	DownloadURL string `json:"download_url"`
}

// brandAtomJSON is the minimal shape of atom.json inside a brand directory.
type brandAtomJSON struct {
	Version string `json:"version"`
}

// brandTOML is the recognized subset of keys in brand.toml. Unknown keys
// are ignored (per §7 — input validation at the boundary).
type brandTOML struct {
	Name  string `toml:"name"`
	Voice string `toml:"voice"`
	Tone  string `toml:"tone"`
}

// brandListEntry carries one row for the `brand list` table.
type brandListEntry struct {
	Slug    string
	Version string
}

// newBrandCmd implements `ai brand {fetch,list}`. See SPEC.md §14.4
// and §7.9.5 (cache discipline).
func newBrandCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "brand",
		Short: "Fetch or list brand atoms from brand-atoms.com",
		Long: `brand resolves W3C design tokens from brand-atoms.com.
The canonical brand for Convergent Systems sites is convergent-systems@1.0.0
— see SPEC.md §14.4. Brand atoms cache to
~/.ai/atoms/cache/brand-atoms/.

See SPEC.md §14.4 + §7.9.5.`,
	}

	// fetch
	fetch := &cobra.Command{
		Use:   "fetch <slug>",
		Short: "Fetch a brand atom into the local cache",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runBrandFetch(cmd.OutOrStdout(), args[0])
		},
	}

	// list
	list := &cobra.Command{
		Use:   "list",
		Short: "List available brand atoms from the registry",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runBrandList(cmd.OutOrStdout())
		},
	}

	c.AddCommand(fetch, list)
	return c
}

// ---- brand list -------------------------------------------------------------

// runBrandList queries the brand-atoms registry for the list of brand
// directories, resolves the version from each brand's atom.json (if
// present), and prints a SLUG | VERSION table via tabwriter.
func runBrandList(w io.Writer) error {
	base := brandRegistryBaseURL()
	url := base + "/brands"

	entries, err := fetchGithubDir(url)
	if err != nil {
		return fmt.Errorf("brand list: fetch directory listing: %w", err)
	}

	// Collect brand dirs (filter out any files at this level).
	var brands []brandListEntry
	for _, e := range entries {
		if e.Type != "dir" {
			continue
		}
		version := resolveBrandVersion(base, e.Name)
		brands = append(brands, brandListEntry{Slug: e.Name, Version: version})
	}

	if len(brands) == 0 {
		fmt.Fprintln(w, "(no brands found)") //nolint:errcheck
		return nil
	}

	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "SLUG\tVERSION") //nolint:errcheck
	for _, b := range brands {
		fmt.Fprintf(tw, "%s\t%s\n", b.Slug, b.Version) //nolint:errcheck
	}
	return tw.Flush()
}

// resolveBrandVersion fetches <base>/brands/<slug>/atom.json and returns the
// version field. Returns "unknown" on any error (missing file, parse
// error, etc.) so a single missing atom.json does not block the whole list.
func resolveBrandVersion(base, slug string) string {
	url := base + "/brands/" + slug + "/atom.json"
	resp, err := brandHTTPGet(url)
	if err != nil {
		return "unknown"
	}
	defer resp.Body.Close() //nolint:errcheck
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "unknown"
	}
	var a brandAtomJSON
	if err := json.NewDecoder(resp.Body).Decode(&a); err != nil {
		return "unknown"
	}
	if a.Version == "" {
		return "unknown"
	}
	return a.Version
}

// ---- brand fetch ------------------------------------------------------------

// runBrandFetch downloads the brand directory for slug from the registry
// into ~/.ai/atoms/cache/brand-atoms/<slug>/. If the directory contains a
// brand.toml, recognized keys (name, voice, tone) are applied to
// ~/.config/aiConstitution/settings.toml.
func runBrandFetch(w io.Writer, slug string) error {
	base := brandRegistryBaseURL()
	url := base + "/brands/" + slug

	entries, err := fetchGithubDir(url)
	if err != nil {
		return fmt.Errorf("brand fetch %q: fetch file list: %w", slug, err)
	}

	// Destination cache directory.
	cacheDir := filepath.Join(paths.AIRoot(), "atoms", "cache", "brand-atoms", slug)
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return fmt.Errorf("brand fetch %q: create cache dir: %w", slug, err)
	}

	var applied []string // names of files written
	var hasBrandTOML bool

	for _, entry := range entries {
		if entry.Type != "file" {
			continue
		}
		if entry.DownloadURL == "" {
			// Submodules or symlinks — skip.
			continue
		}
		destPath := filepath.Join(cacheDir, entry.Name)
		if err := downloadBrandFile(entry.DownloadURL, destPath); err != nil {
			return fmt.Errorf("brand fetch %q: download %q: %w", slug, entry.Name, err)
		}
		applied = append(applied, entry.Name)
		if entry.Name == "brand.toml" {
			hasBrandTOML = true
		}
	}

	fmt.Fprintf(w, "Fetched brand %s\n", slug) //nolint:errcheck
	for _, name := range applied {
		fmt.Fprintf(w, "  %s → %s\n", name, filepath.Join(cacheDir, name)) //nolint:errcheck
	}

	if hasBrandTOML {
		tomlPath := filepath.Join(cacheDir, "brand.toml")
		if err := applyBrandTOML(w, tomlPath); err != nil {
			// brand.toml errors are non-fatal: the files are still cached.
			fmt.Fprintf(w, "  warning: could not apply brand.toml: %v\n", err) //nolint:errcheck
		}
	}

	return nil
}

// applyBrandTOML reads the brand.toml at path and writes recognized
// settings (name, voice, tone) into settings.toml. Prints which keys
// were applied to w.
func applyBrandTOML(w io.Writer, tomlPath string) error {
	data, err := os.ReadFile(tomlPath) //nolint:gosec // G304: path is under cache dir, not user input
	if err != nil {
		return fmt.Errorf("read brand.toml: %w", err)
	}

	var bt brandTOML
	if _, err := toml.Decode(string(data), &bt); err != nil {
		return fmt.Errorf("parse brand.toml: %w", err)
	}

	// Only proceed if at least one recognized key is set.
	if bt.Name == "" && bt.Voice == "" && bt.Tone == "" {
		return nil
	}

	// Load current settings (or defaults) and save them first to ensure the
	// file exists and is well-formed before we append the brand section.
	settings, loadErr := internalconfig.Load()
	if loadErr != nil {
		settings = internalconfig.Defaults()
	}
	if err := internalconfig.Save(settings); err != nil {
		return fmt.Errorf("save settings.toml: %w", err)
	}

	// Append the [brand] block to settings.toml.
	settingsPath := paths.SettingsTOML()
	var brandLines strings.Builder
	brandLines.WriteString("\n[brand]\n")
	var appliedKeys []string
	if bt.Name != "" {
		brandLines.WriteString(fmt.Sprintf("  name = %q\n", bt.Name))
		appliedKeys = append(appliedKeys, "name")
	}
	if bt.Voice != "" {
		brandLines.WriteString(fmt.Sprintf("  voice = %q\n", bt.Voice))
		appliedKeys = append(appliedKeys, "voice")
	}
	if bt.Tone != "" {
		brandLines.WriteString(fmt.Sprintf("  tone = %q\n", bt.Tone))
		appliedKeys = append(appliedKeys, "tone")
	}

	f, err := os.OpenFile(settingsPath, os.O_APPEND|os.O_WRONLY, 0644) //nolint:gosec // G304: path from paths package
	if err != nil {
		return fmt.Errorf("open settings.toml for brand append: %w", err)
	}
	defer f.Close() //nolint:errcheck
	if _, err := f.WriteString(brandLines.String()); err != nil {
		return fmt.Errorf("append brand section to settings.toml: %w", err)
	}

	fmt.Fprintf(w, "  Applied brand.toml keys: %s → %s\n", strings.Join(appliedKeys, ", "), settingsPath) //nolint:errcheck
	return nil
}

// ---- transport helpers ------------------------------------------------------

// fetchGithubDir calls the GitHub Contents API at url and decodes the
// response as a JSON array of githubContentsEntry. Returns an error on
// non-2xx status.
func fetchGithubDir(url string) ([]githubContentsEntry, error) {
	resp, err := brandHTTPGet(url)
	if err != nil {
		return nil, fmt.Errorf("HTTP GET %q: %w", url, err)
	}
	defer resp.Body.Close() //nolint:errcheck
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP %d from %q", resp.StatusCode, url)
	}
	var entries []githubContentsEntry
	if err := json.NewDecoder(resp.Body).Decode(&entries); err != nil {
		return nil, fmt.Errorf("decode response from %q: %w", url, err)
	}
	return entries, nil
}

// downloadBrandFile fetches the file at url and writes it to destPath,
// creating parent directories as needed.
func downloadBrandFile(url, destPath string) error {
	resp, err := brandHTTPGet(url)
	if err != nil {
		return fmt.Errorf("HTTP GET: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("HTTP %d from %q", resp.StatusCode, url)
	}
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return fmt.Errorf("mkdir parent: %w", err)
	}
	f, err := os.Create(destPath) //nolint:gosec // G304: destPath is under cache dir
	if err != nil {
		return fmt.Errorf("create dest file: %w", err)
	}
	defer f.Close() //nolint:errcheck
	if _, err := io.Copy(f, resp.Body); err != nil {
		return fmt.Errorf("write file: %w", err)
	}
	return nil
}
