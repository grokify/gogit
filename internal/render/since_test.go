package render

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/grokify/gogit/scanner"
)

func sinceFixture() []scanner.RepoResult {
	now := time.Now()
	return []scanner.RepoResult{
		{
			Name:          "recent-with-dep",
			ModuleName:    "github.com/example/recent-with-dep",
			LatestModTime: now,
			Dependencies:  []string{"github.com/grokify/mogo"},
		},
		{
			Name:          "recent-no-dep",
			LatestModTime: now,
		},
		{
			Name:          "old",
			LatestModTime: now.Add(-30 * 24 * time.Hour),
		},
	}
}

func TestSinceBasicFilter(t *testing.T) {
	var buf bytes.Buffer
	Since(&buf, sinceFixture(), SinceOptions{Duration: 7 * 24 * time.Hour, DurationLabel: "7d"})
	out := buf.String()

	if !strings.Contains(out, "recent-with-dep") || !strings.Contains(out, "recent-no-dep") {
		t.Errorf("expected both recent repos listed: %s", out)
	}
	if strings.Contains(out, "old") {
		t.Errorf("old repo should be filtered out: %s", out)
	}
	if !strings.Contains(out, "Summary: 3 repos scanned, 2 modified within 7d") {
		t.Errorf("unexpected summary: %s", out)
	}
}

func TestSinceDepFilterANDsWithDuration(t *testing.T) {
	var buf bytes.Buffer
	Since(&buf, sinceFixture(), SinceOptions{
		Duration:      7 * 24 * time.Hour,
		DurationLabel: "7d",
		DepFilter:     "github.com/grokify/mogo",
	})
	out := buf.String()

	if !strings.Contains(out, "recent-with-dep") {
		t.Errorf("expected recent-with-dep to match dep filter: %s", out)
	}
	if strings.Contains(out, "recent-no-dep") {
		t.Errorf("recent-no-dep should be excluded by the dep filter: %s", out)
	}
	if !strings.Contains(out, "Summary: 3 repos scanned, 2 modified within 7d, 1 also depend on github.com/grokify/mogo") {
		t.Errorf("unexpected summary: %s", out)
	}
}

func TestSinceUnpushedFilterANDsWithDuration(t *testing.T) {
	results := sinceFixture()
	results[1].HasUncommittedChanges = true // recent-no-dep now "needs push"

	var buf bytes.Buffer
	Since(&buf, results, SinceOptions{
		Duration:      7 * 24 * time.Hour,
		DurationLabel: "7d",
		UnpushedOnly:  true,
	})
	out := buf.String()

	if !strings.Contains(out, "recent-no-dep") {
		t.Errorf("expected recent-no-dep (needs push) to be listed: %s", out)
	}
	if strings.Contains(out, "recent-with-dep") {
		t.Errorf("recent-with-dep has no pending changes, should be excluded: %s", out)
	}
}
