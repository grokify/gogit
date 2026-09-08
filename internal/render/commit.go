// Package render formats gitscan command results for output. It is not
// part of the gogit library's public API — cmd/*.go calls into it so the
// formatting logic can be unit tested without invoking Cobra or capturing
// os.Stdout.
package render

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/grokify/gogit"
)

// CommitFormats lists the format values Commits accepts.
var CommitFormats = []string{"table", "markdown", "json"}

// CommitTimezones lists the --tz values ApplyTimezone accepts.
var CommitTimezones = []string{"original", "local", "utc"}

// ApplyTimezone returns a copy of commits with each CommitDate converted
// per tz: "original" (or "") leaves each commit's own recorded offset
// untouched, "local" converts to the calling machine's local timezone, and
// "utc" converts to UTC.
func ApplyTimezone(commits []gogit.Commit, tz string) ([]gogit.Commit, error) {
	var loc *time.Location
	switch tz {
	case "", "original":
		return commits, nil
	case "local":
		loc = time.Local
	case "utc":
		loc = time.UTC
	default:
		return nil, fmt.Errorf("render: invalid tz %q, must be one of %s", tz, strings.Join(CommitTimezones, ", "))
	}

	out := make([]gogit.Commit, len(commits))
	for i, c := range commits {
		c.CommitDate = c.CommitDate.In(loc)
		out[i] = c
	}
	return out, nil
}

// CommitReport describes a labeled list of commits from one repository, as
// produced by the pending and pushed commands. The rendered table/markdown/
// json bodies are identical across modes; only the per-repo header differs.
type CommitReport struct {
	Repo string
	// Mode selects the per-repo header and JSON "mode" value:
	//   "unpushed"     - commits ahead of Ref (the push baseline)
	//   "unpushed-all" - no push baseline; every local commit is pending
	//   "since-commit" - commits after the explicit Ref hash
	//   "pushed"       - commits already pushed, read from Ref (the baseline)
	Mode string
	// Ref is the baseline the commits are measured against: an upstream or
	// remote-tracking ref, or an explicit commit hash. Empty when Mode is
	// "unpushed-all", or "pushed" with no push target.
	Ref     string
	Commits []gogit.Commit
	// Err, when non-empty, records why this repository could not be read.
	Err string
}

// Commits renders a set of per-repo CommitReports to w in the given format:
// "table" (aligned columns via text/tabwriter, for direct terminal reading),
// "markdown" (copy-pasteable GitHub-flavored tables), or "json" (structured
// envelope, for agents).
//
// The output shape is invariant in the number of repositories: a single
// repository is simply a set of one, so callers and JSON consumers never have
// to branch on how many repos were reported. In multi-repo output, repos with
// no matching commits (and no error) are omitted from the human-readable
// formats; the summary always reflects every repo scanned.
func Commits(w io.Writer, format string, reports []CommitReport) error {
	switch format {
	case "json":
		return commitsJSON(w, reports)
	case "markdown":
		commitsText(w, reports, true)
	case "table":
		commitsText(w, reports, false)
	default:
		return fmt.Errorf("render: invalid format %q, must be one of %s", format, strings.Join(CommitFormats, ", "))
	}
	return nil
}

// commitRowJSON is the JSON representation of one commit.
type commitRowJSON struct {
	Hash      string `json:"hash"`
	Weekday   string `json:"weekday"`   // Sun, Mon, ..., Sat, in the commit's rendered timezone
	Date      string `json:"date"`      // 2006-01-02, in the commit's rendered timezone
	Time      string `json:"time"`      // 15:04:05, in the commit's rendered timezone
	Timestamp string `json:"timestamp"` // RFC3339, in the commit's rendered timezone
	Message   string `json:"message"`
}

// commitRepoJSON is one repository's entry in the JSON envelope.
type commitRepoJSON struct {
	Repo    string          `json:"repo"`
	Mode    string          `json:"mode"` // see CommitReport.Mode
	Ref     string          `json:"ref"`  // baseline ref or hash; may be empty
	Count   int             `json:"count"`
	Commits []commitRowJSON `json:"commits"`
	Error   string          `json:"error,omitempty"`
}

// commitSummaryJSON aggregates a commit report across repositories.
type commitSummaryJSON struct {
	ReposScanned     int `json:"reposScanned"`
	ReposWithCommits int `json:"reposWithCommits"`
	CommitsTotal     int `json:"commitsTotal"`
}

// commitEnvelopeJSON is the (always list-shaped) JSON envelope.
type commitEnvelopeJSON struct {
	Repos   []commitRepoJSON  `json:"repos"`
	Summary commitSummaryJSON `json:"summary"`
}

