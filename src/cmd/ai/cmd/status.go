package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/convergent-systems-co/aiConstitution/src/internal/paths"
	"github.com/spf13/cobra"
)

// newStatusCmd implements `ai status`. The existing surface from v0.1,
// extended in §3.2 and §3.3 to include the review-cadence nag and the
// last-seen-version check.
func newStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Print a short status report (sync state, review cadence, doctor warnings)",
		Long: `status prints a one-screen status report:
  - AI root path and git status
  - Constitution files: each with ✓/✗ and last-modified date
  - Hooks: installed count vs wired count in settings.json
  - Memory: entry count in MEMORY.md
  - Audit: last interaction date, violations in last 7 days
  - Doctor: critical check warnings inline

See SPEC.md §3.2 + §3.3.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runStatus(cmd)
		},
	}
}

// aiRootForStatus resolves the ~/.ai/ root, honoring AI_ROOT env var.
func aiRootForStatus() string {
	if env := os.Getenv("AI_ROOT"); env != "" {
		return env
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ".ai"
	}
	return filepath.Join(home, ".ai")
}

func runStatus(cmd *cobra.Command) error {
	out := cmd.OutOrStdout()
	root := aiRootForStatus()
	home, _ := os.UserHomeDir()
	if home == "" {
		home = os.Getenv("HOME")
	}

	// --- AI Root ---
	fmt.Fprintf(out, "AI Root: %s\n", root)

	// --- Git status of ~/.ai/ ---
	gitOut, err := exec.Command("git", "-C", root, "status", "--short").CombinedOutput()
	if err != nil {
		fmt.Fprintf(out, "Git:     not a git repo\n")
	} else {
		branch := gitBranch(root)
		status := strings.TrimSpace(string(gitOut))
		if status == "" {
			status = "clean"
		}
		fmt.Fprintf(out, "Git:     %s | %s\n", branch, status)
	}

	fmt.Fprintln(out)

	// --- Constitution files ---
	fmt.Fprintln(out, "Constitution files:")
	proseFiles := []string{"Constitution.md", "Common.md", "Code.md", "Writing.md"}
	for _, name := range proseFiles {
		path := filepath.Join(root, name)
		info, err := os.Stat(path)
		if os.IsNotExist(err) {
			fmt.Fprintf(out, "  ✗ %-20s missing\n", name)
		} else if err != nil {
			fmt.Fprintf(out, "  ✗ %-20s error: %v\n", name, err)
		} else {
			fmt.Fprintf(out, "  ✓ %-20s (modified: %s)\n", name, info.ModTime().UTC().Format("2006-01-02"))
		}
	}

	fmt.Fprintln(out)

	// --- Hooks ---
	fmt.Fprintln(out, "Hooks:")
	hooksDir := filepath.Join(root, "hooks")
	installedCount := countInstalledHooks(hooksDir)
	fmt.Fprintf(out, "  Installed: %d in ~/.ai/hooks/\n", installedCount)

	settingsPath := filepath.Join(home, ".claude", "settings.json")
	wiredCount, wiredNames := countWiredHooks(settingsPath)
	fmt.Fprintf(out, "  Wired:     %d in ~/.claude/settings.json\n", wiredCount)

	// Check for expected hooks not wired
	requiredWired := []string{"audit.py", "branch-guard.py"}
	for _, rw := range requiredWired {
		if !containsStr(wiredNames, rw) {
			fmt.Fprintf(out, "  ⚠ %s not wired in settings.json\n", rw)
		}
	}

	fmt.Fprintln(out)

	// --- Memory ---
	fmt.Fprintln(out, "Memory:")
	memPath := filepath.Join(root, "memory", "MEMORY.md")
	entryCount := countMemoryEntries(memPath)
	fmt.Fprintf(out, "  Entries: %d (in MEMORY.md)\n", entryCount)

	fmt.Fprintln(out)

	// --- Audit ---
	fmt.Fprintln(out, "Audit:")
	interDir := filepath.Join(root, "audit", "interactions")
	lastInteraction := latestInteractionDate(interDir)
	if lastInteraction.IsZero() {
		fmt.Fprintln(out, "  Last interaction: (none)")
	} else {
		fmt.Fprintf(out, "  Last interaction: %s\n", lastInteraction.UTC().Format("2006-01-02 15:04Z"))
	}

	violDir := filepath.Join(root, "audit", "violations")
	recentViolations := countRecentViolations(violDir, 7)
	fmt.Fprintf(out, "  Violations (last 7 days): %d\n", recentViolations)

	fmt.Fprintln(out)

	// --- Doctor critical checks inline ---
	fmt.Fprintln(out, "Doctor (critical checks):")
	doctorWarnings := criticalDoctorChecks(root, home)
	if len(doctorWarnings) == 0 {
		fmt.Fprintln(out, "  All critical checks pass")
	} else {
		for _, w := range doctorWarnings {
			fmt.Fprintf(out, "  ⚠ %s\n", w)
		}
	}

	// --- Active personas ---
	fmt.Fprintln(out)
	fmt.Fprintln(out, "Active personas (CLAUDE.md block):")
	claudeMD := paths.ClaudeMD()
	active := readActivePersonas(claudeMD)
	if len(active) == 0 {
		fmt.Fprintln(out, "  (no personas block — run `ai compress --wire` or `ai mode`)")
	} else {
		for _, p := range active {
			fmt.Fprintf(out, "  %s\n", p)
		}
	}

	return nil
}

// readActivePersonas parses the <!-- ai:personas --> block in CLAUDE.md.
func readActivePersonas(claudeMDPath string) []string {
	data, err := os.ReadFile(claudeMDPath) //nolint:gosec
	if err != nil {
		return nil
	}
	content := string(data)
	start := strings.Index(content, "<!-- ai:personas")
	end := strings.Index(content, "<!-- /ai:personas -->")
	if start == -1 || end == -1 || end <= start {
		return nil
	}
	block := content[start:end]
	var slugs []string
	for _, line := range strings.Split(block, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "@") {
			continue
		}
		base := filepath.Base(line[1:])
		if !strings.HasSuffix(base, ".md") {
			continue
		}
		slugs = append(slugs, strings.ToLower(strings.TrimSuffix(base, ".md")))
	}
	return slugs
}

// gitBranch returns the current branch name for the given repo root.
func gitBranch(repoRoot string) string {
	out, err := exec.Command("git", "-C", repoRoot, "rev-parse", "--abbrev-ref", "HEAD").Output()
	if err != nil {
		return "(unknown)"
	}
	return strings.TrimSpace(string(out))
}

// countInstalledHooks returns the count of files in the hooks directory.
func countInstalledHooks(hooksDir string) int {
	entries, err := os.ReadDir(hooksDir)
	if err != nil {
		return 0
	}
	count := 0
	for _, e := range entries {
		if !e.IsDir() {
			count++
		}
	}
	return count
}

// countWiredHooks parses settings.json and returns the count and names of
// distinct hook filenames referenced in any event's hooks array.
func countWiredHooks(settingsPath string) (int, []string) {
	data, err := os.ReadFile(filepath.Clean(settingsPath))
	if err != nil {
		return 0, nil
	}
	var settings map[string]any
	if err := json.Unmarshal(data, &settings); err != nil {
		return 0, nil
	}
	hooks, ok := settings["hooks"].(map[string]any)
	if !ok {
		return 0, nil
	}

	seen := make(map[string]bool)
	for _, val := range hooks {
		matchers, ok := val.([]any)
		if !ok {
			continue
		}
		for _, m := range matchers {
			matcher, ok := m.(map[string]any)
			if !ok {
				continue
			}
			hookList, ok := matcher["hooks"].([]any)
			if !ok {
				continue
			}
			for _, h := range hookList {
				hookMap, ok := h.(map[string]any)
				if !ok {
					continue
				}
				cmd, _ := hookMap["command"].(string)
				if cmd != "" {
					base := filepath.Base(cmd)
					seen[base] = true
				}
			}
		}
	}

	names := make([]string, 0, len(seen))
	for name := range seen {
		names = append(names, name)
	}
	return len(seen), names
}

// countMemoryEntries counts lines matching "- [" in MEMORY.md.
func countMemoryEntries(memPath string) int {
	f, err := os.Open(filepath.Clean(memPath))
	if err != nil {
		return 0
	}
	defer func() { _ = f.Close() }()

	count := 0
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		if strings.HasPrefix(strings.TrimSpace(scanner.Text()), "- [") {
			count++
		}
	}
	return count
}

// latestInteractionDate returns the modification time of the newest JSONL file
// in the interactions directory.
func latestInteractionDate(interDir string) time.Time {
	entries, err := os.ReadDir(interDir)
	if err != nil {
		return time.Time{}
	}
	var latest time.Time
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".jsonl") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if info.ModTime().After(latest) {
			latest = info.ModTime()
		}
	}
	return latest
}

// countRecentViolations counts violation files modified within the last n days.
func countRecentViolations(violDir string, days int) int {
	entries, err := os.ReadDir(violDir)
	if err != nil {
		return 0
	}
	threshold := time.Now().AddDate(0, 0, -days)
	count := 0
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if info.ModTime().After(threshold) {
			count++
		}
	}
	return count
}

// criticalDoctorChecks runs the 5 most critical checks and returns warning messages.
func criticalDoctorChecks(root, home string) []string {
	var warnings []string

	// Checks 1-4: prose files
	for _, name := range []string{"Constitution.md", "Common.md", "Code.md", "Writing.md"} {
		if _, err := os.Stat(filepath.Join(root, name)); os.IsNotExist(err) {
			warnings = append(warnings, name+" missing")
		}
	}

	// Check 5: hook files
	requiredHooks := []string{"audit.py", "branch-guard.py", "secret-block.py", "worktree-guard.py"}
	for _, hook := range requiredHooks {
		if _, err := os.Stat(filepath.Join(root, "hooks", hook)); os.IsNotExist(err) {
			warnings = append(warnings, "Hook "+hook+" not found in ~/.ai/hooks/")
		}
	}
	_ = home

	return warnings
}

// containsStr returns true if s is in slice.
func containsStr(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}
