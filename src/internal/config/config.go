// Package config loads and saves ~/.config/aiConstitution/settings.toml.
//
// Schema is defined inline here as Go structs (mirroring
// settings.toml.example at the repo root) and is validated against
// governance/schemas/settings.schema.json (TBD — morning work) at
// load and write time.
//
// Precedence per SPEC.md §13.3:
//  1. Environment variable (e.g., AICONST_REVIEW_CADENCE_DAYS).
//  2. settings.toml.
//  3. Compiled-in defaults.
package config

// Settings is the in-memory representation of settings.toml.
//
// TBD: the full struct mirrors settings.toml.example at the repo
// root; this skeleton declares the top-level sections so callers
// can take a *Settings without further setup. Filling in field
// definitions is morning work (see GOALS.md "Out of scope for v0.8").
type Settings struct {
	SchemaVersion string `toml:"schemaVersion"`

	Upstream        UpstreamSettings        `toml:"upstream"`
	Review          ReviewSettings          `toml:"review"`
	Update          UpdateSettings          `toml:"update"`
	Telemetry       TelemetrySettings       `toml:"telemetry"`
	SecretScanning  SecretScanningSettings  `toml:"secret_scanning"`
	CommandWrappers CommandWrappersSettings `toml:"command_wrappers"`
	Atoms           AtomsSettings           `toml:"atoms"`
	Plugins         PluginsSettings         `toml:"plugins"`
	Drafts          DraftsSettings          `toml:"drafts"`
	Focus           FocusSettings           `toml:"focus"`
	Wizard          WizardSettings          `toml:"wizard"`
	Sync            SyncSettings            `toml:"sync"`
	Paths           PathsSettings           `toml:"paths"`
}

// UpstreamSettings carries the [upstream] section of settings.toml —
// how AI-authored hooks, skills, and findings are filed back to the
// public aiConstitution repo.
type UpstreamSettings struct {
	ShareNewHooks      bool   `toml:"shareNewHooks"`
	ShareNewSkills     bool   `toml:"shareNewSkills"`
	ShareMajorFindings bool   `toml:"shareMajorFindings"`
	SkipReviewWindow   bool   `toml:"skipReviewWindow"`
	UpstreamRepo       string `toml:"upstreamRepo"`
}

// ReviewSettings carries the [review] section — memory-review cadence
// and what `ai review --check` reports on.
type ReviewSettings struct {
	CadenceDays            int  `toml:"cadenceDays"`
	IncludeMemory          bool `toml:"includeMemory"`
	IncludeAuditOverrides  bool `toml:"includeAuditOverrides"`
	IncludeAuditViolations bool `toml:"includeAuditViolations"`
}

// UpdateSettings carries the [update] section — migration-prompt
// behavior after a binary upgrade.
type UpdateSettings struct {
	AutoMigratePrompt  bool `toml:"autoMigratePrompt"`
	AutoMigrateApprove bool `toml:"autoMigrateApprove"`
}

// TelemetrySettings carries the [telemetry] section. Only the opt-in
// install ping is permitted; adding any other key here is a defect.
type TelemetrySettings struct {
	InstallPing bool `toml:"installPing"`
}

// SecretScanningSettings carries the [secret_scanning] section —
// pre-commit hook install scope and --no-verify bypass policy.
type SecretScanningSettings struct {
	InstallScope        string `toml:"installScope"`
	AllowNoVerifyBypass bool   `toml:"allowNoVerifyBypass"`
	CIScanner           string `toml:"ciScanner"`
}

// CommandWrappersSettings carries the [command_wrappers] section —
// the master switch and per-command toggles for the ~/.ai/bin/
// wrapper facade.
type CommandWrappersSettings struct {
	Enabled      bool              `toml:"enabled"`
	AllowDisable bool              `toml:"allowDisable"`
	Commands     map[string]string `toml:"commands"`
}

// AtomsSettings carries the [atoms] section — registry URL overrides
// and content-hash verification flag.
type AtomsSettings struct {
	PersonaRegistry   string `toml:"personaRegistry"`
	ProfileRegistry   string `toml:"profileRegistry"`
	SkillRegistry     string `toml:"skillRegistry"`
	BrandRegistry     string `toml:"brandRegistry"`
	VerifyContentHash bool   `toml:"verifyContentHash"`
}

