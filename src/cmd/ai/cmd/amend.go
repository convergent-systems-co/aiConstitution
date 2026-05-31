package cmd

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// aiRoot returns the canonical ~/.ai/ root. Honors $AI_ROOT for testing.
// Mirrors aiRoot() from the internal/paths package without importing it
// across module boundaries.
func aiRoot() string {
	if env := os.Getenv("AI_ROOT"); env != "" {
		return env
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ".ai"
	}
	return filepath.Join(home, ".ai")
}

// newAmendCmd implements `ai amend` with subcommands: draft, apply, list, show,
// publish. See SPEC.md §3.6 and §6 (Memory → Amendment Lifecycle).
func newAmendCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "amend",
		Short: "Amendment lifecycle: draft, apply, list, show, publish",
		Long: `amend manages the full lifecycle of a governance amendment against
one of the four canonical files (Constitution, Common, Code, Writing).

Subcommands:
  draft   <violation-path>   Parse a violation file and write an amendment stub.
  apply   <slug-or-path>     Patch the canonical file from a stub.
  list                       List all pending stubs newest-first.
  show    <slug>             Print a stub by slug prefix.
  publish <slug>             Validate and (dry-run) construct the release command.

See SPEC.md §3.6 and §6.`,
		// No RunE — subcommand dispatch only.
	}

	c.AddCommand(
		newAmendDraftCmd(),
		newAmendApplyCmd(),
		newAmendListCmd(),
		newAmendShowCmd(),
		newAmendPublishCmd(),
	)

	return c
}

// ─── draft ────────────────────────────────────────────────────────────────────

