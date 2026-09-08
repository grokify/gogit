package render

import (
	"fmt"
	"io"
	"strings"

	"github.com/grokify/gogit/scanner"
)

// Order renders gitscan order's topological update-order report to w.
// sorted is the (already topologically-sorted and filtered) list to
// display; allResults is the full result set used to look up internal
// dependency names for each row.
func Order(w io.Writer, sorted []scanner.RepoResult, allResults []scanner.RepoResult) {
	maxNameLen := 0
	for _, r := range sorted {
		if len(r.Name) > maxNameLen {
			maxNameLen = len(r.Name)
		}
	}

	fmt.Fprintln(w, "\nUpdate order (dependencies first):")
	fmt.Fprintln(w, "----------------------------------")

	for i, r := range sorted {
		internalDeps := scanner.GetInternalDeps(r, allResults)
		depStr := ""
		if len(internalDeps) > 0 {
			depStr = fmt.Sprintf(" (depends on: %s)", strings.Join(internalDeps, ", "))
		}

		modTime := ""
		if !r.LatestModTime.IsZero() {
			modTime = r.LatestModTime.Format("2006-01-02 15:04")
		}

		fmt.Fprintf(w, "%3d. %-*s  %s%s\n", i+1, maxNameLen, r.Name, modTime, depStr)
	}

	fmt.Fprintf(w, "\nTotal: %d repos in dependency order\n", len(sorted))
}
