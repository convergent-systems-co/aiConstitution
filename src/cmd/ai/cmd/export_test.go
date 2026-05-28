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

// PurgeOldHookEntriesForTest exposes purgeOldHookEntries to the external
// _test package without making the underlying function part of the public API.
func PurgeOldHookEntriesForTest(settings map[string]any) {
	purgeOldHookEntries(settings)
}

// InstallClaudeHooksForTest exposes installClaudeHooks to the external
// _test package without making the underlying function part of the public API.
func InstallClaudeHooksForTest(repoRoot, hooksDir string) (int, error) {
	return installClaudeHooks(repoRoot, hooksDir)
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
