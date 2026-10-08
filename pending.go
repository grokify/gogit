package gogit

import (
	"context"
	"sort"
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
	// "origin/main"), the remote's default branch when this branch has never
	// been pushed under its own name (e.g. a feature branch forked from
	// "origin/main"), or an explicit since-commit hash. It is empty only
	// when none of these resolve — the repository has no remote, or nothing
	// has ever been pushed to it — in which case every commit reachable from
	// HEAD is pending.
	Baseline string
	// Branch is the repository's current branch at query time (or "HEAD"
	// when detached), so a caller reporting "no upstream configured" can
	// also say which branch that refers to — easy to misread against the
	// wrong checkout otherwise.
	Branch string
	// Files is the merged, de-duplicated, sorted list of paths touched by the
	// pending commits, relative to the repository root. It is populated only
	// by PendingFiles. A path appears once however many commits changed it,
	// and includes paths that were later deleted or renamed away, because
	// those commits still carry them.
	Files []string
}

// PendingCommits returns commits that exist locally but have not yet been
// pushed, oldest first, along with the baseline they were computed against.
//
// By default the baseline is the branch's push target: its configured
// upstream, or failing that the matching remote-tracking branch (e.g.
// origin/main). When the branch has no such baseline — most commonly a
// freshly created feature branch that has never been pushed under its own
// name — it falls back to the remote's default branch (e.g. origin/main),
// so only commits unique to this branch are reported as pending rather than
// its parent branch's entire history. Only when that also fails to resolve
// — no remote configured, or nothing has ever been pushed to it — is every
// commit reachable from HEAD treated as pending, with Baseline empty. A
// detached HEAD has no branch to compare and yields no baseline.
//
// If sinceCommit is non-empty it overrides the baseline entirely
// ("sinceCommit..HEAD"), regardless of any upstream or default branch.
func (r *Repo) PendingCommits(ctx context.Context, sinceCommit string) (PendingResult, error) {
	base := sinceCommit
	if base == "" {
		resolved, err := r.pendingBaseline(ctx)
		if err != nil {
			return PendingResult{}, err
		}
		base = resolved
	}

	commits, err := r.Log(ctx, LogOptions{SinceCommit: base, Reverse: true})
	if err != nil {
		return PendingResult{}, err
	}
	branch, err := r.currentBranchOrEmpty(ctx)
	if err != nil {
		return PendingResult{}, err
	}
	return PendingResult{Commits: commits, Baseline: base, Branch: branch}, nil
}

// PendingFiles is PendingCommits plus the merged list of file paths touched
// by those commits, in PendingResult.Files. The baseline is the one
// PendingCommits resolves, so the commits and the files always describe the
// same range. With no pending commits, Files is empty.
//
// Renames are reported as a deletion of the old path and an addition of the
// new one, so both paths appear. Merge commits contribute nothing of their
// own; the commits they merge are in the range and are counted directly.
func (r *Repo) PendingFiles(ctx context.Context, sinceCommit string) (PendingResult, error) {
	res, err := r.PendingCommits(ctx, sinceCommit)
	if err != nil {
		return PendingResult{}, err
	}
	if len(res.Commits) == 0 {
		return res, nil
	}
	args := []string{"log", "--name-only", "--no-renames", "--format=", "-z"}
	if res.Baseline != "" {
		args = append(args, res.Baseline+"..HEAD")
	}
	out, err := r.git(ctx, args...)
	if err != nil {
		return PendingResult{}, err
	}
	res.Files = uniqueSorted(splitNUL(out))
	return res, nil
}

// uniqueSorted returns the distinct values of in, sorted.
func uniqueSorted(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, v := range in {
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	sort.Strings(out)
	return out
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
	// Branch is the repository's current branch at query time (or "HEAD"
	// when detached).
	Branch string
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
	branch, err := r.currentBranchOrEmpty(ctx)
	if err != nil {
		return PushedResult{}, err
	}
	base, err := r.pushBaseline(ctx)
	if err != nil {
		return PushedResult{}, err
	}
	if base == "" {
		return PushedResult{Branch: branch}, nil
	}

	commits, err := r.Log(ctx, LogOptions{Rev: base, MaxCount: max(0, limit)})
	if err != nil {
		return PushedResult{}, err
	}
	return PushedResult{Commits: commits, Baseline: base, Branch: branch}, nil
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

// pendingBaseline resolves the baseline for PendingCommits. It tries the
// branch's own push target first (pushBaseline: the configured upstream, or
// a same-named remote-tracking branch). Failing that — a branch that has
// never been pushed under its own name — it falls back to the remote's
// default branch (e.g. origin/main). Git's two-dot log range
// ("origin/main..HEAD") already excludes every commit reachable from
// origin/main by ancestry, which is equivalent to computing the merge-base
// of the two branches and listing commits after it, without an extra `git
// merge-base` call.
func (r *Repo) pendingBaseline(ctx context.Context) (string, error) {
	base, err := r.pushBaseline(ctx)
	if err != nil || base != "" {
		return base, err
	}

	branch, err := r.currentBranchOrEmpty(ctx)
	if err != nil {
		return "", err
	}
	if branch == "" || branch == "HEAD" {
		return "", nil // unborn or detached HEAD: no branch to compare
	}

	remote := r.branchRemote(ctx, branch)
	return r.remoteDefaultBranch(ctx, remote), nil
}

// remoteDefaultBranch returns remote's default branch ref (e.g.
// "origin/main"), preferring the remote's recorded HEAD
// (refs/remotes/<remote>/HEAD, set by a full `git clone` or `git remote
// set-head`) and falling back to "main" then "master" when that is not
// configured locally — as with a remote added by `git remote add` followed
// by `git push -u`, which does not set it. Returns "" when none resolve to
// a commit (e.g. the remote has no branches at all).
func (r *Repo) remoteDefaultBranch(ctx context.Context, remote string) string {
	if out, err := r.git(ctx, "symbolic-ref", "--short", "-q", "refs/remotes/"+remote+"/HEAD"); err == nil {
		if name := strings.TrimSpace(out); name != "" && r.refExists(ctx, name) {
			return name
		}
	}
	for _, candidate := range []string{"main", "master"} {
		ref := remote + "/" + candidate
		if r.refExists(ctx, ref) {
			return ref
		}
	}
	return ""
}

// currentBranchOrEmpty returns the current branch (or "HEAD" when detached),
// or "" for an unborn HEAD (freshly init'd repo, no commits yet), which
// otherwise makes Branch return an error.
func (r *Repo) currentBranchOrEmpty(ctx context.Context) (string, error) {
	if !r.refExists(ctx, "HEAD") {
		return "", nil
	}
	return r.Branch(ctx)
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
