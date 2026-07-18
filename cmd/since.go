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

var (
	sinceDepFilter    string
	sinceUnpushedOnly bool
)

var sinceCmd = &cobra.Command{
	Use:   "since <duration> [directory]",
	Short: "Filter repos by modification time",
	Long: `Filter repositories by modification time with optional dependency and unpushed filtering.

The duration specifies the time window for filtering. Repos modified within
that duration are shown. When combined with --dep and/or --unpushed, filters
are applied with AND logic.

Duration formats:
  7d   - 7 days
  2w   - 2 weeks
  1m   - 1 month (30 days)
  24h  - 24 hours

Examples:
  gitscan since 7d ~/go/src                          # Modified in last 7 days
  gitscan since 7d --dep github.com/foo/bar ~/go/src # AND depends on module
  gitscan since 7d -u ~/go/src                       # AND has unpushed changes`,
	Args: cobra.RangeArgs(1, 2),
	RunE: runSince,
}

func init() {
	sinceCmd.Flags().StringVar(&sinceDepFilter, "dep", "", "Also filter by dependency (AND logic)")
	sinceCmd.Flags().BoolVarP(&sinceUnpushedOnly, "unpushed", "u", false, "Only show repos with uncommitted changes or unpushed commits")
	sinceCmd.Flags().BoolVar(&useGoGit, "go-git", false, "Use go-git library instead of git CLI")
	sinceCmd.Flags().BoolVarP(&recurse, "recurse", "r", false, "Check nested go.mod files")
	rootCmd.AddCommand(sinceCmd)
}

func runSince(cmd *cobra.Command, args []string) error {
	// Parse duration from first argument
	sinceStr := args[0]
	sinceDuration, err := parseDuration(sinceStr)
	if err != nil {
		return fmt.Errorf("invalid duration %q: %v\nValid formats: 7d (days), 2w (weeks), 1m (months), 24h (hours)", sinceStr, err)
	}

	// Get directory from second argument or flag
	var scanDir string
	if len(args) > 1 {
		scanDir = args[1]
	} else if dirPath != "" {
		scanDir = dirPath
	} else {
		return fmt.Errorf("directory path required\nUsage: gitscan since <duration> [directory]")
	}

	// Resolve path
	absPath, err := resolvePath(scanDir)
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

	progressFn := func(current, total int, name string) {
		renderer.Update(current, total, name)
	}

	opts := scanner.ScanOptions{
		Recurse:       recurse,
		CheckModTime:  true,
		CheckUnpushed: sinceUnpushedOnly,
		GitBackend:    createGitBackend(useGoGit),
	}
	results, err := scanner.ScanDirectoryWithProgress(absPath, progressFn, opts)
	if err != nil {
		return fmt.Errorf("error scanning directory: %w", err)
	}

	renderer.Done("Scan complete!")

	// Sort results alphabetically
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

	// Filter and display
	var (
		totalRepos         = len(results)
		sinceMatchCount    int
		depMatchCount      int
		unpushedMatchCount int
	)

	rowNum := 0
	for _, result := range results {
		// Check since filter
		matchesSince := result.ModifiedSince(sinceDuration)
		if !matchesSince {
			continue
		}
		sinceMatchCount++

		// Check dependency filter (AND logic)
		if sinceDepFilter != "" {
			hasDep := result.HasDependency(sinceDepFilter)
			if !hasDep {
				continue
			}
			depMatchCount++
		}

		// Check unpushed filter (AND logic)
		if sinceUnpushedOnly {
			if !result.NeedsPush() {
				continue
			}
			unpushedMatchCount++
		}

		rowNum++

		// Output format depends on whether --dep is set
		modTime := result.LatestModTime.Format("2006-01-02 15:04")
		if sinceDepFilter != "" {
			// Show: repo name + module name + timestamp
			fmt.Printf("%3d. %-*s  [%s]  %s\n", rowNum, maxNameLen, result.Name, result.ModuleName, modTime)
		} else {
			// Show: repo name + timestamp + internal deps
			internalDeps := scanner.GetInternalDeps(result, results)
			depStr := ""
			if len(internalDeps) > 0 {
				depStr = fmt.Sprintf(" (depends on: %s)", strings.Join(internalDeps, ", "))
			}
			fmt.Printf("%3d. %-*s  %s%s\n", rowNum, maxNameLen, result.Name, modTime, depStr)
		}
	}

	// Summary
	fmt.Println()
	fmt.Println("----------------------------------------")
	switch {
	case sinceDepFilter != "" && sinceUnpushedOnly:
		fmt.Printf("Summary: %d repos scanned, %d modified within %s, %d depend on %s, %d with unpushed changes\n",
			totalRepos, sinceMatchCount, sinceStr, depMatchCount, sinceDepFilter, unpushedMatchCount)
	case sinceDepFilter != "":
		fmt.Printf("Summary: %d repos scanned, %d modified within %s, %d also depend on %s\n",
			totalRepos, sinceMatchCount, sinceStr, depMatchCount, sinceDepFilter)
	case sinceUnpushedOnly:
		fmt.Printf("Summary: %d repos scanned, %d modified within %s, %d with unpushed changes\n",
			totalRepos, sinceMatchCount, sinceStr, unpushedMatchCount)
	default:
		fmt.Printf("Summary: %d repos scanned, %d modified within %s\n",
			totalRepos, sinceMatchCount, sinceStr)
	}

	return nil
}
