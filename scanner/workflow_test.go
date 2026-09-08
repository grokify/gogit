package scanner

import (
	"os"
	"path/filepath"
	"testing"
)

func writeWorkflowFile(t *testing.T, repoPath, name, content string) {
	t.Helper()
	dir := filepath.Join(repoPath, ".github", "workflows")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestCheckWorkflowComplianceNoWorkflowsDir(t *testing.T) {
	result := CheckWorkflowCompliance(t.TempDir(), WorkflowCheckOptions{})
	if result.HasWorkflows {
		t.Error("expected HasWorkflows false when .github/workflows doesn't exist")
	}
	if result.ComplianceLevel != "none" {
		t.Errorf("ComplianceLevel = %q, want none", result.ComplianceLevel)
	}
}

func TestCheckWorkflowComplianceEmptyWorkflowsDir(t *testing.T) {
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, ".github", "workflows"), 0o750); err != nil {
		t.Fatal(err)
	}
	result := CheckWorkflowCompliance(repo, WorkflowCheckOptions{})
	if result.HasWorkflows {
		t.Error("expected HasWorkflows false for an empty workflows dir")
	}
	if result.ComplianceLevel != "none" {
		t.Errorf("ComplianceLevel = %q, want none", result.ComplianceLevel)
	}
}

func TestCheckWorkflowCompliancePartialNoRefRepoMatch(t *testing.T) {
	repo := t.TempDir()
	writeWorkflowFile(t, repo, "ci.yaml", "name: CI\njobs:\n  build:\n    runs-on: ubuntu-latest\n")

	result := CheckWorkflowCompliance(repo, WorkflowCheckOptions{})
	if !result.HasWorkflows {
		t.Fatal("expected HasWorkflows true")
	}
	if result.ComplianceLevel != "partial" {
		t.Errorf("ComplianceLevel = %q, want partial (no reusable workflow references a ref repo)", result.ComplianceLevel)
	}
}

func TestCheckWorkflowComplianceFull(t *testing.T) {
	repo := t.TempDir()
	writeWorkflowFile(t, repo, "go-ci.yaml", `name: Go CI
jobs:
  build:
    uses: plexusone/.github/.github/workflows/go-ci.yaml@main
`)
	writeWorkflowFile(t, repo, "go-lint.yaml", `name: Go Lint
jobs:
  lint:
    uses: plexusone/.github/.github/workflows/go-lint.yaml@main
`)
	writeWorkflowFile(t, repo, "go-sast-codeql.yaml", `name: SAST
jobs:
  scan:
    uses: plexusone/.github/.github/workflows/go-sast-codeql.yaml@main
`)

	result := CheckWorkflowCompliance(repo, WorkflowCheckOptions{
		RefRepo:       "plexusone/.github",
		RequiredTypes: DefaultGoWorkflowTypes(),
	})
	if !result.UsesReusable || !result.RefRepoMatch {
		t.Fatalf("expected UsesReusable and RefRepoMatch true, got %+v", result)
	}
	if len(result.MissingWorkflows) != 0 {
		t.Errorf("expected no missing workflows, got %v", result.MissingWorkflows)
	}
	if result.ComplianceLevel != "full" {
		t.Errorf("ComplianceLevel = %q, want full", result.ComplianceLevel)
	}
}

func TestCheckWorkflowCompliancePartialMissingRequired(t *testing.T) {
	repo := t.TempDir()
	writeWorkflowFile(t, repo, "go-ci.yaml", `name: Go CI
jobs:
  build:
    uses: plexusone/.github/.github/workflows/go-ci.yaml@main
`)

	result := CheckWorkflowCompliance(repo, WorkflowCheckOptions{
		RefRepo:       "plexusone/.github",
		RequiredTypes: DefaultGoWorkflowTypes(),
	})
	if len(result.MissingWorkflows) != 2 {
		t.Fatalf("expected 2 missing workflows (lint, sast), got %v", result.MissingWorkflows)
	}
	if result.ComplianceLevel != "partial" {
		t.Errorf("ComplianceLevel = %q, want partial", result.ComplianceLevel)
	}
}

func TestParseReusableWorkflowRefs(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "wf.yaml")
	content := `name: CI
jobs:
  lint:
    uses: grokify/.github/.github/workflows/go-lint.yaml@main
  build:
    runs-on: ubuntu-latest
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	refs := parseReusableWorkflowRefs(path)
	if len(refs) != 1 || refs[0] != "grokify/.github/.github/workflows/go-lint.yaml@main" {
		t.Errorf("refs = %v, want a single ref to the reusable workflow", refs)
	}
}

func TestParseReusableWorkflowRefsMissingOrInvalid(t *testing.T) {
	if refs := parseReusableWorkflowRefs(filepath.Join(t.TempDir(), "missing.yaml")); refs != nil {
		t.Errorf("expected nil for a missing file, got %v", refs)
	}

	dir := t.TempDir()
	badPath := filepath.Join(dir, "bad.yaml")
	if err := os.WriteFile(badPath, []byte("not: [valid yaml"), 0o600); err != nil {
		t.Fatal(err)
	}
	if refs := parseReusableWorkflowRefs(badPath); refs != nil {
		t.Errorf("expected nil for invalid YAML, got %v", refs)
	}
}

func TestGetExpectedFilenames(t *testing.T) {
	tests := map[string][]string{
		"go-ci":          {"go-ci.yaml", "go-ci.yml", "ci.yaml", "ci.yml"},
		"go-lint":        {"go-lint.yaml", "go-lint.yml", "lint.yaml", "lint.yml"},
		"go-sast-codeql": {"go-sast-codeql.yaml", "go-sast-codeql.yml", "sast_codeql.yaml", "sast_codeql.yml", "codeql.yaml", "codeql.yml"},
		"custom-type":    {"custom-type.yaml", "custom-type.yml"},
	}
	for workflowType, want := range tests {
		got := getExpectedFilenames(workflowType)
		if len(got) != len(want) {
			t.Errorf("getExpectedFilenames(%q) = %v, want %v", workflowType, got, want)
			continue
		}
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("getExpectedFilenames(%q)[%d] = %q, want %q", workflowType, i, got[i], want[i])
			}
		}
	}
}

func TestDefaultGoWorkflowTypes(t *testing.T) {
	got := DefaultGoWorkflowTypes()
	want := []string{"go-ci", "go-lint", "go-sast-codeql"}
	if len(got) != len(want) {
		t.Fatalf("DefaultGoWorkflowTypes() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("DefaultGoWorkflowTypes()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestDirExists(t *testing.T) {
	if !dirExists(t.TempDir()) {
		t.Error("expected true for an existing directory")
	}
	if dirExists(filepath.Join(t.TempDir(), "nope")) {
		t.Error("expected false for a missing path")
	}
	file := filepath.Join(t.TempDir(), "f.txt")
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if dirExists(file) {
		t.Error("expected false for a path that is a file, not a directory")
	}
}
