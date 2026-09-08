package render

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/grokify/gogit"
)

func testReport() CommitReport {
	loc := time.FixedZone("", -7*3600)
	return CommitReport{
		Repo: "/repo",
		Mode: "unpushed",
		Ref:  "@{upstream}",
		Commits: []gogit.Commit{
			{
				Hash:       "1234567890abcdef1234567890abcdef12345678",
				CommitDate: time.Date(2026, 9, 7, 12, 16, 2, 0, loc),
				Subject:    "feat: add a | pipe",
			},
			{
				Hash:       "abcdef1234567890abcdef1234567890abcdef12",
				CommitDate: time.Date(2026, 9, 7, 12, 16, 5, 0, loc),
				Subject:    "feat: add c",
			},
		},
	}
}

func one(report CommitReport) []CommitReport { return []CommitReport{report} }

func TestPendingInvalidFormat(t *testing.T) {
	var buf bytes.Buffer
	err := Commits(&buf, "bogus", one(testReport()))
	if err == nil {
		t.Fatal("expected an error for an invalid format")
	}
}

func TestPendingTable(t *testing.T) {
	var buf bytes.Buffer
	if err := Commits(&buf, "table", one(testReport())); err != nil {
		t.Fatal(err)
	}
	out := buf.String()

	if !strings.Contains(out, "Repo: /repo") {
		t.Errorf("missing repo line: %s", out)
	}
	if !strings.Contains(out, "1234567") {
		t.Errorf("missing short hash: %s", out)
	}
	if strings.Contains(out, "1234567890abcdef") {
		t.Errorf("table format should abbreviate hash, got full hash: %s", out)
	}
	if !strings.Contains(out, "2026-09-07T12:16:02-07:00") {
		t.Errorf("expected RFC3339 timestamp with offset in table output: %s", out)
	}
	if !strings.Contains(out, "Mon") {
		t.Errorf("expected weekday abbreviation in table output: %s", out)
	}
	// A literal "|" in the message must survive unescaped in table format
	// (tabwriter, not markdown).
	if !strings.Contains(out, "feat: add a | pipe") {
		t.Errorf("expected unescaped pipe in table output: %s", out)
	}
	// A single-repo report carries no fleet summary line.
	if strings.Contains(out, "Summary:") {
		t.Errorf("single-repo output should not include a summary line: %s", out)
	}
}

func TestPendingTableEmpty(t *testing.T) {
	var buf bytes.Buffer
	report := testReport()
	report.Commits = nil
	if err := Commits(&buf, "table", one(report)); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if strings.Contains(out, "HASH") {
		t.Errorf("expected no table header for zero commits: %s", out)
	}
	// The single repo still gets a header showing the zero count.
	if !strings.Contains(out, "Repo: /repo") {
		t.Errorf("expected the single repo header even with no commits: %s", out)
	}
}

func TestPendingMarkdownEscapesPipe(t *testing.T) {
	var buf bytes.Buffer
	if err := Commits(&buf, "markdown", one(testReport())); err != nil {
		t.Fatal(err)
	}
	out := buf.String()

	if !strings.Contains(out, "| # | Hash | Day | Timestamp | Message |") {
		t.Errorf("missing markdown header: %s", out)
	}
	if !strings.Contains(out, "| Mon | 2026-09-07T12:16:02-07:00 |") {
		t.Errorf("expected weekday and RFC3339 timestamp with offset in markdown output: %s", out)
	}
	if !strings.Contains(out, `feat: add a \| pipe`) {
		t.Errorf("expected escaped pipe in markdown output: %s", out)
	}
}

// commitEnvelope mirrors the JSON envelope for decoding in tests.
type commitEnvelope struct {
	Repos []struct {
		Repo    string `json:"repo"`
		Mode    string `json:"mode"`
		Ref     string `json:"ref"`
		Count   int    `json:"count"`
		Error   string `json:"error"`
		Commits []struct {
			Hash      string `json:"hash"`
			Weekday   string `json:"weekday"`
			Date      string `json:"date"`
			Time      string `json:"time"`
			Timestamp string `json:"timestamp"`
			Message   string `json:"message"`
		} `json:"commits"`
	} `json:"repos"`
	Summary struct {
		ReposScanned     int `json:"reposScanned"`
		ReposWithCommits int `json:"reposWithCommits"`
		CommitsTotal     int `json:"commitsTotal"`
	} `json:"summary"`
}

