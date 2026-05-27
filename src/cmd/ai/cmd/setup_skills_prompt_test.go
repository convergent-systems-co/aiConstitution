package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// mockFetchDir returns a fixed set of directory entries.
func mockFetchDir(entries []skillAtomDirEntry) fetchDirFn {
	return func() ([]skillAtomDirEntry, error) {
		return entries, nil
	}
}

// mockFetchAtom returns a fixed skill atom for each download URL.
func mockFetchAtom(atoms map[string]*skillAtom) fetchAtomFn {
	return func(url string) (*skillAtom, error) {
		a, ok := atoms[url]
		if !ok {
			return nil, nil //nolint:nilnil // test stub: absent key means no atom
		}
		return a, nil
	}
}

// testInstallRecorder returns an installFn that records which slugs were installed.
func testInstallRecorder(installed *[]string) installFn {
	return func(_ *cobra.Command, slug string) error {
		*installed = append(*installed, slug)
		return nil
	}
}

// TestSkillSelectionPrompt_SelectAll verifies that inputting "all" causes every
// available skill to be installed.
func TestSkillSelectionPrompt_SelectAll(t *testing.T) {
	entries := []skillAtomDirEntry{
		{Name: "commit.json", DownloadURL: "http://test/commit.json"},
		{Name: "checkpoint.json", DownloadURL: "http://test/checkpoint.json"},
		{Name: "debug.json", DownloadURL: "http://test/debug.json"},
	}
	atoms := map[string]*skillAtom{
		"http://test/commit.json":     {ID: "skill/commit", Name: "commit", Description: "Generate conventional-commit message", Lifecycle: "stable"},
		"http://test/checkpoint.json": {ID: "skill/checkpoint", Name: "checkpoint", Description: "Snapshot in-flight work", Lifecycle: "stable"},
		"http://test/debug.json":      {ID: "skill/debug", Name: "debug", Description: "Five-phase systematic debugging", Lifecycle: "stable"},
	}

	var installed []string
	var out bytes.Buffer
	input := strings.NewReader("all\n")

	err := runSkillSelectionPrompt(&out, input, true, mockFetchDir(entries), mockFetchAtom(atoms), testInstallRecorder(&installed), &cobra.Command{})
	if err != nil {
		t.Fatalf("runSkillSelectionPrompt: %v", err)
	}

	if len(installed) != 3 {
		t.Errorf("SelectAll: installed %d skills, want 3; got %v", len(installed), installed)
	}
	// All three slugs must appear.
	wantSlugs := map[string]bool{"commit": true, "checkpoint": true, "debug": true}
	for _, s := range installed {
		if !wantSlugs[s] {
			t.Errorf("unexpected slug installed: %q", s)
		}
	}
}

// TestSkillSelectionPrompt_SelectSubset verifies that "1,3" installs only the
// first and third skills in the displayed list.
func TestSkillSelectionPrompt_SelectSubset(t *testing.T) {
	entries := []skillAtomDirEntry{
		{Name: "alpha.json", DownloadURL: "http://test/alpha.json"},
		{Name: "beta.json", DownloadURL: "http://test/beta.json"},
		{Name: "gamma.json", DownloadURL: "http://test/gamma.json"},
	}
	atoms := map[string]*skillAtom{
		"http://test/alpha.json": {ID: "skill/alpha", Name: "alpha", Description: "Alpha skill", Lifecycle: "stable"},
		"http://test/beta.json":  {ID: "skill/beta", Name: "beta", Description: "Beta skill", Lifecycle: "stable"},
		"http://test/gamma.json": {ID: "skill/gamma", Name: "gamma", Description: "Gamma skill", Lifecycle: "stable"},
	}

	var installed []string
	var out bytes.Buffer
	input := strings.NewReader("1,3\n")

	err := runSkillSelectionPrompt(&out, input, true, mockFetchDir(entries), mockFetchAtom(atoms), testInstallRecorder(&installed), &cobra.Command{})
	if err != nil {
		t.Fatalf("runSkillSelectionPrompt: %v", err)
	}

	if len(installed) != 2 {
		t.Errorf("SelectSubset: installed %d skills, want 2; got %v", len(installed), installed)
	}
	// Items 1 and 3 are alpha and gamma.
	wantSlugs := map[string]bool{"alpha": true, "gamma": true}
	for _, s := range installed {
		if !wantSlugs[s] {
			t.Errorf("unexpected slug installed: %q", s)
		}
	}
}

