package constitution_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/convergent-systems-co/aiConstitution/src/internal/constitution"
)

func writeFile(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestLoadAllFourFiles(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "Constitution.md", "# Constitution\n**Version:** 0.3\n")
	writeFile(t, dir, "Common.md", "# Common\n**Version:** 0.17\n")
	writeFile(t, dir, "Code.md", "# Code\n**Version:** 0.6\n")
	writeFile(t, dir, "Writing.md", "# Writing\n**Version:** 0.4\n")

	files, err := constitution.Load(dir)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if files.Constitution == "" {
		t.Error("Constitution is empty")
	}
	if files.Common == "" {
		t.Error("Common is empty")
	}
	if files.Code == "" {
		t.Error("Code is empty")
	}
	if files.Writing == "" {
		t.Error("Writing is empty")
	}
}

func TestValidatePassesWithAllFiles(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "Constitution.md", "# Constitution\n**Version:** 0.3\n")
	writeFile(t, dir, "Common.md", "# Common\n**Version:** 0.17\n")
	writeFile(t, dir, "Code.md", "# Code\n**Version:** 0.6\n")
	writeFile(t, dir, "Writing.md", "# Writing\n**Version:** 0.4\n")

	files, err := constitution.Load(dir)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	findings := files.Validate()
	if len(findings) != 0 {
		t.Errorf("Validate() = %v, want no findings", findings)
	}
}

func TestLoadErrorsOnMissingFile(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "Constitution.md", "# Constitution\n")
	// Common.md, Code.md, Writing.md absent

	_, err := constitution.Load(dir)
	if err == nil {
		t.Fatal("expected error for missing files, got nil")
	}
}

func TestFileStatusAllPresent(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "Constitution.md", "# Constitution\n**Version:** 0.3\n")
	writeFile(t, dir, "Common.md", "# Common\n**Version:** 0.17\n")
	writeFile(t, dir, "Code.md", "# Code\n**Version:** 0.6\n")
	writeFile(t, dir, "Writing.md", "# Writing\n**Version:** 0.4\n")

	status := constitution.FileStatus(dir)
	for _, name := range constitution.FileNames {
		if !status[name] {
			t.Errorf("FileStatus[%q] = false, want true", name)
		}
	}
}

func TestFileStatusMissingFile(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "Constitution.md", "# Constitution\n")
	// Common.md, Code.md, Writing.md absent

	status := constitution.FileStatus(dir)
	if status["Common.md"] {
		t.Error("FileStatus[Common.md] = true, want false")
	}
	if !status["Constitution.md"] {
		t.Error("FileStatus[Constitution.md] = false, want true")
	}
}

func TestLocalOverrideLoadedWhenPresent(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "Constitution.md", "# Constitution\n")
	writeFile(t, dir, "Common.md", "# Common\n")
	writeFile(t, dir, "Code.md", "# Code\n")
	writeFile(t, dir, "Writing.md", "# Writing\n")
	writeFile(t, dir, "Constitution.local.md", "# Local Override\n")

	files, err := constitution.Load(dir)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if files.Local == "" {
		t.Error("Local is empty, expected Constitution.local.md content")
	}
}

func TestParseSectionsExtractsPersonaRules(t *testing.T) {
	content := `# AI Constitution

## 0. Governance Rules
Some meta text.

## 1. Common Rules
**P1. Honesty.** MUST NOT fabricate.

**P2. Cost ceiling.** Ask before exceeding $5.

## 2. Code Rules
**2.1 Function length.** Functions MUST be ≤30 lines.
`
	sections := constitution.ParseSections(content)
	if len(sections) != 2 {
		t.Fatalf("ParseSections: got %d sections, want 2 (Governance skipped)", len(sections))
	}
	if sections[0].Number != 1 || sections[0].Name != "Common" || sections[0].Slug != "common" || sections[0].FileName != "Common.md" {
		t.Errorf("sections[0] = %+v, want Number=1 Name=Common Slug=common FileName=Common.md", sections[0])
	}
	if sections[1].Number != 2 || sections[1].Name != "Code" || sections[1].Slug != "code" || sections[1].FileName != "Code.md" {
		t.Errorf("sections[1] = %+v", sections[1])
	}
}

func TestParseSectionsBodyContent(t *testing.T) {
	content := `## 1. Common Rules
**P1. Honesty.** MUST NOT fabricate.
## 2. Code Rules
**2.1 Length.** MUST be ≤30 lines.
`
	sections := constitution.ParseSections(content)
	if !strings.Contains(sections[0].Body, "Honesty") {
		t.Errorf("sections[0].Body missing expected content, got: %q", sections[0].Body)
	}
	if !strings.Contains(sections[1].Body, "Length") {
		t.Errorf("sections[1].Body missing expected content, got: %q", sections[1].Body)
	}
}

func TestParseSectionsEmptyReturnsNil(t *testing.T) {
	if got := constitution.ParseSections(""); len(got) != 0 {
		t.Errorf("ParseSections(\"\") = %v, want empty", got)
	}
}