// newAmendDraftCmd implements `ai amend draft <violation-path>`.
//
// Coder A OWNS this function (#184, #185).
func newAmendDraftCmd() *cobra.Command {
	var fromViolation string

	c := &cobra.Command{
		Use:   "draft <violation-path>",
		Short: "Parse a violation file and write an amendment stub",
		Long: `draft reads a violation audit file and produces a stub amendment plan at
$AI_ROOT/governance/plans/<UTC>-<slug>.md.

When $EDITOR is set the stub is opened for editing. When $EDITOR is unset
the stub path is printed to stdout.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Resolve the violation file path from args or flag.
			violationPath := fromViolation
			if len(args) == 1 {
				violationPath = args[0]
			}
			if violationPath == "" {
				return fmt.Errorf("amend draft: provide <violation-path> or --from-violation=<path>")
			}

			stub, err := parseViolationFile(violationPath)
			if err != nil {
				return fmt.Errorf("amend draft: parse violation file: %w", err)
			}

			stubPath, err := writeAmendmentStub(stub)
			if err != nil {
				return fmt.Errorf("amend draft: write stub: %w", err)
			}

			editor := os.Getenv("EDITOR")
			if editor != "" {
				return execEditor(editor, stubPath)
			}

			// No editor set: print path.
			fmt.Fprintln(cmd.OutOrStdout(), stubPath) //nolint:errcheck
			return nil
		},
	}

	c.Flags().StringVar(&fromViolation, "from-violation", "", "path to the violation file (alternative to positional arg)")

	return c
}

// violationFields holds the fields parsed from a violation audit file.
type violationFields struct {
	ruleRef           string // "File / Rule violated:" value
	whatHappened      string // "What happened:" value
	proposedAmendment string // "Proposed amendment (if any):" value
}

// parseViolationFile reads a violation audit file and extracts the three
// fields used by the amendment stub.
func parseViolationFile(path string) (violationFields, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return violationFields{}, err
	}

	var out violationFields
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		line := scanner.Text()
		if v, ok := extractField(line, "**File / Rule violated:**"); ok {
			out.ruleRef = v
		} else if v, ok := extractField(line, "**Section / Rule violated:**"); ok {
			out.ruleRef = v
		} else if v, ok := extractField(line, "**What happened:**"); ok {
			out.whatHappened = v
		} else if v, ok := extractField(line, "**Proposed amendment (if any):**"); ok {
			out.proposedAmendment = v
		}
	}
	if err := scanner.Err(); err != nil {
		return violationFields{}, err
	}
	if out.ruleRef == "" {
		return violationFields{}, fmt.Errorf("violation file missing 'File / Rule violated:' field")
	}
	return out, nil
}

// extractField checks whether line contains "- <label> <value>" and returns
// the trimmed value. Returns ("", false) when the label is absent.
func extractField(line, label string) (string, bool) {
	// Lines are of the form "- **Label:** value"
	idx := strings.Index(line, label)
	if idx < 0 {
		return "", false
	}
	val := strings.TrimSpace(line[idx+len(label):])
	return val, true
}

// writeAmendmentStub derives the slug, creates the plans directory, writes
// the stub file, and returns the absolute path.
func writeAmendmentStub(fields violationFields) (string, error) {
	slug := deriveSlug(fields.ruleRef)
	utc := time.Now().UTC().Format("20060102T150405Z")
	filename := fmt.Sprintf("%s-%s.md", utc, slug)

	plansDir := filepath.Join(aiRoot(), "governance", "plans")
	if err := os.MkdirAll(plansDir, 0o755); err != nil {
		return "", fmt.Errorf("create plans dir: %w", err)
	}

	stubPath := filepath.Join(plansDir, filename)
	content := buildStubContent(slug, buildTargetBlock(fields.ruleRef), fields)
	if err := os.WriteFile(stubPath, []byte(content), 0o644); err != nil {
		return "", err
	}
	return stubPath, nil
}

// deriveSlug converts a rule reference string into a kebab-case slug ≤ 32 chars.
//
// "Common.md/U17 — worktree placement" → "common-md-u17-worktree-placemen"
func deriveSlug(ruleRef string) string {
	// Normalize: lower-case, replace non-alphanumeric runs with a dash.
	re := regexp.MustCompile(`[^a-z0-9]+`)
	slug := re.ReplaceAllString(strings.ToLower(ruleRef), "-")
	// Trim leading/trailing dashes.
	slug = strings.Trim(slug, "-")
	// Truncate to 32 chars.
	if len(slug) > 32 {
		slug = slug[:32]
	}
	// Remove trailing dash that truncation may have introduced.
	slug = strings.TrimRight(slug, "-")
	return slug
}

// buildStubContent assembles the amendment stub file body.
func buildStubContent(slug, targetBlock string, fields violationFields) string {
	proposed := fields.proposedAmendment
	if proposed == "" {
		proposed = "(fill in proposed change)"
	}
	rationale := fields.whatHappened
	if rationale == "" {
		rationale = "(fill in rationale)"
	}
	return fmt.Sprintf("# Amendment Draft — %s\n\n## Target\n%s\n\n## Proposed Change\n%s\n\n## Rationale\n%s\n",
		slug, targetBlock, proposed, rationale)
}

// execEditor launches the user's $EDITOR with stubPath. Blocks until editor
// exits, then returns nil.
func execEditor(editor, stubPath string) error {
	// Some editors are specified as a path+flags string (e.g. "code --wait").
	// Split naively on spaces for the first word; remaining words become args.
	parts := strings.Fields(editor)
	args := append(parts[1:], stubPath)
	c := exec.Command(parts[0], args...) //nolint:gosec // editor is user-controlled
	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return c.Run()
}

// ─── apply ────────────────────────────────────────────────────────────────────

// newAmendApplyCmd implements `ai amend apply <slug-or-path>`.
//
// Coder B OWNS this function (#186, #187).
func newAmendApplyCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "apply <slug-or-path>",
		Short: "Patch the canonical file from an amendment stub",
		Long: `apply reads the named stub from $AI_ROOT/governance/plans/, locates
the target section in the referenced canonical file, replaces the section body,
bumps the file's minor version, and appends a Changelog entry.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			slugOrPath := args[0]

			// Resolve to plan file path.
			planPath, err := resolvePlanPath(slugOrPath)
			if err != nil {
				return fmt.Errorf("amend apply: %w", err)
			}

			planContent, err := os.ReadFile(planPath)
			if err != nil {
				return fmt.Errorf("amend apply: read plan: %w", err)
			}

			target, proposedChange, err := parsePlanStub(string(planContent))
			if err != nil {
				return fmt.Errorf("amend apply: parse plan: %w", err)
			}

			targetFile := target.File
			if targetFile == "" {
				targetFile = "Constitution.md"
			}
			if err := validateCanonicalAmendFile(targetFile); err != nil {
				return fmt.Errorf("amend apply: %w", err)
			}
			constPath := filepath.Join(aiRoot(), targetFile)

			constData, err := os.ReadFile(constPath)
			if err != nil {
				return fmt.Errorf("amend apply: read %s: %w", targetFile, err)
			}

			patched, newVersion, err := patchConstitution(string(constData), target, proposedChange, filepath.Base(planPath))
			if err != nil {
				return fmt.Errorf("amend apply: patch: %w", err)
			}

			if err := os.WriteFile(constPath, []byte(patched), 0o644); err != nil {
				return fmt.Errorf("amend apply: write %s: %w", targetFile, err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Applied: bumped version to %s\n", newVersion) //nolint:errcheck
			return nil
		},
	}
}

type planTarget struct {
	Raw     string
	File    string
	Section string
	Rule    string
}

// parsePlanStub extracts the ## Target and ## Proposed Change sections from a
// plan stub file.
func parsePlanStub(content string) (target planTarget, proposedChange string, err error) {
	// Extract ## Target section.
	targetBlock := extractSection(content, "## Target")
	if targetBlock == "" {
		return planTarget{}, "", fmt.Errorf("plan stub missing '## Target' section")
	}
	target = parsePlanTarget(targetBlock)

	// Extract ## Proposed Change section.
	proposedChange = extractSection(content, "## Proposed Change")
	if proposedChange == "" {
		return planTarget{}, "", fmt.Errorf("plan stub missing '## Proposed Change' section")
	}

	return target, proposedChange, nil
}

// extractSection returns the text between a `## <header>` line and the next
// `##` line (or end of file), trimmed of leading/trailing whitespace.
func extractSection(content, header string) string {
	lines := strings.Split(content, "\n")
	var inSection bool
	var buf strings.Builder

	for _, line := range lines {
		if strings.TrimSpace(line) == header {
			inSection = true
			continue
		}
		if inSection {
			// Stop at next ## heading.
			if strings.HasPrefix(line, "## ") {
				break
			}
			buf.WriteString(line)
			buf.WriteString("\n")
		}
	}
	return strings.TrimSpace(buf.String())
}

// patchConstitution locates the target block in constitutionText, appends the
// proposed change at the smallest safe granularity, bumps the minor version,
// appends a Changelog entry, and returns the patched text plus the new version
// string.
func patchConstitution(constitutionText string, target planTarget, proposedChange, planFilename string) (patched, newVersion string, err error) {
	// 1. Bump version.
	currentVersion, err := extractVersion(constitutionText)
	if err != nil {
		return "", "", err
	}
	newVersion, err = bumpMinor(currentVersion)
	if err != nil {
		return "", "", err
	}
	patched = strings.Replace(constitutionText,
		fmt.Sprintf("**Version:** %s", currentVersion),
		fmt.Sprintf("**Version:** %s", newVersion),
		1)

	// 2. Find the safest insertion point and append the proposed change.
	patched, err = appendTargetedChange(patched, target, proposedChange)
	if err != nil {
		return "", "", err
	}

	// 3. Append changelog entry.
	slug := slugFromPlanFilename(planFilename)
	firstLine := firstNonEmpty(strings.Split(proposedChange, "\n"))
	entry := fmt.Sprintf("- **%s** — %s: %s", newVersion, slug, firstLine)
	patched = appendChangelogEntry(patched, entry)

	return patched, newVersion, nil
}

// extractVersion finds "**Version:** x.y" in the first 20 lines of content
// and returns "x.y".
func extractVersion(content string) (string, error) {
	scanner := bufio.NewScanner(strings.NewReader(content))
	lineNum := 0
	re := regexp.MustCompile(`\*\*Version:\*\*\s+(\S+)`)
	for scanner.Scan() && lineNum < 20 {
		lineNum++
		if m := re.FindStringSubmatch(scanner.Text()); m != nil {
			return m[1], nil
		}
	}
	return "", fmt.Errorf("version string '**Version:** x.y' not found in first 20 lines")
}

// bumpMinor parses "major.minor" and returns "major.(minor+1)".
func bumpMinor(version string) (string, error) {
	parts := strings.SplitN(version, ".", 2)
	if len(parts) != 2 {
		return "", fmt.Errorf("version %q is not in major.minor format", version)
	}
	minor, err := strconv.Atoi(parts[1])
	if err != nil {
		return "", fmt.Errorf("version minor part %q is not an integer: %w", parts[1], err)
	}
	return fmt.Sprintf("%s.%d", parts[0], minor+1), nil
}

// replaceSectionBody finds the section whose heading matches target (case-insensitive
// normalized) and replaces its body (lines between this heading and next ##)
// with proposedChange.
func appendTargetedChange(content string, target planTarget, proposedChange string) (string, error) {
	resolved, err := resolvePlanTarget(content, target)
	if err != nil {
		return "", err
	}

	insertAt, err := findInsertOffset(content, resolved)
	if err != nil {
		return "", err
	}
	return spliceInsertedText(content, insertAt, proposedChange), nil
}

// appendChangelogEntry appends entry as a new bullet under the "## Changelog"
// section. If no Changelog section exists, appends one at the end.
func appendChangelogEntry(content, entry string) string {
	changelogHeading := "## Changelog"
	if !strings.Contains(content, changelogHeading) {
		return content + "\n" + changelogHeading + "\n\n" + entry + "\n"
	}

	// Insert after the "## Changelog" line. We want to add the new entry
	// immediately after the heading (newest-first ordering within the section).
	lines := strings.Split(content, "\n")
	var result []string
	inserted := false
	for i, line := range lines {
		result = append(result, line)
		if !inserted && strings.TrimSpace(line) == changelogHeading {
			// Peek ahead: skip blank lines immediately after heading.
			j := i + 1
			for j < len(lines) && strings.TrimSpace(lines[j]) == "" {
				j++
			}
			// Insert the new entry.
			result = append(result, "")
			result = append(result, entry)
			inserted = true
		}
	}
	return strings.Join(result, "\n")
}

// slugFromPlanFilename extracts the slug from a plan filename.
// "20260524T120000Z-prime-directives.md" → "prime-directives"
func slugFromPlanFilename(filename string) string {
	name := strings.TrimSuffix(filename, ".md")
	// UTC prefix is 16 chars: YYYYMMDDTHHMMSSZ
	if len(name) > 17 {
		return name[17:]
	}
	return name
}

// firstNonEmpty returns the first non-empty string from the slice.
func firstNonEmpty(ss []string) string {
	for _, s := range ss {
		if strings.TrimSpace(s) != "" {
			return strings.TrimSpace(s)
		}
	}
	return ""
}

// ─── list ─────────────────────────────────────────────────────────────────────

// newAmendListCmd implements `ai amend list`.
//
// Coder C OWNS this function (#188).
func newAmendListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List amendment stubs newest-first",
		RunE: func(cmd *cobra.Command, _ []string) error {
			plansDir := filepath.Join(aiRoot(), "governance", "plans")
			entries, err := os.ReadDir(plansDir)
			if err != nil {
				if os.IsNotExist(err) {
					// No plans directory yet — print nothing.
					return nil
				}
				return fmt.Errorf("amend list: %w", err)
			}

			// Filter to .md files and collect.
			type planEntry struct {
				filename string
				firstLine string
			}
			var plans []planEntry
			for _, e := range entries {
				if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
					continue
				}
				fullPath := filepath.Join(plansDir, e.Name())
				fl, _ := readFirstLine(fullPath)
				plans = append(plans, planEntry{
					filename:  e.Name(),
					firstLine: fl,
				})
			}

			// Sort newest-first: UTC prefix means reverse lexicographic = newest-first.
			sort.Slice(plans, func(i, j int) bool {
				return plans[i].filename > plans[j].filename
			})

			for _, p := range plans {
				slug := slugFromPlanFilename(p.filename)
				fmt.Fprintf(cmd.OutOrStdout(), "%-40s  %s\n", slug, p.firstLine) //nolint:errcheck
			}
			return nil
		},
	}
}

