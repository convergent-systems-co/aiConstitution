package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ErrHookNotInCatalog is returned by installHookFromCatalog when the requested
// slug is absent from the ai-atoms.com catalog OR when the atom is present but
// has no script field (backward-compat: catalog transition period).
//
// Callers distinguish this sentinel from network/decode errors to decide
// whether to fall back to the embedded copy or surface a real failure.
var ErrHookNotInCatalog = errors.New("hook not found in catalog or has no script")

// installHookFromCatalog fetches the ai-atoms.com catalog and writes the hook
// script for the given slug to hooksDir/<slug>.<ext> with mode 0755.
//
// Extension is determined by the atom's Language field:
//   - "sh", "shell", or "bash" → .sh
//   - anything else (including empty, "python") → .py
//
// When the atom's DependsOn list references other hook atoms, those are
// installed recursively. Dependency failures are non-fatal: the install
// continues and only the top-level result is returned.
//
// Returns ErrHookNotInCatalog when:
//   - no atom with type "hook" and id "hook/<slug>" is found, OR
//   - the atom is found but its Script field is empty.
//
// Returns a wrapped error (not ErrHookNotInCatalog) when the catalog fetch
// itself fails (network error, HTTP non-200, decode error).
func installHookFromCatalog(slug, hooksDir string) error {
	atoms, err := fetchAiAtomsCatalog()
	if err != nil {
		return fmt.Errorf("hooks catalog: fetch: %w", err)
	}

	for _, a := range atoms {
		if a.Type != "hook" {
			continue
		}
		// Catalog IDs are "hook/<slug>".
		atomSlug := strings.TrimPrefix(a.ID, "hook/")
		if atomSlug != slug {
			continue
		}
		// Atom found. A missing/empty script is a soft "not available yet".
		if a.Script == "" {
			return ErrHookNotInCatalog
		}

		ext := hookExtForLanguage(a.Language)
		// The catalog uses "hook/lib" but the convention on disk (and in Python
		// imports) is "_lib.py". Map "lib" → "_lib" to match the import name.
		filename := slug
		if slug == "lib" {
			filename = "_lib"
		}
		dest := filepath.Join(hooksDir, filename+ext)

		if err := os.MkdirAll(hooksDir, 0o750); err != nil {
			return fmt.Errorf("hooks catalog: mkdir: %w", err)
		}
		// 0755 is intentional: hook scripts must be executable.
		if err := os.WriteFile(dest, []byte(a.Script), 0o755); err != nil { //nolint:gosec // G306: executable hook
			return fmt.Errorf("hooks catalog: write %s: %w", dest, err)
		}

		// Install declared dependencies (e.g. hook/lib). Non-fatal: best effort.
		for _, dep := range a.DependsOn {
			depSlug := strings.TrimPrefix(dep, "hook/")
			// Ignore errors — a missing dep means the catalog hasn't published it yet.
			_ = installHookFromCatalog(depSlug, hooksDir)
		}

		return nil
	}

	// No matching hook atom found.
	return ErrHookNotInCatalog
}

// hookExtForLanguage returns the file extension for a hook based on its
// declared language. Shell variants get ".sh"; everything else gets ".py".
func hookExtForLanguage(lang string) string {
	switch strings.ToLower(lang) {
	case "sh", "shell", "bash":
		return ".sh"
	default:
		return ".py"
	}
}
