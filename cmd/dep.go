package cmd

import (
	"fmt"
	"os"
	"sort"

	"github.com/grokify/gogit/scanner"
	"github.com/grokify/mogo/fmt/progress"
	"github.com/spf13/cobra"
)

var depCmd = &cobra.Command{
	Use:   "dep <module> [directory]",
	Short: "Filter repos by dependency",
	Long: `Filter repositories by dependency on a specific module.

Lists all repositories that depend on the specified Go module path.

Examples:
  gitscan dep github.com/grokify/mogo ~/go/src
  gitscan dep github.com/spf13/cobra ~/go/src -r`,
	Args: cobra.RangeArgs(1, 2),
	RunE: runDep,
}

func init() {
	depCmd.Flags().BoolVarP(&recurse, "recurse", "r", false, "Check nested go.mod files")
	rootCmd.AddCommand(depCmd)
}

func runDep(cmd *cobra.Command, args []string) error {
	// Parse module path from first argument
	depFilter := args[0]

	// Get directory from second argument or flag
	var scanDir string
	if len(args) > 1 {
		scanDir = args[1]
	} else if dirPath != "" {
		scanDir = dirPath
	} else {
		return fmt.Errorf("directory path required\nUsage: gitscan dep <module> [directory]")
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
		Recurse:    recurse,
		GitBackend: createGitBackend(),
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
	totalRepos := len(results)
	depMatchCount := 0
	rowNum := 0

	for _, result := range results {
		hasDep := result.HasDependency(depFilter)
		if !hasDep {
			continue
		}
		depMatchCount++
		rowNum++

		if recurse && len(result.GoModFiles) > 0 {
			fmt.Printf("%3d. %-*s  [%s + %d nested]\n", rowNum, maxNameLen, result.Name, result.ModuleName, len(result.GoModFiles))
		} else {
			fmt.Printf("%3d. %-*s  [%s]\n", rowNum, maxNameLen, result.Name, result.ModuleName)
		}
	}

	// Summary
	fmt.Println()
	fmt.Println("----------------------------------------")
	fmt.Printf("Summary: %d repos scanned, %d depend on %s\n", totalRepos, depMatchCount, depFilter)

	return nil
}
