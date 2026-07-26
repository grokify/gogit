package gogit

import (
	"context"
	"sort"
	"time"
)

// CategoryStats holds commit and LOC counts for one conventional-commit category.
type CategoryStats struct {
	Category   string `json:"category"`
	Commits    int    `json:"commits"`
	Insertions int    `json:"insertions"`
	Deletions  int    `json:"deletions"`
}

// NetAdditions returns insertions minus deletions.
func (s CategoryStats) NetAdditions() int {
	return s.Insertions - s.Deletions
}

// RepoCommitStats is the commit/LOC breakdown for a single repository over a time range.
type RepoCommitStats struct {
	Path       string                   `json:"path"`
	Since      time.Time                `json:"since"`
	Until      time.Time                `json:"until"`
	TotalStats CategoryStats            `json:"total"`
	ByCategory map[string]CategoryStats `json:"byCategory"`
}

// CommitStatsOptions configures commit statistics collection.
type CommitStatsOptions struct {
	// Since and Until bound the commit date range (inclusive).
	Since time.Time
	Until time.Time
	// NoMerges excludes merge commits from statistics.
	NoMerges bool
	// Author filters commits by author (git regex).
	Author string
}

// CollectCommitStats aggregates commit counts and LOC by conventional-commit
// category for a single repository over the given time range. Non-conventional
// commits are grouped under "uncategorized". Returns an error if the repo
// cannot be read; an empty repo returns zero stats, not an error.
func (r *Repo) CollectCommitStats(ctx context.Context, opts CommitStatsOptions) (*RepoCommitStats, error) {
	commits, err := r.Log(ctx, LogOptions{
		Since:        opts.Since,
		Until:        opts.Until,
		NoMerges:     opts.NoMerges,
		Author:       opts.Author,
		IncludeStats: true,
		Reverse:      false,
	})
	if err != nil {
		return nil, err
	}

	stats := &RepoCommitStats{
		Path:       r.path,
		Since:      opts.Since,
		Until:      opts.Until,
		ByCategory: make(map[string]CategoryStats),
	}

	for _, c := range commits {
		category := "uncategorized"
		if cc := c.ParseConventional(); cc != nil {
			category = cc.Type
		}

		cat := stats.ByCategory[category]
		cat.Category = category
		cat.Commits++
		cat.Insertions += c.Insertions
		cat.Deletions += c.Deletions
		stats.ByCategory[category] = cat

		stats.TotalStats.Commits++
		stats.TotalStats.Insertions += c.Insertions
		stats.TotalStats.Deletions += c.Deletions
	}
	stats.TotalStats.Category = "total"

	return stats, nil
}

// MultiRepoCommitStats is the aggregated commit/LOC breakdown across multiple repositories.
type MultiRepoCommitStats struct {
	Since      time.Time                `json:"since"`
	Until      time.Time                `json:"until"`
	TotalStats CategoryStats            `json:"total"`
	ByCategory map[string]CategoryStats `json:"byCategory"`
	ByRepo     []RepoCommitStats        `json:"byRepo"`
	Errors     []RepoError              `json:"errors,omitempty"`
}

// RepoError records a repository that failed during collection.
type RepoError struct {
	Path  string `json:"path"`
	Error string `json:"error"`
}

// CategoryBreakdown returns categories sorted by commit count (descending).
func (m *MultiRepoCommitStats) CategoryBreakdown() []CategoryStats {
	cats := make([]CategoryStats, 0, len(m.ByCategory))
	for _, s := range m.ByCategory {
		cats = append(cats, s)
	}
	sort.Slice(cats, func(i, j int) bool {
		return cats[i].Commits > cats[j].Commits
	})
	return cats
}

// CategoryPercentages returns categories with their percentage of total commits.
func (m *MultiRepoCommitStats) CategoryPercentages() map[string]float64 {
	pcts := make(map[string]float64, len(m.ByCategory))
	if m.TotalStats.Commits == 0 {
		return pcts
	}
	for cat, s := range m.ByCategory {
		pcts[cat] = float64(s.Commits) / float64(m.TotalStats.Commits) * 100
	}
	return pcts
}

// AggregateCommitStats collects commit statistics across multiple repositories
// in parallel. Repositories that fail are recorded in Errors but don't stop
// the aggregation. Workers controls concurrency; 0 defaults to GOMAXPROCS.
func AggregateCommitStats(ctx context.Context, paths []string, opts CommitStatsOptions, workers int) *MultiRepoCommitStats {
	results := RunAll(ctx, paths, func(ctx context.Context, repo *Repo) (*RepoCommitStats, error) {
		return repo.CollectCommitStats(ctx, opts)
	}, workers)

	agg := &MultiRepoCommitStats{
		Since:      opts.Since,
		Until:      opts.Until,
		ByCategory: make(map[string]CategoryStats),
		ByRepo:     make([]RepoCommitStats, 0, len(results)),
	}

	for _, r := range results {
		if r.Err != nil {
			agg.Errors = append(agg.Errors, RepoError{Path: r.Path, Error: r.Err.Error()})
			continue
		}
		if r.Value == nil {
			continue
		}

		agg.ByRepo = append(agg.ByRepo, *r.Value)

		// Roll up totals
		agg.TotalStats.Commits += r.Value.TotalStats.Commits
		agg.TotalStats.Insertions += r.Value.TotalStats.Insertions
		agg.TotalStats.Deletions += r.Value.TotalStats.Deletions

		// Roll up by category
		for cat, s := range r.Value.ByCategory {
			existing := agg.ByCategory[cat]
			existing.Category = cat
			existing.Commits += s.Commits
			existing.Insertions += s.Insertions
			existing.Deletions += s.Deletions
			agg.ByCategory[cat] = existing
		}
	}
	agg.TotalStats.Category = "total"

	return agg
}
