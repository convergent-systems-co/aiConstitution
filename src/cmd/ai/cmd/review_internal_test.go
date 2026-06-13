package cmd

import (
	"strings"
	"testing"
)

func TestEvaluatePanel_CodeReviewFailsWithoutTests(t *testing.T) {
	inspection := inspectDiff(strings.Join([]string{
		"diff --git a/src/cmd/ai/cmd/review.go b/src/cmd/ai/cmd/review.go",
		"+++ b/src/cmd/ai/cmd/review.go",
		"+func something() {}",
	}, "\n"))

	result := evaluatePanel("code-review", inspection)
	if result.Pass {
		t.Fatal("code-review unexpectedly passed without test changes")
	}
	if len(result.Findings) == 0 || !strings.Contains(result.Findings[0], "without accompanying test file changes") {
		t.Fatalf("unexpected finding: %+v", result.Findings)
	}
}

func TestEvaluatePanel_SecurityReviewDetectsSecrets(t *testing.T) {
	inspection := inspectDiff(strings.Join([]string{
		"diff --git a/config.env b/config.env",
		"+++ b/config.env",
		"+AWS_ACCESS_KEY_ID=AKIAABCDEFGHIJKLMNOP",
	}, "\n"))

	result := evaluatePanel("security-review", inspection)
	if result.Pass {
		t.Fatal("security-review unexpectedly passed with secret-like material")
	}
	if len(result.Findings) == 0 || !strings.Contains(result.Findings[0], "Potential credential or secret material") {
		t.Fatalf("unexpected finding: %+v", result.Findings)
	}
}

func TestEvaluatePanel_DocumentationReviewFailsForUserFacingChangeWithoutDocs(t *testing.T) {
	inspection := inspectDiff(strings.Join([]string{
		"diff --git a/src/cmd/ai/cmd/clone.go b/src/cmd/ai/cmd/clone.go",
		"+++ b/src/cmd/ai/cmd/clone.go",
		"+func changedCLI() {}",
	}, "\n"))

	result := evaluatePanel("documentation-review", inspection)
	if result.Pass {
		t.Fatal("documentation-review unexpectedly passed without docs changes")
	}
	if len(result.Findings) == 0 || !strings.Contains(result.Findings[0], "without documentation updates") {
		t.Fatalf("unexpected finding: %+v", result.Findings)
	}
}

func TestEvaluatePanel_PassesWhenDocsAndTestsPresent(t *testing.T) {
	inspection := inspectDiff(strings.Join([]string{
		"diff --git a/src/cmd/ai/cmd/clone.go b/src/cmd/ai/cmd/clone.go",
		"+++ b/src/cmd/ai/cmd/clone.go",
		"+func changedCLI() {}",
		"diff --git a/src/cmd/ai/cmd/clone_test.go b/src/cmd/ai/cmd/clone_test.go",
		"+++ b/src/cmd/ai/cmd/clone_test.go",
		"+func TestChangedCLI(t *testing.T) {}",
		"diff --git a/README.md b/README.md",
		"+++ b/README.md",
		"+Documented the change.",
	}, "\n"))

	codeResult := evaluatePanel("code-review", inspection)
	if !codeResult.Pass {
		t.Fatalf("code-review unexpectedly failed: %+v", codeResult)
	}

	docResult := evaluatePanel("documentation-review", inspection)
	if !docResult.Pass {
		t.Fatalf("documentation-review unexpectedly failed: %+v", docResult)
	}
}
