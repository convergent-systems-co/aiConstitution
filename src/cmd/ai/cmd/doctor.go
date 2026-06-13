package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/convergent-systems-co/aiConstitution/src/internal/constitution"
	"github.com/convergent-systems-co/aiConstitution/src/internal/paths"
	"github.com/spf13/cobra"
)

// newDoctorCmd implements `ai doctor`. See SPEC.md §3.3.
func newDoctorCmd() *cobra.Command {
	var fix bool
	var resetHead string

	c := &cobra.Command{
		Use:   "doctor",
		Short: "Detect and repair structural damage to the ~/.ai/ tree",
		Long: `doctor checks the predictable failure modes of the constitution
tree and either reports them or fixes them:

  1.  Broken symlinks under ~/.claude/, ~/.copilot/, .cursor/, etc.
  2.  Missing or misregistered hooks.
  3.  Dirty working tree on ~/.ai/.
  4.  Divergent HEAD vs origin.
  5.  Stale ai binary vs governance/last-seen-version.
  6.  Missing brand-cache; missing persona/profile/skill cache for
      pinned atoms.
  7.  Audit/interactions log writable.
  8.  Mutable state in ~/.config/aiConstitution/ exists and parses.
  9.  Settings file present and within validation ranges.
  10. last-seen-version marker matches the binary.
  11. terminal-notifier installed (macOS only).

Flags:
  --fix                  Attempt to repair each detected issue.
  --reset-head=<ref>     If the tree is dirty or HEAD is divergent,
                         reset to <ref> (refuses on dirty tree
                         without --force-hard-reset).

See SPEC.md §3.3.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runDoctor(os.Stdout, fix, resetHead)
		},
	}

	c.Flags().BoolVar(&fix, "fix", false, "attempt to repair each detected issue")
	c.Flags().StringVar(&resetHead, "reset-head", "", "reset HEAD to <ref> (use with caution)")

	return c
}

// runDoctor executes all doctor checks and writes the report to w.
// It returns nil unless an unexpected internal error occurs; individual
// check failures are surfaced as [⚠] lines in the output, not as errors.
//
// fix and resetHead are reserved for future implementation of --fix and
// --reset-head; they are accepted here so the function signature is stable
// and tests can exercise the check output without triggering mutations.
func runDoctor(w io.Writer, fix bool, resetHead string) error {
	_ = resetHead

	checkTerminalNotifier(w)
	checkPersonasBlock(w)
	checkDerivativeFiles(w)
	checkHookWiring(w, paths.AIRoot(), homeDir())
	checkGovernedHookCoverage(w, paths.AIRoot())
	checkWrapperHookDrift(w)
	checkWrapperPath(w, paths.BinDir(), os.Getenv("PATH"))
	checkToolIntegrations(w, homeDir(), mustGetwd())
	checkPythonAvailable(w, fix)
	checkCompactConstitution(w, fix, paths.AIRoot(), homeDir())
	_ = checkInstalledSkills(w)

	return nil
}

// homeDir returns the current user's home directory, or empty string on failure.
func homeDir() string {
	h, _ := os.UserHomeDir()
	return h
}

func mustGetwd() string {
	cwd, err := os.Getwd()
	if err != nil {
		return "."
	}
	return cwd
}

// fileExists returns true if path exists and is accessible.
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// readWiredHookNames parses settings.json and returns the set of hook basenames
// that appear in any hooks.<event> array with a command referencing /.ai/hooks/.
func readWiredHookNames(settingsPath string) map[string]bool {
	data, err := os.ReadFile(filepath.Clean(settingsPath))
	if err != nil {
		return map[string]bool{}
	}
	var settings map[string]any
	if err := json.Unmarshal(data, &settings); err != nil {
		return map[string]bool{}
	}
	hooks, ok := settings["hooks"].(map[string]any)
	if !ok {
		return map[string]bool{}
	}

	wired := make(map[string]bool)
	extractHookBase := func(cmd string) {
		// Portable format (v1.3+): "ai hooks run <slug>"
		if strings.HasPrefix(cmd, "ai hooks run ") {
			slug := strings.TrimPrefix(cmd, "ai hooks run ")
			slug = strings.TrimSpace(strings.Fields(slug)[0])
			// slug has no extension; try .py first (most hooks are Python)
			wired[slug+".py"] = true
			return
		}
		// Legacy absolute-path format: "python3 /abs/.ai/hooks/audit.py" or bare path.
		// Check both POSIX and Windows path separators.
		if strings.Contains(cmd, "/.ai/hooks/") || strings.Contains(cmd, "\\.ai\\hooks\\") {
			parts := strings.Fields(cmd)
			for _, p := range parts {
				if strings.Contains(p, "/.ai/hooks/") || strings.Contains(p, "\\.ai\\hooks\\") {
					wired[filepath.Base(p)] = true
					return
				}
			}
		}
	}

	for _, val := range hooks {
		entries, ok := val.([]any)
		if !ok {
			continue
		}
		for _, entry := range entries {
			m, ok := entry.(map[string]any)
			if !ok {
				continue
			}
			// Group format: {"hooks": [{"command": "..."}]}
			if hookList, ok := m["hooks"].([]any); ok {
				for _, h := range hookList {
					if hm, ok := h.(map[string]any); ok {
						if cmd, _ := hm["command"].(string); cmd != "" {
							extractHookBase(cmd)
						}
					}
				}
				continue
			}
			// Flat format: {"type": "...", "command": "python3 ..."}
			if cmd, _ := m["command"].(string); cmd != "" {
				extractHookBase(cmd)
			}
		}
	}
	return wired
}

// governedHook describes a hook that enforces one or more governance rules.
// The mapping lives here — not in the hook scripts — so hooks stay policy-agnostic.
type governedHook struct {
	slug     string   // bare name, no extension
	rules    []string
	desc     string // one-line plain-English description of what it checks
	wrapHint string // non-empty when full coverage requires the ai git shim
}

// governedHooks is the canonical rule→hook mapping for the ai constitution.
// Add entries here when a new hook covers a governance rule.
var governedHooks = []governedHook{
	{
		slug:  "branch-guard",
		rules: []string{"§3.2.10"},
		desc:  "blocks direct mutation of protected branches",
	},
	{
		slug:  "secret-block",
		rules: []string{"§3.5", "§4.1 (P4)", "§4.7.1"},
		desc:  "blocks secrets from appearing in tool outputs and artifacts",
	},
	{
		slug:  "worktree-guard",
		rules: []string{"§3.2.10", "§4.11.3"},
		desc:  "enforces canonical worktree placement",
	},
	{
		slug:  "dirty-tree-guard",
		rules: []string{"§4.11.1", "§4.11.2", "§13.2"},
		desc:  "detects uncommitted changes and unpushed commits at session end",
	},
	{
		slug:  "test-coverage-gate",
		rules: []string{"§4.3.1"},
		desc:  "detects source changes without corresponding test changes",
	},
	{
		slug:  "no-commented-code",
		rules: []string{"§4.1.5"},
		desc:  "detects commented-out executable code in written files",
	},
	{
		slug:  "audit-logger",
		rules: []string{"§1.3.4", "§1.5.5", "§5.2"},
		desc:  "logs every tool use to the audit trail",
	},
	{
		slug:     "push-guard",
		rules:    []string{"§3.2.10"},
		desc:     "blocks force-pushes to protected branches",
		wrapHint: "routes every git push through this guard regardless of caller — run: ai setup --git-shim",
	},
}

// checkGovernedHookCoverage reports which governance-mapped hooks are not yet
// installed in the hooks directory. Does not check wiring — that is
// checkHookWiring's job. This check tells you about gaps in coverage, not
// gaps in wiring.
func checkGovernedHookCoverage(w io.Writer, aiRoot string) {
	hooksDir := filepath.Join(aiRoot, "hooks")
	var missing []governedHook
	for _, gh := range governedHooks {
		if !fileExists(filepath.Join(hooksDir, gh.slug+".py")) {
			missing = append(missing, gh)
		}
	}
	if len(missing) == 0 {
		fmt.Fprintln(w, "[✓] Governance hook coverage complete")
	} else {
		for _, gh := range missing {
			fmt.Fprintf(w, "[⚠] %s not installed — covers %s (%s) — run: ai hooks install %s\n",
				gh.slug, strings.Join(gh.rules, ", "), gh.desc, gh.slug)
		}
	}
	// Report wrap hints for installed hooks that only achieve full coverage
	// when git is routed through the ai shim. Advisory only — never a failure.
	for _, gh := range governedHooks {
		if gh.wrapHint == "" {
			continue
		}
		if !fileExists(filepath.Join(hooksDir, gh.slug+".py")) {
			continue // already reported as missing above
		}
		shimPath := filepath.Join(homeDir(), ".ai", "bin", "git")
		if !fileExists(shimPath) {
			fmt.Fprintf(w, "[i] %s has partial coverage — %s\n", gh.slug, gh.wrapHint)
		}
	}
}

// checkHookWiring verifies that each required hook that is installed in the
// hooks directory is also wired in ~/.claude/settings.json.
func checkHookWiring(w io.Writer, aiRoot, home string) {
	requiredHooks := []string{
		"audit-logger.py",
		"branch-guard.py",
		"secret-block.py",
		"worktree-guard.py",
		"dirty-tree-guard.py",
	}
	legacyOptionalHooks := []string{}

	hooksDir := filepath.Join(aiRoot, "hooks")
	settingsPath := filepath.Join(home, ".claude", "settings.json")

	wiredSet := readWiredHookNames(settingsPath)

	allOK := true
	for _, hook := range requiredHooks {
		hookPath := filepath.Join(hooksDir, hook)
		if !fileExists(hookPath) {
			continue // not installed — separate warning handles this
		}
		if !wiredSet[hook] {
			fmt.Fprintf(w, "[⚠] %s installed but not wired — run: ai hooks install --claude\n", hook)
			allOK = false
		}
	}
	for _, hook := range legacyOptionalHooks {
		hookPath := filepath.Join(hooksDir, hook)
		if !fileExists(hookPath) {
			continue
		}
		if !wiredSet[hook] {
			fmt.Fprintf(w, "[⚠] %s installed but not wired — legacy HANDOFF.md hook is disabled by default\n", hook)
			allOK = false
		}
	}
	if allOK {
		fmt.Fprintln(w, "[✓] Hook wiring complete")
	}
}

// checkWrapperHookDrift verifies that every blocking pre-hook referenced in
// command-wrappers.toml is installed on disk. A missing hook means runWrap
// will fail closed (ENFORCEMENT DEGRADED) on the next invocation; doctor
// surfaces this proactively so the user can fix it before it blocks work.
func checkWrapperHookDrift(w io.Writer) {
	cfg, err := loadCommandWrappers()
	if err != nil {
		fmt.Fprintf(w, "[⚠] command-wrappers.toml unreadable: %v — run: ai hooks install --all\n", err)
		return
	}

	hooksDir := paths.HooksDir()
	var missing []string

	for _, entry := range cfg.Command {
		if !entry.isEnabled() {
			continue
		}
		for _, h := range entry.PreHooks {
			if !h.isBlocking() {
				continue
			}
			slug := hookSlug(h.Script)
			hookPath := filepath.Join(hooksDir, slug+".py")
			if _, err := os.Stat(hookPath); os.IsNotExist(err) {
				missing = append(missing, slug)
			}
		}
	}

	if len(missing) == 0 {
		fmt.Fprintln(w, "[✓] All blocking wrapper hooks installed")
		return
	}
	for _, slug := range missing {
		fmt.Fprintf(w, "[⚠] Blocking hook %q not installed — run: ai hooks install --all\n", slug)
	}
}

// checkPythonAvailable verifies that Python 3 is discoverable for hook
// execution. On Windows, the Microsoft Store App Execution Aliases can shadow
// a real Python installation with a stub that shows a Store prompt instead of
// running Python — this breaks every hook. When fix=true and the stub is
// detected, doctor removes it from %LOCALAPPDATA%\Microsoft\WindowsApps\.
func checkPythonAvailable(w io.Writer, fix bool) {
	pyArgs := discoverPythonArgs() // defined in hooks.go
	if pyArgs == nil {
		fmt.Fprintln(w, "[⚠] Python 3 not found — hooks cannot run")
		fmt.Fprintln(w, "     Install Python 3 and ensure it is on PATH")
		fmt.Fprintln(w, "     Windows: winget install Python.Python.3.12")
		return
	}

	// Verify the found Python actually executes (not a Store stub).
	// Store stubs are zero-byte or fail when invoked with --version.
	out, err := exec.Command(pyArgs[0], "--version").CombinedOutput() //nolint:gosec
	if err != nil || len(out) == 0 {
		if runtime.GOOS == "windows" && fix {
			removed := fixWindowsPythonStubs(w)
			if removed > 0 {
				fmt.Fprintf(w, "[✓] Removed %d Python App Execution Alias stub(s) — re-run: ai doctor\n", removed)
			} else {
				fmt.Fprintln(w, "[⚠] Python stub detected but could not remove — disable manually:")
				fmt.Fprintln(w, "     Settings → Apps → Advanced app settings → App execution aliases → python.exe OFF")
			}
		} else {
			fmt.Fprintf(w, "[⚠] Python found at %s but does not execute\n", pyArgs[0])
			if runtime.GOOS == "windows" {
				fmt.Fprintln(w, "     Windows App Execution Aliases may be interfering")
				fmt.Fprintln(w, "     Run: ai doctor --fix  (removes Store Python stubs)")
				fmt.Fprintln(w, "     Or:  Settings → Apps → Advanced app settings → App execution aliases → python.exe OFF")
			}
		}
		return
	}
	fmt.Fprintf(w, "[✓] Python 3 available: %s\n", strings.TrimSpace(string(out)))
}

// fixWindowsPythonStubs removes zero-byte Windows Store App Execution Alias
// stubs for python.exe and python3.exe from %LOCALAPPDATA%\Microsoft\WindowsApps\.
// These stubs shadow a real Python installation and produce a Store prompt
// instead of running Python, which breaks all hooks.
func fixWindowsPythonStubs(w io.Writer) int {
	appDir := os.Getenv("LOCALAPPDATA")
	if appDir == "" {
		return 0
	}
	stubDir := filepath.Join(appDir, "Microsoft", "WindowsApps")
	stubs := []string{"python.exe", "python3.exe", "python3.12.exe", "python3.11.exe"}
	removed := 0
	for _, name := range stubs {
		p := filepath.Join(stubDir, name)
		info, err := os.Stat(p)
		if err != nil {
			continue
		}
		// Only remove if it is a stub (zero bytes or very small — real Python is megabytes)
		if info.Size() > 4096 {
			continue // looks like a real binary, leave it
		}
		if removeErr := os.Remove(p); removeErr == nil {
			fmt.Fprintf(w, "     Removed stub: %s\n", p)
			removed++
		}
	}
	return removed
}

// checkCompactConstitution verifies that:
//  1. Constitution.compact.md exists in AI_ROOT (if Constitution.md exists).
//  2. ~/.claude/CLAUDE.md references the compact form, not the full form.
//
// With fix=true: generates the compact form if missing; updates CLAUDE.md to
// use the compact include.
func checkCompactConstitution(w io.Writer, fix bool, aiRoot, home string) {
	fullPath := filepath.Join(aiRoot, "Constitution.md")
	compactPath := filepath.Join(aiRoot, "Constitution.compact.md")
	claudeMD := filepath.Join(home, ".claude", "CLAUDE.md")

	// Only check if the full constitution exists.
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		return
	}

	// Check 1: compact form exists.
	if _, err := os.Stat(compactPath); os.IsNotExist(err) {
		if fix {
			// Generate compact form.
			values, _ := extractPersonalValues(aiRoot)
			full, readErr := os.ReadFile(fullPath) //nolint:gosec
			compact := renderCompactConstitution(values, string(full), extractConstitutionVersion(string(full)))
			if readErr == nil {
				if writeErr := os.WriteFile(compactPath, []byte(compact), 0o600); writeErr == nil {
					fmt.Fprintf(w, "[✓] Generated Constitution.compact.md (%d bytes)\n", len(compact))
					return
				}
			}
			fmt.Fprintln(w, "[⚠] Could not generate Constitution.compact.md — run: ai compress")
		} else {
			fmt.Fprintln(w, "[⚠] Constitution.compact.md missing — run: ai compress  (or: ai doctor --fix)")
		}
		return
	}

	// Check 2: CLAUDE.md uses compact form.
	claudeData, err := os.ReadFile(claudeMD) //nolint:gosec
	if err != nil {
		return // CLAUDE.md absent — not our concern here
	}
	content := string(claudeData)
	const oldInclude = "@~/.ai/Constitution.md"
	const newInclude = "@~/.ai/Constitution.compact.md"

	if strings.Contains(content, oldInclude) && !strings.Contains(content, newInclude) {
		if fix {
			updated := strings.ReplaceAll(content, oldInclude, newInclude)
			if writeErr := os.WriteFile(claudeMD, []byte(updated), 0o640); writeErr == nil { //nolint:gosec
				fmt.Fprintln(w, "[✓] CLAUDE.md updated to use Constitution.compact.md")
			} else {
				fmt.Fprintf(w, "[⚠] Could not update CLAUDE.md: %v\n", writeErr)
			}
		} else {
			fmt.Fprintln(w, "[⚠] CLAUDE.md still uses full Constitution.md — run: ai doctor --fix")
		}
		return
	}

	fmt.Fprintln(w, "[✓] Constitution.compact.md present and wired")
}

// checkInstalledSkills reports whether any skills are installed under the
// canonical ~/.ai/skills/ directory. When skills are installed but one or more
// are missing a known consumer symlink, it also warns and suggests
// `ai skills link`.
//
// Output format:
//
//	OK    N skill(s) installed
//	WARN  No skills installed
//	      Run: ai skills available  (to see what's installable)
//	      Run: ai skills install <name>  (to install)
//	WARN  Skills installed but not linked to <consumer> — run: ai skills link
func checkInstalledSkills(w io.Writer) error {
	installedSkills, _ := listSkillDirs(skillsManifestDir())
	if len(installedSkills) == 0 {
		fmt.Fprintln(w, "  WARN  No skills installed")
		fmt.Fprintln(w, "        Run: ai skills available  (to see what's installable)")
		fmt.Fprintln(w, "        Run: ai skills install <name>  (to install)")
		return nil
	}

	fmt.Fprintf(w, "  OK    %d skill(s) installed\n", len(installedSkills))

	consumers := []struct {
		name string
		dir  string
	}{
		{name: "Claude", dir: claudeSkillsDir()},
		{name: "Copilot", dir: copilotSkillsDir()},
		{name: "Codex", dir: codexSkillsDir()},
	}
	for _, consumer := range consumers {
		if consumer.dir == "" {
			continue
		}
		if _, err := os.Stat(consumer.dir); err != nil {
			continue
		}
		for _, skillPath := range installedSkills {
			slug := filepath.Base(skillPath)
			linkPath := filepath.Join(consumer.dir, slug)
			if _, err := os.Lstat(linkPath); os.IsNotExist(err) {
				fmt.Fprintf(w, "  WARN  Skills installed but not linked to %s — run: ai skills link\n", consumer.name)
				return nil
			}
		}
	}
	return nil
}

// checkPersonasBlock verifies the <!-- ai:personas --> block exists in CLAUDE.md,
// but only when Constitution.md actually contains persona sections to wire.
// If Constitution.md has no persona sections, the block is not required and no
// warning is emitted — otherwise users with minimal constitutions get a false positive.
func checkPersonasBlock(w io.Writer) {
	// Only warn if Constitution.md actually has persona sections to wire.
	root := paths.AIRoot()
	constPath := filepath.Join(root, "Constitution.md")
	constData, err := os.ReadFile(constPath) //nolint:gosec
	if err != nil {
		return // can't read constitution — skip this check silently
	}
	sections := constitution.ParseSections(string(constData))
	if len(sections) == 0 {
		// No persona sections → nothing to wire → not a problem.
		return
	}

	// Persona sections exist — check that CLAUDE.md has the wiring block.
	claudeMD := paths.ClaudeMD()
	claudeData, err := os.ReadFile(claudeMD) //nolint:gosec
	if err != nil || !strings.Contains(string(claudeData), "<!-- ai:personas") {
		fmt.Fprintln(w, "[⚠] CLAUDE.md personas block missing — run `ai compress --wire` or `ai mode` to create it")
		return
	}
	fmt.Fprintln(w, "[✓] CLAUDE.md personas block")
}

// checkDerivativeFiles verifies that YAML derivatives exist for all
// ## N. <Persona> Rules sections in Constitution.md.
func checkDerivativeFiles(w io.Writer) {
	root := paths.AIRoot()
	constPath := filepath.Join(root, "Constitution.md")
	data, err := os.ReadFile(constPath) //nolint:gosec
	if err != nil {
		return
	}
	for _, s := range constitution.ParseSections(string(data)) {
		yamlPath := filepath.Join(root, s.FileName)
		if _, statErr := os.Stat(yamlPath); statErr != nil {
			fmt.Fprintf(w, "[⚠] %s missing — run `ai compress --personas`\n", s.FileName)
		} else {
			fmt.Fprintf(w, "[✓] %s present\n", s.FileName)
		}
	}
}

// checkTerminalNotifier verifies that terminal-notifier is on PATH.
// The check runs only on macOS (darwin); it is silently skipped on other
// platforms so doctor remains cross-platform without platform-specific output.
//
// Output format:
//
//	[✓] terminal-notifier: found at <path>
//	[⚠] terminal-notifier: not found — run: brew install terminal-notifier
func checkTerminalNotifier(w io.Writer) {
	if runtime.GOOS != "darwin" {
		return
	}

	path, err := exec.LookPath("terminal-notifier")
	if err == nil {
		fmt.Fprintf(w, "[✓] terminal-notifier: found at %s\n", path)
		return
	}

	// Not installed — ask the user if they want to install it now.
	fmt.Fprint(w, "terminal-notifier not found. Install it now? [y/N] ")
	scanner := bufio.NewScanner(os.Stdin)
	if scanner.Scan() && (scanner.Text() == "y" || scanner.Text() == "Y") {
		fmt.Fprintln(w, "Running: brew install terminal-notifier")
		cmd := exec.Command("brew", "install", "terminal-notifier") //nolint:gosec
		cmd.Stdout = w
		cmd.Stderr = w
		if err := cmd.Run(); err != nil {
			fmt.Fprintf(w, "[✗] terminal-notifier: install failed: %v\n", err)
			return
		}
		fmt.Fprintln(w, "[✓] terminal-notifier: installed")
	} else {
		fmt.Fprintln(w, "[⚠] terminal-notifier: skipped — install later with: brew install terminal-notifier")
	}
}

// PathStatus and companion types — needed by export_test.go and integrate_test.go
type PathStatus int

const (
	PathOK PathStatus = iota
	PathMissing
	PathShadowed
)

func checkBinPath(binDir, pathVar string) (PathStatus, string) {
	if binDir == "" {
		return PathOK, ""
	}
	binDir = filepath.Clean(binDir)
	var systemBins []string
	if runtime.GOOS == "windows" {
		systemBins = []string{} // Windows PATH ordering check not applicable
	} else {
		systemBins = []string{"/usr/local/bin", "/opt/homebrew/bin"}
	}
	entries := strings.Split(pathVar, string(os.PathListSeparator))
	binIdx := -1
	systemIdxs := map[string]int{}
	for i, e := range entries {
		clean := filepath.Clean(strings.TrimSpace(e))
		if clean == binDir && binIdx < 0 {
			binIdx = i
		}
		for _, s := range systemBins {
			if clean == s {
				if _, ok := systemIdxs[s]; !ok {
					systemIdxs[s] = i
				}
			}
		}
	}
	if binIdx < 0 {
		return PathMissing, fmt.Sprintf("%s not on PATH", binDir)
	}
	for _, s := range systemBins {
		if si, ok := systemIdxs[s]; ok && si < binIdx {
			return PathShadowed, fmt.Sprintf("%s after %s", binDir, s)
		}
	}
	return PathOK, fmt.Sprintf("%s before system bins", binDir)
}

func checkWrapperPath(w io.Writer, binDir, pathVar string) {
	status, message := checkBinPath(binDir, pathVar)
	switch status {
	case PathOK:
		if message != "" {
			fmt.Fprintf(w, "[✓] command wrappers on PATH: %s\n", message)
		}
	case PathMissing:
		fmt.Fprintf(w, "[⚠] command wrappers not on PATH: %s — run: ai hooks install command-wrappers and add ~/.ai/bin early to PATH\n", message)
	case PathShadowed:
		fmt.Fprintf(w, "[⚠] command wrappers shadowed: %s — move ~/.ai/bin before system Git/GitHub CLI paths\n", message)
	}
}

type doctorStatus int

const (
	doctorOK doctorStatus = iota
	doctorWarn
	doctorSkip
)

type doctorResult struct {
	name    string
	status  doctorStatus
	message string
}

func checkDoctorCopilot(home string) doctorResult {
	dir := filepath.Join(home, ".copilot", "instructions")
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return doctorResult{status: doctorSkip}
	}
	if _, err := os.Lstat(filepath.Join(dir, "constitution.md")); os.IsNotExist(err) {
		return doctorResult{status: doctorWarn, message: "Copilot symlink missing"}
	}
	return doctorResult{status: doctorOK, message: "Copilot symlink present"}
}
func checkDoctorCursor(cwd string) doctorResult {
	if _, err := os.Stat(filepath.Join(cwd, ".cursor", "rules")); os.IsNotExist(err) {
		return doctorResult{status: doctorSkip}
	}
	if _, err := os.Lstat(filepath.Join(cwd, ".cursor", "rules", "constitution.md")); os.IsNotExist(err) {
		return doctorResult{status: doctorWarn}
	}
	return doctorResult{status: doctorOK}
}
func checkDoctorAgentsMD(cwd string) doctorResult {
	data, err := os.ReadFile(filepath.Join(cwd, "AGENTS.md")) //nolint:gosec
	if os.IsNotExist(err) {
		return doctorResult{status: doctorSkip}
	}
	if err != nil {
		return doctorResult{status: doctorWarn}
	}
	// Accept both compact form (current) and full form (legacy installs).
	content := string(data)
	if strings.Contains(content, "@~/.ai/Constitution.compact.md") || strings.Contains(content, "@~/.ai/Constitution.md") {
		return doctorResult{status: doctorOK}
	}
	return doctorResult{status: doctorWarn}
}

func checkToolIntegrations(w io.Writer, home, cwd string) {
	results := []doctorResult{
		checkDoctorCopilot(home),
		checkDoctorCursor(cwd),
		checkDoctorAgentsMD(cwd),
	}
	results[0].name = "Copilot"
	results[1].name = "Cursor"
	results[2].name = "Codex"
	for _, result := range results {
		switch result.status {
		case doctorOK:
			if result.message == "" {
				result.message = result.name + " integration present"
			}
			fmt.Fprintf(w, "[✓] %s\n", result.message)
		case doctorWarn:
			if result.message == "" {
				result.message = result.name + " integration missing or stale"
			}
			fmt.Fprintf(w, "[⚠] %s — run: %s\n", result.message, integrationRepairCommand(result.name))
		case doctorSkip:
			// Tool surface absent; do not warn on machines that have not opted in.
		}
	}
}

func integrationRepairCommand(name string) string {
	switch name {
	case "Copilot":
		return "ai hooks install --copilot"
	case "Cursor":
		return "ai init-integrate --cursor"
	case "Codex":
		return "ai init-integrate --codex"
	default:
		return "ai doctor --fix"
	}
}
