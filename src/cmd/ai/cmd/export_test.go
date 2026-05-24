package cmd

// CheckBinPathForTest exposes checkBinPath to the external _test
// package without making the underlying function part of the public
// API.
func CheckBinPathForTest(binDir, pathVar string) (PathStatus, string) {
	return checkBinPath(binDir, pathVar)
}