// readFirstLine reads the first non-empty line from a file.
func readFirstLine(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close() //nolint:errcheck

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			return line, nil
		}
	}
	return "", scanner.Err()
}

// ─── show ─────────────────────────────────────────────────────────────────────

// newAmendShowCmd implements `ai amend show <slug>`.
//
// Coder C OWNS this function (#189).
func newAmendShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show <slug>",
		Short: "Print an amendment stub by slug prefix",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			prefix := args[0]

			planPath, err := findPlanBySlug(prefix)
			if err != nil {
				return fmt.Errorf("amend show: %w", err)
			}

			content, err := os.ReadFile(planPath)
			if err != nil {
				return fmt.Errorf("amend show: read plan: %w", err)
			}

			fmt.Fprint(cmd.OutOrStdout(), string(content)) //nolint:errcheck
			return nil
		},
	}
}

// findPlanBySlug locates a plan file whose slug part matches the given prefix.
// Returns the full path or an error when no match is found.
func findPlanBySlug(prefix string) (string, error) {
	plansDir := filepath.Join(aiRoot(), "governance", "plans")
	entries, err := os.ReadDir(plansDir)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("no plan matching %q (plans dir does not exist)", prefix)
		}
		return "", err
	}

	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		// Match against the slug portion (after UTC prefix) OR against the
		// full filename stem. This allows both `"prime-directives"` and
		// `"20260524T120000Z-prime-directives"` to resolve to the same file.
		stem := strings.TrimSuffix(e.Name(), ".md")
		slug := slugFromPlanFilename(e.Name())
		if strings.HasPrefix(slug, prefix) || strings.HasPrefix(stem, prefix) {
			return filepath.Join(plansDir, e.Name()), nil
		}
	}
	return "", fmt.Errorf("no plan matching prefix %q", prefix)
}

