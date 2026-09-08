package render

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/grokify/gogit/scanner"
)

// ScanFormats lists the format values Scan accepts.
var ScanFormats = []string{"list", "table"}

// ScanOptions configures Scan's output.
type ScanOptions struct {
	Format         string // "list" (default) or "table"
	ShowClean      bool   // also print repos with no issues
	ShowSummary    bool   // print the summary block at the end
	CheckWorkflows bool   // include workflow compliance columns/summary
}

// Scan renders gitscan's root issue-scan report to w: results sorted by
// name, one row per repo that has an issue (or every repo, if
// opts.ShowClean), followed by an optional summary.
func Scan(w io.Writer, results []scanner.RepoResult, opts ScanOptions) error {
	if opts.Format != "list" && opts.Format != "table" {
		return fmt.Errorf("render: invalid format %q, must be one of %s", opts.Format, strings.Join(ScanFormats, ", "))
	}

	sorted := make([]scanner.RepoResult, len(results))
	copy(sorted, results)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Name < sorted[j].Name })

	maxNameLen := 0
	for _, r := range sorted {
		if len(r.Name) > maxNameLen {
			maxNameLen = len(r.Name)
		}
	}

	var (
		totalRepos, reposWithIssues                                     int
		uncommittedCount, replaceCount, mismatchCount, statusErrorCount int
		workflowFullCount, workflowPartialCount, workflowNoneCount      int
	)

	if opts.Format == "table" {
		scanTableHeader(w)
	}

	rowNum := 0
	for _, result := range sorted {
		totalRepos++
		hasIssues := result.HasIssues()

		if hasIssues {
			reposWithIssues++
			if result.HasUncommittedChanges {
				uncommittedCount++
			}
			if result.HasReplaceDirectives {
				replaceCount++
			}
			if result.HasModuleMismatch {
				mismatchCount++
			}
			if result.StatusError != "" {
				statusErrorCount++
			}
		}

		if opts.CheckWorkflows {
			switch result.WorkflowCompliance.ComplianceLevel {
			case "full":
				workflowFullCount++
			case "partial":
				workflowPartialCount++
			case "none":
				workflowNoneCount++
			}
		}

		// Show repos with issues, workflow compliance issues, or clean
		// repos if requested.
		showRepo := hasIssues || opts.ShowClean
		if opts.CheckWorkflows && result.WorkflowCompliance.ComplianceLevel != "full" {
			showRepo = true
		}
		if showRepo {
			rowNum++
			if opts.Format == "table" {
				scanTableRow(w, rowNum, result)
			} else {
				internalDeps := scanner.GetInternalDeps(result, sorted)
				scanListRow(w, rowNum, result, maxNameLen, internalDeps, opts.CheckWorkflows)
			}
		}
	}

	fmt.Fprintln(w)
	if opts.ShowSummary {
		fmt.Fprintln(w, "----------------------------------------")
		fmt.Fprintf(w, "Summary: %d repos scanned, %d with issues\n", totalRepos, reposWithIssues)
		fmt.Fprintf(w, "  - Uncommitted changes: %d\n", uncommittedCount)
		fmt.Fprintf(w, "  - Replace directives:  %d\n", replaceCount)
		fmt.Fprintf(w, "  - Module mismatches:   %d\n", mismatchCount)
		if statusErrorCount > 0 {
			fmt.Fprintf(w, "  - Status check failed:  %d (git status could not be determined; not counted as clean)\n", statusErrorCount)
		}
		if opts.CheckWorkflows {
			fmt.Fprintln(w)
			fmt.Fprintln(w, "Workflow Compliance:")
			fmt.Fprintf(w, "  - Full:    %d (%.1f%%)\n", workflowFullCount, percent(workflowFullCount, totalRepos))
			fmt.Fprintf(w, "  - Partial: %d (%.1f%%)\n", workflowPartialCount, percent(workflowPartialCount, totalRepos))
			fmt.Fprintf(w, "  - None:    %d (%.1f%%)\n", workflowNoneCount, percent(workflowNoneCount, totalRepos))
		}
	}

	return nil
}

func scanTableHeader(w io.Writer) {
	fmt.Fprintln(w)
	fmt.Fprintln(w, "| # | Repository | Uncommitted | Replace | Mismatch | Git | go.mod |")
	fmt.Fprintln(w, "|---|------------|-------------|---------|----------|-----|--------|")
}

func scanTableRow(w io.Writer, num int, r scanner.RepoResult) {
	uncommitted := ""
	switch {
	case r.StatusError != "":
		uncommitted = "?"
	case r.HasUncommittedChanges:
		uncommitted = "X"
	}

	replace := ""
	if r.HasReplaceDirectives {
		replace = fmt.Sprintf("%d", r.ReplaceCount)
	}

	mismatch := ""
	if r.HasModuleMismatch {
		mismatch = "X"
	}

	git := "Y"
	if !r.IsGitRepo {
		git = "-"
	}

	gomod := "Y"
	if !r.HasGoMod {
		gomod = "-"
	}

	fmt.Fprintf(w, "| %d | %s | %s | %s | %s | %s | %s |\n",
		num, r.Name, uncommitted, replace, mismatch, git, gomod)
}

func scanListRow(w io.Writer, num int, r scanner.RepoResult, maxNameLen int, internalDeps []string, showWorkflow bool) {
	var issues []string
	if r.HasUncommittedChanges {
		issues = append(issues, "uncommitted")
	}
	if r.HasReplaceDirectives {
		issues = append(issues, fmt.Sprintf("replace:%d", r.ReplaceCount))
	}
	if r.HasModuleMismatch {
		issues = append(issues, "mismatch")
	}
	if r.StatusError != "" {
		issues = append(issues, "status-check-failed")
	}
	if !r.IsGitRepo {
		issues = append(issues, "no-git")
	}
	if !r.HasGoMod {
		issues = append(issues, "no-gomod")
	}

	if showWorkflow {
		switch r.WorkflowCompliance.ComplianceLevel {
		case "full":
			issues = append(issues, "wf:✓")
		case "partial":
			issues = append(issues, "wf:~")
		case "none":
			issues = append(issues, "wf:✗")
		}
	}

	depStr := ""
	if len(internalDeps) > 0 {
		depStr = fmt.Sprintf(" (depends on: %s)", strings.Join(internalDeps, ", "))
	}

	if len(issues) > 0 {
		fmt.Fprintf(w, "%3d. %-*s  [%s]%s\n", num, maxNameLen, r.Name, strings.Join(issues, ", "), depStr)
	} else {
		fmt.Fprintf(w, "%3d. %-*s%s\n", num, maxNameLen, r.Name, depStr)
	}
}

func percent(count, total int) float64 {
	if total == 0 {
		return 0
	}
	return float64(count) / float64(total) * 100
}
