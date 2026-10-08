package gogit

import (
	"context"
	"os"
	"path/filepath"
	"slices"
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
	if res.Branch != "main" {
		t.Errorf("expected branch %q, got %q", "main", res.Branch)
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
	if res.Branch != "main" {
		t.Errorf("expected branch %q even with no push target, got %q", "main", res.Branch)
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

// A freshly created feature branch — no upstream configured, and no
// origin/<branch> exists because it was never pushed under its own name —
// falls back to the remote's default branch, reporting only the commits
// unique to the feature branch rather than every commit reachable from HEAD
// (which would include the whole of main's history).
func TestPendingCommitsFallsBackToRemoteDefaultBranch(t *testing.T) {
	dir, _ := pushableRepo(t)
	run(t, dir, "checkout", "-qb", "feat/x")
	commitFile(t, dir, "b.txt", "one\n", "feat: add b")
	commitFile(t, dir, "c.txt", "two\n", "feat: add c")

	r, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	has, err := r.HasUpstream(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if has {
		t.Fatal("precondition: a freshly checked out branch must have no upstream")
	}

	res, err := r.PendingCommits(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Commits) != 2 {
		t.Fatalf("expected only the 2 commits unique to feat/x, got %d: %+v", len(res.Commits), res.Commits)
	}
	if res.Baseline != "origin/main" {
		t.Errorf("expected baseline origin/main, got %q", res.Baseline)
	}
	if res.Branch != "feat/x" {
		t.Errorf("expected branch %q, got %q", "feat/x", res.Branch)
	}
	if res.Commits[0].Subject != "feat: add b" || res.Commits[1].Subject != "feat: add c" {
		t.Errorf("unexpected order: %q, %q", res.Commits[0].Subject, res.Commits[1].Subject)
	}
}

// When the remote has no branches at all (nothing has ever been pushed),
// the default-branch fallback also fails to resolve, and every commit
// reachable from HEAD remains pending — the pre-existing "never pushed
// anywhere" behavior must not regress.
func TestPendingCommitsNoRemoteDefaultBranchListsAll(t *testing.T) {
	root := t.TempDir()
	origin := filepath.Join(root, "origin.git")
	run(t, root, "init", "-q", "--bare", "-b", "main", origin)

	dir := filepath.Join(root, "work")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	initRepo(t, dir)
	run(t, dir, "remote", "add", "origin", origin)
	commitFile(t, dir, "a.txt", "hello\n", "chore: init")
	commitFile(t, dir, "b.txt", "world\n", "feat: add b")

	r, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	res, err := r.PendingCommits(context.Background(), "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(res.Commits) != 2 {
		t.Fatalf("expected all 2 commits reported as pending, got %d", len(res.Commits))
	}
	if res.Baseline != "" {
		t.Errorf("expected empty baseline when the remote has no branches, got %q", res.Baseline)
	}
}

func TestRemoteDefaultBranchViaSymbolicRef(t *testing.T) {
	dir, _ := pushableRepo(t)
	// git push -u does not set refs/remotes/origin/HEAD; a real clone would.
	// Set it explicitly to exercise the symbolic-ref path.
	run(t, dir, "remote", "set-head", "origin", "main")

	r, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got := r.remoteDefaultBranch(context.Background(), "origin"); got != "origin/main" {
		t.Errorf("remoteDefaultBranch = %q, want %q", got, "origin/main")
	}
}

func TestRemoteDefaultBranchFallsBackToMainHeuristic(t *testing.T) {
	dir, _ := pushableRepo(t) // push -u does not set origin/HEAD
	r, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got := r.remoteDefaultBranch(context.Background(), "origin"); got != "origin/main" {
		t.Errorf("remoteDefaultBranch = %q, want %q", got, "origin/main")
	}
}

func TestRemoteDefaultBranchNoneResolves(t *testing.T) {
	dir := t.TempDir()
	initRepo(t, dir)
	r, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got := r.remoteDefaultBranch(context.Background(), "origin"); got != "" {
		t.Errorf("remoteDefaultBranch = %q, want empty (no remote configured)", got)
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
	if res.Branch != "main" {
		t.Errorf("expected branch %q, got %q", "main", res.Branch)
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

// Files touched by several pending commits are merged into one sorted list,
// with each path once; files from already-pushed commits are excluded.
func TestPendingFilesMergesAndDedupes(t *testing.T) {
	dir, _ := pushableRepo(t) // a.txt is pushed
	commitFile(t, dir, "b.txt", "one\n", "feat: add b")
	commitFile(t, dir, "sub dir with space.txt", "x\n", "feat: add spaced path")
	commitFile(t, dir, "b.txt", "two\n", "fix: change b again")

	r, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	res, err := r.PendingFiles(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Commits) != 3 {
		t.Fatalf("expected 3 pending commits, got %d", len(res.Commits))
	}
	want := []string{"b.txt", "sub dir with space.txt"}
	if !slices.Equal(res.Files, want) {
		t.Fatalf("Files = %q, want %q", res.Files, want)
	}
}

// A path deleted or renamed away within the pending range is still listed,
// along with the new name, because the pending commits carry both.
func TestPendingFilesIncludesDeletedAndRenamed(t *testing.T) {
	dir, _ := pushableRepo(t)
	commitFile(t, dir, "gone.txt", "x\n", "feat: add gone")
	run(t, dir, "rm", "-q", "gone.txt")
	run(t, dir, "commit", "-q", "-m", "chore: remove gone")
	run(t, dir, "mv", "a.txt", "renamed.txt")
	run(t, dir, "commit", "-q", "-m", "refactor: rename a")

	r, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	res, err := r.PendingFiles(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"a.txt", "gone.txt", "renamed.txt"}
	if !slices.Equal(res.Files, want) {
		t.Fatalf("Files = %q, want %q", res.Files, want)
	}
}

func TestPendingFilesNothingPending(t *testing.T) {
	dir, _ := pushableRepo(t)
	r, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	res, err := r.PendingFiles(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Commits) != 0 || len(res.Files) != 0 {
		t.Fatalf("expected no commits or files, got %d commits, files %q", len(res.Commits), res.Files)
	}
}

// With no push target every commit is pending, so every path is listed.
func TestPendingFilesNoUpstreamListsAll(t *testing.T) {
	dir := t.TempDir()
	initRepo(t, dir)
	commitFile(t, dir, "a.txt", "hello\n", "chore: init")
	commitFile(t, dir, "b.txt", "world\n", "feat: add b")
	r, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	res, err := r.PendingFiles(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	if res.Baseline != "" || !slices.Equal(res.Files, []string{"a.txt", "b.txt"}) {
		t.Fatalf("baseline = %q, Files = %q", res.Baseline, res.Files)
	}
}

// --since-commit pins the range, so only later commits' paths are listed.
func TestPendingFilesSinceHash(t *testing.T) {
	dir, first := pushableRepo(t)
	commitFile(t, dir, "b.txt", "one\n", "feat: add b")
	r, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	res, err := r.PendingFiles(context.Background(), first)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(res.Files, []string{"b.txt"}) {
		t.Fatalf("Files = %q, want [b.txt]", res.Files)
	}
}