// ─── publish ──────────────────────────────────────────────────────────────────

// newAmendPublishCmd implements `ai amend publish [--dry-run] <slug>`.
//
// Both --dry-run and the default mode are stub-only per spec notes for v0.8.
// Coder C OWNS this function (#190, #191).
func newAmendPublishCmd() *cobra.Command {
	var dryRun bool

	c := &cobra.Command{
		Use:   "publish <slug>",
		Short: "Validate and construct the release command for an applied amendment",
		Long: `publish validates that the named amendment stub has been applied
to Constitution.md and prints the gh release create command that would tag
the release. No actual gh invocation is made in v0.8 (stub/dry-run only).`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			slug := args[0]

			planPath, err := findPlanBySlug(slug)
			if err != nil {
				return fmt.Errorf("amend publish: %w", err)
			}

			planContent, err := os.ReadFile(planPath)
			if err != nil {
				return fmt.Errorf("amend publish: read plan: %w", err)
			}

			_, proposedChange, err := parsePlanStub(string(planContent))
			if err != nil {
				return fmt.Errorf("amend publish: parse plan: %w", err)
			}

			// Read current Constitution.md version.
			constPath := filepath.Join(aiRoot(), "Constitution.md")
			constData, err := os.ReadFile(constPath)
			if err != nil {
				return fmt.Errorf("amend publish: read Constitution.md: %w", err)
			}

			version, err := extractVersion(string(constData))
			if err != nil {
				return fmt.Errorf("amend publish: %w", err)
			}

			// Validate: check that the proposed change body appears in the constitution.
			if !strings.Contains(string(constData), proposedChange) {
				return fmt.Errorf("amend publish: stub not yet applied — section body from plan not found in Constitution.md (run `ai amend apply %s` first)", slug)
			}

			releaseCmd := fmt.Sprintf("gh release create v%s --title \"Constitution v%s\" --notes \"Amendment: %s\"",
				version, version, slug)

			if dryRun {
				fmt.Fprintf(cmd.OutOrStdout(), "Would run: %s\n", releaseCmd) //nolint:errcheck
			} else {
				// v0.8: also stub — print the command only.
				fmt.Fprintf(cmd.OutOrStdout(), "Would run: %s\n", releaseCmd)          //nolint:errcheck
				fmt.Fprintln(cmd.OutOrStdout(), "(actual gh release create deferred to v0.9)") //nolint:errcheck
			}
			return nil
		},
	}

	c.Flags().BoolVar(&dryRun, "dry-run", false, "validate and print the would-be release command without executing")

	return c
}

