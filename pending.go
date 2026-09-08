package gogit

import (
	"context"
	"strings"
)

// HasUpstream reports whether the repository's current branch has an
// upstream (remote-tracking) branch configured that resolves to a commit.
// A detached HEAD, a branch with no upstream, or a configured-but-missing
// upstream ref all return (false, nil) rather than an error.
func (r *Repo) HasUpstream(ctx context.Context) (bool, error) {
	_, err := r.git(ctx, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{upstream}")
	if err != nil {
		msg := strings.ToLower(err.Error())
		if strings.Contains(msg, "no upstream configured") ||
			strings.Contains(msg, "does not point to a branch") ||
			strings.Contains(msg, "unknown revision") {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// PendingResult is the outcome of a PendingCommits query.
type PendingResult struct {
	// Commits are the pending commits, oldest first.
	Commits []Commit
	// Baseline is the ref the commits were computed as being ahead of: the
	// configured upstream ("@{upstream}"), a remote-tracking branch (e.g.
	// "origin/main"), or an explicit since-commit hash. It is empty when the
	// branch has no push baseline at all — never pushed, or no upstream — in
	// which case every commit reachable from HEAD is pending.
	Baseline string
}

// PendingCommits returns commits that exist locally but have not yet been
// pushed, oldest first, along with the baseline they were computed against.
//
// By default the baseline is the branch's push target: its configured
// upstream, or failing that the matching remote-tracking branch (e.g.
// origin/main). When the branch has no such baseline — it was never pushed,
// or has no upstream configured — every commit reachable from HEAD is
// treated as pending and Baseline is empty. This mirrors gitscan's scan
// semantics, where a real branch with no upstream counts as having unpushed
// work. A detached HEAD has no branch to push and yields no baseline.
//
// If sinceCommit is non-empty it overrides the baseline entirely
// ("sinceCommit..HEAD"), regardless of any upstream.
func (r *Repo) PendingCommits(ctx context.Context, sinceCommit string) (PendingResult, error) {
	base := sinceCommit
	if base == "" {
		resolved, err := r.pushBaseline(ctx)
		if err != nil {
			return PendingResult{}, err
		}
		base = resolved
	}

	commits, err := r.Log(ctx, LogOptions{SinceCommit: base, Reverse: true})
	if err != nil {
		return PendingResult{}, err
	}
	return PendingResult{Commits: commits, Baseline: base}, nil
}

// PushedResult is the outcome of a PushedCommits query.
type PushedResult struct {
	// Commits are the pushed commits, most recent first.
	Commits []Commit
	// Baseline is the ref the commits were read from: the configured
	// upstream ("@{upstream}") or the matching remote-tracking branch (e.g.
	// "origin/main"). It is empty when the branch has no push target, in
	// which case nothing has been pushed and Commits is empty.
	Baseline string
}

// PushedCommits returns up to limit commits that have already been pushed on
// the current branch — those reachable from its push target (the configured
// upstream, or failing that the matching remote-tracking branch such as
// origin/main) — most recent first. A limit of zero or less returns all
// pushed commits.
//
// When the branch has no push target (never pushed, no upstream), nothing is
// considered pushed: Commits is empty and Baseline is "". This is the mirror
// image of PendingCommits, which reports every commit as pending in the same
// situation.
func (r *Repo) PushedCommits(ctx context.Context, limit int) (PushedResult, error) {
	base, err := r.pushBaseline(ctx)
	if err != nil {
		return PushedResult{}, err
	}
	if base == "" {
		return PushedResult{}, nil
	}

	commits, err := r.Log(ctx, LogOptions{Rev: base, MaxCount: max(0, limit)})
	if err != nil {
		return PushedResult{}, err
	}
	return PushedResult{Commits: commits, Baseline: base}, nil
}

// pushBaseline returns the ref representing what the current branch has
// already been pushed to, for computing unpushed ("pending") commits. It
// prefers the configured upstream (@{upstream}); failing that, the
// remote-tracking branch matching the current branch (e.g. origin/main). It
// returns "" when neither resolves to a commit — the branch was never
// pushed, so every commit is pending — and likewise for a detached HEAD,
// which has no branch to push.
func (r *Repo) pushBaseline(ctx context.Context) (string, error) {
	if r.refExists(ctx, "@{upstream}") {
		return "@{upstream}", nil
	}
	// An unborn HEAD (freshly init'd repo, no commits) has no branch tip and
	// nothing to push; Branch would error on it, so bail out early.
	if !r.refExists(ctx, "HEAD") {
		return "", nil
	}
	branch, err := r.Branch(ctx)
	if err != nil {
		return "", err
	}
	if branch == "HEAD" {
		return "", nil // detached HEAD: no branch to track
	}
	tracking := r.branchRemote(ctx, branch) + "/" + branch
	if r.refExists(ctx, tracking) {
		return tracking, nil
	}
	return "", nil
}

// refExists reports whether rev resolves to a commit object.
func (r *Repo) refExists(ctx context.Context, rev string) bool {
	_, err := r.git(ctx, "rev-parse", "--verify", "--quiet", rev+"^{commit}")
	return err == nil
}

// branchRemote returns the remote configured for branch
// (branch.<name>.remote), defaulting to "origin". A non-zero exit from
// `git config --get` is the normal "key unset" case, not a failure, so it
// falls back to "origin" rather than surfacing an error.
func (r *Repo) branchRemote(ctx context.Context, branch string) string {
	out, err := r.git(ctx, "config", "--get", "branch."+branch+".remote")
	if err != nil {
		return "origin"
	}
	if remote := strings.TrimSpace(out); remote != "" {
		return remote
	}
	return "origin"
}
