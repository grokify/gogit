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

// AIStats holds AI-assisted commit statistics.
type AIStats struct {
	TotalCommits    int                   `json:"totalCommits"`
	AIAssistedCount int                   `json:"aiAssistedCount"`
	AIAssistedPct   float64               `json:"aiAssistedPct"`
	ByTool          map[string]ToolStats  `json:"byTool,omitempty"`
	ByModel         map[string]ModelStats `json:"byModel,omitempty"`
}

// ToolStats holds per-tool commit counts.
type ToolStats struct {
	Tool       string `json:"tool"`
	Commits    int    `json:"commits"`
	Insertions int    `json:"insertions"`
	Deletions  int    `json:"deletions"`
}

// ModelStats holds per-model commit counts (tool + model).
type ModelStats struct {
	Tool       string `json:"tool"`
	Model      string `json:"model"`
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
	AIStats    AIStats                  `json:"aiStats"`
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
		AIStats: AIStats{
			ByTool:  make(map[string]ToolStats),
			ByModel: make(map[string]ModelStats),
		},
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

		// AI co-author tracking
		ais := c.AICoAuthors()
		if len(ais) > 0 {
			stats.AIStats.AIAssistedCount++
			for _, ai := range ais {
				// Per-tool
				ts := stats.AIStats.ByTool[ai.Tool]
				ts.Tool = ai.Tool
				ts.Commits++
				ts.Insertions += c.Insertions
				ts.Deletions += c.Deletions
				stats.AIStats.ByTool[ai.Tool] = ts

				// Per-model (tool + model key)
				if ai.Model != "" {
					modelKey := ai.Tool + "/" + ai.Model
					ms := stats.AIStats.ByModel[modelKey]
					ms.Tool = ai.Tool
					ms.Model = ai.Model
					ms.Commits++
					ms.Insertions += c.Insertions
					ms.Deletions += c.Deletions
					stats.AIStats.ByModel[modelKey] = ms
				}
			}
		}
	}
	stats.TotalStats.Category = "total"
	stats.AIStats.TotalCommits = stats.TotalStats.Commits
	if stats.TotalStats.Commits > 0 {
		stats.AIStats.AIAssistedPct = float64(stats.AIStats.AIAssistedCount) / float64(stats.TotalStats.Commits) * 100
	}

	return stats, nil
}

// MultiRepoCommitStats is the aggregated commit/LOC breakdown across multiple repositories.
type MultiRepoCommitStats struct {
	Since      time.Time                `json:"since"`
	Until      time.Time                `json:"until"`
	TotalStats CategoryStats            `json:"total"`
	ByCategory map[string]CategoryStats `json:"byCategory"`
	AIStats    AIStats                  `json:"aiStats"`
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
		AIStats: AIStats{
			ByTool:  make(map[string]ToolStats),
			ByModel: make(map[string]ModelStats),
		},
		ByRepo: make([]RepoCommitStats, 0, len(results)),
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

		// Roll up AI stats
		agg.AIStats.AIAssistedCount += r.Value.AIStats.AIAssistedCount
		for tool, ts := range r.Value.AIStats.ByTool {
			existing := agg.AIStats.ByTool[tool]
			existing.Tool = tool
			existing.Commits += ts.Commits
			existing.Insertions += ts.Insertions
			existing.Deletions += ts.Deletions
			agg.AIStats.ByTool[tool] = existing
		}
		for key, ms := range r.Value.AIStats.ByModel {
			existing := agg.AIStats.ByModel[key]
			existing.Tool = ms.Tool
			existing.Model = ms.Model
			existing.Commits += ms.Commits
			existing.Insertions += ms.Insertions
			existing.Deletions += ms.Deletions
			agg.AIStats.ByModel[key] = existing
		}
	}
	agg.TotalStats.Category = "total"
	agg.AIStats.TotalCommits = agg.TotalStats.Commits
	if agg.TotalStats.Commits > 0 {
		agg.AIStats.AIAssistedPct = float64(agg.AIStats.AIAssistedCount) / float64(agg.TotalStats.Commits) * 100
	}

	return agg
}
