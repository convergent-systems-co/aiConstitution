package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	cbterm "github.com/charmbracelet/x/term"
	"github.com/spf13/cobra"
)

// SkillAtomsBaseURL is the base URL for fetching skill atoms from the GitHub
// API. Tests may override this to point at an httptest server.
//
// The full URL for a slug is:
//
//	<SkillAtomsBaseURL>/skills/skill/<slug>.json
var SkillAtomsBaseURL = "https://api.github.com/repos/convergent-systems-co/skill-atoms/contents"

// skillAtom is the JSON schema returned by the skill-atoms GitHub API endpoint.
// Only the fields relevant for install/upgrade/listing are decoded.
type skillAtom struct {
	ID                   string   `json:"id"`
	Version              string   `json:"version"`
	Name                 string   `json:"name"`
	Description          string   `json:"description"`
	SystemPromptFragment string   `json:"system_prompt_fragment"`
	Lifecycle            string   `json:"lifecycle"`
	DependsOn            []string `json:"depends_on,omitempty"`
	// Events is populated for ai-hook atoms (type: "ai-hook") and lists the
	// Claude hook events the hook attaches to (e.g. "PreToolUse", "PostToolUse").
	Events []string `json:"events,omitempty"`
}


// skillAtomDirEntry is a single entry in the GitHub Contents API directory
// listing returned when fetching the /skills/skill path.
type skillAtomDirEntry struct {
	Name        string `json:"name"`
	DownloadURL string `json:"download_url"`
}

// fetchSkillsDirectory fetches the GitHub Contents API directory listing for
// the skills/skill path and returns the raw entries. Only entries whose name
// ends with ".json" are included in the result.
func fetchSkillsDirectory() ([]skillAtomDirEntry, error) {
	url := SkillAtomsBaseURL + "/skills/skill"
	req, err := http.NewRequest(http.MethodGet, url, nil) //nolint:noctx // CLI tool
	if err != nil {
		return nil, fmt.Errorf("skills: build directory request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("skills: fetch directory: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("skills: fetch directory: HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("skills: read directory response: %w", err)
	}

	var entries []skillAtomDirEntry
	if err := json.Unmarshal(body, &entries); err != nil {
		return nil, fmt.Errorf("skills: parse directory JSON: %w", err)
	}

	// Filter to .json files only.
	filtered := entries[:0]
	for _, e := range entries {
		if strings.HasSuffix(e.Name, ".json") {
			filtered = append(filtered, e)
		}
	}
	return filtered, nil
}

// fetchSkillAtomFromURL fetches a skill atom JSON from an explicit download URL.
// Used by runSkillsAvailable to hydrate each directory entry.
func fetchSkillAtomFromURL(downloadURL string) (*skillAtom, error) {
	req, err := http.NewRequest(http.MethodGet, downloadURL, nil) //nolint:noctx // CLI tool
	if err != nil {
		return nil, fmt.Errorf("skills: build atom request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("skills: fetch atom: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("skills: fetch atom: HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("skills: read atom response: %w", err)
	}

	var atom skillAtom
	if err := json.Unmarshal(body, &atom); err != nil {
		return nil, fmt.Errorf("skills: parse atom JSON: %w", err)
	}
	return &atom, nil
}

// runSkillsAvailable implements `ai skills available`.
// It lists all skills published in the ai-atoms.com catalog, excluding
// deprecated and retired entries. Sub-skills (referenced via depends_on) are
// deduplicated from the top-level listing and shown only via the "(+N)" count.
func runSkillsAvailable(cmd *cobra.Command, _ []string) error {
	catalog, err := fetchAiAtomsCatalog()
	if err != nil {
		return err
	}

	// Filter to active skill atoms only.
	var skillAtoms []aiAtomEntry
	for _, a := range catalog {
		lc := strings.ToLower(a.Lifecycle)
		if a.Type == "skill" && lc != "deprecated" && lc != "retired" {
			skillAtoms = append(skillAtoms, a)
		}
	}

	// First pass: collect sub-skill slugs from depends_on across all skill atoms.
	type hydrated struct {
		slug, name, version, description string
		dependsOn                        []string
	}
	var all []hydrated
	subSkills := map[string]bool{}

	for _, a := range skillAtoms {
		slug := strings.TrimPrefix(a.ID, "skill/")
		name := a.Name
		if name == "" {
			name = slug
		}
		all = append(all, hydrated{slug, name, a.Version, a.Description, a.DependsOn})
		for _, dep := range a.DependsOn {
			subSkills[dep] = true
		}
	}

	// Second pass: exclude sub-skills from the top-level rows.
	type row struct{ slug, name, version, description string }
	var rows []row
	for _, h := range all {
		if subSkills[h.slug] {
			continue
		}
		dep := ""
		if len(h.dependsOn) > 0 {
			dep = fmt.Sprintf(" (+%d)", len(h.dependsOn))
		}
		rows = append(rows, row{h.slug + dep, h.name, h.version, h.description})
	}

	if len(rows) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "(no skills available)")
		return nil
	}

	out := cmd.OutOrStdout()
	// Find longest slug for alignment.
	maxSlug := 4 // len("SLUG")
	for _, r := range rows {
		if len(r.slug) > maxSlug {
			maxSlug = len(r.slug)
		}
	}
	fmt.Fprintf(out, "  %-*s  %s\n", maxSlug, "SLUG", "DESCRIPTION")
	fmt.Fprintf(out, "  %-*s  %s\n", maxSlug, strings.Repeat("─", maxSlug), strings.Repeat("─", 50))
	for _, r := range rows {
		desc := r.description
		if len(desc) > 70 {
			desc = desc[:67] + "..."
		}
		fmt.Fprintf(out, "  %-*s  %s\n", maxSlug, r.slug, desc)
	}
	return nil
}

// newSkillsAvailableCmd returns the cobra command for `ai skills available`.
func newSkillsAvailableCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "available",
		Short: "List skills available to install from skill-atoms.com",
		Args:  cobra.NoArgs,
		RunE:  runSkillsAvailable,
	}
}

