// Package buildinfo carries build-time metadata stamped via -ldflags.
//
// Build with:
//
//	go build -ldflags "-X github.com/convergent-systems-co/aiConstitution/src/cmd/ai/internal/buildinfo.version=v0.8.0-dev" ...
package buildinfo

// version is the embedded version string. Override via -ldflags.
var version = "v0.8.0-dev (unstamped)"

// Version returns the embedded version.
func Version() string {
	return version
}