func TestPendingJSON(t *testing.T) {
	var buf bytes.Buffer
	if err := Commits(&buf, "json", one(testReport())); err != nil {
		t.Fatal(err)
	}

	var decoded commitEnvelope
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, buf.String())
	}

	// Even a single repo is reported as an array of one.
	if len(decoded.Repos) != 1 {
		t.Fatalf("expected 1 repo in the envelope, got %d", len(decoded.Repos))
	}
	repo := decoded.Repos[0]
	if repo.Repo != "/repo" || repo.Mode != "unpushed" || repo.Ref != "@{upstream}" {
		t.Errorf("unexpected repo entry: %+v", repo)
	}
	if repo.Count != 2 || len(repo.Commits) != 2 {
		t.Fatalf("expected 2 commits, got count=%d len=%d", repo.Count, len(repo.Commits))
	}
	if decoded.Summary.ReposScanned != 1 || decoded.Summary.ReposWithCommits != 1 || decoded.Summary.CommitsTotal != 2 {
		t.Errorf("unexpected summary: %+v", decoded.Summary)
	}

	first := repo.Commits[0]
	if first.Hash != "1234567890abcdef1234567890abcdef12345678" {
		t.Errorf("JSON format should carry the full hash, got %q", first.Hash)
	}
	if first.Date != "2026-09-07" || first.Time != "12:16:02" {
		t.Errorf("unexpected date/time: date=%q time=%q", first.Date, first.Time)
	}
	if first.Weekday != "Mon" {
		t.Errorf("unexpected weekday: %q, want Mon", first.Weekday)
	}
	if first.Timestamp != "2026-09-07T12:16:02-07:00" {
		t.Errorf("unexpected timestamp: %q", first.Timestamp)
	}
	if first.Message != "feat: add a | pipe" {
		t.Errorf("JSON message must not be escaped, got %q", first.Message)
	}
}

func TestApplyTimezoneOriginal(t *testing.T) {
	commits := testReport().Commits
	got, err := ApplyTimezone(commits, "")
	if err != nil {
		t.Fatal(err)
	}
	if got[0].CommitDate.Format("2006-01-02T15:04:05Z07:00") != "2026-09-07T12:16:02-07:00" {
		t.Errorf("expected the original offset preserved, got %v", got[0].CommitDate)
	}

	got, err = ApplyTimezone(commits, "original")
	if err != nil {
		t.Fatal(err)
	}
	if got[0].CommitDate.Format("2006-01-02T15:04:05Z07:00") != "2026-09-07T12:16:02-07:00" {
		t.Errorf("expected the original offset preserved, got %v", got[0].CommitDate)
	}
}

func TestApplyTimezoneUTC(t *testing.T) {
	got, err := ApplyTimezone(testReport().Commits, "utc")
	if err != nil {
		t.Fatal(err)
	}
	// -07:00 -> UTC is +7 hours: 12:16:02 -> 19:16:02.
	want := "2026-09-07T19:16:02Z"
	if got[0].CommitDate.Format("2006-01-02T15:04:05Z07:00") != want {
		t.Errorf("CommitDate = %v, want formatted as %q", got[0].CommitDate, want)
	}
}

func TestApplyTimezoneLocal(t *testing.T) {
	commits := testReport().Commits
	got, err := ApplyTimezone(commits, "local")
	if err != nil {
		t.Fatal(err)
	}
	if !got[0].CommitDate.Equal(commits[0].CommitDate) {
		t.Errorf("local conversion changed the instant in time: got %v, want same instant as %v", got[0].CommitDate, commits[0].CommitDate)
	}
	if got[0].CommitDate.Location() != time.Local {
		t.Errorf("expected Location() == time.Local, got %v", got[0].CommitDate.Location())
	}
}

func TestApplyTimezoneInvalid(t *testing.T) {
	if _, err := ApplyTimezone(testReport().Commits, "mars"); err == nil {
		t.Fatal("expected an error for an invalid tz value")
	}
}

func TestApplyTimezoneDoesNotMutateInput(t *testing.T) {
	commits := testReport().Commits
	original := commits[0].CommitDate
	if _, err := ApplyTimezone(commits, "utc"); err != nil {
		t.Fatal(err)
	}
	if !commits[0].CommitDate.Equal(original) || commits[0].CommitDate.Location() != original.Location() {
		t.Errorf("ApplyTimezone must not mutate its input slice: got %v, want unchanged %v", commits[0].CommitDate, original)
	}
}

func TestPendingSinceCommitMode(t *testing.T) {
	var buf bytes.Buffer
	report := testReport()
	report.Mode = "since-commit"
	report.Ref = "abc1234"
	if err := Commits(&buf, "table", one(report)); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "Commits after abc1234: 2") {
		t.Errorf("expected since-commit header, got: %s", buf.String())
	}
}

