package gogit

import (
	"context"
	"fmt"
	"strings"
)

// HasUpstream reports whether the repository's current branch has an
// upstream (remote-tracking) branch configured. A detached HEAD, or a
// branch with no upstream, both return (false, nil) rather than an error.
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

// PendingCommits returns commits that exist locally but have not yet been
// pushed: those reachable from HEAD but not from the current branch's
// upstream.
//
// If sinceCommit is non-empty, it is used as the lower bound instead of the
// upstream (i.e. "sinceCommit..HEAD"), and no upstream is required. Commits
// are returned oldest first.
func (r *Repo) PendingCommits(ctx context.Context, sinceCommit string) ([]Commit, error) {
	ref := sinceCommit
	if ref == "" {
		hasUpstream, err := r.HasUpstream(ctx)
		if err != nil {
			return nil, err
		}
		if !hasUpstream {
			return nil, fmt.Errorf("gogit: no upstream configured for the current branch; pass an explicit commit hash to filter by")
		}
		ref = "@{upstream}"
	}
	return r.Log(ctx, LogOptions{SinceCommit: ref, Reverse: true})
}
