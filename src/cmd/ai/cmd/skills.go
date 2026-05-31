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
	"strconv"
	"strings"

	cbterm "github.com/charmbracelet/x/term"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
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

type skillFrontmatter struct {
	Name          string   `yaml:"name"`
	Description   string   `yaml:"description"`
	Version       string   `yaml:"version,omitempty"`
	UserInvocable bool     `yaml:"user-invocable,omitempty"`
	AllowedTools  []string `yaml:"allowed-tools,omitempty"`
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

// catalogSkillEntry extends skillRow with sub-skill count for display.
type catalogSkillEntry struct {
	slug     string
	name     string
	fullDesc string // full, untruncated description
	subCount int
	category string // primary category slug ("" → uncategorized)
}

// runSkillsAvailable implements `ai skills available`.
// With -p/--pick it launches an interactive picker; --category <slug> limits
// the listing to one category. Otherwise it prints skills grouped by category.
func runSkillsAvailable(cmd *cobra.Command, _ []string) error {
	pick, _ := cmd.Flags().GetBool("pick")
	categoryFilter, _ := cmd.Flags().GetString("category")
	if categoryFilter != "" && !isValidCategory(categoryFilter) {
		return fmt.Errorf("skills available: unknown category %q (valid: %s)",
			categoryFilter, strings.Join(categorySlugs(), ", "))
	}

	catalog, err := fetchAiAtomsCatalog()
	if err != nil {
		return err
	}

	// Collect sub-skill slugs so they are hidden from the top-level list.
	subSkills := map[string]bool{}
	for _, a := range catalog {
		if strings.ToLower(a.Lifecycle) == "deprecated" || strings.ToLower(a.Lifecycle) == "retired" {
			continue
		}
		if a.Type == "skill" {
			for _, dep := range a.DependsOn {
				// depends_on entries are namespaced ("skill/make-work") while
				// the slug compared below is the bare form ("make-work"). Strip
				// the prefix so sub-skills are matched and hidden.
				subSkills[strings.TrimPrefix(dep, "skill/")] = true
			}
		}
	}

	var entries []catalogSkillEntry
	for _, a := range catalog {
		lc := strings.ToLower(a.Lifecycle)
		if a.Type != "skill" || lc == "deprecated" || lc == "retired" {
			continue
		}
		slug := strings.TrimPrefix(a.ID, "skill/")
		if subSkills[slug] {
			continue
		}
		if categoryFilter != "" && a.Category != categoryFilter {
			continue
		}
		name := a.Name
		if name == "" {
			name = slug
		}
		entries = append(entries, catalogSkillEntry{
			slug:     slug,
			name:     name,
			fullDesc: a.Description,
			subCount: len(a.DependsOn),
			category: a.Category,
		})
	}

	if len(entries) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "(no skills available)")
		return nil
	}

	if pick {
		return runCatalogSkillPicker(cmd, entries)
	}

	out := cmd.OutOrStdout()
	hint := "  •  --category <slug> to filter"
	if categoryFilter != "" {
		hint = fmt.Sprintf("  •  category: %s", categoryDisplay(categoryFilter))
	}
	fmt.Fprintf(out, "\n  %d skills  •  ai skills install <slug>  •  -p to pick interactively%s\n\n",
		len(entries), hint)

	printSkillsGroupedByCategory(out, entries)
	return nil
}

// terminalWidth returns the current terminal width, or 80 when stdout is not a
// TTY (piped output, CI) so table layout stays deterministic.
func terminalWidth() int {
	if w, _, err := cbterm.GetSize(os.Stdout.Fd()); err == nil && w > 20 {
		return w
	}
	return 80
}

// wrapText word-wraps s to width columns, hard-breaking any single token longer
// than width (e.g. URLs). Always returns at least one (possibly empty) line.
func wrapText(s string, width int) []string {
	if width < 1 {
		width = 1
	}
	var lines []string
	for _, para := range strings.Split(s, "\n") {
		var line string
		for _, word := range strings.Fields(para) {
			for len(word) > width { // hard-break a token that can't fit on one line
				if line != "" {
					lines = append(lines, line)
					line = ""
				}
				lines = append(lines, word[:width])
				word = word[width:]
			}
			switch {
			case word == "":
				// fully consumed by the hard-break above
			case line == "":
				line = word
			case len(line)+1+len(word) <= width:
				line += " " + word
			default:
				lines = append(lines, line)
				line = word
			}
		}
		if line != "" {
			lines = append(lines, line)
		}
	}
	if len(lines) == 0 {
		lines = append(lines, "")
	}
	return lines
}