// Deprecated: fetchSkillAtomJSON uses the GitHub API. Use fetchSkillAtomFromCatalog instead.
func fetchSkillAtomJSON(slug string) (*skillAtom, error) {
	url := SkillAtomsBaseURL + "/skills/skill/" + slug + ".json"
	req, err := http.NewRequest(http.MethodGet, url, nil) //nolint:noctx // CLI tool
	if err != nil {
		return nil, fmt.Errorf("skills: build request for %q: %w", slug, err)
	}
	req.Header.Set("Accept", "application/vnd.github.raw+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("skills: fetch %q: %w", slug, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("skills: skill %q not found in registry (HTTP 404)", slug)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("skills: fetch %q: HTTP %d", slug, resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("skills: read response for %q: %w", slug, err)
	}

	var atom skillAtom
	if err := json.Unmarshal(body, &atom); err != nil {
		return nil, fmt.Errorf("skills: parse atom JSON for %q: %w", slug, err)
	}
	return &atom, nil
}

// claudeSkillsDir returns the canonical ~/.claude/skills/ path.
// Override priority: $CLAUDE_SKILLS_DIR env var, then $HOME/.claude/skills/.
// Returns "" if the directory does not exist (symlinks are only created when
// the consumer directory is present).
func claudeSkillsDir() string {
	if env := os.Getenv("CLAUDE_SKILLS_DIR"); env != "" {
		if _, err := os.Stat(env); err == nil {
			return env
		}
		return env // return it even if missing — callers check existence
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".claude", "skills")
}

// copilotInstructionsDir returns the canonical ~/.copilot/instructions/ path.
// Override priority: $COPILOT_INSTRUCTIONS_DIR env var, then
// $HOME/.copilot/instructions/ (only when the directory actually exists).
// Returns "" when the directory is absent — Copilot wiring is silently
// skipped on machines that have not set up GitHub Copilot.
func copilotInstructionsDir() string {
	if env := os.Getenv("COPILOT_INSTRUCTIONS_DIR"); env != "" {
		if _, err := os.Stat(env); err == nil {
			return env
		}
		return "" // env override but directory is absent — skip
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	dir := filepath.Join(home, ".copilot", "instructions")
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return ""
	}
	return dir
}

