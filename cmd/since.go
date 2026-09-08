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

var (
	sinceDepFilter    string
	sinceUnpushedOnly bool
	sinceRecurse      bool
)

var sinceCmd = &cobra.Command{
	Use:   "since <duration> [directory]",
	Short: "Filter repos by modification time",
	Long: `Filter repositories by modification time with optional dependency and unpushed filtering.

The duration specifies the time window for filtering. Repos modified within
that duration are shown. When combined with --dep and/or --unpushed, filters
are applied with AND logic.

directory defaults to the current directory.

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
	sinceCmd.Flags().BoolVarP(&sinceRecurse, "recurse", "r", false, "Check nested go.mod files")
	rootCmd.AddCommand(sinceCmd)
}

func runSince(cmd *cobra.Command, args []string) error {
	sinceStr := args[0]
	sinceDuration, err := scanner.ParseDuration(sinceStr)
	if err != nil {
		return fmt.Errorf("invalid duration %q: %v\nValid formats: 7d (days), 2w (weeks), 1m (months), 24h (hours)", sinceStr, err)
	}

	absPath, err := cliutil.ResolvePath(dirArg(args, 1))
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
		Recurse:       sinceRecurse,
		CheckModTime:  true,
		CheckUnpushed: sinceUnpushedOnly,
		GitBackend:    createGitBackend(),
	}
	results, err := scanner.ScanDirectoryWithProgress(absPath, progressFn, opts)
	if err != nil {
		return fmt.Errorf("error scanning directory: %w", err)
	}
	renderer.Done("Scan complete!")

	render.Since(os.Stdout, results, render.SinceOptions{
		Duration:      sinceDuration,
		DurationLabel: sinceStr,
		DepFilter:     sinceDepFilter,
		UnpushedOnly:  sinceUnpushedOnly,
	})
	return nil
}