// printSkillsGroupedByCategory prints each category as a heading followed by a
// bordered Skill|Description table whose description cell wraps to fit the
// terminal, so slugs and descriptions never cross over.
func printSkillsGroupedByCategory(out io.Writer, entries []catalogSkillEntry) {
	byCat := map[string][]catalogSkillEntry{}
	for _, e := range entries {
		key := e.category
		if !isValidCategory(key) {
			key = uncategorizedSlug
		}
		byCat[key] = append(byCat[key], e)
	}

	width := terminalWidth()
	for _, slug := range append(categorySlugs(), uncategorizedSlug) {
		group := byCat[slug]
		if len(group) == 0 {
			continue
		}
		fmt.Fprintf(out, "\n  \033[1;4m%s\033[0m  (%d)\n", categoryDisplay(slug), len(group))
		printSkillTable(out, group, width)
	}
}

// printSkillTable renders entries as a two-column box-drawing table. The Skill
// column is sized to the widest slug (capped); the Description column wraps to
// the remaining width so text stays inside its cell.
func printSkillTable(out io.Writer, entries []catalogSkillEntry, termWidth int) {
	const (
		indent     = "  "
		maxSlugCol = 32
		minDescCol = 24
	)
	type cell struct{ slug, desc string }
	cells := make([]cell, len(entries))
	slugCol := len("Skill")
	for i, e := range entries {
		s := e.slug
		if e.subCount > 0 {
			s += fmt.Sprintf(" (+%d)", e.subCount)
		}
		cells[i] = cell{slug: s, desc: e.fullDesc}
		if len(s) > slugCol {
			slugCol = len(s)
		}
	}
	if slugCol > maxSlugCol {
		slugCol = maxSlugCol
	}
	// Row layout: indent + "│ " + <slugCol> + " │ " + <descCol> + " │"
	descCol := termWidth - len(indent) - slugCol - 7
	if descCol < minDescCol {
		descCol = minDescCol
	}

	rule := func(l, m, r string) string {
		return indent + l + strings.Repeat("─", slugCol+2) + m + strings.Repeat("─", descCol+2) + r
	}
	fmt.Fprintln(out, rule("┌", "┬", "┐"))
	for i, c := range cells {
		if i > 0 {
			fmt.Fprintln(out, rule("├", "┼", "┤"))
		}
		slugLines := wrapText(c.slug, slugCol)
		descLines := wrapText(c.desc, descCol)
		rows := len(slugLines)
		if len(descLines) > rows {
			rows = len(descLines)
		}
		for k := 0; k < rows; k++ {
			s, d := "", ""
			if k < len(slugLines) {
				s = slugLines[k]
			}
			if k < len(descLines) {
				d = descLines[k]
			}
			// Bold the slug on its first line only; pad with plain spaces so the
			// ANSI escape doesn't throw off column alignment.
			sField := s + strings.Repeat(" ", slugCol-len(s))
			if k == 0 && s != "" {
				sField = "\033[1m" + s + "\033[0m" + strings.Repeat(" ", slugCol-len(s))
			}
			fmt.Fprintf(out, "%s│ %s │ %-*s │\n", indent, sField, descCol, d)
		}
	}
	fmt.Fprintln(out, rule("└", "┴", "┘"))
}

// runCatalogSkillPicker launches the Bubble Tea checkbox TUI for skill selection.
func runCatalogSkillPicker(cmd *cobra.Command, entries []catalogSkillEntry) error {
	return runCatalogSkillPickerTUI(cmd, entries)
}

