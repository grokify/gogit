package gogit

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCollectCommitStats(t *testing.T) {
	// Use this repo itself as the test subject
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	repo, err := Open(wd)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	ctx := context.Background()
	stats, err := repo.CollectCommitStats(ctx, CommitStatsOptions{
		NoMerges: true,
	})
	if err != nil {
		t.Fatalf("CollectCommitStats: %v", err)
	}

	// Should have some commits
	if stats.TotalStats.Commits == 0 {
		t.Error("expected at least one commit")
	}

	// Should have at least one category (this repo uses conventional commits)
	if len(stats.ByCategory) == 0 {
		t.Error("expected at least one category")
	}

	// Category totals should sum to overall totals
	var sumCommits, sumIns, sumDel int
	for _, cat := range stats.ByCategory {
		sumCommits += cat.Commits
		sumIns += cat.Insertions
		sumDel += cat.Deletions
	}
	if sumCommits != stats.TotalStats.Commits {
		t.Errorf("category commit sum %d != total %d", sumCommits, stats.TotalStats.Commits)
	}
	if sumIns != stats.TotalStats.Insertions {
		t.Errorf("category insertions sum %d != total %d", sumIns, stats.TotalStats.Insertions)
	}
	if sumDel != stats.TotalStats.Deletions {
		t.Errorf("category deletions sum %d != total %d", sumDel, stats.TotalStats.Deletions)
	}
}

func TestCollectCommitStatsDateRange(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	repo, err := Open(wd)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	ctx := context.Background()

	// Query a very narrow time range that should have no commits
	farFuture := time.Date(2099, 1, 1, 0, 0, 0, 0, time.UTC)
	stats, err := repo.CollectCommitStats(ctx, CommitStatsOptions{
		Since: farFuture,
		Until: farFuture.Add(24 * time.Hour),
	})
	if err != nil {
		t.Fatalf("CollectCommitStats: %v", err)
	}

	if stats.TotalStats.Commits != 0 {
		t.Errorf("expected 0 commits in far-future range, got %d", stats.TotalStats.Commits)
	}
}

func TestAggregateCommitStats(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	// Find a second repo if available (parent might be a multi-repo workspace)
	var paths []string
	paths = append(paths, wd)

	// Try to find another repo in the same parent directory
	parent := filepath.Dir(wd)
	entries, _ := os.ReadDir(parent)
	for _, e := range entries {
		if !e.IsDir() || e.Name() == filepath.Base(wd) {
			continue
		}
		candidate := filepath.Join(parent, e.Name())
		if _, err := os.Stat(filepath.Join(candidate, ".git")); err == nil {
			paths = append(paths, candidate)
			break
		}
	}

	ctx := context.Background()
	agg := AggregateCommitStats(ctx, paths, CommitStatsOptions{NoMerges: true}, 0)

	// Should have processed at least the current repo
	if len(agg.ByRepo) == 0 {
		t.Error("expected at least one repo result")
	}

	// No errors expected for valid repos
	if len(agg.Errors) > 0 {
		t.Errorf("unexpected errors: %v", agg.Errors)
	}

	// Total should equal sum of per-repo totals
	var sumCommits int
	for _, r := range agg.ByRepo {
		sumCommits += r.TotalStats.Commits
	}
	if sumCommits != agg.TotalStats.Commits {
		t.Errorf("repo sum %d != aggregated total %d", sumCommits, agg.TotalStats.Commits)
	}
}

func TestCategoryPercentages(t *testing.T) {
	agg := &MultiRepoCommitStats{
		TotalStats: CategoryStats{Commits: 100},
		ByCategory: map[string]CategoryStats{
			"feat":  {Category: "feat", Commits: 40},
			"fix":   {Category: "fix", Commits: 30},
			"chore": {Category: "chore", Commits: 20},
			"docs":  {Category: "docs", Commits: 10},
		},
	}

	pcts := agg.CategoryPercentages()

	if pcts["feat"] != 40.0 {
		t.Errorf("feat: expected 40%%, got %.1f%%", pcts["feat"])
	}
	if pcts["fix"] != 30.0 {
		t.Errorf("fix: expected 30%%, got %.1f%%", pcts["fix"])
	}
}

func TestCategoryBreakdown(t *testing.T) {
	agg := &MultiRepoCommitStats{
		ByCategory: map[string]CategoryStats{
			"docs":  {Category: "docs", Commits: 10},
			"feat":  {Category: "feat", Commits: 40},
			"fix":   {Category: "fix", Commits: 30},
			"chore": {Category: "chore", Commits: 20},
		},
	}

	breakdown := agg.CategoryBreakdown()

	if len(breakdown) != 4 {
		t.Fatalf("expected 4 categories, got %d", len(breakdown))
	}

	// Should be sorted descending by commit count
	if breakdown[0].Category != "feat" {
		t.Errorf("first category should be feat, got %s", breakdown[0].Category)
	}
	if breakdown[1].Category != "fix" {
		t.Errorf("second category should be fix, got %s", breakdown[1].Category)
	}
}

func TestNetAdditions(t *testing.T) {
	s := CategoryStats{Insertions: 100, Deletions: 40}
	if s.NetAdditions() != 60 {
		t.Errorf("expected 60 net additions, got %d", s.NetAdditions())
	}
}
