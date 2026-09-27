package gogit

import (
	"context"
	"strings"
)

// LsFiles returns the repository's tracked files (git ls-files). When
// includeUntracked is true, untracked files not covered by .gitignore are
// included as well (git ls-files -o --exclude-standard), so callers can scan
// brand-new files before they are staged.
func (r *Repo) LsFiles(ctx context.Context, includeUntracked bool) ([]string, error) {
	tracked, err := r.git(ctx, "ls-files", "-z")
	if err != nil {
		return nil, err
	}
	files := splitNUL(tracked)
	if includeUntracked {
		others, err := r.git(ctx, "ls-files", "-z", "--others", "--exclude-standard")
		if err != nil {
			return nil, err
		}
		files = append(files, splitNUL(others)...)
	}
	return files, nil
}

// StagedFiles returns files with staged additions, copies, or modifications
// (git diff --cached), i.e. the paths whose staged content a pre-commit
// check should inspect.
func (r *Repo) StagedFiles(ctx context.Context) ([]string, error) {
	out, err := r.git(ctx, "diff", "--cached", "--name-only", "-z", "--diff-filter=ACM")
	if err != nil {
		return nil, err
	}
	return splitNUL(out), nil
}

// ShowContent returns the content of a git object spec via git show, e.g.
// ":path" for the staged (index) version of a file or "<rev>:path" for a
// file at a revision.
func (r *Repo) ShowContent(ctx context.Context, spec string) (string, error) {
	return r.git(ctx, "show", spec)
}

// LsTree returns the paths of all files present at a revision (git ls-tree
// -r), i.e. the full file list of that commit's tree.
func (r *Repo) LsTree(ctx context.Context, rev string) ([]string, error) {
	out, err := r.git(ctx, "ls-tree", "-r", "--name-only", "-z", rev)
	if err != nil {
		return nil, err
	}
	return splitNUL(out), nil
}

// LastCommitTouching returns the hash of the most recent commit at or before
// at (a revision, e.g. "HEAD") that modified path, or "" if no commit
// reachable from at ever touched it (e.g. the path doesn't exist there).
func (r *Repo) LastCommitTouching(ctx context.Context, at, path string) (string, error) {
	out, err := r.git(ctx, "log", "-1", "--format=%H", at, "--", path)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// splitNUL splits NUL-delimited git output into non-empty entries.
func splitNUL(s string) []string {
	var out []string
	for _, part := range strings.Split(s, "\x00") {
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}
