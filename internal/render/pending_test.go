package render

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/grokify/gogit"
)

func testReport() PendingReport {
	loc := time.FixedZone("", -7*3600)
	return PendingReport{
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

func TestPendingInvalidFormat(t *testing.T) {
	var buf bytes.Buffer
	err := Pending(&buf, "bogus", testReport())
	if err == nil {
		t.Fatal("expected an error for an invalid format")
	}
}

func TestPendingTable(t *testing.T) {
	var buf bytes.Buffer
	if err := Pending(&buf, "table", testReport()); err != nil {
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
	// A literal "|" in the message must survive unescaped in table format
	// (tabwriter, not markdown).
	if !strings.Contains(out, "feat: add a | pipe") {
		t.Errorf("expected unescaped pipe in table output: %s", out)
	}
}

func TestPendingTableEmpty(t *testing.T) {
	var buf bytes.Buffer
	report := testReport()
	report.Commits = nil
	if err := Pending(&buf, "table", report); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(buf.String(), "HASH") {
		t.Errorf("expected no table header for zero commits: %s", buf.String())
	}
}

func TestPendingMarkdownEscapesPipe(t *testing.T) {
	var buf bytes.Buffer
	if err := Pending(&buf, "markdown", testReport()); err != nil {
		t.Fatal(err)
	}
	out := buf.String()

	if !strings.Contains(out, "| # | Hash | Date | Time | Message |") {
		t.Errorf("missing markdown header: %s", out)
	}
	if !strings.Contains(out, `feat: add a \| pipe`) {
		t.Errorf("expected escaped pipe in markdown output: %s", out)
	}
}

func TestPendingJSON(t *testing.T) {
	var buf bytes.Buffer
	if err := Pending(&buf, "json", testReport()); err != nil {
		t.Fatal(err)
	}

	var decoded struct {
		Repo    string `json:"repo"`
		Mode    string `json:"mode"`
		Ref     string `json:"ref"`
		Count   int    `json:"count"`
		Commits []struct {
			Hash      string `json:"hash"`
			Date      string `json:"date"`
			Time      string `json:"time"`
			Timestamp string `json:"timestamp"`
			Message   string `json:"message"`
		} `json:"commits"`
	}
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, buf.String())
	}

	if decoded.Repo != "/repo" || decoded.Mode != "unpushed" || decoded.Ref != "@{upstream}" {
		t.Errorf("unexpected envelope: %+v", decoded)
	}
	if decoded.Count != 2 || len(decoded.Commits) != 2 {
		t.Fatalf("expected 2 commits, got count=%d len=%d", decoded.Count, len(decoded.Commits))
	}
	first := decoded.Commits[0]
	if first.Hash != "1234567890abcdef1234567890abcdef12345678" {
		t.Errorf("JSON format should carry the full hash, got %q", first.Hash)
	}
	if first.Date != "2026-09-07" || first.Time != "12:16:02" {
		t.Errorf("unexpected date/time: date=%q time=%q", first.Date, first.Time)
	}
	if first.Timestamp != "2026-09-07T12:16:02-07:00" {
		t.Errorf("unexpected timestamp: %q", first.Timestamp)
	}
	if first.Message != "feat: add a | pipe" {
		t.Errorf("JSON message must not be escaped, got %q", first.Message)
	}
}

func TestPendingSinceCommitMode(t *testing.T) {
	var buf bytes.Buffer
	report := testReport()
	report.Mode = "since-commit"
	report.Ref = "abc1234"
	if err := Pending(&buf, "table", report); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "Commits after abc1234: 2") {
		t.Errorf("expected since-commit header, got: %s", buf.String())
	}
}