func TestCommitsPushedMode(t *testing.T) {
	var buf bytes.Buffer
	report := testReport()
	report.Mode = "pushed"
	report.Ref = "origin/main"
	if err := Commits(&buf, "table", one(report)); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "Pushed commits (most recent first, from origin/main): 2") {
		t.Errorf("expected pushed header, got: %s", buf.String())
	}
}

func TestCommitsPushedModeNoTarget(t *testing.T) {
	var buf bytes.Buffer
	report := testReport()
	report.Mode = "pushed"
	report.Ref = ""
	report.Commits = nil
	if err := Commits(&buf, "table", one(report)); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "branch has no upstream or remote-tracking branch") {
		t.Errorf("expected no-push-target header, got: %s", buf.String())
	}
}

func TestPendingUnpushedAllMode(t *testing.T) {
	var buf bytes.Buffer
	report := testReport()
	report.Mode = "unpushed-all"
	report.Ref = "" // no baseline
	if err := Commits(&buf, "table", one(report)); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "no upstream configured; all local commits unpushed): 2") {
		t.Errorf("expected no-upstream header, got: %s", out)
	}
	// The empty baseline must not leak into the header as a stray "to :".
	if strings.Contains(out, "not yet pushed to") {
		t.Errorf("unpushed-all should not render a baseline ref: %s", out)
	}
}

// fleetReports builds three repos: two with commits, one clean.
func fleetReports() []CommitReport {
	loc := time.FixedZone("", -7*3600)
	commit := func(subj string) gogit.Commit {
		return gogit.Commit{Hash: "aaaaaaa0000000000000000000000000000000", CommitDate: time.Date(2026, 9, 7, 9, 0, 0, 0, loc), Subject: subj}
	}
	return []CommitReport{
		{Repo: "/a", Mode: "unpushed", Ref: "@{upstream}", Commits: []gogit.Commit{commit("feat: a1"), commit("feat: a2")}},
		{Repo: "/b", Mode: "unpushed", Ref: "@{upstream}"}, // clean
		{Repo: "/c", Mode: "unpushed-all", Ref: "", Commits: []gogit.Commit{commit("feat: c1")}},
	}
}

func TestCommitsFleetHidesCleanAndSummarizes(t *testing.T) {
	var buf bytes.Buffer
	if err := Commits(&buf, "table", fleetReports()); err != nil {
		t.Fatal(err)
	}
	out := buf.String()

	if !strings.Contains(out, "Repo: /a") || !strings.Contains(out, "Repo: /c") {
		t.Errorf("expected repos with commits shown: %s", out)
	}
	if strings.Contains(out, "Repo: /b") {
		t.Errorf("a clean repo should be hidden in a fleet sweep: %s", out)
	}
	if !strings.Contains(out, "Summary: 3 repos scanned, 2 with unpushed commits, 3 commits total") {
		t.Errorf("unexpected summary: %s", out)
	}
}

func TestCommitsFleetJSONShape(t *testing.T) {
	var buf bytes.Buffer
	if err := Commits(&buf, "json", fleetReports()); err != nil {
		t.Fatal(err)
	}
	var decoded commitEnvelope
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, buf.String())
	}
	// Only the two repos with commits appear; the summary counts all three.
	if len(decoded.Repos) != 2 {
		t.Fatalf("expected 2 repos with commits in the array, got %d", len(decoded.Repos))
	}
	if decoded.Summary.ReposScanned != 3 || decoded.Summary.ReposWithCommits != 2 || decoded.Summary.CommitsTotal != 3 {
		t.Errorf("unexpected summary: %+v", decoded.Summary)
	}
}

func TestCommitsErrorReported(t *testing.T) {
	var buf bytes.Buffer
	reports := []CommitReport{
		{Repo: "/broken", Err: "gogit: git rev-parse failed"},
		{Repo: "/ok", Mode: "unpushed", Ref: "@{upstream}"},
	}
	if err := Commits(&buf, "table", reports); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "Repo: /broken") || !strings.Contains(out, "error: gogit: git rev-parse failed") {
		t.Errorf("expected the errored repo surfaced: %s", out)
	}
}

func TestCommitsNoRepos(t *testing.T) {
	var buf bytes.Buffer
	if err := Commits(&buf, "table", nil); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "No repositories found.") {
		t.Errorf("expected a no-repositories message, got: %s", buf.String())
	}
}