// writeSkillMD writes a SKILL.md file for the given atom at destPath,
// creating parent directories as needed.
func writeSkillMD(destPath string, atom *skillAtom) error {
	if err := os.MkdirAll(filepath.Dir(destPath), 0o750); err != nil {
		return fmt.Errorf("skills: create skill dir: %w", err)
	}

	slug := atom.Name
	if slug == "" {
		// Derive from id "skill/<slug>"
		slug = strings.TrimPrefix(atom.ID, "skill/")
	}

	content := fmt.Sprintf(`---
name: %s
description: %s
version: %s
user-invocable: true
allowed-tools:
  - Bash
  - Read
---
# %s

%s
`, slug, atom.Description, atom.Version, atom.Name, atom.SystemPromptFragment)

	return os.WriteFile(destPath, []byte(content), 0o644) //nolint:gosec // 0644 is intentional for skill files
}

// ensureSymlink creates or replaces a symlink at linkPath → target.
// If linkPath already exists (as a symlink or file), it is removed first.
func ensureSymlink(target, linkPath string) error {
	if _, err := os.Lstat(linkPath); err == nil {
		if removeErr := os.Remove(linkPath); removeErr != nil {
			return fmt.Errorf("skills: remove existing symlink %s: %w", linkPath, removeErr)
		}
	}
	return os.Symlink(target, linkPath)
}

// runSkillsInstall is the implementation of `ai skills install <name>[@version]`.
// It fetches the atom JSON, writes SKILL.md, and optionally creates a Claude symlink.
func runSkillsInstall(cmd *cobra.Command, slug string) error {
	// Strip any @version suffix — v1 always fetches latest.
	slug, _, _ = strings.Cut(slug, "@")

	atom, err := fetchSkillAtomFromCatalog(slug)
	if err != nil {
		// Not in ai-atoms.com catalog yet — fall back to skill-atoms GitHub API.
		// This covers the 18+ skills that exist in skill-atoms.com but haven't
		// been migrated to ai-atoms.com.
		var fallbackErr error
		atom, fallbackErr = fetchSkillAtomJSON(slug)
		if fallbackErr != nil {
			return err // return the original catalog error, not the fallback error
		}
	}

	skillsDir := skillsManifestDir()
	destDir := filepath.Join(skillsDir, slug)
	destMD := filepath.Join(destDir, "SKILL.md")

	if err := writeSkillMD(destMD, atom); err != nil {
		return err
	}

	// Optional: symlink ~/.claude/skills/<slug> → ~/.ai/skills/<slug>
	claudeDir := claudeSkillsDir()
	if claudeDir != "" {
		if _, err := os.Stat(claudeDir); err == nil {
			linkPath := filepath.Join(claudeDir, slug)
			if symlinkErr := ensureSymlink(destDir, linkPath); symlinkErr != nil {
				// Non-fatal: warn but don't abort.
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: could not create Claude symlink: %v\n", symlinkErr)
			}
		}
	}

	// Optional: symlink ~/.copilot/instructions/<slug>.md → SKILL.md
	if copilotDir := copilotInstructionsDir(); copilotDir != "" {
		skillMD := filepath.Join(destDir, "SKILL.md")
		if _, err := os.Stat(skillMD); err == nil {
			linkPath := filepath.Join(copilotDir, slug+".md")
			if symlinkErr := ensureSymlink(skillMD, linkPath); symlinkErr != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: could not create Copilot symlink: %v\n", symlinkErr)
			}
		}
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Installed %s v%s\n", slug, atom.Version)

	// If the skill declares dependencies, offer to install them.
	if len(atom.DependsOn) > 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "\nThis skill depends on: %s\n", strings.Join(atom.DependsOn, ", "))
		if cbterm.IsTerminal(os.Stdout.Fd()) {
			fmt.Fprint(cmd.OutOrStdout(), "Install dependencies too? [Y/n]: ")
			var answer string
			fmt.Scanln(&answer) //nolint:errcheck // best-effort readline; empty = yes
			answer = strings.TrimSpace(strings.ToLower(answer))
			if answer == "" || answer == "y" || answer == "yes" {
				for _, dep := range atom.DependsOn {
					fmt.Fprintf(cmd.OutOrStdout(), "Installing %s...\n", dep)
					if depErr := runSkillsInstall(cmd, dep); depErr != nil {
						fmt.Fprintf(cmd.ErrOrStderr(), "warning: failed to install %s: %v\n", dep, depErr)
					}
				}
			}
		} else {
			// Non-interactive: install deps automatically without prompting.
			for _, dep := range atom.DependsOn {
				fmt.Fprintf(cmd.OutOrStdout(), "Installing %s...\n", dep)
				if depErr := runSkillsInstall(cmd, dep); depErr != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: failed to install %s: %v\n", dep, depErr)
				}
			}
		}
	}

	return nil
}

