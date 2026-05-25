package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
)

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
		// Retained stubs for install/uninstall/upgrade/upgrade-all/share.
		&cobra.Command{Use: "install <name>[@<version>]", Short: "Resolve from skill-atoms.com; cache; symlink", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
			notice("skills install:", args[0])
			return stub("skills install", "§7.10.2")
		}},
		&cobra.Command{Use: "uninstall <name>", Short: "Remove manifest + symlink (content stays cached)", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
			notice("skills uninstall:", args[0])
			return stub("skills uninstall", "§7.10.2")
		}},
		&cobra.Command{Use: "upgrade <name> [<version>]", Short: "Bump manifest, refetch, re-symlink", Args: cobra.RangeArgs(1, 2), RunE: func(cmd *cobra.Command, args []string) error {
			notice("skills upgrade:", args)
			return stub("skills upgrade", "§7.10.2")
		}},
		&cobra.Command{Use: "upgrade-all", Short: "Bump every installed skill to its latest stable", RunE: func(cmd *cobra.Command, _ []string) error {
			notice("skills upgrade-all")
			return stub("skills upgrade-all", "§7.10.2")
		}},
		&cobra.Command{Use: "share <name>", Short: "File a skill draft upstream as an atom PR", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
			notice("skills share:", args[0])
			return stub("skills share", "§7.10.3")
		}},
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