// newSkillsAvailableCmd returns the cobra command for `ai skills available`.
func newSkillsAvailableCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "available",
		Short: "List skills available to install from skill-atoms.com",
		Args:  cobra.NoArgs,
		RunE:  runSkillsAvailable,
	}
	c.Flags().BoolP("pick", "p", false, "interactively pick and install skills")
	c.Flags().String("category", "", "limit listing to one category slug (see `ai skills categories`)")
	return c
}

// runSkillsCategories implements `ai skills categories`: a browse menu listing
// each category with its available (non-sub-skill) count, to narrow selection.
func runSkillsCategories(cmd *cobra.Command, _ []string) error {
	catalog, err := fetchAiAtomsCatalog()
	if err != nil {
		return err
	}

	subSkills := map[string]bool{}
	for _, a := range catalog {
		if a.Type == "skill" {
			for _, dep := range a.DependsOn {
				subSkills[strings.TrimPrefix(dep, "skill/")] = true
			}
		}
	}

	counts := map[string]int{}
	for _, a := range catalog {
		lc := strings.ToLower(a.Lifecycle)
		if a.Type != "skill" || lc == "deprecated" || lc == "retired" {
			continue
		}
		if subSkills[strings.TrimPrefix(a.ID, "skill/")] {
			continue
		}
		key := a.Category
		if !isValidCategory(key) {
			key = uncategorizedSlug
		}
		counts[key]++
	}

	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "\n  Categories  •  ai skills available --category <slug>\n")
	printCategoriesTable(out, counts)
	return nil
}

// printCategoriesTable renders the category counts as a bordered
// Slug | N | Category table, skipping categories with no skills.
func printCategoriesTable(out io.Writer, counts map[string]int) {
	const indent = "  "
	type row struct {
		slug, name string
		n          int
	}
	var rows []row
	slugCol, nameCol := len("Slug"), len("Category")
	for _, slug := range append(categorySlugs(), uncategorizedSlug) {
		if counts[slug] == 0 {
			continue
		}
		r := row{slug: slug, name: categoryDisplay(slug), n: counts[slug]}
		rows = append(rows, r)
		if len(r.slug) > slugCol {
			slugCol = len(r.slug)
		}
		if len(r.name) > nameCol {
			nameCol = len(r.name)
		}
	}
	const countCol = 3 // up to 999 skills
	rule := func(l, m1, m2, r string) string {
		return indent + l + strings.Repeat("─", slugCol+2) + m1 +
			strings.Repeat("─", countCol+2) + m2 + strings.Repeat("─", nameCol+2) + r
	}
	fmt.Fprintln(out, rule("┌", "┬", "┬", "┐"))
	fmt.Fprintf(out, "%s│ \033[1m%-*s\033[0m │ %*s │ \033[1m%-*s\033[0m │\n",
		indent, slugCol, "Slug", countCol, "N", nameCol, "Category")
	fmt.Fprintln(out, rule("├", "┼", "┼", "┤"))
	for _, r := range rows {
		fmt.Fprintf(out, "%s│ %-*s │ %*d │ %-*s │\n",
			indent, slugCol, r.slug, countCol, r.n, nameCol, r.name)
	}
	fmt.Fprintln(out, rule("└", "┴", "┴", "┘"))
}

// newSkillsCategoriesCmd returns the cobra command for `ai skills categories`.
func newSkillsCategoriesCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "categories",
		Short: "List skill categories with counts (browse menu to cut catalog overwhelm)",
		Args:  cobra.NoArgs,
		RunE:  runSkillsCategories,
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

// copilotSkillsDir returns the canonical ~/.copilot/skills/ path.
// Override priority: $COPILOT_SKILLS_DIR env var, then
// $HOME/.copilot/skills/ (only when the directory actually exists).
// Returns "" when the directory is absent — Copilot skill wiring is silently
// skipped on machines that have not set up GitHub Copilot skills.
func copilotSkillsDir() string {
	if env := os.Getenv("COPILOT_SKILLS_DIR"); env != "" {
		if _, err := os.Stat(env); err == nil {
			return env
		}
		return "" // env override but directory is absent — skip
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	dir := filepath.Join(home, ".copilot", "skills")
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return ""
	}
	return dir
}