func commitsJSON(w io.Writer, reports []CommitReport) error {
	out := commitEnvelopeJSON{
		Repos:   make([]commitRepoJSON, 0, len(reports)),
		Summary: summarize(reports),
	}
	for _, report := range reports {
		if !showRepo(report, len(reports)) {
			continue
		}
		repo := commitRepoJSON{
			Repo:    report.Repo,
			Mode:    report.Mode,
			Ref:     report.Ref,
			Count:   len(report.Commits),
			Commits: make([]commitRowJSON, 0, len(report.Commits)),
			Error:   report.Err,
		}
		for _, c := range report.Commits {
			repo.Commits = append(repo.Commits, commitRowJSON{
				Hash:      c.Hash,
				Weekday:   weekday(c.CommitDate),
				Date:      c.CommitDate.Format("2006-01-02"),
				Time:      c.CommitDate.Format("15:04:05"),
				Timestamp: rfc3339(c.CommitDate),
				Message:   c.Subject,
			})
		}
		out.Repos = append(out.Repos, repo)
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

// showRepo reports whether a repo appears in the human-readable output: it is
// shown when it has commits or an error, or when it is the only repo scanned
// (so an explicit single-repo query always prints something).
func showRepo(report CommitReport, total int) bool {
	return report.Err != "" || len(report.Commits) > 0 || total == 1
}

// summarize aggregates counts across every scanned repository.
func summarize(reports []CommitReport) commitSummaryJSON {
	s := commitSummaryJSON{ReposScanned: len(reports)}
	for _, report := range reports {
		if len(report.Commits) > 0 {
			s.ReposWithCommits++
			s.CommitsTotal += len(report.Commits)
		}
	}
	return s
}

// commitHeader writes one repo's header lines.
func commitHeader(w io.Writer, report CommitReport) {
	fmt.Fprintf(w, "Repo: %s\n", report.Repo)
	if report.Err != "" {
		fmt.Fprintf(w, "  error: %s\n\n", report.Err)
		return
	}
	switch report.Mode {
	case "since-commit":
		fmt.Fprintf(w, "Commits after %s: %d\n\n", report.Ref, len(report.Commits))
	case "unpushed-all":
		fmt.Fprintf(w, "Pending commits (no upstream configured; all local commits unpushed): %d\n\n", len(report.Commits))
	case "pushed":
		if report.Ref == "" {
			fmt.Fprintf(w, "Pushed commits: %d (branch has no upstream or remote-tracking branch)\n\n", len(report.Commits))
		} else {
			fmt.Fprintf(w, "Pushed commits (most recent first, from %s): %d\n\n", report.Ref, len(report.Commits))
		}
	default: // "unpushed"
		fmt.Fprintf(w, "Pending commits (not yet pushed to %s): %d\n\n", report.Ref, len(report.Commits))
	}
}

// commitsText renders every shown repository followed by a summary. When
// markdown is true each commit list is a GitHub-flavored table; otherwise
// it is an aligned text/tabwriter table for terminal reading.
func commitsText(w io.Writer, reports []CommitReport, markdown bool) {
	if len(reports) == 0 {
		fmt.Fprintln(w, "No repositories found.")
		return
	}

	for _, report := range reports {
		if !showRepo(report, len(reports)) {
			continue
		}
		commitHeader(w, report)
		if len(report.Commits) == 0 {
			continue
		}
		if markdown {
			commitRowsMarkdown(w, report.Commits)
		} else {
			commitRowsTable(w, report.Commits)
		}
		fmt.Fprintln(w)
	}

	// The per-repo header already conveys the count for a single repo; only
	// summarize when a fleet was scanned.
	if len(reports) > 1 {
		s := summarize(reports)
		fmt.Fprintf(w, "Summary: %d repos scanned, %d with %s commits, %d commits total\n",
			s.ReposScanned, s.ReposWithCommits, summaryNoun(reports), s.CommitsTotal)
	}
}

// summaryNoun labels the summary's per-repo count based on what was reported.
func summaryNoun(reports []CommitReport) string {
	for _, report := range reports {
		if report.Mode == "pushed" {
			return "pushed"
		}
	}
	return "unpushed"
}

// commitRowsTable renders commits as an aligned text/tabwriter table.
func commitRowsTable(w io.Writer, commits []gogit.Commit) {
	tw := tabwriter.NewWriter(w, 0, 2, 2, ' ', 0)
	fmt.Fprintln(tw, "#\tHASH\tDAY\tTIMESTAMP\tMESSAGE")
	for i, c := range commits {
		fmt.Fprintf(tw, "%d\t%s\t%s\t%s\t%s\n",
			i+1, shortHash(c.Hash), weekday(c.CommitDate), rfc3339(c.CommitDate),
			sanitizeTabwriterCell(c.Subject))
	}
	tw.Flush()
}

// commitRowsMarkdown renders commits as a copy-pasteable GitHub-flavored
// markdown table (e.g. for pasting into a PR description or issue).
func commitRowsMarkdown(w io.Writer, commits []gogit.Commit) {
	fmt.Fprintln(w, "| # | Hash | Day | Timestamp | Message |")
	fmt.Fprintln(w, "|---|------|-----|-----------|---------|")
	for i, c := range commits {
		fmt.Fprintf(w, "| %d | %s | %s | %s | %s |\n",
			i+1, shortHash(c.Hash), weekday(c.CommitDate), rfc3339(c.CommitDate),
			escapeMarkdownCell(c.Subject))
	}
}

// rfc3339 formats t as RFC 3339 with an explicit timezone: "Z" for exact
// UTC, otherwise a numeric offset (e.g. "-07:00").
func rfc3339(t time.Time) string {
	return t.Format("2006-01-02T15:04:05Z07:00")
}

// weekday returns t's day of week as a 3-letter abbreviation: Sun, Mon,
// Tue, Wed, Thu, Fri, or Sat.
func weekday(t time.Time) string {
	return t.Format("Mon")
}

// shortHash returns the standard 7-character abbreviated form of a commit
// hash.
func shortHash(hash string) string {
	if len(hash) > 7 {
		return hash[:7]
	}
	return hash
}

// escapeMarkdownCell escapes characters that would otherwise break a
// markdown table cell.
func escapeMarkdownCell(s string) string {
	return strings.ReplaceAll(s, "|", "\\|")
}

// sanitizeTabwriterCell strips characters that would otherwise confuse
// tabwriter's column alignment (it uses raw tab bytes as its own column
// separator).
func sanitizeTabwriterCell(s string) string {
	s = strings.ReplaceAll(s, "\t", "    ")
	return strings.ReplaceAll(s, "\n", " ")
}
