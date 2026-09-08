package gogit

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// pushableRepo creates a bare "origin" and a local clone-equivalent repo
// with origin configured and the initial commit pushed with upstream
// tracking (git push -u).
func pushableRepo(t *testing.T) (dir string, firstHash string) {
	t.Helper()
	root := t.TempDir()
	origin := filepath.Join(root, "origin.git")
	run(t, root, "init", "-q", "--bare", "-b", "main", origin)

	dir = filepath.Join(root, "work")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	initRepo(t, dir)
	commitFile(t, dir, "a.txt", "hello\n", "chore: init")
	run(t, dir, "remote", "add", "origin", origin)
	run(t, dir, "push", "-q", "-u", "origin", "main")

	r, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	commits, err := r.Log(context.Background(), LogOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(commits) != 1 {
		t.Fatalf("expected 1 commit after push, got %d", len(commits))
	}
	return dir, commits[0].Hash
}

func TestHasUpstream(t *testing.T) {
	dir, _ := pushableRepo(t)
	r, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	has, err := r.HasUpstream(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !has {
		t.Error("expected upstream configured after push -u")
	}
}

func TestHasUpstreamNone(t *testing.T) {
	dir := t.TempDir()
	initRepo(t, dir)
	commitFile(t, dir, "a.txt", "hello\n", "chore: init")
	r, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	has, err := r.HasUpstream(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if has {
		t.Error("expected no upstream configured")
	}
}

func TestPendingCommitsUpstream(t *testing.T) {
	dir, _ := pushableRepo(t)
	commitFile(t, dir, "b.txt", "one\n", "feat: add b")
	commitFile(t, dir, "c.txt", "two\n", "feat: add c")

	r, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	res, err := r.PendingCommits(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Commits) != 2 {
		t.Fatalf("expected 2 pending commits, got %d", len(res.Commits))
	}
	if res.Baseline != "@{upstream}" {
		t.Errorf("expected baseline @{upstream}, got %q", res.Baseline)
	}
	// Oldest first.
	if res.Commits[0].Subject != "feat: add b" || res.Commits[1].Subject != "feat: add c" {
		t.Errorf("unexpected order: %q, %q", res.Commits[0].Subject, res.Commits[1].Subject)
	}
}

// A branch with no upstream and no remote-tracking ref has never been
// pushed, so every commit is pending and Baseline is empty — no error.
func TestPendingCommitsNoUpstreamListsAll(t *testing.T) {
	dir := t.TempDir()
	initRepo(t, dir)
	commitFile(t, dir, "a.txt", "hello\n", "chore: init")
	commitFile(t, dir, "b.txt", "world\n", "feat: add b")
	r, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	res, err := r.PendingCommits(context.Background(), "")
	if err != nil {
		t.Fatalf("expected no error for a branch with no upstream, got %v", err)
	}
	if len(res.Commits) != 2 {
		t.Fatalf("expected all 2 commits reported as pending, got %d", len(res.Commits))
	}
	if res.Baseline != "" {
		t.Errorf("expected empty baseline when there is no push target, got %q", res.Baseline)
	}
	if res.Commits[0].Subject != "chore: init" || res.Commits[1].Subject != "feat: add b" {
		t.Errorf("unexpected order: %q, %q", res.Commits[0].Subject, res.Commits[1].Subject)
	}
}

// A branch pushed without -u has a remote-tracking ref (origin/main) but no
// configured upstream; PendingCommits should fall back to that ref.
func TestPendingCommitsRemoteTrackingBaseline(t *testing.T) {
	dir, _ := pushableRepo(t)
	// Drop the upstream configuration but keep the origin/main tracking ref.
	run(t, dir, "branch", "--unset-upstream")
	commitFile(t, dir, "b.txt", "one\n", "feat: add b")

	r, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	has, err := r.HasUpstream(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if has {
		t.Fatal("precondition: expected no upstream after --unset-upstream")
	}

	res, err := r.PendingCommits(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Commits) != 1 {
		t.Fatalf("expected 1 pending commit measured against origin/main, got %d", len(res.Commits))
	}
	if res.Baseline != "origin/main" {
		t.Errorf("expected baseline origin/main, got %q", res.Baseline)
	}
	if res.Commits[0].Subject != "feat: add b" {
		t.Errorf("unexpected commit: %q", res.Commits[0].Subject)
	}
}

func TestPendingCommitsEmptyRepo(t *testing.T) {
	dir := t.TempDir()
	initRepo(t, dir)
	r, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	res, err := r.PendingCommits(context.Background(), "")
	if err != nil {
		t.Fatalf("expected no error for an empty repo, got %v", err)
	}
	if len(res.Commits) != 0 {
		t.Errorf("expected no commits in an empty repo, got %d", len(res.Commits))
	}
}

func TestPendingCommitsSinceHash(t *testing.T) {
	dir, firstHash := pushableRepo(t)
	commitFile(t, dir, "b.txt", "one\n", "feat: add b")
	commitFile(t, dir, "c.txt", "two\n", "feat: add c")

	r, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	res, err := r.PendingCommits(context.Background(), firstHash)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Commits) != 2 {
		t.Fatalf("expected 2 commits after %s, got %d", firstHash, len(res.Commits))
	}
	if res.Baseline != firstHash {
		t.Errorf("expected baseline %s, got %q", firstHash, res.Baseline)
	}
	if res.Commits[0].Subject != "feat: add b" || res.Commits[1].Subject != "feat: add c" {
		t.Errorf("unexpected order: %q, %q", res.Commits[0].Subject, res.Commits[1].Subject)
	}
}

func TestPushedCommitsUpstream(t *testing.T) {
	dir, _ := pushableRepo(t)
	// One commit is pushed (chore: init); these two are not.
	commitFile(t, dir, "b.txt", "one\n", "feat: add b")
	commitFile(t, dir, "c.txt", "two\n", "feat: add c")

	r, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	res, err := r.PushedCommits(context.Background(), 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Commits) != 1 {
		t.Fatalf("expected only the 1 pushed commit, got %d", len(res.Commits))
	}
	if res.Baseline != "@{upstream}" {
		t.Errorf("expected baseline @{upstream}, got %q", res.Baseline)
	}
	if res.Commits[0].Subject != "chore: init" {
		t.Errorf("unexpected pushed commit: %q", res.Commits[0].Subject)
	}
}

func TestPushedCommitsLimitAndOrder(t *testing.T) {
	dir, _ := pushableRepo(t)
	// Push two more so three commits are on origin, newest last.
	commitFile(t, dir, "b.txt", "one\n", "feat: add b")
	commitFile(t, dir, "c.txt", "two\n", "feat: add c")
	run(t, dir, "push", "-q", "origin", "main")

	r, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	res, err := r.PushedCommits(context.Background(), 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Commits) != 2 {
		t.Fatalf("expected the limit of 2 pushed commits, got %d", len(res.Commits))
	}
	// Most recent first.
	if res.Commits[0].Subject != "feat: add c" || res.Commits[1].Subject != "feat: add b" {
		t.Errorf("expected newest-first order, got %q, %q", res.Commits[0].Subject, res.Commits[1].Subject)
	}
}

func TestPushedCommitsNoUpstream(t *testing.T) {
	dir := t.TempDir()
	initRepo(t, dir)
	commitFile(t, dir, "a.txt", "hello\n", "chore: init")
	r, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	res, err := r.PushedCommits(context.Background(), 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Commits) != 0 {
		t.Errorf("expected nothing pushed with no upstream, got %d", len(res.Commits))
	}
	if res.Baseline != "" {
		t.Errorf("expected empty baseline with no push target, got %q", res.Baseline)
	}
}
