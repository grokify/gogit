package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/grokify/gogit/internal/cliutil"
	"github.com/grokify/gogit/internal/render"
	"github.com/grokify/gogit/scanner"
	"github.com/grokify/mogo/fmt/progress"
	"github.com/spf13/cobra"
)

var (
	orderSinceStr     string
	includeTransitive bool
	unpushedOnly      bool
)

var orderCmd = &cobra.Command{
	Use:   "order [directory]",
	Short: "Show repos in dependency order (update dependencies first)",
	Long: `Analyze go.mod files and display repositories in topological order.
Repos with no internal dependencies are listed first, then repos that depend on them.
This helps determine the correct order to update and release Go modules.

When using --since with --transitive, also includes repos that transitively depend
on modified repos (even if they weren't directly modified).

Use --unpushed to only show repos with uncommitted changes or unpushed commits.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runOrder,
}

func init() {
	orderCmd.Flags().StringVarP(&dirPath, "dir", "d", "", "Directory to scan")
	orderCmd.Flags().StringVarP(&orderSinceStr, "since", "s", "", "Filter repos modified within duration (e.g., 7d, 14d, 2w, 1m)")
	orderCmd.Flags().BoolVarP(&includeTransitive, "transitive", "t", false, "Include repos that transitively depend on modified repos")
	orderCmd.Flags().BoolVarP(&unpushedOnly, "unpushed", "u", false, "Only show repos with uncommitted changes or unpushed commits")
	rootCmd.AddCommand(orderCmd)
}

func runOrder(cmd *cobra.Command, args []string) error {
	if len(args) > 0 && dirPath == "" {
		dirPath = args[0]
	}
	if dirPath == "" {
		return fmt.Errorf("directory path required\nUsage: gitscan order [directory] or gitscan order -d <directory>")
	}

	var sinceDuration time.Duration
	if orderSinceStr != "" {
		var err error
		sinceDuration, err = scanner.ParseDuration(orderSinceStr)
		if err != nil {
			return fmt.Errorf("invalid duration %q: %v\nValid formats: 7d (days), 2w (weeks), 1m (months), 24h (hours)", orderSinceStr, err)
		}
	}

	absPath, err := cliutil.ResolvePath(dirPath)
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
		Recurse:       false,
		CheckModTime:  true,         // Always need mod time for ordering
		CheckUnpushed: unpushedOnly, // Only check unpushed if filtering by it
		GitBackend:    createGitBackend(),
	}
	results, err := scanner.ScanDirectoryWithProgress(absPath, progressFn, opts)
	if err != nil {
		return fmt.Errorf("error scanning directory: %w", err)
	}
	renderer.Done("Scan complete!")

	// Filter by modification time if specified
	allResults := results // Keep original for transitive lookup
	if sinceDuration > 0 {
		var filtered []scanner.RepoResult
		for _, r := range results {
			if r.ModifiedSince(sinceDuration) {
				filtered = append(filtered, r)
			}
		}

		if includeTransitive && len(filtered) > 0 {
			results = scanner.GetTransitiveDependents(filtered, allResults)
			fmt.Printf("Found %d repos modified within %s, expanded to %d with transitive dependents\n",
				len(filtered), orderSinceStr, len(results))
		} else {
			results = filtered
			fmt.Printf("Filtered to %d repos modified within %s\n", len(results), orderSinceStr)
		}
	}

	sorted, cycles := scanner.TopologicalSort(results)
	if len(cycles) > 0 {
		fmt.Println("\nWarning: Circular dependencies detected:")
		for _, mod := range cycles {
			fmt.Printf("  - %s\n", mod)
		}
		fmt.Println()
	}

	if unpushedOnly {
		var unpushed []scanner.RepoResult
		for _, r := range sorted {
			if r.NeedsPush() {
				unpushed = append(unpushed, r)
			}
		}
		fmt.Printf("Filtered to %d repos with unpushed changes\n", len(unpushed))
		sorted = unpushed
	}

	render.Order(os.Stdout, sorted, results)
	return nil
}
