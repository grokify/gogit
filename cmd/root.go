package cmd

import (
	"fmt"
	"os"

	"github.com/grokify/gogit/internal/cliutil"
	"github.com/grokify/gogit/internal/render"
	"github.com/grokify/gogit/scanner"
	"github.com/grokify/mogo/fmt/progress"
	"github.com/spf13/cobra"
)

const (
	version          = "0.9.0"
	progressBarWidth = 40
)

var (
	showClean      bool
	showSummary    bool
	format         string
	checkWorkflows bool
	refRepo        string
)

var rootCmd = &cobra.Command{
	Use:   "gitscan [directory]",
	Short: "Scan git repositories for common issues",
	Long: `gitscan scans multiple Git repositories and identifies repos that need attention.
It helps developers prioritize which repositories to update, commit, and push
by detecting uncommitted changes, replace directives, and module mismatches.

directory defaults to the current directory.

Subcommands:
  gitscan since <duration> [dir]   Filter by modification time
  gitscan dep <module> [dir]       Filter by dependency
  gitscan order [dir]              Show repos in dependency order
  gitscan pending [dir]            List commits not yet pushed in one repo`,
	Version: version,
	Args:    cobra.MaximumNArgs(1),
	RunE:    runScan,
}

func init() {
	rootCmd.Flags().BoolVar(&showClean, "show-clean", false, "Show repos with no issues")
	rootCmd.Flags().BoolVar(&showSummary, "summary", true, "Show summary at the end")
	rootCmd.Flags().StringVarP(&format, "format", "f", "list", "Output format: list or table")
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
	absPath, err := cliutil.ResolvePath(dirArg(args, 0))
	if err != nil {
		return err
	}

	fmt.Printf("Scanning: %s\n", absPath)

	total, err := scanner.CountDirectories(absPath)
	if err != nil {
		return fmt.Errorf("error counting directories: %w", err)
	}
	fmt.Printf("Found %d directories to scan\n\n", total)

	renderer := progress.NewSingleStageRenderer(os.Stdout).WithBarWidth(progressBarWidth)
	progressFn := func(current, total int, name string) {
		renderer.Update(current, total, name)
	}

	opts := scanner.ScanOptions{
		GitBackend: createGitBackend(),
	}
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
	renderer.Done("Scan complete!")

	return render.Scan(os.Stdout, results, render.ScanOptions{
		Format:         format,
		ShowClean:      showClean,
		ShowSummary:    showSummary,
		CheckWorkflows: checkWorkflows,
	})
}
