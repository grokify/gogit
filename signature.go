package gogit

import (
	"context"
	"fmt"
	"strings"
)

// applySignatures sets Signed on each commit by reading the raw commit
// objects over the same range as the main log query.
func (r *Repo) applySignatures(ctx context.Context, opts LogOptions, commits []Commit) error {
	args := append([]string{"log", "--format=raw", "--no-abbrev-commit"}, logFilterArgs(opts)...)
	args = append(args, logRangeArgs(opts)...)
	out, err := r.git(ctx, args...)
	if err != nil {
		return err
	}
	signed := parseSignedHashes(out)
	for i := range commits {
		v, ok := signed[commits[i].Hash]
		if !ok {
			return fmt.Errorf("gogit: no raw record for commit %s", commits[i].Hash)
		}
		commits[i].Signed = &v
	}
	return nil
}

// parseSignedHashes reads `git log --format=raw` output and returns, for
// every commit in it, whether the commit object carries a signature. A
// signature is a "gpgsig" (or "gpgsig-sha256") header at column 0; commit
// message lines are indented in raw output, so message text that merely
// mentions "gpgsig" or "commit <hash>" cannot be mistaken for a header.
func parseSignedHashes(raw string) map[string]bool {
	signed := make(map[string]bool)
	var hash string
	for _, line := range strings.Split(raw, "\n") {
		switch {
		case strings.HasPrefix(line, "commit "):
			hash, _, _ = strings.Cut(strings.TrimPrefix(line, "commit "), " ")
			signed[hash] = false
		case hash != "" && strings.HasPrefix(line, "gpgsig"):
			signed[hash] = true
		}
	}
	return signed
}

// HasSignature reports whether signature state was collected for c
// (Signed is non-nil).
func (c Commit) HasSignature() bool { return c.Signed != nil }

// IsUnsigned reports whether c is known to be unsigned. A commit whose
// signature state was not collected is not reported as unsigned.
func (c Commit) IsUnsigned() bool { return c.Signed != nil && !*c.Signed }

// CountUnsigned returns how many of commits are known to be unsigned.
func CountUnsigned(commits []Commit) int {
	n := 0
	for _, c := range commits {
		if c.IsUnsigned() {
			n++
		}
	}
	return n
}

// HasSignatureInfo reports whether signature state was collected for any of
// commits.
func HasSignatureInfo(commits []Commit) bool {
	for _, c := range commits {
		if c.HasSignature() {
			return true
		}
	}
	return false
}

// SignatureWarning returns a one-line warning such as "2 of 3 commits are
// not signed", or "" when there is nothing to warn about. It warns only when
// more than one commit is listed and at least one is unsigned: a lone
// commit's state is already visible on its own row.
func SignatureWarning(commits []Commit) string {
	n := CountUnsigned(commits)
	if n == 0 || len(commits) < 2 {
		return ""
	}
	return fmt.Sprintf("%d of %d commits are not signed", n, len(commits))
}

// UnsignedCount returns the number of pending commits known to be unsigned.
func (p PendingResult) UnsignedCount() int { return CountUnsigned(p.Commits) }

// UnsignedWarning returns SignatureWarning for the pending commits.
func (p PendingResult) UnsignedWarning() string { return SignatureWarning(p.Commits) }
