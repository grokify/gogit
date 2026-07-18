package cmd

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/grokify/gogit/scanner"
	"github.com/grokify/mogo/fmt/progress"
	"github.com/spf13/cobra"
)

const (
	version          = "0.4.0"
	progressBarWidth = 40
)

var (
	showClean   bool
	showSummary bool
	format      string
)

var rootCmd = &cobra.Command{
	Use:   "gitscan [directory]",
	Short: "Scan git repositories for common issues",
	Long: `gitscan scans multiple Git repositories and identifies repos that need attention.
It helps developers prioritize which repositories to update, commit, and push
by detecting uncommitted changes, replace directives, and module mismatches.

Use subcommands for filtering:
  gitscan since <duration> [dir]   Filter by modification time
  gitscan dep <module> [dir]       Filter by dependency
  gitscan order [dir]              Show repos in dependency order`,
	Version: version,
	Args:    cobra.MaximumNArgs(1),
	RunE:    runScan,
}

func init() {
	rootCmd.Flags().StringVarP(&dirPath, "dir", "d", "", "Directory to scan")
	rootCmd.Flags().BoolVar(&showClean, "show-clean", false, "Show repos with no issues")
	rootCmd.Flags().BoolVar(&showSummary, "summary", true, "Show summary at the end")
	rootCmd.Flags().StringVarP(&format, "format", "f", "list", "Output format: list or table")
	rootCmd.Flags().BoolVar(&useGoGit, "go-git", false, "Use go-git library instead of git CLI (pure Go, no process spawning)")
	rootCmd.Flags().BoolVar(&checkWorkflows, "check-workflows", false, "Check workflow compliance against reference repo")
	rootCmd.Flags().StringVar(&refRepo, "ref-repo", "plexusone/.github", "Reference workflow repository for compliance checking")
}

// Execute runs the root command
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func runScan(cmd *cobra.Command, args []string) error {
	// Handle positional argument
	if len(args) > 0 && dirPath == "" {
		dirPath = args[0]
	}

	if dirPath == "" {
		return fmt.Errorf("directory path required\nUsage: gitscan [directory] or gitscan -d <directory>")
	}

	// Validate format
	if format != "list" && format != "table" {
		return fmt.Errorf("invalid format %q, must be 'list' or 'table'", format)
	}

	// Resolve path
	absPath, err := resolvePath(dirPath)
	if err != nil {
		return err
	}

	fmt.Printf("Scanning: %s\n", absPath)

	// Count directories first
	total, err := scanner.CountDirectories(absPath)
	if err != nil {
		return fmt.Errorf("error counting directories: %w", err)
	}
	fmt.Printf("Found %d directories to scan\n\n", total)

	// Progress renderer
	renderer := progress.NewSingleStageRenderer(os.Stdout).WithBarWidth(progressBarWidth)

	// Progress callback
	progressFn := func(current, total int, name string) {
		renderer.Update(current, total, name)
	}

	opts := scanner.ScanOptions{
		GitBackend: createGitBackend(useGoGit),
	}

	// Configure workflow checking if enabled
	if checkWorkflows {
		opts.Workflow = scanner.WorkflowCheckOptions{
			Enabled:       true,
			RefRepo:       refRepo,
			RefBranch:     "main",
			RequiredTypes: scanner.DefaultGoWorkflowTypes(),
		}
	}

	results, err := scanner.ScanDirectoryWithProgress(absPath, progressFn, opts)
	if err != nil {
		return fmt.Errorf("error scanning directory: %w", err)
	}

	// Clear the progress line and show completion
	renderer.Done("Scan complete!")

	// Sort results alphabetically by name
	sort.Slice(results, func(i, j int) bool {
		return results[i].Name < results[j].Name
	})

	// Calculate max name length for alignment
	maxNameLen := 0
	for _, r := range results {
		if len(r.Name) > maxNameLen {
			maxNameLen = len(r.Name)
		}
	}

	// Display results based on format
	var (
		totalRepos           int
		reposWithIssues      int
		uncommittedCount     int
		replaceCount         int
		mismatchCount        int
		workflowFullCount    int
		workflowPartialCount int
		workflowNoneCount    int
	)

	if format == "table" {
		printTableHeader()
	}

	rowNum := 0
	for _, result := range results {
		totalRepos++
		hasIssues := result.HasUncommittedChanges || result.HasReplaceDirectives || result.HasModuleMismatch

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
		}

		// Track workflow compliance stats
		if checkWorkflows {
			switch result.WorkflowCompliance.ComplianceLevel {
			case "full":
				workflowFullCount++
			case "partial":
				workflowPartialCount++
			case "none":
				workflowNoneCount++
			}
		}

		// Show repos with issues, or clean repos if requested
		// Show repos with issues, workflow compliance issues, or clean repos if requested
		showRepo := hasIssues || showClean
		if checkWorkflows && result.WorkflowCompliance.ComplianceLevel != "full" {
			showRepo = true
		}
		if showRepo {
			rowNum++
			if format == "table" {
				printTableRow(rowNum, result)
			} else {
				internalDeps := scanner.GetInternalDeps(result, results)
				printResult(rowNum, result, maxNameLen, internalDeps, checkWorkflows)
			}
		}
	}

	fmt.Println()
	if showSummary {
		fmt.Println("----------------------------------------")
		fmt.Printf("Summary: %d repos scanned, %d with issues\n", totalRepos, reposWithIssues)
		fmt.Printf("  - Uncommitted changes: %d\n", uncommittedCount)
		fmt.Printf("  - Replace directives:  %d\n", replaceCount)
		fmt.Printf("  - Module mismatches:   %d\n", mismatchCount)
		if checkWorkflows {
			fmt.Println()
			fmt.Println("Workflow Compliance:")
			fmt.Printf("  - Full:    %d (%.1f%%)\n", workflowFullCount, percent(workflowFullCount, totalRepos))
			fmt.Printf("  - Partial: %d (%.1f%%)\n", workflowPartialCount, percent(workflowPartialCount, totalRepos))
			fmt.Printf("  - None:    %d (%.1f%%)\n", workflowNoneCount, percent(workflowNoneCount, totalRepos))
		}
	}

	return nil
}

func printTableHeader() {
	fmt.Println()
	fmt.Println("| # | Repository | Uncommitted | Replace | Mismatch | Git | go.mod |")
	fmt.Println("|---|------------|-------------|---------|----------|-----|--------|")
}

func printTableRow(num int, r scanner.RepoResult) {
	uncommitted := ""
	if r.HasUncommittedChanges {
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

	fmt.Printf("| %d | %s | %s | %s | %s | %s | %s |\n",
		num, r.Name, uncommitted, replace, mismatch, git, gomod)
}

func printResult(num int, r scanner.RepoResult, maxNameLen int, internalDeps []string, showWorkflow bool) {
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
	if !r.IsGitRepo {
		issues = append(issues, "no-git")
	}
	if !r.HasGoMod {
		issues = append(issues, "no-gomod")
	}

	// Add workflow compliance status
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
		fmt.Printf("%3d. %-*s  [%s]%s\n", num, maxNameLen, r.Name, joinIssues(issues), depStr)
	} else {
		fmt.Printf("%3d. %-*s%s\n", num, maxNameLen, r.Name, depStr)
	}
}

func joinIssues(issues []string) string {
	result := ""
	for i, issue := range issues {
		if i > 0 {
			result += ", "
		}
		result += issue
	}
	return result
}

func percent(count, total int) float64 {
	if total == 0 {
		return 0
	}
	return float64(count) / float64(total) * 100
}
