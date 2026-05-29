package cmd

import "io"

// CheckBinPathForTest exposes checkBinPath to the external _test
// package without making the underlying function part of the public
// API.
func CheckBinPathForTest(binDir, pathVar string) (PathStatus, string) {
	return checkBinPath(binDir, pathVar)
}

// CheckPersonasBlockForTest exposes checkPersonasBlock to the external _test
// package without making the underlying function part of the public API.
func CheckPersonasBlockForTest(w io.Writer) {
	checkPersonasBlock(w)
}

// RunHooksPropose exposes runHooksPropose to the external _test package
// without making the underlying function part of the public API.
func RunHooksPropose(name, fromViolation, lang, aiRoot string, out io.Writer) error {
	return runHooksPropose(name, fromViolation, lang, aiRoot, out)
}

// ApplyIdentityRoutingForTest exposes applyIdentityRouting to the
// external _test package without making the underlying function part of
// the public API.
func ApplyIdentityRoutingForTest(out io.Writer, cloneURL, cloneDir, forceName string) error {
	return applyIdentityRouting(out, cloneURL, cloneDir, forceName)
}

// PurgeMalformedHookEntriesForTest exposes purgeMalformedHookEntries to the
// external _test package without making the underlying function part of the
// public API.
func PurgeMalformedHookEntriesForTest(settings map[string]any) {
	purgeMalformedHookEntries(settings)
}

// UpdateSettingsJSONForTest exposes updateSettingsJSON to the external _test
// package so tests can drive the canonical writer directly.
func UpdateSettingsJSONForTest(settingsPath, hooksDir string) error {
	return updateSettingsJSON(settingsPath, hooksDir)
}

// AiAtomsCatalogURLForTest exposes AiAtomsCatalogURL to the external _test
// package so tests can point the catalog fetch at a local httptest server.
var AiAtomsCatalogURLForTest = &AiAtomsCatalogURL

// InstallHookFromCatalogForTest exposes installHookFromCatalog to the external
// _test package without making the underlying function part of the public API.
func InstallHookFromCatalogForTest(slug, hooksDir string) error {
	return installHookFromCatalog(slug, hooksDir)
}

// ErrHookNotInCatalogForTest exposes ErrHookNotInCatalog to the external _test
// package for use with errors.Is assertions.
var ErrHookNotInCatalogForTest = ErrHookNotInCatalog

// SkillAtomsBaseURLForTest allows tests to redirect skill-atom GitHub
// Contents API calls to an httptest server.
var SkillAtomsBaseURLForTest = &SkillAtomsBaseURL

// PluginAtomsBaseURLForTest allows tests to redirect plugin-atoms.com
// resolution to an httptest server.
var PluginAtomsBaseURLForTest = &PluginAtomsBaseURL

// BrandHTTPGetForTest allows tests to replace the brand HTTP GET seam.
var BrandHTTPGetForTest = &brandHTTPGet

// PluginHTTPGetForTest allows tests to replace the plugin archive HTTP
// GET seam.
var PluginHTTPGetForTest = &pluginHTTPGet

// FindRealBinaryForTest exposes findRealBinary to external tests.
func FindRealBinaryForTest(tool, override string) (string, error) {
	return findRealBinary(tool, override)
}

// HookSlugForTest exposes hookSlug to external tests.
func HookSlugForTest(scriptPath string) string { return hookSlug(scriptPath) }

// HookAppliesForTest exposes hookApplies to external tests.
func HookAppliesForTest(h hookDef, subCmd string) bool { return hookApplies(h, subCmd) }

// ApplyStripArgsForTest exposes applyStripArgs to external tests.
func ApplyStripArgsForTest(args, strip []string) []string { return applyStripArgs(args, strip) }

// NewHookDefForTest constructs a hookDef for tests without exposing the type directly.
func NewHookDefForTest(script string, subcommands, stripArgs []string) hookDef {
	return hookDef{Script: script, Subcommands: subcommands, StripArgs: stripArgs}
}

// RunHookForWrapForTest exposes runHookForWrap to external tests.
// blocking=true mirrors hookDef.isBlocking()=true (security-gate hooks).
func RunHookForWrapForTest(slug string, toolArgs, extraEnv []string, blocking bool) int {
	return runHookForWrap(slug, toolArgs, extraEnv, blocking)
}

// IsBlockingForTest exposes hookDef.isBlocking() via the enforcement string.
func IsBlockingForTest(enforcement string) bool {
	return (hookDef{Enforcement: enforcement}).isBlocking()
}

// LoadCommandWrappersForTest exposes loadCommandWrappers to external tests.
func LoadCommandWrappersForTest() (*commandWrappersConfig, error) {
	return loadCommandWrappers()
}

// NormalizeFlagForTest is added in the flag-normalization commit (Task 4).
