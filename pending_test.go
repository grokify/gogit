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
	commits, err := r.PendingCommits(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	if len(commits) != 2 {
		t.Fatalf("expected 2 pending commits, got %d", len(commits))
	}
	// Oldest first.
	if commits[0].Subject != "feat: add b" || commits[1].Subject != "feat: add c" {
		t.Errorf("unexpected order: %q, %q", commits[0].Subject, commits[1].Subject)
	}
}

func TestPendingCommitsNoUpstreamRequiresHash(t *testing.T) {
	dir := t.TempDir()
	initRepo(t, dir)
	commitFile(t, dir, "a.txt", "hello\n", "chore: init")
	r, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.PendingCommits(context.Background(), ""); err == nil {
		t.Fatal("expected error when no upstream and no sinceCommit given")
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
	commits, err := r.PendingCommits(context.Background(), firstHash)
	if err != nil {
		t.Fatal(err)
	}
	if len(commits) != 2 {
		t.Fatalf("expected 2 commits after %s, got %d", firstHash, len(commits))
	}
	if commits[0].Subject != "feat: add b" || commits[1].Subject != "feat: add c" {
		t.Errorf("unexpected order: %q, %q", commits[0].Subject, commits[1].Subject)
	}
}