// runSkillsUninstall is the implementation of `ai skills uninstall <name>`.
func runSkillsUninstall(cmd *cobra.Command, name string) error {
	skillsDir := skillsManifestDir()
	dir, err := findSkillDir(skillsDir, name)
	if err != nil {
		return fmt.Errorf("skills uninstall: %w", err)
	}
	if dir == "" {
		return fmt.Errorf("skills uninstall: skill %q is not installed", name)
	}

	slug := filepath.Base(dir)

	if err := os.RemoveAll(dir); err != nil {
		return fmt.Errorf("skills uninstall: remove skill dir: %w", err)
	}

	// Remove symlink from ~/.claude/skills/ if it exists.
	claudeDir := claudeSkillsDir()
	if claudeDir != "" {
		linkPath := filepath.Join(claudeDir, slug)
		if _, lstatErr := os.Lstat(linkPath); lstatErr == nil {
			if removeErr := os.Remove(linkPath); removeErr != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: could not remove Claude symlink: %v\n", removeErr)
			}
		}
	}

	// Remove symlink from ~/.copilot/instructions/ if it exists.
	if copilotDir := copilotInstructionsDir(); copilotDir != "" {
		linkPath := filepath.Join(copilotDir, slug+".md")
		if _, lstatErr := os.Lstat(linkPath); lstatErr == nil {
			_ = os.Remove(linkPath)
		}
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Uninstalled %s\n", slug)
	return nil
}

// runSkillsUpgrade is the implementation of `ai skills upgrade <name> [<version>]`.
func runSkillsUpgrade(cmd *cobra.Command, name string) error {
	skillsDir := skillsManifestDir()
	dir, err := findSkillDir(skillsDir, name)
	if err != nil {
		return fmt.Errorf("skills upgrade: %w", err)
	}
	if dir == "" {
		return fmt.Errorf("skills upgrade: skill %q is not installed — run `ai skills install` first", name)
	}

	slug := filepath.Base(dir)

	// Read current version from frontmatter.
	oldVersion := "(unknown)"
	mdPath := filepath.Join(dir, "SKILL.md")
	if data, readErr := os.ReadFile(mdPath); readErr == nil {
		if v, ok := parseFrontmatter(string(data))["version"]; ok && v != "" {
			oldVersion = v
		}
	}

	atom, err := fetchSkillAtomJSON(slug)
	if err != nil {
		return err
	}

	if atom.Version == oldVersion {
		fmt.Fprintf(cmd.OutOrStdout(), "%s is already up-to-date (v%s)\n", slug, oldVersion)
		return nil
	}

	if err := writeSkillMD(mdPath, atom); err != nil {
		return err
	}

	// Re-create symlink in Claude skills dir if it existed before.
	claudeDir := claudeSkillsDir()
	if claudeDir != "" {
		linkPath := filepath.Join(claudeDir, slug)
		if _, lstatErr := os.Lstat(linkPath); lstatErr == nil {
			if symlinkErr := ensureSymlink(dir, linkPath); symlinkErr != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: could not refresh Claude symlink: %v\n", symlinkErr)
			}
		}
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Upgraded %s from v%s to v%s\n", slug, oldVersion, atom.Version)
	return nil
}

// runSkillsUpgradeAll is the implementation of `ai skills upgrade-all`.
func runSkillsUpgradeAll(cmd *cobra.Command) error {
	skillsDir := skillsManifestDir()
	dirs, err := listSkillDirs(skillsDir)
	if err != nil {
		return fmt.Errorf("skills upgrade-all: %w", err)
	}
	if len(dirs) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "(no skills installed)")
		return nil
	}

	upgraded := 0
	for _, d := range dirs {
		slug := filepath.Base(d)
		if upgradeErr := runSkillsUpgrade(cmd, slug); upgradeErr != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "warning: could not upgrade %s: %v\n", slug, upgradeErr)
		} else {
			upgraded++
		}
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Upgraded %d skill(s)\n", upgraded)
	return nil
}

