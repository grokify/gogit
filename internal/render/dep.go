package render

import (
	"fmt"
	"io"
	"sort"

	"github.com/grokify/gogit/scanner"
)

// DepOptions configures Dep's filtering and output.
type DepOptions struct {
	ModulePath string
	DirectOnly bool
	Prefix     bool
	Recurse    bool // include the nested-go.mod-count annotation
}

// Dep renders gitscan dep's report to w: results sorted by name, filtered
// to those matching opts.ModulePath (per RepoResult.MatchesDependency),
// followed by a summary.
func Dep(w io.Writer, results []scanner.RepoResult, opts DepOptions) {
	sorted := make([]scanner.RepoResult, len(results))
	copy(sorted, results)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Name < sorted[j].Name })

	maxNameLen := 0
	for _, r := range sorted {
		if len(r.Name) > maxNameLen {
			maxNameLen = len(r.Name)
		}
	}

	totalRepos := len(sorted)
	depMatchCount := 0
	rowNum := 0

	for _, result := range sorted {
		if !result.MatchesDependency(opts.ModulePath, opts.DirectOnly, opts.Prefix) {
			continue
		}
		depMatchCount++
		rowNum++

		if opts.Recurse && len(result.GoModFiles) > 0 {
			fmt.Fprintf(w, "%3d. %-*s  [%s + %d nested]\n", rowNum, maxNameLen, result.Name, result.ModuleName, len(result.GoModFiles))
		} else {
			fmt.Fprintf(w, "%3d. %-*s  [%s]\n", rowNum, maxNameLen, result.Name, result.ModuleName)
		}
	}

	fmt.Fprintln(w)
	fmt.Fprintln(w, "----------------------------------------")
	fmt.Fprintf(w, "Summary: %d repos scanned, %d depend on %s\n", totalRepos, depMatchCount, opts.ModulePath)
}
