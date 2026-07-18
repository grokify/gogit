package cmd

import (
	"fmt"
	"os"
	"strings"
	"time"

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
	// Handle positional argument
	if len(args) > 0 && dirPath == "" {
		dirPath = args[0]
	}

	if dirPath == "" {
		return fmt.Errorf("directory path required\nUsage: gitscan order [directory] or gitscan order -d <directory>")
	}

	// Parse since duration
	var sinceDuration time.Duration
	if orderSinceStr != "" {
		var err error
		sinceDuration, err = parseDuration(orderSinceStr)
		if err != nil {
			return fmt.Errorf("invalid duration %q: %v\nValid formats: 7d (days), 2w (weeks), 1m (months), 24h (hours)", orderSinceStr, err)
		}
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
		Recurse:       false,
		CheckModTime:  true,         // Always need mod time for ordering
		CheckUnpushed: unpushedOnly, // Only check unpushed if filtering by it
		GitBackend:    createGitBackend(),
	}
	results, err := scanner.ScanDirectoryWithProgress(absPath, progressFn, opts)
	if err != nil {
		return fmt.Errorf("error scanning directory: %w", err)
	}

	// Clear the progress line and show completion
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
			// Expand to include transitive dependents
			results = scanner.GetTransitiveDependents(filtered, allResults)
			fmt.Printf("Found %d repos modified within %s, expanded to %d with transitive dependents\n",
				len(filtered), orderSinceStr, len(results))
		} else {
			results = filtered
			fmt.Printf("Filtered to %d repos modified within %s\n", len(results), orderSinceStr)
		}
	}

	// Topological sort
	sorted, cycles := scanner.TopologicalSort(results)

	if len(cycles) > 0 {
		fmt.Println("\nWarning: Circular dependencies detected:")
		for _, mod := range cycles {
			fmt.Printf("  - %s\n", mod)
		}
		fmt.Println()
	}

	// Filter to only unpushed repos if requested
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

	// Calculate max name length for alignment
	maxNameLen := 0
	for _, r := range sorted {
		if len(r.Name) > maxNameLen {
			maxNameLen = len(r.Name)
		}
	}

	fmt.Println("\nUpdate order (dependencies first):")
	fmt.Println("----------------------------------")

	for i, r := range sorted {
		internalDeps := scanner.GetInternalDeps(r, results)
		depStr := ""
		if len(internalDeps) > 0 {
			depStr = fmt.Sprintf(" (depends on: %s)", strings.Join(internalDeps, ", "))
		}

		modTime := ""
		if !r.LatestModTime.IsZero() {
			modTime = r.LatestModTime.Format("2006-01-02 15:04")
		}

		fmt.Printf("%3d. %-*s  %s%s\n", i+1, maxNameLen, r.Name, modTime, depStr)
	}

	fmt.Printf("\nTotal: %d repos in dependency order\n", len(sorted))

	return nil
}