// runSkillsLink creates symlinks for all installed skills in both
// ~/.claude/skills/ and ~/.copilot/instructions/. It is idempotent:
// running it multiple times over already-linked skills produces no error.
func runSkillsLink(cmd *cobra.Command, _ []string) error {
	skillsDir := skillsManifestDir()
	dirs, err := listSkillDirs(skillsDir)
	if err != nil {
		return fmt.Errorf("skills link: list installed: %w", err)
	}

	claudeDir := claudeSkillsDir()
	copilotDir := copilotInstructionsDir()

	var linkedClaude, linkedCopilot int
	for _, skillPath := range dirs {
		slug := filepath.Base(skillPath)

		if claudeDir != "" {
			if _, statErr := os.Stat(claudeDir); statErr == nil {
				linkPath := filepath.Join(claudeDir, slug)
				if err := ensureSymlink(skillPath, linkPath); err == nil {
					linkedClaude++
				}
			}
		}

		if copilotDir != "" {
			skillMD := filepath.Join(skillPath, "SKILL.md")
			if _, statErr := os.Stat(skillMD); statErr == nil {
				linkPath := filepath.Join(copilotDir, slug+".md")
				if err := ensureSymlink(skillMD, linkPath); err == nil {
					linkedCopilot++
				}
			}
		}
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Linked %d skill(s) to Claude, %d to Copilot\n", linkedClaude, linkedCopilot)
	return nil
}

// newSkillsLinkCmd returns the cobra command for `ai skills link`.
func newSkillsLinkCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "link",
		Short: "Symlink all installed skills to ~/.claude/skills/ and ~/.copilot/instructions/",
		Long: `link iterates every installed skill under ~/.ai/skills/ and creates
symlinks in both consumer directories:

  ~/.claude/skills/<slug>           → ~/.ai/skills/<slug>/
  ~/.copilot/instructions/<slug>.md → ~/.ai/skills/<slug>/SKILL.md

Consumer directories that do not exist are silently skipped.
The command is idempotent: re-running over already-linked skills is safe.`,
		Args: cobra.NoArgs,
		RunE: runSkillsLink,
	}
}

// skillsManifestDir returns the canonical ~/.ai/skills/ path.
// Override priority: $AI_ROOT env var, then $HOME/.ai/.
func skillsManifestDir() string {
	if env := os.Getenv("AI_ROOT"); env != "" {
		return filepath.Join(env, "skills")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".ai", "skills")
	}
	return filepath.Join(home, ".ai", "skills")
}

// skillInfo holds parsed metadata from a SKILL.md frontmatter block.
type skillInfo struct {
	name        string
	description string
	dir         string // absolute path to the skill directory
}

// parseFrontmatter extracts the frontmatter key→value map from a SKILL.md.
// Frontmatter is a YAML-lite block delimited by "---" lines at the top of
// the file. Only simple "key: value" pairs are extracted; nested structures
// and multi-line values are not needed for the required name/description fields.
func parseFrontmatter(content string) map[string]string {
	fields := make(map[string]string)
	lines := strings.Split(content, "\n")
	inFrontmatter := false
	started := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "---" {
			if !started {
				inFrontmatter = true
				started = true
				continue
			}
			// closing delimiter
			break
		}
		if inFrontmatter {
			idx := strings.Index(trimmed, ":")
			if idx > 0 {
				k := strings.TrimSpace(trimmed[:idx])
				v := strings.TrimSpace(trimmed[idx+1:])
				fields[k] = v
			}
		}
	}
	return fields
}

// loadSkillInfo reads the SKILL.md in dir and returns a populated skillInfo.
// If SKILL.md is missing, it returns a skillInfo with only the dir and a
// derived name from the directory name.
func loadSkillInfo(dir string) skillInfo {
	name := filepath.Base(dir)
	info := skillInfo{name: name, dir: dir}

	mdPath := filepath.Join(dir, "SKILL.md")
	data, err := os.ReadFile(mdPath)
	if err != nil {
		// No SKILL.md — leave description empty so callers can detect it.
		return info
	}

	fields := parseFrontmatter(string(data))
	if n, ok := fields["name"]; ok && n != "" {
		info.name = n
	}
	info.description = fields["description"]
	return info
}