// TestSkillSelectionPrompt_Skip verifies that empty input installs nothing.
func TestSkillSelectionPrompt_Skip(t *testing.T) {
	entries := []skillAtomDirEntry{
		{Name: "commit.json", DownloadURL: "http://test/commit.json"},
	}
	atoms := map[string]*skillAtom{
		"http://test/commit.json": {ID: "skill/commit", Name: "commit", Description: "Commit skill", Lifecycle: "stable"},
	}

	var installed []string
	var out bytes.Buffer
	input := strings.NewReader("\n") // empty — press Enter to skip

	err := runSkillSelectionPrompt(&out, input, true, mockFetchDir(entries), mockFetchAtom(atoms), testInstallRecorder(&installed), &cobra.Command{})
	if err != nil {
		t.Fatalf("runSkillSelectionPrompt: %v", err)
	}

	if len(installed) != 0 {
		t.Errorf("Skip: expected 0 installs, got %v", installed)
	}
}

// TestSkillSelectionPrompt_NonTTY verifies that when isTTY is false the skill
// prompt is skipped entirely — no fetch, no install.
func TestSkillSelectionPrompt_NonTTY(t *testing.T) {
	fetchCalled := false
	fetchDir := func() ([]skillAtomDirEntry, error) {
		fetchCalled = true
		return nil, nil
	}
	fetchAtom := func(_ string) (*skillAtom, error) { return nil, nil }

	var installed []string
	var out bytes.Buffer
	input := strings.NewReader("all\n")

	err := runSkillSelectionPrompt(&out, input, false /* isTTY=false → non-interactive */, fetchDir, fetchAtom, testInstallRecorder(&installed), &cobra.Command{})
	if err != nil {
		t.Fatalf("runSkillSelectionPrompt non-TTY: %v", err)
	}

	if fetchCalled {
		t.Error("NonTTY: fetchSkillsDirectory should not be called when isTTY is false")
	}
	if len(installed) != 0 {
		t.Errorf("NonTTY: expected 0 installs, got %v", installed)
	}
}

// TestSkillSelectionPrompt_InvalidInput verifies that non-numeric, non-"all"
// tokens in input are silently skipped — setup does not fail, and valid tokens
// are still processed.
func TestSkillSelectionPrompt_InvalidInput(t *testing.T) {
	entries := []skillAtomDirEntry{
		{Name: "commit.json", DownloadURL: "http://test/commit.json"},
		{Name: "debug.json", DownloadURL: "http://test/debug.json"},
	}
	atoms := map[string]*skillAtom{
		"http://test/commit.json": {ID: "skill/commit", Name: "commit", Description: "Commit skill", Lifecycle: "stable"},
		"http://test/debug.json":  {ID: "skill/debug", Name: "debug", Description: "Debug skill", Lifecycle: "stable"},
	}

	var installed []string
	var out bytes.Buffer
	// "abc" is an invalid token; "2" is valid. Result: only item 2 (debug) installed.
	input := strings.NewReader("abc,2\n")

	err := runSkillSelectionPrompt(&out, input, true, mockFetchDir(entries), mockFetchAtom(atoms), testInstallRecorder(&installed), &cobra.Command{})
	if err != nil {
		t.Fatalf("runSkillSelectionPrompt invalid input: %v", err)
	}

	if len(installed) != 1 || installed[0] != "debug" {
		t.Errorf("InvalidInput: want [debug], got %v", installed)
	}
}
