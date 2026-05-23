// Package buildinfo carries build-time metadata stamped via -ldflags.
//
// Build with:
//
//	go build -ldflags "-X .../buildinfo.version=v0.8.0 -X .../buildinfo.commit=$(git rev-parse HEAD) -X .../buildinfo.date=$(date -u +%FT%TZ)" ...
//
// .goreleaser.yaml stamps all three at release-build time. Local
// `make build` leaves the defaults below in place.
package buildinfo

import "fmt"

// Override these via -ldflags. Defaults indicate an un-stamped local
// build; release builds stamp them with the tag, commit SHA, and
// build date.
var (
	version = "v0.8.0-dev"
	commit  = "unknown"
	date    = "unknown"
)

// Version returns the embedded version string. Format depends on
// whether ldflags stamping occurred:
//   - Stamped:   "v0.8.0  (commit abc1234, built 2026-05-23T08:00:00Z)"
//   - Unstamped: "v0.8.0-dev  (commit unknown, built unknown)"
func Version() string {
	return fmt.Sprintf("%s  (commit %s, built %s)", version, shortCommit(), date)
}

// Raw returns just the bare version string, suitable for
// machine-readable invocations like `ai version --short`.
func Raw() string { return version }

// Commit returns the build-time commit SHA, or "unknown" if unstamped.
func Commit() string { return commit }

// Date returns the build-time RFC-3339 date, or "unknown" if unstamped.
func Date() string { return date }

// shortCommit returns the first 7 chars of the embedded commit SHA,
// or "unknown" if the full string is shorter than that.
func shortCommit() string {
	if len(commit) < 7 {
		return commit
	}
	return commit[:7]
}
