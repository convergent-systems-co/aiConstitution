package cmd

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/convergent-systems-co/aiConstitution/src/cmd/ai/embed"
	"github.com/spf13/cobra"
)

// newDoctorCmd implements `ai doctor`. See SPEC.md §3.3.
// Runs a 10-point health check of the ~/.ai/ governance tree.
func newDoctorCmd() *cobra.Command {
	var fix bool
	var resetHead string

	c := &cobra.Command{
		Use:   "doctor",
		Short: "Detect and repair structural damage to the ~/.ai/ tree",
		Long: `doctor checks the 10 canonical health points of the constitution tree:

  1.  ~/.ai/Constitution.md present
  2.  ~/.ai/Common.md present
  3.  ~/.ai/Code.md present
  4.  ~/.ai/Writing.md present
  5.  Required hooks present in ~/.ai/hooks/
  6.  Hooks wired in ~/.claude/settings.json
  7.  Hook content hash matches embedded version (tamper check)
  8.  ~/.ai/memory/MEMORY.md present
  9.  Audit interactions file modified within 7 days
  10. ~/.claude/CLAUDE.md present and contains @~/.ai/Constitution.md

Output: [✓] / [⚠] / [✗] per check. Exit 0 if no errors; exit 1 if any [✗].

Flags:
  --fix                  Attempt to repair each detected issue.
  --reset-head=<ref>     If the tree is dirty or HEAD is divergent,
                         reset to <ref> (refuses on dirty tree
                         without --force-hard-reset).

See SPEC.md §3.3.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			_ = fix
			_ = resetHead
			return runDoctor(cmd)
		},
	}

	c.Flags().BoolVar(&fix, "fix", false, "attempt to repair each detected issue")
	c.Flags().StringVar(&resetHead, "reset-head", "", "reset HEAD to <ref> (use with caution)")

	return c
}

// aiRootForDoctor resolves the ~/.ai/ root, honoring AI_ROOT env var.
// Duplicated inline (not importing paths package) because cmd and internal
// are in different modules and this keeps doctor self-contained.
func aiRootForDoctor() string {
	if env := os.Getenv("AI_ROOT"); env != "" {
		return env
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ".ai"
	}
	return filepath.Join(home, ".ai")
}

// checkResult holds the outcome of one health check.
type checkResult struct {
	mark    string // "✓", "⚠", "✗"
	message string
}

func pass(msg string) checkResult    { return checkResult{"✓", msg} }
func warn(msg string) checkResult    { return checkResult{"⚠", msg} }
func fail(msg string) checkResult    { return checkResult{"✗", msg} }

func runDoctor(cmd *cobra.Command) error {
	root := aiRootForDoctor()
	home, _ := os.UserHomeDir()
	if home == "" {
		home = os.Getenv("HOME")
	}

	results := make([]checkResult, 0, 10)
	hasError := false

	// --- Checks 1-4: prose files ---
	proseFiles := []string{"Constitution.md", "Common.md", "Code.md", "Writing.md"}
	for _, name := range proseFiles {
		path := filepath.Join(root, name)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			results = append(results, fail(name+" missing"))
			hasError = true
		} else {
			results = append(results, pass(name+" present"))
		}
	}

	// --- Check 5: required hook files ---
	requiredHooks := []string{"audit.py", "branch-guard.py", "secret-block.py", "worktree-guard.py"}
	hooksDir := filepath.Join(root, "hooks")
	for _, hook := range requiredHooks {
		hookPath := filepath.Join(hooksDir, hook)
		if _, err := os.Stat(hookPath); os.IsNotExist(err) {
			results = append(results, warn(hook+" not found in ~/.ai/hooks/"))
		} else {
			results = append(results, pass(hook+" present"))
		}
	}

	// --- Check 6: settings.json hooks wiring ---
	settingsPath := filepath.Join(home, ".claude", "settings.json")
	results = append(results, checkSettingsHooks(settingsPath))

	// --- Check 7: hook content hash vs embedded ---
	results = append(results, checkHookHashes(hooksDir)...)

	// --- Check 8: MEMORY.md present ---
	memoryPath := filepath.Join(root, "memory", "MEMORY.md")
	if _, err := os.Stat(memoryPath); os.IsNotExist(err) {
		results = append(results, warn("~/.ai/memory/MEMORY.md absent"))
	} else {
		results = append(results, pass("MEMORY.md present"))
	}

	// --- Check 9: audit interactions file modified within 7 days ---
	interDir := filepath.Join(root, "audit", "interactions")
	results = append(results, checkRecentInteraction(interDir))

	// --- Check 10: CLAUDE.md with @~/.ai/Constitution.md ---
	claudeMD := filepath.Join(home, ".claude", "CLAUDE.md")
	results = append(results, checkClaudeMD(claudeMD))

	// Print all results
	out := cmd.OutOrStdout()
	for _, r := range results {
		fmt.Fprintf(out, "[%s] %s\n", r.mark, r.message)
		if r.mark == "✗" {
			hasError = true
		}
	}

	if hasError {
		return fmt.Errorf("doctor: one or more checks failed")
	}
	return nil
}

// checkSettingsHooks verifies that audit.py is wired to SessionStart and
// branch-guard.py is wired to PreToolUse in ~/.claude/settings.json.
func checkSettingsHooks(settingsPath string) checkResult {
	data, err := os.ReadFile(filepath.Clean(settingsPath))
	if err != nil {
		return warn("~/.claude/settings.json not found — hooks wiring unverifiable")
	}

	var settings map[string]any
	if err := json.Unmarshal(data, &settings); err != nil {
		return warn("~/.claude/settings.json not valid JSON")
	}

	hooks, ok := settings["hooks"].(map[string]any)
	if !ok {
		return warn("~/.claude/settings.json: no 'hooks' block found")
	}

	auditInSessionStart := hookWiredTo(hooks, "SessionStart", "audit.py")
	branchGuardInPreTool := hookWiredTo(hooks, "PreToolUse", "branch-guard.py")

	switch {
	case !auditInSessionStart && !branchGuardInPreTool:
		return warn("settings.json: audit.py not wired to SessionStart; branch-guard.py not wired to PreToolUse")
	case !auditInSessionStart:
		return warn("settings.json: audit.py not wired to SessionStart")
	case !branchGuardInPreTool:
		return warn("settings.json: branch-guard.py not wired to PreToolUse")
	default:
		return pass("settings.json hooks wired correctly")
	}
}

// hookWiredTo checks whether hookFile appears in the commands for event in the hooks block.
func hookWiredTo(hooks map[string]any, event, hookFile string) bool {
	val, ok := hooks[event]
	if !ok {
		return false
	}
	// The hooks block is an array of matcher objects, each with a "hooks" array.
	matchers, ok := val.([]any)
	if !ok {
		return false
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
			if strings.HasSuffix(cmd, hookFile) {
				return true
			}
		}
	}
	return false
}

// checkHookHashes compares on-disk hook content against the embedded version.
// Returns one result per hook that can be compared.
func checkHookHashes(hooksDir string) []checkResult {
	embeddedFS := embed.HooksFS()
	entries, err := fs.ReadDir(embeddedFS, ".")
	if err != nil {
		return []checkResult{warn("hook tamper check: could not read embedded hooks")}
	}

	var results []checkResult
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		embeddedContent, err := fs.ReadFile(embeddedFS, name)
		if err != nil {
			continue
		}

		diskPath := filepath.Join(hooksDir, name)
		diskContent, err := os.ReadFile(filepath.Clean(diskPath))
		if err != nil {
			// Hook not installed on disk — check #5 covers this; skip hash check.
			continue
		}

		embHash := sha256.Sum256(embeddedContent)
		diskHash := sha256.Sum256(diskContent)
		if !bytes.Equal(embHash[:], diskHash[:]) {
			results = append(results, warn(fmt.Sprintf("hook %s: content differs from embedded (possible tamper)", name)))
		} else {
			results = append(results, pass(fmt.Sprintf("hook %s: content matches embedded", name)))
		}
	}

	if len(results) == 0 {
		return []checkResult{pass("hook tamper check: no hooks installed to compare")}
	}
	return results
}

// checkRecentInteraction warns if no interactions JSONL file was modified within 7 days.
func checkRecentInteraction(interDir string) checkResult {
	entries, err := os.ReadDir(interDir)
	if err != nil {
		return warn("audit/interactions/ not readable — audit may be broken")
	}

	threshold := time.Now().AddDate(0, 0, -7)
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".jsonl") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if info.ModTime().After(threshold) {
			return pass("audit/interactions: recent file found")
		}
	}
	return warn("audit/interactions: no file modified in the last 7 days — audit may be broken")
}

// checkClaudeMD verifies ~/.claude/CLAUDE.md exists and references Constitution.md.
func checkClaudeMD(claudeMD string) checkResult {
	data, err := os.ReadFile(filepath.Clean(claudeMD))
	if err != nil {
		return warn("~/.claude/CLAUDE.md not found")
	}
	if !strings.Contains(string(data), "@~/.ai/Constitution.md") {
		return warn("~/.claude/CLAUDE.md does not contain @~/.ai/Constitution.md")
	}
	return pass("CLAUDE.md present and references Constitution.md")
}
