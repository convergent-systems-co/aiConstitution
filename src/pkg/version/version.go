// Package version exposes the binary's build-time version to anything
// outside the binary module (the Go workspace's `pkg/`).
//
// The actual version string is stamped at build time into
// src/cmd/ai/internal/buildinfo via -ldflags. This package is a
// pure constant table — it carries metadata that is part of the
// spec contract (the CURRENT_SPEC_VERSION the binary speaks) and
// is independent of the binary's own version.
package version

// SpecVersion is the SPEC.md draft the current code base targets.
// Bump on every spec revision that affects what this binary
// produces (file layouts, JSONL schema, settings.toml schema).
const SpecVersion = "0.8"

// QuestionsVersion is the questions.yaml taxonomy version the
// binary speaks. Bumped together with the file.
const QuestionsVersion = "0.8"

// SettingsSchemaVersion is the settings.toml schemaVersion that the
// config loader expects on read.
const SettingsSchemaVersion = "0.2"

// CanonicalRegistries are the four atom registry roots, as a tabular
// constant. settings.toml [atoms] may override any of them; this is
// the compiled-in default.
type Registry struct {
	Name string
	URL  string
}

var CanonicalRegistries = []Registry{
	{"brand-atoms", "https://brand-atoms.com"},
	{"persona-atoms", "https://persona-atoms.com"},
	{"profile-atoms", "https://profile-atoms.com"},
	{"skill-atoms", "https://skill-atoms.com"},
}
