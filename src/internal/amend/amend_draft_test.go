package amend_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/convergent-systems-co/aiConstitution/src/internal/amend"
)

func TestParseRefValid(t *testing.T) {
	file, section, err := amend.ParseRef("Common.md/U17")
	if err != nil {
		t.Fatalf("ParseRef error: %v", err)
	}
	if file != "Common.md" || section != "U17" {
		t.Errorf("got (%q, %q), want (\"Common.md\", \"U17\")", file, section)
	}
}

func TestParseRefInvalid(t *testing.T) {
	cases := []string{"", "Common.md", "/U17", "Common.md/"}
	for _, c := range cases {
		if _, _, err := amend.ParseRef(c); err == nil {
			t.Errorf("ParseRef(%q) expected error, got nil", c)
		}
	}
}

func TestLocateSectionFinds(t *testing.T) {
	content := "# Title\n\n## U17 Some Heading\n\nbody body\n\n## U18 Next\n\nmore body\n"
	start, end, found := amend.LocateSection(content, "U17")
	if !found {
		t.Fatal("section U17 not found")
	}
	chunk := content[start:end]
	if !strings.Contains(chunk, "U17") {
		t.Errorf("section chunk missing 'U17': %q", chunk)
	}
	if strings.Contains(chunk, "U18") {
		t.Errorf("section chunk leaked into next section: %q", chunk)
	}
}

func TestLocateSectionNumeric(t *testing.T) {
	content := "## 11.2 Commit and PR discipline\n\nbody\n\n## 11.3 Next\n\nmore\n"
	_, _, found := amend.LocateSection(content, "11.2")
	if !found {
		t.Error("section 11.2 not found")
	}
}

func TestLocateSectionMissing(t *testing.T) {
	content := "# Title\n\n## U17 Heading\n\nbody\n"
	_, _, found := amend.LocateSection(content, "U99")
	if found {
		t.Error("U99 should not be found")
	}
}

func TestWriteDraftCreatesFile(t *testing.T) {
	dir := t.TempDir()
	d := amend.Draft{
		File:    "Common.md",
		Section: "U17",
	}
	path, err := amend.WriteDraft(d, dir)
	if err != nil {
		t.Fatalf("WriteDraft error: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("draft file missing: %v", err)
	}
	if filepath.Dir(path) != dir {
		t.Errorf("draft written to %s, want under %s", path, dir)
	}
	if !strings.Contains(filepath.Base(path), "Common-md-U17") {
		t.Errorf("draft filename missing slug: %s", filepath.Base(path))
	}
}

func TestWriteDraftContainsFrontmatter(t *testing.T) {
	dir := t.TempDir()
	d := amend.Draft{
		File:           "Common.md",
		Section:        "U17",
		AuditRef:       "audit/violations/2026-05-22T173810Z.md",
		ProposedChange: "Proposed body text.",
	}
	path, err := amend.WriteDraft(d, dir)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	body := string(data)
	for _, want := range []string{
		"file: Common.md",
		"section: U17",
		"audit_ref: audit/violations/2026-05-22T173810Z.md",
		"## Proposed change",
		"Proposed body text.",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("draft missing %q\nfull body:\n%s", want, body)
		}
	}
}

func TestWriteDraftErrorsOnEmptyFields(t *testing.T) {
	dir := t.TempDir()
	if _, err := amend.WriteDraft(amend.Draft{}, dir); err == nil {
		t.Error("expected error for empty Draft")
	}
}