// ─── helpers shared by apply and publish ─────────────────────────────────────

// resolvePlanPath resolves a slug-or-path argument to an absolute file path.
// If the argument looks like a file path (contains a slash or ends in .md),
// it is returned as-is (after existence check). Otherwise it is treated as a
// slug and looked up via findPlanBySlug.
func resolvePlanPath(slugOrPath string) (string, error) {
	if strings.Contains(slugOrPath, string(os.PathSeparator)) || strings.HasSuffix(slugOrPath, ".md") {
		if _, err := os.Stat(slugOrPath); err != nil {
			return "", fmt.Errorf("plan file %q not found: %w", slugOrPath, err)
		}
		return slugOrPath, nil
	}
	return findPlanBySlug(slugOrPath)
}

var canonicalAmendFiles = map[string]struct{}{
	"Constitution.md": {},
	"Common.md":       {},
	"Code.md":         {},
	"Writing.md":      {},
}

func validateCanonicalAmendFile(name string) error {
	base := filepath.Base(strings.TrimSpace(name))
	if base != strings.TrimSpace(name) {
		return fmt.Errorf("target file %q is not a canonical governance file", name)
	}
	if _, ok := canonicalAmendFiles[base]; !ok {
		return fmt.Errorf("target file %q is not a supported canonical governance file", name)
	}
	return nil
}

