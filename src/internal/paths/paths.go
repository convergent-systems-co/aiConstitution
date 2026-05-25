// Package paths resolves the canonical filesystem locations used by
// the `ai` system. Two roots:
//
//   - AIRoot()    — ~/.ai/  (canonical, synced via git, zero mutable
//     state). Holds the four-file constitution, audit records, memory,
//     hooks, profile references, plans/specs work products.
//
//   - ConfigDir() — ~/.config/aiConstitution/ on macOS + Linux (the
//     spec deliberately pins macOS to the XDG-style path for
//     cross-platform predictability), %APPDATA%\aiConstitution\ on
//     Windows. Holds ALL per-machine mutable state: settings.toml,
//     mode.json, state.json, *-drafts/, *-cache/.
//
// Per SPEC.md §15. Either root MAY be overridden via the
// [paths] aiRoot / configDir keys in settings.toml; this package
// honors those if a Config function is plugged in via SetOverrides.
package paths

import (
	"os"
	"path/filepath"
	"runtime"
)

// overrides captures user-set path overrides (typically from
// settings.toml [paths]). Empty string means "use default."
type overrides struct {
	aiRoot    string
	configDir string
}

var current overrides

// SetOverrides applies user-configured path overrides. Empty string
// for either field means "leave default in place."
func SetOverrides(aiRoot, configDir string) {
	current.aiRoot = aiRoot
	current.configDir = configDir
}

// AIRoot returns the canonical ~/.ai/ root. Override priority:
//
//  1. SetOverrides() value (from settings.toml).
//  2. $AI_ROOT environment variable.
//  3. $HOME/.ai/ (default).
func AIRoot() string {
	if current.aiRoot != "" {
		return current.aiRoot
	}
	if env := os.Getenv("AI_ROOT"); env != "" {
		return env
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ".ai"
	}
	return filepath.Join(home, ".ai")
}

// ConfigDir returns the canonical per-machine mutable-state directory.
// Override priority is the same as AIRoot:
//
//  1. SetOverrides() value.
//  2. $AICONST_CONFIG_DIR environment variable.
//  3. macOS / Linux: $XDG_CONFIG_HOME/aiConstitution or
//     $HOME/.config/aiConstitution.
//     Windows: %APPDATA%\aiConstitution.
func ConfigDir() string {
	if current.configDir != "" {
		return current.configDir
	}
	if env := os.Getenv("AICONST_CONFIG_DIR"); env != "" {
		return env
	}
	if runtime.GOOS == "windows" {
		if appdata := os.Getenv("APPDATA"); appdata != "" {
			return filepath.Join(appdata, "aiConstitution")
		}
	}
	// os.UserConfigDir handles the XDG / macOS / Windows resolution.
	cfg, err := os.UserConfigDir()
	if err != nil {
		// Last-resort fallback. Doctor will surface this as a finding.
		return ".config/aiConstitution"
	}
	return filepath.Join(cfg, "aiConstitution")
}

// AuditDir returns ~/.ai/audit/.
func AuditDir() string { return filepath.Join(AIRoot(), "audit") }

// MemoryDir returns ~/.ai/memory/.
func MemoryDir() string { return filepath.Join(AIRoot(), "memory") }

// HooksDir returns ~/.ai/hooks/.
func HooksDir() string { return filepath.Join(AIRoot(), "hooks") }

// GovernanceDir returns ~/.ai/governance/.
func GovernanceDir() string { return filepath.Join(AIRoot(), "governance") }

// PlansDir returns ~/.ai/plans/.
func PlansDir() string { return filepath.Join(AIRoot(), "plans") }

// SpecsDir returns ~/.ai/specs/.
func SpecsDir() string { return filepath.Join(AIRoot(), "specs") }

// SkillsManifestDir returns ~/.ai/skills/ (manifest files, not skill content).
func SkillsManifestDir() string { return filepath.Join(AIRoot(), "skills") }

// MetadataDir returns ~/.ai/metadata/.
func MetadataDir() string { return filepath.Join(AIRoot(), "metadata") }

// BinDir returns ~/.ai/bin/ (helper scripts; not the `ai` binary itself).
func BinDir() string { return filepath.Join(AIRoot(), "bin") }

// SettingsTOML returns the canonical settings.toml path under ConfigDir().
func SettingsTOML() string { return filepath.Join(ConfigDir(), "settings.toml") }

// ModeJSON returns the per-session mode.json path under ConfigDir().
func ModeJSON() string { return filepath.Join(ConfigDir(), "mode.json") }

// StateJSON returns the consolidated state.json path under ConfigDir().
func StateJSON() string { return filepath.Join(ConfigDir(), "state.json") }

// PersonaDraftsDir returns the pre-publication agentic-persona drafts dir.
func PersonaDraftsDir() string { return filepath.Join(ConfigDir(), "persona-drafts") }

// ReviewerDraftsDir returns the pre-publication reviewer-persona drafts dir.
func ReviewerDraftsDir() string { return filepath.Join(ConfigDir(), "reviewer-drafts") }

// ProfileDraftsDir returns the pre-publication profile drafts dir.
func ProfileDraftsDir() string { return filepath.Join(ConfigDir(), "profile-drafts") }

// SkillDraftsDir returns the pre-publication skill drafts dir.
func SkillDraftsDir() string { return filepath.Join(ConfigDir(), "skill-drafts") }

// PersonaCacheDir returns the cached persona-atoms tree (content-addressed).
func PersonaCacheDir() string { return filepath.Join(ConfigDir(), ".persona-cache") }

// ProfileCacheDir returns the cached profile-atoms tree (content-addressed).
func ProfileCacheDir() string { return filepath.Join(ConfigDir(), ".profile-cache") }

// SkillCacheDir returns the cached skill-atoms tree (content-addressed).
func SkillCacheDir() string { return filepath.Join(ConfigDir(), ".skill-cache") }

// BrandCacheDir returns the cached brand-atoms tree.
func BrandCacheDir() string { return filepath.Join(ConfigDir(), ".brand-cache") }

// CheckpointsDir returns the per-project HANDOFF.md checkpoint dir.
func CheckpointsDir() string { return filepath.Join(ConfigDir(), "checkpoints") }

// ClaudeMD returns the global Claude Code user instruction file.
// This is the file that holds the <!-- ai:personas --> block.
func ClaudeMD() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".claude/CLAUDE.md"
	}
	return filepath.Join(home, ".claude", "CLAUDE.md")
}

// ProjectYAML returns the project.yaml path under dir. Returns "" if dir is empty.
func ProjectYAML(dir string) string {
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, "project.yaml")
}
