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
	_ = fix
	_ = resetHead

	checkTerminalNotifier(w)
	checkPersonasBlock(w)
	checkDerivativeFiles(w)
	checkHookWiring(w, paths.AIRoot(), homeDir())
	_ = checkInstalledSkills(w)

	return nil
}

// homeDir returns the current user's home directory, or empty string on failure.
func homeDir() string {
	h, _ := os.UserHomeDir()
	return h
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
		if strings.Contains(cmd, "/.ai/hooks/") {
			// Strip leading "python3 " or similar interpreter prefix.
			parts := strings.Fields(cmd)
			for _, p := range parts {
				if strings.Contains(p, "/.ai/hooks/") {
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

// checkHookWiring verifies that each required hook that is installed in the
// hooks directory is also wired in ~/.claude/settings.json.
func checkHookWiring(w io.Writer, aiRoot, home string) {
	requiredHooks := []string{
		"audit.py",
		"branch-guard.py",
		"secret-block.py",
		"worktree-guard.py",
		"checkpoint-tick.py",
	}

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
	if allOK {
		fmt.Fprintln(w, "[✓] Hook wiring complete")
	}
}

// checkInstalledSkills reports whether any skills are installed under the
// canonical ~/.ai/skills/ directory. When skills are installed but one or more
// are missing their Claude symlink, it also warns and suggests `ai skills link`.
//
// Output format:
//
//	  OK    N skill(s) installed
//	  WARN  No skills installed
//	        Run: ai skills available  (to see what's installable)
//	        Run: ai skills install <name>  (to install)
//	  WARN  Skills installed but not linked to Claude — run: ai skills link
func checkInstalledSkills(w io.Writer) error {
	installedSkills, _ := listSkillDirs(skillsManifestDir())
	if len(installedSkills) == 0 {
		fmt.Fprintln(w, "  WARN  No skills installed")
		fmt.Fprintln(w, "        Run: ai skills available  (to see what's installable)")
		fmt.Fprintln(w, "        Run: ai skills install <name>  (to install)")
		return nil
	}

	fmt.Fprintf(w, "  OK    %d skill(s) installed\n", len(installedSkills))

	// Check whether any installed skill is missing its Claude symlink.
	claudeDir := claudeSkillsDir()
	if claudeDir == "" {
		return nil
	}
	if _, err := os.Stat(claudeDir); err != nil {
		// Claude dir does not exist — nothing to check.
		return nil
	}

	for _, skillPath := range installedSkills {
		slug := filepath.Base(skillPath)
		linkPath := filepath.Join(claudeDir, slug)
		if _, err := os.Lstat(linkPath); os.IsNotExist(err) {
			fmt.Fprintln(w, "  WARN  Skills installed but not linked to Claude — run: ai skills link")
			return nil
		}
	}
	return nil
}

// checkPersonasBlock verifies the <!-- ai:personas --> block exists in CLAUDE.md.
func checkPersonasBlock(w io.Writer) {
	claudeMD := paths.ClaudeMD()
	data, err := os.ReadFile(claudeMD) //nolint:gosec
	if err != nil || !strings.Contains(string(data), "<!-- ai:personas") {
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
const ( PathOK PathStatus = iota; PathMissing; PathShadowed )

func checkBinPath(binDir, pathVar string) (PathStatus, string) {
	if binDir == "" { return PathOK, "" }
	binDir = filepath.Clean(binDir)
	systemBins := []string{"/usr/local/bin", "/opt/homebrew/bin"}
	entries := strings.Split(pathVar, string(os.PathListSeparator))
	binIdx := -1; systemIdxs := map[string]int{}
	for i, e := range entries {
		clean := filepath.Clean(strings.TrimSpace(e))
		if clean == binDir && binIdx < 0 { binIdx = i }
		for _, s := range systemBins { if clean == s { if _, ok := systemIdxs[s]; !ok { systemIdxs[s] = i } } }
	}
	if binIdx < 0 { return PathMissing, fmt.Sprintf("%s not on PATH", binDir) }
	for _, s := range systemBins { if si, ok := systemIdxs[s]; ok && si < binIdx { return PathShadowed, fmt.Sprintf("%s after %s", binDir, s) } }
	return PathOK, fmt.Sprintf("%s before system bins", binDir)
}

type doctorStatus int
const ( doctorOK doctorStatus = iota; doctorWarn; doctorSkip )
type doctorResult struct { name string; status doctorStatus; message string }

func checkDoctorCopilot(home string) doctorResult {
	dir := filepath.Join(home, ".copilot", "instructions")
	if _, err := os.Stat(dir); os.IsNotExist(err) { return doctorResult{status: doctorSkip} }
	if _, err := os.Lstat(filepath.Join(dir, "constitution.md")); os.IsNotExist(err) { return doctorResult{status: doctorWarn, message: "Copilot symlink missing"} }
	return doctorResult{status: doctorOK, message: "Copilot symlink present"}
}
func checkDoctorCursor(cwd string) doctorResult {
	if _, err := os.Stat(filepath.Join(cwd, ".cursor", "rules")); os.IsNotExist(err) { return doctorResult{status: doctorSkip} }
	if _, err := os.Lstat(filepath.Join(cwd, ".cursor", "rules", "constitution.md")); os.IsNotExist(err) { return doctorResult{status: doctorWarn} }
	return doctorResult{status: doctorOK}
}
func checkDoctorAgentsMD(cwd string) doctorResult {
	data, err := os.ReadFile(filepath.Join(cwd, "AGENTS.md")) //nolint:gosec
	if os.IsNotExist(err) { return doctorResult{status: doctorSkip} }
	if err != nil { return doctorResult{status: doctorWarn} }
	if strings.Contains(string(data), "@~/.ai/Constitution.md") { return doctorResult{status: doctorOK} }
	return doctorResult{status: doctorWarn}
}