// PluginsSettings carries the [plugins] section — enabled Claude
// plugins and the planner-persona-fallback toggle.
type PluginsSettings struct {
	Enabled                []string `toml:"enabled"`
	PlannerPersonaFallback bool     `toml:"plannerPersonaFallback"`
}

// DraftsSettings carries the [drafts] section — publish-nudge cadence
// for unpublished local atom drafts.
type DraftsSettings struct {
	PublishNudgeAfterDays int  `toml:"publishNudgeAfterDays"`
	SuppressNudge         bool `toml:"suppressNudge"`
}

// FocusSettings carries the [focus] section — the default mode loaded
// on session start.
type FocusSettings struct {
	DefaultMode          string `toml:"defaultMode"`
	PreferStableVersions bool   `toml:"preferStableVersions"`
}

// WizardSettings carries the [wizard] section — the questions.yaml
// version the user's answers were generated against.
type WizardSettings struct {
	LastSeenWizardVersion string `toml:"lastSeenWizardVersion"`
}

// SyncSettings carries the [sync] section — whether settings.toml
// itself is included in `ai sync push`.
type SyncSettings struct {
	IncludeSettingsFile bool `toml:"includeSettingsFile"`
}

// PathsSettings carries the [paths] section — overrides for the
// AIRoot and ConfigDir defaults resolved by package paths.
type PathsSettings struct {
	AIRoot    string `toml:"aiRoot"`
	ConfigDir string `toml:"configDir"`
}

// Defaults returns the canonical default settings (matches
// settings.toml.example at the repo root).
func Defaults() Settings {
	return Settings{
		SchemaVersion: "0.2",
		Upstream: UpstreamSettings{
			ShareNewHooks:      true,
			ShareNewSkills:     false,
			ShareMajorFindings: false,
			SkipReviewWindow:   false,
			UpstreamRepo:       "convergent-systems-co/ai",
		},
		Review: ReviewSettings{
			CadenceDays:            30,
			IncludeMemory:          true,
			IncludeAuditOverrides:  true,
			IncludeAuditViolations: true,
		},
		Update: UpdateSettings{AutoMigratePrompt: true, AutoMigrateApprove: false},
		Telemetry: TelemetrySettings{InstallPing: false},
		SecretScanning: SecretScanningSettings{
			InstallScope:        "all-repos",
			AllowNoVerifyBypass: false,
			CIScanner:           "none",
		},
		CommandWrappers: CommandWrappersSettings{
			Enabled:      true,
			AllowDisable: true,
			Commands: map[string]string{
				"git":       "enabled",
				"gh":        "enabled",
				"terraform": "disabled",
				"kubectl":   "disabled",
			},
		},
		Atoms: AtomsSettings{
			PersonaRegistry:   "https://persona-atoms.com",
			ProfileRegistry:   "https://profile-atoms.com",
			SkillRegistry:     "https://skill-atoms.com",
			BrandRegistry:     "https://brand-atoms.com",
			VerifyContentHash: true,
		},
		Plugins: PluginsSettings{Enabled: []string{}, PlannerPersonaFallback: true},
		Drafts:  DraftsSettings{PublishNudgeAfterDays: 30, SuppressNudge: false},
		Focus:   FocusSettings{DefaultMode: "none", PreferStableVersions: true},
		Wizard:  WizardSettings{LastSeenWizardVersion: "0.2"},
		Sync:    SyncSettings{IncludeSettingsFile: true},
		Paths:   PathsSettings{AIRoot: "", ConfigDir: ""},
	}
}

// Load reads ~/.config/aiConstitution/settings.toml and returns the
// parsed Settings, layered atop Defaults().
//
// TBD: actual TOML parsing + env-var overlay. Stub for v0.8.
func Load() (Settings, error) {
	return Defaults(), nil
}

// Save writes Settings to ~/.config/aiConstitution/settings.toml.
//
// TBD for v0.8.
func Save(_ Settings) error {
	return nil
}