// copilotSkillsTargetDir returns the intended ~/.copilot/skills path (or the
// $COPILOT_SKILLS_DIR override) WITHOUT requiring it to exist.
// `ai skills link` uses this to create-then-link; the existence-gated
// copilotSkillsDir() is for callers (install/upgrade) that should skip
// Copilot wiring silently on machines without it.
func copilotSkillsTargetDir() string {
	// An explicitly-set override (even empty) wins: empty means "skip Copilot"
	// and must NOT fall through to the real $HOME (which would touch the home
	// dir in tests and on machines that opted out via an empty override).
	if env, ok := os.LookupEnv("COPILOT_SKILLS_DIR"); ok {
		return env
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".copilot", "skills")
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

	content, err := renderSkillMD(skillFrontmatter{
		Name:          slug,
		Description:   atom.Description,
		Version:       atom.Version,
		UserInvocable: true,
		AllowedTools:  []string{"Bash", "Read"},
	}, atom.Name, atom.SystemPromptFragment)
	if err != nil {
		return err
	}

	return os.WriteFile(destPath, []byte(content), 0o644) //nolint:gosec // 0644 is intentional for skill files
}

func renderSkillMD(meta skillFrontmatter, title, body string) (string, error) {
	fm, err := yaml.Marshal(meta)
	if err != nil {
		return "", fmt.Errorf("skills: marshal frontmatter: %w", err)
	}
	if title == "" {
		title = meta.Name
	}

	var b strings.Builder
	b.WriteString("---\n")
	b.Write(fm)
	b.WriteString("---\n")
	if title != "" {
		b.WriteString("# ")
		b.WriteString(title)
		b.WriteString("\n\n")
	}
	b.WriteString(body)
	if body != "" && !strings.HasSuffix(body, "\n") {
		b.WriteString("\n")
	}
	return b.String(), nil
}

func repairLegacySkillMD(mdPath string) error {
	data, err := os.ReadFile(mdPath)
	if err != nil {
		return err
	}
	content := string(data)
	lines := strings.Split(content, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return nil
	}

	end := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			end = i
			break
		}
	}
	if end == -1 {
		return nil
	}

	var meta skillFrontmatter
	if err := yaml.Unmarshal([]byte(strings.Join(lines[1:end], "\n")), &meta); err == nil {
		return nil
	}

	fields := parseFrontmatter(content)
	if fields["name"] == "" || fields["description"] == "" {
		return nil
	}

	meta.Name = unquoteFrontmatterValue(fields["name"])
	meta.Description = unquoteFrontmatterValue(fields["description"])
	meta.Version = unquoteFrontmatterValue(fields["version"])
	meta.UserInvocable = strings.EqualFold(unquoteFrontmatterValue(fields["user-invocable"]), "true")
	meta.AllowedTools = parseAllowedTools(lines[1:end])

	body := strings.Join(lines[end+1:], "\n")
	body = strings.TrimPrefix(body, "\n")
	title := meta.Name
	if strings.HasPrefix(body, "# ") {
		if nl := strings.IndexByte(body, '\n'); nl >= 0 {
			title = strings.TrimSpace(strings.TrimPrefix(body[:nl], "# "))
			body = strings.TrimPrefix(body[nl+1:], "\n")
		} else {
			title = strings.TrimSpace(strings.TrimPrefix(body, "# "))
			body = ""
		}
	}

	rewritten, err := renderSkillMD(meta, title, body)
	if err != nil {
		return err
	}
	if rewritten == content {
		return nil
	}
	return os.WriteFile(mdPath, []byte(rewritten), 0o644) //nolint:gosec // 0644 is intentional for skill files
}

func parseAllowedTools(lines []string) []string {
	var tools []string
	inAllowedTools := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		switch {
		case trimmed == "allowed-tools:":
			inAllowedTools = true
		case inAllowedTools && strings.HasPrefix(trimmed, "- "):
			tools = append(tools, strings.TrimSpace(strings.TrimPrefix(trimmed, "- ")))
		case inAllowedTools && trimmed == "":
			continue
		case inAllowedTools:
			return tools
		}
	}
	return tools
}

func unquoteFrontmatterValue(v string) string {
	v = strings.TrimSpace(v)
	if unquoted, err := strconv.Unquote(v); err == nil {
		return unquoted
	}
	return v
}