type resolvedTarget struct {
	File       string
	Section    string
	RuleID     string
	RuleLine   string
	SourceText string
}

type documentRule struct {
	Major   string
	Section string
	RuleID  string
	RuleLine string
}

type documentHeading struct {
	Body string
}

var (
	headingLineRe     = regexp.MustCompile(`^(#+)\s+(.*)$`)
	ruleStartRe       = regexp.MustCompile(`^\s*(?:[-*]\s+)?\*\*([A-Z]\d+|\d+(?:\.\d+)+)\.`)
	filePrefixRe      = regexp.MustCompile(`^([A-Za-z0-9_.-]+\.md)\s+(.+)$`)
	explicitRuleIDRe  = regexp.MustCompile(`(?:^|[^A-Za-z0-9])([A-Z]\d+|\d+(?:\.\d+)+)(?:[^A-Za-z0-9]|$)`)
	compactOrdinalRe  = regexp.MustCompile(`^§?(\d+)\.(\d+)$`)
)

func buildTargetBlock(ruleRef string) string {
	resolved, ok := resolveRuleReference(ruleRef, aiRoot())
	if !ok {
		return strings.TrimSpace(ruleRef)
	}

	lines := []string{"File: " + resolved.File}
	if resolved.Section != "" {
		lines = append(lines, "Section: "+resolved.Section)
	}
	if resolved.RuleID != "" {
		lines = append(lines, "Rule: "+resolved.RuleID)
	}
	return strings.Join(lines, "\n")
}

func parsePlanTarget(block string) planTarget {
	target := planTarget{Raw: strings.TrimSpace(block)}
	for _, line := range strings.Split(block, "\n") {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "File: "):
			target.File = strings.TrimSpace(strings.TrimPrefix(line, "File: "))
		case strings.HasPrefix(line, "Section: "):
			target.Section = strings.TrimSpace(strings.TrimPrefix(line, "Section: "))
		case strings.HasPrefix(line, "Rule: "):
			target.Rule = strings.TrimSpace(strings.TrimPrefix(line, "Rule: "))
		}
	}
	return target
}

