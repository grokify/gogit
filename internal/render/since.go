package render

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/grokify/gogit/scanner"
)

// SinceOptions configures Since's filtering and output.
type SinceOptions struct {
	Duration      time.Duration
	DurationLabel string // the original duration string, for the summary line
	DepFilter     string // AND-combined with the modification-time filter
	UnpushedOnly  bool   // AND-combined with the modification-time filter
}

// Since renders gitscan since's report to w: results sorted by name,
// filtered to those modified within opts.Duration (further AND-filtered
// by opts.DepFilter/opts.UnpushedOnly when set), followed by a summary.
func Since(w io.Writer, results []scanner.RepoResult, opts SinceOptions) {
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
		totalRepos         = len(sorted)
		sinceMatchCount    int
		depMatchCount      int
		unpushedMatchCount int
	)

	rowNum := 0
	for _, result := range sorted {
		if !result.ModifiedSince(opts.Duration) {
			continue
		}
		sinceMatchCount++

		if opts.DepFilter != "" {
			if !result.HasDependency(opts.DepFilter) {
				continue
			}
			depMatchCount++
		}

		if opts.UnpushedOnly {
			if !result.NeedsPush() {
				continue
			}
			unpushedMatchCount++
		}

		rowNum++
		modTime := result.LatestModTime.Format("2006-01-02 15:04")
		if opts.DepFilter != "" {
			fmt.Fprintf(w, "%3d. %-*s  [%s]  %s\n", rowNum, maxNameLen, result.Name, result.ModuleName, modTime)
		} else {
			internalDeps := scanner.GetInternalDeps(result, sorted)
			depStr := ""
			if len(internalDeps) > 0 {
				depStr = fmt.Sprintf(" (depends on: %s)", strings.Join(internalDeps, ", "))
			}
			fmt.Fprintf(w, "%3d. %-*s  %s%s\n", rowNum, maxNameLen, result.Name, modTime, depStr)
		}
	}

	fmt.Fprintln(w)
	fmt.Fprintln(w, "----------------------------------------")
	switch {
	case opts.DepFilter != "" && opts.UnpushedOnly:
		fmt.Fprintf(w, "Summary: %d repos scanned, %d modified within %s, %d depend on %s, %d with unpushed changes\n",
			totalRepos, sinceMatchCount, opts.DurationLabel, depMatchCount, opts.DepFilter, unpushedMatchCount)
	case opts.DepFilter != "":
		fmt.Fprintf(w, "Summary: %d repos scanned, %d modified within %s, %d also depend on %s\n",
			totalRepos, sinceMatchCount, opts.DurationLabel, depMatchCount, opts.DepFilter)
	case opts.UnpushedOnly:
		fmt.Fprintf(w, "Summary: %d repos scanned, %d modified within %s, %d with unpushed changes\n",
			totalRepos, sinceMatchCount, opts.DurationLabel, unpushedMatchCount)
	default:
		fmt.Fprintf(w, "Summary: %d repos scanned, %d modified within %s\n",
			totalRepos, sinceMatchCount, opts.DurationLabel)
	}
}
