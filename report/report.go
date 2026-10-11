// Package report builds and renders commit reports — the pending (unpushed)
// and pushed commit lists that gitscan prints — so CLI, library and MCP
// callers share one implementation.
//
// A Report is one repository's commit list. Pending, PendingReport and
// PushedReport produce reports from gogit queries; BuildEnvelope turns
// reports into the JSON-ready Envelope (which an MCP tool can return as is);
// Write renders reports as an aligned table, markdown, or JSON.
package report

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/grokify/gogit"
)

// Formats lists the format values Commits accepts.
var Formats = []string{"table", "markdown", "json"}

// Timezones lists the --tz values ApplyTimezone accepts.
var Timezones = []string{"original", "local", "utc"}

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
		return nil, fmt.Errorf("report: invalid tz %q, must be one of %s", tz, strings.Join(Timezones, ", "))
	}

	out := make([]gogit.Commit, len(commits))
	for i, c := range commits {
		c.CommitDate = c.CommitDate.In(loc)
		out[i] = c
	}
	return out, nil
}

// Report describes a labeled list of commits from one repository, as
// produced by the pending and pushed commands. The rendered table/markdown/
// json bodies are identical across modes; only the per-repo header differs.
type Report struct {
	Repo string
	// Branch is the repository's current branch (or "HEAD" when detached),
	// shown in the header so a report is never read against the wrong
	// checkout — e.g. "no upstream configured" on a fresh feature branch is
	// easy to misread as "main has unpushed work" without it. Empty when the
	// branch could not be determined (surfaced via Err instead).
	Branch string
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
	// Files is the merged, de-duplicated list of paths touched by Commits,
	// relative to the repository root. Empty unless the caller asked for it.
	Files []string
	// Err, when non-empty, records why this repository could not be read.
	Err string
}

// Write renders a set of per-repo Reports to w in the given format:
// "table" (aligned columns via text/tabwriter, for direct terminal reading),
// "markdown" (copy-pasteable GitHub-flavored tables), or "json" (structured
// envelope, for agents).
//
// The output shape is invariant in the number of repositories: a single
// repository is simply a set of one, so callers and JSON consumers never have
// to branch on how many repos were reported. In multi-repo output, repos with
// no matching commits (and no error) are omitted from the human-readable
// formats; the summary always reflects every repo scanned.
func Write(w io.Writer, format string, reports []Report) error {
	switch format {
	case "json":
		return commitsJSON(w, reports)
	case "markdown":
		commitsText(w, reports, true)
	case "table":
		commitsText(w, reports, false)
	default:
		return fmt.Errorf("report: invalid format %q, must be one of %s", format, strings.Join(Formats, ", "))
	}
	return nil
}

// Row is the JSON representation of one commit.
type Row struct {
	Hash      string `json:"hash"`
	Weekday   string `json:"weekday"`   // Sun, Mon, ..., Sat, in the commit's rendered timezone
	Date      string `json:"date"`      // 2006-01-02, in the commit's rendered timezone
	Time      string `json:"time"`      // 15:04:05, in the commit's rendered timezone
	Timestamp string `json:"timestamp"` // RFC3339, in the commit's rendered timezone
	Message   string `json:"message"`
	Signed    *bool  `json:"signed,omitempty"` // nil when signature state was not collected
}

// RepoEntry is one repository's entry in the JSON envelope.
type RepoEntry struct {
	Repo   string `json:"repo"`
	Branch string `json:"branch,omitempty"` // see Report.Branch
	Mode   string `json:"mode"`             // see Report.Mode
	Ref    string `json:"ref"`              // baseline ref or hash; may be empty
	Count  int    `json:"count"`
	// Unsigned counts commits known to be unsigned; omitted when none are.
	Unsigned int      `json:"unsigned,omitempty"`
	Commits  []Row    `json:"commits"`
	Files    []string `json:"files,omitempty"` // merged paths touched by the commits
	Error    string   `json:"error,omitempty"`
}

// Summary aggregates a commit report across repositories.
type Summary struct {
	ReposScanned     int `json:"reposScanned"`
	ReposWithCommits int `json:"reposWithCommits"`
	CommitsTotal     int `json:"commitsTotal"`
}

// Envelope is the (always list-shaped) JSON envelope.
type Envelope struct {
	Repos   []RepoEntry `json:"repos"`
	Summary Summary     `json:"summary"`
}

// BuildEnvelope converts reports into the JSON envelope: the repos that
// would be shown in human-readable output plus a summary over every report.
// It is what Write encodes for the "json" format, exposed so callers (an MCP
// server, say) can return the structure directly.
func BuildEnvelope(reports []Report) Envelope {
	out := Envelope{
		Repos:   make([]RepoEntry, 0, len(reports)),
		Summary: Summarize(reports),
	}
	for _, report := range reports {
		if !showRepo(report, len(reports)) {
			continue
		}
		repo := RepoEntry{
			Repo:     report.Repo,
			Branch:   report.Branch,
			Mode:     report.Mode,
			Ref:      report.Ref,
			Count:    len(report.Commits),
			Unsigned: gogit.CountUnsigned(report.Commits),
			Commits:  make([]Row, 0, len(report.Commits)),
			Files:    report.Files,
			Error:    report.Err,
		}
		for _, c := range report.Commits {
			repo.Commits = append(repo.Commits, Row{
				Hash:      c.Hash,
				Weekday:   weekday(c.CommitDate),
				Date:      c.CommitDate.Format("2006-01-02"),
				Time:      c.CommitDate.Format("15:04:05"),
				Timestamp: rfc3339(c.CommitDate),
				Message:   c.Subject,
				Signed:    c.Signed,
			})
		}
		out.Repos = append(out.Repos, repo)
	}
	return out
}