func resolvePlanTarget(content string, target planTarget) (resolvedTarget, error) {
	file := target.File
	if file == "" {
		file = "Constitution.md"
	}

	if target.Section != "" || target.Rule != "" {
		rt := resolvedTarget{
			File:       file,
			Section:    strings.TrimSpace(target.Section),
			RuleID:     strings.TrimSpace(target.Rule),
			SourceText: strings.TrimSpace(target.Raw),
		}
		if rt.Section == "" && rt.RuleID == "" {
			rt.SourceText = strings.TrimSpace(target.Raw)
		}
		if rt.RuleID != "" {
			rules := scanDocumentRules(content)
			for _, rule := range rules {
				if strings.EqualFold(rule.RuleID, rt.RuleID) {
					rt.Section = rule.Section
					rt.RuleLine = rule.RuleLine
					break
				}
			}
			if rt.RuleLine == "" {
				return resolvedTarget{}, fmt.Errorf("rule %q not found in %s", rt.RuleID, file)
			}
		}
		if rt.Section != "" {
			if !documentHasSection(content, rt.Section) {
				return resolvedTarget{}, fmt.Errorf("section %q not found in %s", rt.Section, file)
			}
		}
		if rt.Section == "" && rt.RuleLine == "" {
			return resolvedTarget{}, fmt.Errorf("target %q could not be resolved in %s", target.Raw, file)
		}
		return rt, nil
	}

	resolved, ok := resolveRuleReferenceContent(file, strings.TrimSpace(target.Raw), content)
	if !ok {
		return resolvedTarget{}, fmt.Errorf("target %q not found in %s", target.Raw, file)
	}
	return resolved, nil
}

func resolveRuleReference(ruleRef, root string) (resolvedTarget, bool) {
	file, token := splitRuleReference(ruleRef)
	if err := validateCanonicalAmendFile(file); err != nil {
		return resolvedTarget{}, false
	}
	data, err := os.ReadFile(filepath.Join(root, file))
	if err != nil {
		return resolvedTarget{}, false
	}
	return resolveRuleReferenceContent(file, token, string(data))
}

func resolveRuleReferenceContent(file, ruleRef, content string) (resolvedTarget, bool) {
	token := trimRuleReference(ruleRef)
	if token == "" {
		return resolvedTarget{}, false
	}

	headings := scanDocumentHeadings(content)
	for _, heading := range headings {
		if headingMatches(heading.Body, token) {
			return resolvedTarget{File: file, Section: heading.Body, SourceText: ruleRef}, true
		}
	}

	rules := scanDocumentRules(content)
	if ruleID, ok := extractExplicitRuleID(token); ok {
		for _, rule := range rules {
			if strings.EqualFold(rule.RuleID, ruleID) {
				return resolvedTarget{
					File:       file,
					Section:    rule.Section,
					RuleID:     rule.RuleID,
					RuleLine:   rule.RuleLine,
					SourceText: ruleRef,
				}, true
			}
		}
	}

	if m := compactOrdinalRe.FindStringSubmatch(strings.TrimPrefix(token, file+" ")); m != nil {
		major := m[1]
		ordinal, err := strconv.Atoi(m[2])
		if err == nil && ordinal > 0 {
			count := 0
			for _, rule := range rules {
				if rule.Major != major {
					continue
				}
				count++
				if count == ordinal {
					return resolvedTarget{
						File:       file,
						Section:    rule.Section,
						RuleID:     rule.RuleID,
						RuleLine:   rule.RuleLine,
						SourceText: ruleRef,
					}, true
				}
			}
		}
	}

	return resolvedTarget{}, false
}

func splitRuleReference(ruleRef string) (file, token string) {
	ref := strings.TrimSpace(ruleRef)
	if ref == "" {
		return "Constitution.md", ""
	}
	if slash := strings.Index(ref, "/"); slash > 0 && strings.HasSuffix(ref[:slash], ".md") {
		return strings.TrimSpace(ref[:slash]), strings.TrimSpace(ref[slash+1:])
	}
	if m := filePrefixRe.FindStringSubmatch(ref); m != nil {
		return m[1], strings.TrimSpace(m[2])
	}
	return "Constitution.md", ref
}

func trimRuleReference(ruleRef string) string {
	s := strings.TrimSpace(ruleRef)
	for _, sep := range []string{" — ", " – ", " - "} {
		if idx := strings.Index(s, sep); idx >= 0 {
			return strings.TrimSpace(s[:idx])
		}
	}
	return s
}

func scanDocumentHeadings(content string) []documentHeading {
	lines := strings.Split(content, "\n")
	headings := make([]documentHeading, 0, len(lines))
	for _, line := range lines {
		if m := headingLineRe.FindStringSubmatch(line); m != nil {
			headings = append(headings, documentHeading{Body: strings.TrimSpace(m[2])})
		}
	}
	return headings
}