// ensureSymlink creates or replaces a symlink at linkPath → target.
// If linkPath already exists (as a symlink or file), it is removed first.
func ensureSymlink(target, linkPath string) error {
	if _, err := os.Lstat(linkPath); err == nil {
		if removeErr := os.Remove(linkPath); removeErr != nil {
			return fmt.Errorf("skills: remove existing symlink %s: %w", linkPath, removeErr)
		}
	}
	return symlinkOrCopy(target, linkPath)
}

// runSkillsInstall is the implementation of `ai skills install <name>[@version]`.
// It fetches the atom JSON, writes SKILL.md, and optionally creates consumer symlinks.
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

	// Optional: symlink ~/.copilot/skills/<slug> → ~/.ai/skills/<slug>
	if copilotDir := copilotSkillsDir(); copilotDir != "" {
		linkPath := filepath.Join(copilotDir, slug)
		if symlinkErr := ensureSymlink(destDir, linkPath); symlinkErr != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "warning: could not create Copilot symlink: %v\n", symlinkErr)
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

	// Remove symlink from ~/.copilot/skills/ if it exists.
	if copilotDir := copilotSkillsDir(); copilotDir != "" {
		linkPath := filepath.Join(copilotDir, slug)
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

	// Re-create symlink in Copilot skills dir if it existed before.
	copilotDir := copilotSkillsDir()
	if copilotDir != "" {
		linkPath := filepath.Join(copilotDir, slug)
		if _, lstatErr := os.Lstat(linkPath); lstatErr == nil {
			if symlinkErr := ensureSymlink(dir, linkPath); symlinkErr != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: could not refresh Copilot symlink: %v\n", symlinkErr)
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
// ~/.claude/skills/ and ~/.copilot/skills/. It is idempotent:
// running it multiple times over already-linked skills produces no error.
func runSkillsLink(cmd *cobra.Command, _ []string) error {
	skillsDir := skillsManifestDir()
	dirs, err := listSkillDirs(skillsDir)
	if err != nil {
		return fmt.Errorf("skills link: list installed: %w", err)
	}

	// `link` is an explicit user action, so create the target dirs if they do
	// not already exist rather than silently no-op (#479). This uses the
	// unconditional *target* resolvers, not the existence-gated ones used by
	// install/upgrade (which skip Copilot wiring on non-Copilot machines).
	claudeDir := claudeSkillsDir()
	copilotDir := copilotSkillsTargetDir()
	if claudeDir != "" {
		if mkErr := os.MkdirAll(claudeDir, 0o755); mkErr != nil {
			return fmt.Errorf("skills link: create Claude skills dir %q: %w", claudeDir, mkErr)
		}
	}
	if copilotDir != "" {
		if mkErr := os.MkdirAll(copilotDir, 0o755); mkErr != nil {
			return fmt.Errorf("skills link: create Copilot skills dir %q: %w", copilotDir, mkErr)
		}
	}

	var linkedClaude, linkedCopilot int
	for _, skillPath := range dirs {
		slug := filepath.Base(skillPath)
		if err := repairLegacySkillMD(filepath.Join(skillPath, "SKILL.md")); err != nil && !os.IsNotExist(err) {
			fmt.Fprintf(cmd.ErrOrStderr(), "warning: could not repair SKILL.md for %s: %v\n", slug, err)
		}

		if claudeDir != "" {
			linkPath := filepath.Join(claudeDir, slug)
			if err := ensureSymlink(skillPath, linkPath); err == nil {
				linkedClaude++
			}
		}

		if copilotDir != "" {
			linkPath := filepath.Join(copilotDir, slug)
			if err := ensureSymlink(skillPath, linkPath); err == nil {
				linkedCopilot++
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
		Short: "Symlink all installed skills to ~/.claude/skills/ and ~/.copilot/skills/",
		Long: `link iterates every installed skill under ~/.ai/skills/ and creates
symlinks in both consumer directories:

  ~/.claude/skills/<slug>           → ~/.ai/skills/<slug>/
  ~/.copilot/skills/<slug>          → ~/.ai/skills/<slug>/

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
		newSkillsCategoriesCmd(),
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
