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

// PendingFormats lists the format values Pending accepts.
var PendingFormats = []string{"table", "markdown", "json"}

// PendingTimezones lists the --tz values ApplyTimezone accepts.
var PendingTimezones = []string{"original", "local", "utc"}

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
		return nil, fmt.Errorf("render: invalid tz %q, must be one of %s", tz, strings.Join(PendingTimezones, ", "))
	}

	out := make([]gogit.Commit, len(commits))
	for i, c := range commits {
		c.CommitDate = c.CommitDate.In(loc)
		out[i] = c
	}
	return out, nil
}

// PendingReport describes a set of commits pending push, for rendering.
type PendingReport struct {
	Repo    string
	Mode    string // "unpushed" or "since-commit"
	Ref     string // "@{upstream}" or an explicit commit hash
	Commits []gogit.Commit
}

// Pending renders a PendingReport to w in the given format: "table"
// (aligned columns via text/tabwriter, for direct terminal reading),
// "markdown" (copy-pasteable GitHub-flavored table), or "json" (structured
// envelope, for agents).
func Pending(w io.Writer, format string, report PendingReport) error {
	switch format {
	case "json":
		return pendingJSON(w, report)
	case "markdown":
		pendingMarkdown(w, report)
	case "table":
		pendingTable(w, report)
	default:
		return fmt.Errorf("render: invalid format %q, must be one of %s", format, strings.Join(PendingFormats, ", "))
	}
	return nil
}

// pendingCommitJSON is the JSON representation of one pending commit.
type pendingCommitJSON struct {
	Hash      string `json:"hash"`
	Weekday   string `json:"weekday"`   // Sun, Mon, ..., Sat, in the commit's rendered timezone
	Date      string `json:"date"`      // 2006-01-02, in the commit's rendered timezone
	Time      string `json:"time"`      // 15:04:05, in the commit's rendered timezone
	Timestamp string `json:"timestamp"` // RFC3339, in the commit's rendered timezone
	Message   string `json:"message"`
}

// pendingReportJSON is the JSON envelope for the pending-commits report.
type pendingReportJSON struct {
	Repo    string              `json:"repo"`
	Mode    string              `json:"mode"` // "unpushed" or "since-commit"
	Ref     string              `json:"ref"`  // "@{upstream}" or the given hash
	Count   int                 `json:"count"`
	Commits []pendingCommitJSON `json:"commits"`
}

func pendingJSON(w io.Writer, report PendingReport) error {
	out := pendingReportJSON{
		Repo:    report.Repo,
		Mode:    report.Mode,
		Ref:     report.Ref,
		Count:   len(report.Commits),
		Commits: make([]pendingCommitJSON, 0, len(report.Commits)),
	}
	for _, c := range report.Commits {
		out.Commits = append(out.Commits, pendingCommitJSON{
			Hash:      c.Hash,
			Weekday:   weekday(c.CommitDate),
			Date:      c.CommitDate.Format("2006-01-02"),
			Time:      c.CommitDate.Format("15:04:05"),
			Timestamp: rfc3339(c.CommitDate),
			Message:   c.Subject,
		})
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

// pendingHeader writes the report summary line shared by every non-JSON
// format.
func pendingHeader(w io.Writer, report PendingReport) {
	fmt.Fprintf(w, "Repo: %s\n", report.Repo)
	if report.Mode == "unpushed" {
		fmt.Fprintf(w, "Pending commits (not yet pushed to %s): %d\n\n", report.Ref, len(report.Commits))
	} else {
		fmt.Fprintf(w, "Commits after %s: %d\n\n", report.Ref, len(report.Commits))
	}
}

// pendingTable renders an aligned plain-text table via text/tabwriter,
// meant to be read directly in a terminal.
func pendingTable(w io.Writer, report PendingReport) {
	pendingHeader(w, report)
	if len(report.Commits) == 0 {
		return
	}

	tw := tabwriter.NewWriter(w, 0, 2, 2, ' ', 0)
	fmt.Fprintln(tw, "#\tHASH\tDAY\tTIMESTAMP\tMESSAGE")
	for i, c := range report.Commits {
		fmt.Fprintf(tw, "%d\t%s\t%s\t%s\t%s\n",
			i+1, shortHash(c.Hash), weekday(c.CommitDate), rfc3339(c.CommitDate),
			sanitizeTabwriterCell(c.Subject))
	}
	tw.Flush()
}

// pendingMarkdown renders a copy-pasteable GitHub-flavored markdown table
// (e.g. for pasting into a PR description or issue).
func pendingMarkdown(w io.Writer, report PendingReport) {
	pendingHeader(w, report)
	if len(report.Commits) == 0 {
		return
	}

	fmt.Fprintln(w, "| # | Hash | Day | Timestamp | Message |")
	fmt.Fprintln(w, "|---|------|-----|-----------|---------|")
	for i, c := range report.Commits {
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