// listSkillDirs returns a sorted list of skill directories under skillsDir.
// Returns nil (not an error) if skillsDir does not exist.
func listSkillDirs(skillsDir string) ([]string, error) {
	entries, err := os.ReadDir(skillsDir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var dirs []string
	for _, e := range entries {
		if e.IsDir() {
			dirs = append(dirs, filepath.Join(skillsDir, e.Name()))
		}
	}
	return dirs, nil
}

// findSkillDir locates a skill directory by exact name then prefix match.
// Returns ("", nil) when not found and no error occurred.
func findSkillDir(skillsDir, query string) (string, error) {
	dirs, err := listSkillDirs(skillsDir)
	if err != nil {
		return "", err
	}
	// Exact match first.
	for _, d := range dirs {
		if filepath.Base(d) == query {
			return d, nil
		}
	}
	// Prefix match.
	var matches []string
	for _, d := range dirs {
		if strings.HasPrefix(filepath.Base(d), query) {
			matches = append(matches, d)
		}
	}
	if len(matches) == 1 {
		return matches[0], nil
	}
	return "", nil
}

// newSkillsCmd implements `ai skills {list,show,validate,templates,...}`.
// See SPEC.md §7.10.
func newSkillsCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "skills",
		Short: "Manage skill atoms (tarball bundles: SKILL.md + templates + assets)",
		Long: `skills manages skill atoms from skill-atoms.com. The local layout
holds manifests (TOML pinning atom@version); the content lives in the
~/.config/aiConstitution/.skill-cache/.

See SPEC.md §7.10.`,
	}

	c.AddCommand(
		newSkillsListCmd(),
		newSkillsShowCmd(),
		newSkillsValidateCmd(),
		newSkillsTemplatesCmd(),
		newSkillsAvailableCmd(),
		// install/uninstall/upgrade/upgrade-all — implemented in §7.10.2 (#347).
		func() *cobra.Command {
			var installAll bool
			cmd := &cobra.Command{
				Use:   "install [<name>[@<version>]]",
				Short: "Fetch from skill-atoms.com and install to ~/.ai/skills/",
				Long: `install fetches a skill atom from the skill-atoms registry and
installs it to ~/.ai/skills/<name>/SKILL.md. If ~/.claude/skills/ exists,
a symlink is created there for Claude Code to discover.

Use --all to install every available skill at once.`,
				Args: func(cmd *cobra.Command, args []string) error {
					if installAll {
						return cobra.NoArgs(cmd, args)
					}
					return cobra.ExactArgs(1)(cmd, args)
				},
				RunE: func(c *cobra.Command, args []string) error {
					if installAll {
						slugs, err := fetchSkillsDirectory()
						if err != nil {
							return fmt.Errorf("skills install --all: fetch list: %w", err)
						}
						var errs []string
						for _, s := range slugs {
							slug := strings.TrimSuffix(s.Name, ".json")
							if installErr := runSkillsInstall(c, slug); installErr != nil {
								errs = append(errs, slug+": "+installErr.Error())
							}
						}
						if len(errs) > 0 {
							return fmt.Errorf("some skills failed to install:\n  %s", strings.Join(errs, "\n  "))
						}
						return nil
					}
					return runSkillsInstall(c, args[0])
				},
			}
			cmd.Flags().BoolVar(&installAll, "all", false, "install every available skill")
			return cmd
		}(),
		&cobra.Command{
			Use:   "uninstall <name>",
			Short: "Remove a skill and its Claude symlink",
			Args:  cobra.ExactArgs(1),
			RunE: func(c *cobra.Command, args []string) error {
				return runSkillsUninstall(c, args[0])
			},
		},
		&cobra.Command{
			Use:   "upgrade <name> [<version>]",
			Short: "Upgrade an installed skill to its latest (or specified) version",
			Args:  cobra.RangeArgs(1, 2),
			RunE: func(c *cobra.Command, args []string) error {
				return runSkillsUpgrade(c, args[0])
			},
		},
		&cobra.Command{
			Use:   "upgrade-all",
			Short: "Upgrade every installed skill to its latest stable version",
			RunE: func(c *cobra.Command, _ []string) error {
				return runSkillsUpgradeAll(c)
			},
		},
		&cobra.Command{Use: "share <slug>", Short: "File a skill draft upstream as a contribution issue", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
			filePath := filepath.Join(skillsManifestDir(), args[0], "SKILL.md")
			return runShareUpstream(args[0], filePath, "convergent-systems-co/skill-atoms", "", cmd.OutOrStdout())
		}},
		newSkillsLinkCmd(),
	)
	return c
}

