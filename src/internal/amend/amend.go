// Package amend implements the amendment lifecycle for the four
// canonical constitution files (~/.ai/{Constitution,Common,Code,Writing}.md).
//
// Two phases:
//
//  1. Draft (see WriteDraft in draft.go): the user supplies a
//     file/section reference like "Common.md/U17". A draft amendment
//     file is written to ~/.ai/governance/plans/<UTC>-<slug>.md and
//     opened in $EDITOR. The draft is a hand-editable YAML-frontmatter
//     + Markdown body describing the proposed change.
//
//  2. Apply (see Apply in apply.go): the user runs
//     `ai amend apply <draft-path>`. The proposed-change text is
//     appended to the matching section in the target ~/.ai/<file>.md,
//     the file's `**Version:** X.Y` line is bumped (minor), and a new
//     Changelog bullet is appended. An audit record is written to
//     ~/.ai/audit/overrides/<UTC>-<slug>.md.
//
// See SPEC.md §3.5 + §6 (Memory → Amendment Lifecycle).
package amend

import (
	"strings"
	"time"
)

// Draft is the in-memory shape of an amendment draft. Persisted as
// YAML frontmatter + Markdown body to ~/.ai/governance/plans/<UTC>-<slug>.md.
type Draft struct {
	// File is the basename of the target constitution file
	// (e.g. "Common.md"). It MUST be one of the four canonical files
	// or "Constitution.local.md".
	File string
	// Section is the section anchor inside File (e.g. "U17", "§11.2",
	// "3.5"). Format is permissive; matched against section headers
	// in File on apply.
	Section string
	// ProposedChange is the Markdown body the user wants appended
	// to the section. Free-form. May be empty when the draft is
	// newly minted; the user fills it in via $EDITOR.
	ProposedChange string
	// AuditRef optionally references an audit/violations/*.md or
	// audit/overrides/*.md file that motivated this amendment.
	AuditRef string
	// Rationale is the one-line "why" used in the Changelog entry on
	// apply. Optional; defaults to ProposedChange's first line.
	Rationale string
	// Slug is the URL-safe identifier used in the draft filename.
	// Auto-derived from File + Section if empty.
	Slug string
	// Created is the UTC timestamp the draft was minted. Used in the
	// draft filename.
	Created time.Time
}

// ApplyResult describes the side effects of Apply().
type ApplyResult struct {
	// TargetPath is the absolute path of the constitution file that
	// was modified.
	TargetPath string
	// OldVersion / NewVersion are the strings written to the
	// **Version:** line (e.g. "0.17", "0.18").
	OldVersion string
	NewVersion string
	// ChangelogEntry is the bullet appended to the Changelog section.
	ChangelogEntry string
	// AuditPath is the absolute path of the audit/overrides record
	// written for this apply.
	AuditPath string
}

// slugify returns a filesystem-safe slug derived from file + section.
// Example: ("Common.md", "U17") → "Common-md-U17".
func slugify(file, section string) string {
	repl := strings.NewReplacer(
		".", "-",
		"/", "-",
		" ", "-",
		"§", "",
		":", "",
		"(", "",
		")", "",
	)
	s := repl.Replace(file + "-" + section)
	// Collapse repeated dashes.
	for strings.Contains(s, "--") {
		s = strings.ReplaceAll(s, "--", "-")
	}
	return strings.Trim(s, "-")
}