func scanDocumentRules(content string) []documentRule {
	lines := strings.Split(content, "\n")
	rules := make([]documentRule, 0)
	currentSection := ""
	currentMajor := ""
	for _, line := range lines {
		if m := headingLineRe.FindStringSubmatch(line); m != nil {
			currentSection = strings.TrimSpace(m[2])
			if major := extractHeadingMajor(currentSection); major != "" {
				currentMajor = major
			}
			continue
		}
		if m := ruleStartRe.FindStringSubmatch(line); m != nil {
			rules = append(rules, documentRule{
				Major:    currentMajor,
				Section:  currentSection,
				RuleID:   strings.TrimSpace(m[1]),
				RuleLine: strings.TrimSpace(line),
			})
		}
	}
	return rules
}

func extractHeadingMajor(body string) string {
	body = strings.TrimSpace(body)
	if body == "" || !strings.HasPrefix(body, "§") {
		return ""
	}
	body = strings.TrimPrefix(body, "§")
	major := body
	if idx := strings.IndexAny(major, " ."); idx >= 0 {
		major = major[:idx]
	}
	if dot := strings.IndexByte(major, '.'); dot >= 0 {
		major = major[:dot]
	}
	return strings.TrimSpace(major)
}

func extractExplicitRuleID(token string) (string, bool) {
	m := explicitRuleIDRe.FindStringSubmatch(token)
	if m == nil {
		return "", false
	}
	return strings.TrimSpace(m[1]), true
}

func headingMatches(body, token string) bool {
	normalizedBody := normalizeTargetText(body)
	normalizedToken := normalizeTargetText(token)
	return normalizedBody == normalizedToken || strings.HasPrefix(normalizedBody, normalizedToken+" ")
}

func normalizeTargetText(s string) string {
	s = strings.TrimSpace(strings.TrimPrefix(s, "#"))
	return strings.Join(strings.Fields(strings.ToLower(s)), " ")
}

func documentHasSection(content, want string) bool {
	for _, heading := range scanDocumentHeadings(content) {
		if headingMatches(heading.Body, want) {
			return true
		}
	}
	return false
}

func findInsertOffset(content string, target resolvedTarget) (int, error) {
	lines := strings.Split(content, "\n")
	offsets := make([]int, len(lines)+1)
	acc := 0
	for i, line := range lines {
		offsets[i] = acc
		acc += len(line) + 1
	}
	offsets[len(lines)] = acc

	sectionLine := -1
	sectionDepth := 0
	for i, line := range lines {
		m := headingLineRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		if headingMatches(strings.TrimSpace(m[2]), target.Section) {
			sectionLine = i
			sectionDepth = len(m[1])
			break
		}
	}
	if sectionLine < 0 {
		return 0, fmt.Errorf("section %q not found in %s", target.Section, target.File)
	}

	sectionEndLine := len(lines)
	for i := sectionLine + 1; i < len(lines); i++ {
		m := headingLineRe.FindStringSubmatch(lines[i])
		if m != nil && len(m[1]) <= sectionDepth {
			sectionEndLine = i
			break
		}
	}

	if target.RuleID == "" {
		return offsets[sectionEndLine], nil
	}

	ruleLine := -1
	for i := sectionLine + 1; i < sectionEndLine; i++ {
		m := ruleStartRe.FindStringSubmatch(lines[i])
		if m != nil && strings.EqualFold(strings.TrimSpace(m[1]), target.RuleID) {
			ruleLine = i
			break
		}
	}
	if ruleLine < 0 {
		return 0, fmt.Errorf("rule %q not found in %s", target.RuleID, target.File)
	}

	ruleEndLine := sectionEndLine
	for i := ruleLine + 1; i < sectionEndLine; i++ {
		if headingLineRe.MatchString(lines[i]) || ruleStartRe.MatchString(lines[i]) {
			ruleEndLine = i
			break
		}
	}
	return offsets[ruleEndLine], nil
}

func spliceInsertedText(content string, insertAt int, proposedChange string) string {
	before := strings.TrimRight(content[:insertAt], "\n")
	after := strings.TrimLeft(content[insertAt:], "\n")
	insert := strings.TrimSpace(proposedChange)
	if after == "" {
		return before + "\n\n" + insert + "\n"
	}
	return before + "\n\n" + insert + "\n\n" + after
}