// newSkillsListCmd implements `ai skills list`.
// Reads ~/.ai/skills/ (or $AI_ROOT/skills/) and prints a two-column table:
// name | description.
func newSkillsListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List installed skills with their descriptions",
		RunE: func(cmd *cobra.Command, _ []string) error {
			skillsDir := skillsManifestDir()
			dirs, err := listSkillDirs(skillsDir)
			if err != nil {
				return fmt.Errorf("reading skills dir: %w", err)
			}
			if len(dirs) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "(no skills installed)")
				return nil
			}

			// Collect infos to compute column widths.
			infos := make([]skillInfo, 0, len(dirs))
			for _, d := range dirs {
				infos = append(infos, loadSkillInfo(d))
			}

			// Column width = longest name.
			maxName := 4 // min width "name"
			for _, si := range infos {
				if len(si.name) > maxName {
					maxName = len(si.name)
				}
			}

			w := cmd.OutOrStdout()
			for _, si := range infos {
				desc := si.description
				if desc == "" {
					hasMD := true
					if _, statErr := os.Stat(filepath.Join(si.dir, "SKILL.md")); os.IsNotExist(statErr) {
						hasMD = false
					}
					if !hasMD {
						desc = "(no SKILL.md)"
					} else {
						desc = "(no description)"
					}
				}
				fmt.Fprintf(w, "%-*s  %s\n", maxName, si.name, desc)
			}
			return nil
		},
	}
}

// newSkillsShowCmd implements `ai skills show <name>`.
// Finds a skill by exact or prefix match and prints the SKILL.md content.
func newSkillsShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show <name>",
		Short: "Show SKILL.md content for a named skill",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			query := args[0]
			skillsDir := skillsManifestDir()

			dir, err := findSkillDir(skillsDir, query)
			if err != nil {
				return fmt.Errorf("searching skills dir: %w", err)
			}
			if dir == "" {
				return fmt.Errorf("skill '%s' not found in %s", query, skillsDir)
			}

			mdPath := filepath.Join(dir, "SKILL.md")
			data, err := os.ReadFile(mdPath)
			if err != nil {
				return fmt.Errorf("reading SKILL.md for '%s': %w", query, err)
			}
			fmt.Fprint(cmd.OutOrStdout(), string(data))
			return nil
		},
	}
}

// newSkillsValidateCmd implements `ai skills validate`.
// Walks each skill subdir, checks SKILL.md existence and frontmatter.
// Always exits 0; warnings are informational.
func newSkillsValidateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "validate",
		Short: "Check each installed skill for a valid SKILL.md and frontmatter",
		RunE: func(cmd *cobra.Command, _ []string) error {
			skillsDir := skillsManifestDir()
			dirs, err := listSkillDirs(skillsDir)
			if err != nil {
				return fmt.Errorf("reading skills dir: %w", err)
			}

			w := cmd.OutOrStdout()
			for _, d := range dirs {
				name := filepath.Base(d)
				mdPath := filepath.Join(d, "SKILL.md")
				data, readErr := os.ReadFile(mdPath)
				if readErr != nil {
					if os.IsNotExist(readErr) {
						fmt.Fprintf(w, "[⚠] %s: SKILL.md missing\n", name)
					} else {
						fmt.Fprintf(w, "[⚠] %s: cannot read SKILL.md: %v\n", name, readErr)
					}
					continue
				}

				fields := parseFrontmatter(string(data))
				if _, ok := fields["name"]; !ok || fields["name"] == "" {
					fmt.Fprintf(w, "[⚠] %s: missing frontmatter field 'name'\n", name)
					continue
				}
				if _, ok := fields["description"]; !ok || fields["description"] == "" {
					fmt.Fprintf(w, "[⚠] %s: missing frontmatter field 'description'\n", name)
					continue
				}
				fmt.Fprintf(w, "[✓] %s\n", name)
			}
			return nil
		},
	}
}

