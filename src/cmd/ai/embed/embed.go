// Package embed bundles the canonical hook library + wrapper templates
// into the `ai` binary at build time, and exposes helpers for
// extracting them onto disk at install / setup time.
//
// Source of truth: the files under embed/hooks/ and embed/wrappers/ in
// this package. The Go binary IS the distribution unit — there are no
// separate shell scripts to ship.
//
// Use:
//
//	ExtractAllHooks(hooksDir, false)        // ai setup / ai hooks install --all
//	ExtractHook(name, hooksDir, false)      // ai hooks install <name>
//	ExtractWrappers(binDir, false)          // ai hooks install command-wrappers
//
// All Extract* helpers refuse to overwrite by default; pass overwrite=true
// to clobber.
package embed

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	stdembed "embed"
)

// `all:` prefix is required so files starting with `_` (e.g.
// `_lib.py`) are included; without it Go's default exclude rule
// drops dot- and underscore-prefixed names.
//
//go:embed all:hooks all:wrappers
var assets stdembed.FS

//go:embed questions.yaml
var questionsYAML []byte

// QuestionsYAML returns the embedded questions.yaml bytes.
func QuestionsYAML() []byte { return questionsYAML }

//go:embed all:templates
var templates stdembed.FS

// ConstitutionTemplate returns the embedded constitution.tmpl bytes.
func ConstitutionTemplate() ([]byte, error) {
	return templates.ReadFile("templates/constitution.tmpl")
}


// HooksFS returns a sub-FS rooted at the embedded hooks/ tree.
func HooksFS() fs.FS {
	sub, err := fs.Sub(assets, "hooks")
	if err != nil {
		// Embed configuration is compile-time; an error here
		// indicates a build-time misconfiguration.
		panic(fmt.Errorf("embed: hooks sub-FS: %w", err))
	}
	return sub
}

// windowsShimBasenames is the set of wrapper basenames that have Windows
// .cmd/.ps1 counterparts. On Windows the bare (bash) form is skipped;
// on POSIX the .cmd/.ps1 forms are skipped.
var windowsShimBasenames = map[string]bool{"git": true, "gh": true}

// wrapperAppliesOnOS reports whether a wrapper filename should be installed
// on the current OS.
//
//   - <tool>.cmd / <tool>.ps1 where tool ∈ windowsShimBasenames → Windows only
//   - bare <tool> where tool ∈ windowsShimBasenames             → POSIX only
//   - everything else (notify-me variants, tests/, etc.)        → all platforms
func wrapperAppliesOnOS(name string) bool {
	isWindows := runtime.GOOS == "windows"
	ext := filepath.Ext(name)
	extLower := strings.ToLower(ext)
	base := strings.TrimSuffix(name, ext)

	// .cmd/.ps1 for a known Windows-shimmed tool → Windows only.
	if (extLower == ".cmd" || extLower == ".ps1") && windowsShimBasenames[base] {
		return isWindows
	}
	// Bare name for a known Windows-shimmed tool → POSIX only.
	if ext == "" && windowsShimBasenames[name] {
		return !isWindows
	}
	// Everything else (notify-me, notify-me.cmd, notify-me.ps1, etc.) → all platforms.
	return true
}

// WrappersFS returns a sub-FS rooted at the embedded wrappers/ tree.
func WrappersFS() fs.FS {
	sub, err := fs.Sub(assets, "wrappers")
	if err != nil {
		panic(fmt.Errorf("embed: wrappers sub-FS: %w", err))
	}
	return sub
}

// HookNames returns the canonical hook filenames present in the
// embedded FS (e.g., "audit.py", "patterns.json"). Helper for
// `ai hooks list` and `ai hooks install --all`.
func HookNames() ([]string, error) {
	entries, err := fs.ReadDir(HooksFS(), ".")
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		out = append(out, e.Name())
	}
	return out, nil
}

// ExtractHook writes a single embedded hook file to dstDir, applying
// executable permissions to .py files. Refuses to overwrite an
// existing file unless overwrite=true. Returns the absolute path
// written.
func ExtractHook(name, dstDir string, overwrite bool) (string, error) {
	src, err := fs.ReadFile(HooksFS(), name)
	if err != nil {
		return "", fmt.Errorf("embed: read %s: %w", name, err)
	}
	return writeFile(filepath.Join(dstDir, name), src, executableForName(name), overwrite)
}

// ExtractAllHooks writes every embedded hook into dstDir. Returns the
// list of paths written. Files that already exist are skipped unless
// overwrite=true.
func ExtractAllHooks(dstDir string, overwrite bool) ([]string, error) {
	names, err := HookNames()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dstDir, 0o750); err != nil {
		return nil, err
	}
	written := make([]string, 0, len(names))
	for _, n := range names {
		p, err := ExtractHook(n, dstDir, overwrite)
		if err != nil {
			if errors.Is(err, errSkipExisting) {
				continue
			}
			return written, err
		}
		written = append(written, p)
	}
	return written, nil
}

// ExtractWrappers writes the wrapper templates (git, gh) to binDir.
// They land as plain executable files (no `.template` suffix) so
// they can sit early on PATH and intercept the underlying command.
// Refuses to overwrite unless overwrite=true.
func ExtractWrappers(binDir string, overwrite bool) ([]string, error) {
	entries, err := fs.ReadDir(WrappersFS(), ".")
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(binDir, 0o750); err != nil {
		return nil, err
	}
	written := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if !wrapperAppliesOnOS(e.Name()) {
			continue
		}
		data, err := fs.ReadFile(WrappersFS(), e.Name())
		if err != nil {
			return written, fmt.Errorf("embed: read wrapper %s: %w", e.Name(), err)
		}
		// 0o755 is intentional: wrappers are executable scripts in
		// the user's own ~/.ai/bin/; the world-readable bits make
		// them easy to inspect.
		p, err := writeFile(filepath.Join(binDir, e.Name()), data, 0o755, overwrite) //nolint:gosec // G306: intentional executable
		if err != nil {
			if errors.Is(err, errSkipExisting) {
				continue
			}
			return written, err
		}
		written = append(written, p)
	}
	return written, nil
}

// errSkipExisting is returned by writeFile when overwrite is false
// and the destination already exists. Callers treat this as a
// non-fatal "leave as-is" outcome.
var errSkipExisting = errors.New("destination exists; not overwriting")

func writeFile(dst string, data []byte, mode os.FileMode, overwrite bool) (string, error) {
	if !overwrite {
		if _, err := os.Stat(dst); err == nil {
			return dst, errSkipExisting
		} else if !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o750); err != nil {
		return "", err
	}
	if err := os.WriteFile(dst, data, mode); err != nil {
		return "", err
	}
	return dst, nil
}

// executableForName chooses the mode for an extracted hook file.
// Python scripts get 0o755 so Claude Code's shell hook runner can
// invoke them directly (/bin/sh -c <path> requires +x). TOML and
// other config files stay 0o644.
func executableForName(name string) os.FileMode {
	if strings.HasSuffix(name, ".py") {
		return 0o755
	}
	return 0o644
}
