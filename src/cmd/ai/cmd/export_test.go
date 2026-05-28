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
