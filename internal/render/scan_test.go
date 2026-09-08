package render

import (
	"bytes"
	"strings"
	"testing"

	"github.com/grokify/gogit/scanner"
)

func TestScanInvalidFormat(t *testing.T) {
	var buf bytes.Buffer
	err := Scan(&buf, nil, ScanOptions{Format: "bogus"})
	if err == nil {
		t.Fatal("expected an error for an invalid format")
	}
}

func TestScanListShowsOnlyIssuesByDefault(t *testing.T) {
	results := []scanner.RepoResult{
		{Name: "clean-repo", IsGitRepo: true, HasGoMod: true},
		{Name: "dirty-repo", IsGitRepo: true, HasGoMod: true, HasUncommittedChanges: true},
	}
	var buf bytes.Buffer
	if err := Scan(&buf, results, ScanOptions{Format: "list", ShowSummary: true}); err != nil {
		t.Fatal(err)
	}
	out := buf.String()

	if strings.Contains(out, "clean-repo") {
		t.Errorf("clean repo should not be listed without --show-clean: %s", out)
	}
	if !strings.Contains(out, "dirty-repo") || !strings.Contains(out, "[uncommitted]") {
		t.Errorf("expected dirty-repo with [uncommitted] issue tag: %s", out)
	}
	if !strings.Contains(out, "Summary: 2 repos scanned, 1 with issues") {
		t.Errorf("unexpected summary: %s", out)
	}
}

func TestScanListShowClean(t *testing.T) {
	results := []scanner.RepoResult{
		{Name: "clean-repo", IsGitRepo: true, HasGoMod: true},
	}
	var buf bytes.Buffer
	if err := Scan(&buf, results, ScanOptions{Format: "list", ShowClean: true}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "clean-repo") {
		t.Errorf("expected clean-repo to be listed with --show-clean: %s", buf.String())
	}
}

func TestScanStatusErrorSurfacedNotSilentlyClean(t *testing.T) {
	results := []scanner.RepoResult{
		{Name: "broken-repo", IsGitRepo: true, StatusError: "git status failed: exit status 128"},
	}
	var buf bytes.Buffer
	if err := Scan(&buf, results, ScanOptions{Format: "list", ShowSummary: true}); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "status-check-failed") {
		t.Errorf("expected status-check-failed issue tag: %s", out)
	}
	if !strings.Contains(out, "Status check failed:  1") {
		t.Errorf("expected status-check-failed count in summary: %s", out)
	}
}

func TestScanTableStatusErrorShowsQuestionMark(t *testing.T) {
	results := []scanner.RepoResult{
		{Name: "broken-repo", IsGitRepo: true, StatusError: "boom"},
	}
	var buf bytes.Buffer
	if err := Scan(&buf, results, ScanOptions{Format: "table"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "| 1 | broken-repo | ? |") {
		t.Errorf("expected '?' in the Uncommitted column for a status error, got: %s", buf.String())
	}
}

func TestScanWorkflowCompliance(t *testing.T) {
	results := []scanner.RepoResult{
		// Fully compliant with no other issues: hidden, same as any clean repo.
		{Name: "full-repo", IsGitRepo: true, HasGoMod: true, WorkflowCompliance: scanner.WorkflowCompliance{ComplianceLevel: "full"}},
		// Not fully compliant: shown even though it has no other issues.
		{Name: "partial-repo", IsGitRepo: true, HasGoMod: true, WorkflowCompliance: scanner.WorkflowCompliance{ComplianceLevel: "partial"}},
	}
	var buf bytes.Buffer
	if err := Scan(&buf, results, ScanOptions{Format: "list", ShowSummary: true, CheckWorkflows: true}); err != nil {
		t.Fatal(err)
	}
	out := buf.String()

	if strings.Contains(out, "full-repo") {
		t.Errorf("fully-compliant repo with no other issues should be hidden: %s", out)
	}
	if !strings.Contains(out, "partial-repo") || !strings.Contains(out, "wf:~") {
		t.Errorf("expected partial-repo shown with wf:~: %s", out)
	}
	if !strings.Contains(out, "Full:    1") || !strings.Contains(out, "Partial: 1") {
		t.Errorf("expected workflow compliance summary counts: %s", out)
	}
}

func TestScanSortedByName(t *testing.T) {
	results := []scanner.RepoResult{
		{Name: "zeta", HasUncommittedChanges: true},
		{Name: "alpha", HasUncommittedChanges: true},
	}
	var buf bytes.Buffer
	if err := Scan(&buf, results, ScanOptions{Format: "list"}); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if strings.Index(out, "alpha") > strings.Index(out, "zeta") {
		t.Errorf("expected alpha before zeta: %s", out)
	}
}