// newSkillsTemplatesCmd implements `ai skills templates {list,show}`.
func newSkillsTemplatesCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "templates",
		Short: "Work with templates inside a skill directory",
	}
	c.AddCommand(
		newSkillsTemplatesListCmd(),
		newSkillsTemplatesShowCmd(),
	)
	return c
}

// newSkillsTemplatesListCmd implements `ai skills templates list <skill>`.
func newSkillsTemplatesListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list <skill>",
		Short: "List template files in a skill's templates/ directory",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			query := args[0]
			skillsDir := skillsManifestDir()

			dir, err := findSkillDir(skillsDir, query)
			if err != nil {
				return fmt.Errorf("searching skills dir: %w", err)
			}
			if dir == "" {
				return fmt.Errorf("skill '%s' not found in %s", query, skillsDir)
			}

			templatesDir := filepath.Join(dir, "templates")
			entries, err := os.ReadDir(templatesDir)
			if err != nil {
				if os.IsNotExist(err) {
					return fmt.Errorf("skill '%s' has no templates/ directory", query)
				}
				return fmt.Errorf("reading templates dir: %w", err)
			}

			w := cmd.OutOrStdout()
			for _, e := range entries {
				if !e.IsDir() {
					fmt.Fprintln(w, e.Name())
				}
			}
			return nil
		},
	}
}

// varPattern matches $VAR and ${VAR} substitution targets.
var varPattern = regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)\}|\$([A-Za-z_][A-Za-z0-9_]*)`)

// renderTemplate substitutes $VAR / ${VAR} patterns in content.
// Lookup order: flags (flagVars map) first, then environment.
// Unresolved vars are left unchanged.
func renderTemplate(content string, flagVars map[string]string) string {
	var sb strings.Builder
	scanner := bufio.NewScanner(strings.NewReader(content))
	first := true
	for scanner.Scan() {
		if !first {
			sb.WriteByte('\n')
		}
		first = false
		line := varPattern.ReplaceAllStringFunc(scanner.Text(), func(match string) string {
			// Extract the variable name from either $VAR or ${VAR} form.
			sub := varPattern.FindStringSubmatch(match)
			var varName string
			if sub[1] != "" {
				varName = sub[1]
			} else {
				varName = sub[2]
			}
			// Flag takes priority over environment.
			if v, ok := flagVars[varName]; ok {
				return v
			}
			if v := os.Getenv(varName); v != "" {
				return v
			}
			return match // leave unresolved
		})
		sb.WriteString(line)
	}
	// Preserve trailing newline if original content had one.
	if strings.HasSuffix(content, "\n") {
		sb.WriteByte('\n')
	}
	return sb.String()
}

// newSkillsTemplatesShowCmd implements `ai skills templates show <skill> <template>`.
func newSkillsTemplatesShowCmd() *cobra.Command {
	var varFlags []string

	c := &cobra.Command{
		Use:   "show <skill> <template>",
		Short: "Render a skill template with variable substitution",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			query := args[0]
			templateName := args[1]
			skillsDir := skillsManifestDir()

			dir, err := findSkillDir(skillsDir, query)
			if err != nil {
				return fmt.Errorf("searching skills dir: %w", err)
			}
			if dir == "" {
				return fmt.Errorf("skill '%s' not found in %s", query, skillsDir)
			}

			templatePath := filepath.Join(dir, "templates", templateName)
			data, err := os.ReadFile(templatePath)
			if err != nil {
				if os.IsNotExist(err) {
					return fmt.Errorf("template '%s' not found in skill '%s'", templateName, query)
				}
				return fmt.Errorf("reading template: %w", err)
			}

			// Parse --var KEY=VALUE flags into a map.
			flagVars := make(map[string]string, len(varFlags))
			for _, kv := range varFlags {
				idx := strings.Index(kv, "=")
				if idx <= 0 {
					return fmt.Errorf("invalid --var format '%s': expected KEY=VALUE", kv)
				}
				flagVars[kv[:idx]] = kv[idx+1:]
			}

			rendered := renderTemplate(string(data), flagVars)
			fmt.Fprint(cmd.OutOrStdout(), rendered)
			return nil
		},
	}
	c.Flags().StringArrayVar(&varFlags, "var", nil, "variable substitution in KEY=VALUE form (repeatable)")
	return c
}
