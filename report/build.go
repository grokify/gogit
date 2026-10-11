package report

import (
	"context"
	"fmt"

	"github.com/grokify/gogit"
)

// PendingOptions configures Pending.
type PendingOptions struct {
	// SinceCommit lists commits after this hash instead of unpushed commits.
	// It applies to a single repository only.
	SinceCommit string
	// Files also collects the merged list of paths touched by the commits.
	Files bool
	// TZ is the timestamp timezone: "original" (or ""), "local", or "utc".
	TZ string
	// Progress, when set, is called as repositories finish.
	Progress gogit.ProgressFunc
}

// Pending reports unpushed commits for each repository path, in the order
// given. A repository that cannot be read yields a Report with Err set
// rather than failing the whole sweep. It returns an error only for invalid
// options: an unknown TZ, or SinceCommit with more than one repository.
func Pending(ctx context.Context, repos []string, opts PendingOptions) ([]Report, error) {
	if err := validateTZ(opts.TZ); err != nil {
		return nil, err
	}
	if opts.SinceCommit != "" && len(repos) > 1 {
		return nil, fmt.Errorf("report: SinceCommit applies to a single repository, but %d were given", len(repos))
	}

	results := gogit.RunAllWithProgress(ctx, repos,
		func(ctx context.Context, r *gogit.Repo) (gogit.PendingResult, error) {
			if opts.Files {
				return r.PendingFiles(ctx, opts.SinceCommit)
			}
			return r.PendingCommits(ctx, opts.SinceCommit)
		}, 0, opts.Progress)

	reports := make([]Report, len(results))
	for i, res := range results {
		if res.Err != nil {
			reports[i] = Report{Repo: res.Path, Err: res.Err.Error()}
			continue
		}
		rep, err := PendingReport(res.Path, res.Value, opts.SinceCommit, opts.TZ)
		if err != nil {
			return nil, err
		}
		reports[i] = rep
	}
	return reports, nil
}

// PendingReport converts one repository's PendingResult into a Report,
// applying tz to the commit timestamps.
func PendingReport(path string, res gogit.PendingResult, sinceCommit, tz string) (Report, error) {
	commits, err := ApplyTimezone(res.Commits, tz)
	if err != nil {
		return Report{}, err
	}
	return Report{
		Repo:    path,
		Branch:  res.Branch,
		Mode:    PendingMode(sinceCommit, res.Baseline),
		Ref:     res.Baseline,
		Commits: commits,
		Files:   res.Files,
	}, nil
}

// PendingMode maps a pending query to its report Mode.
func PendingMode(sinceCommit, baseline string) string {
	switch {
	case sinceCommit != "":
		return "since-commit"
	case baseline == "":
		return "unpushed-all"
	default:
		return "unpushed"
	}
}

// Pushed reports up to count already-pushed commits (most recent first) for
// the repository at path.
func Pushed(ctx context.Context, path string, count int, tz string) (Report, error) {
	if err := validateTZ(tz); err != nil {
		return Report{}, err
	}
	repo, err := gogit.Open(path)
	if err != nil {
		return Report{}, err
	}
	res, err := repo.PushedCommits(ctx, count)
	if err != nil {
		return Report{}, err
	}
	return PushedReport(path, res, tz)
}

// PushedReport converts a PushedResult into a Report, applying tz.
func PushedReport(path string, res gogit.PushedResult, tz string) (Report, error) {
	commits, err := ApplyTimezone(res.Commits, tz)
	if err != nil {
		return Report{}, err
	}
	return Report{
		Repo:    path,
		Branch:  res.Branch,
		Mode:    "pushed",
		Ref:     res.Baseline,
		Commits: commits,
	}, nil
}

// validateTZ rejects an unknown timezone mode before any git work is done.
func validateTZ(tz string) error {
	_, err := ApplyTimezone(nil, tz)
	return err
}