func commitsJSON(w io.Writer, reports []Report) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(BuildEnvelope(reports))
}

// showRepo reports whether a repo appears in the human-readable output: it is
// shown when it has commits or an error, or when it is the only repo scanned
// (so an explicit single-repo query always prints something).
func showRepo(report Report, total int) bool {
	return report.Err != "" || len(report.Commits) > 0 || total == 1
}

// Summarize aggregates counts across every scanned repository.
func Summarize(reports []Report) Summary {
	s := Summary{ReposScanned: len(reports)}
	for _, report := range reports {
		if len(report.Commits) > 0 {
			s.ReposWithCommits++
			s.CommitsTotal += len(report.Commits)
		}
	}
	return s
}

// commitHeader writes one repo's header lines.
func commitHeader(w io.Writer, report Report) {
	if report.Branch != "" {
		fmt.Fprintf(w, "Repo: %s (branch: %s)\n", report.Repo, report.Branch)
	} else {
		fmt.Fprintf(w, "Repo: %s\n", report.Repo)
	}
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
func commitsText(w io.Writer, reports []Report, markdown bool) {
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
		commitUnsignedWarning(w, report.Commits)
		commitFiles(w, report.Files, markdown)
	}

	// The per-repo header already conveys the count for a single repo; only
	// Summarize when a fleet was scanned.
	if len(reports) > 1 {
		s := Summarize(reports)
		fmt.Fprintf(w, "Summary: %d repos scanned, %d with %s commits, %d commits total\n",
			s.ReposScanned, s.ReposWithCommits, summaryNoun(reports), s.CommitsTotal)
	}
}

// signedLabel renders a commit's signature state for the SIGNED column.
func signedLabel(c gogit.Commit) string {
	switch {
	case !c.HasSignature():
		return "?"
	case c.IsUnsigned():
		return "NO"
	default:
		return "yes"
	}
}

// commitUnsignedWarning prints the library's signature warning, if any, so an
// unsigned commit is noticed before it reaches the remote.
func commitUnsignedWarning(w io.Writer, commits []gogit.Commit) {
	if msg := gogit.SignatureWarning(commits); msg != "" {
		fmt.Fprintf(w, "WARNING: %s.\n\n", msg)
	}
}

// commitFiles prints the merged file list under a repo's commits: a bulleted
// list of code spans in markdown, indented paths otherwise. It prints nothing
// when there are no files.
func commitFiles(w io.Writer, files []string, markdown bool) {
	if len(files) == 0 {
		return
	}
	fmt.Fprintf(w, "Files (%d):\n", len(files))
	if markdown {
		fmt.Fprintln(w)
	}
	for _, f := range files {
		if markdown {
			fmt.Fprintf(w, "- `%s`\n", f)
		} else {
			fmt.Fprintf(w, "  %s\n", f)
		}
	}
	fmt.Fprintln(w)
}

// summaryNoun labels the summary's per-repo count based on what was reported.
func summaryNoun(reports []Report) string {
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
	showSigned := gogit.HasSignatureInfo(commits)
	if showSigned {
		fmt.Fprintln(tw, "#\tHASH\tSIGNED\tDAY\tTIMESTAMP\tMESSAGE")
	} else {
		fmt.Fprintln(tw, "#\tHASH\tDAY\tTIMESTAMP\tMESSAGE")
	}
	for i, c := range commits {
		if showSigned {
			fmt.Fprintf(tw, "%d\t%s\t%s\t%s\t%s\t%s\n",
				i+1, shortHash(c.Hash), signedLabel(c), weekday(c.CommitDate), rfc3339(c.CommitDate),
				sanitizeTabwriterCell(c.Subject))
			continue
		}
		fmt.Fprintf(tw, "%d\t%s\t%s\t%s\t%s\n",
			i+1, shortHash(c.Hash), weekday(c.CommitDate), rfc3339(c.CommitDate),
			sanitizeTabwriterCell(c.Subject))
	}
	tw.Flush()
}

// commitRowsMarkdown renders commits as a copy-pasteable GitHub-flavored
// markdown table (e.g. for pasting into a PR description or issue).
func commitRowsMarkdown(w io.Writer, commits []gogit.Commit) {
	showSigned := gogit.HasSignatureInfo(commits)
	if showSigned {
		fmt.Fprintln(w, "| # | Hash | Signed | Day | Timestamp | Message |")
		fmt.Fprintln(w, "|---|------|--------|-----|-----------|---------|")
	} else {
		fmt.Fprintln(w, "| # | Hash | Day | Timestamp | Message |")
		fmt.Fprintln(w, "|---|------|-----|-----------|---------|")
	}
	for i, c := range commits {
		if showSigned {
			fmt.Fprintf(w, "| %d | %s | %s | %s | %s | %s |\n",
				i+1, shortHash(c.Hash), signedLabel(c), weekday(c.CommitDate), rfc3339(c.CommitDate),
				escapeMarkdownCell(c.Subject))
			continue
		}
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
