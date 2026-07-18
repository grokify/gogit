package gogit

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// initRepo creates a git repository with a deterministic identity.
func initRepo(t *testing.T, dir string) {
	t.Helper()
	run(t, dir, "init", "-q", "-b", "main")
	run(t, dir, "config", "user.name", "Test User")
	run(t, dir, "config", "user.email", "test@example.com")
}

func run(t *testing.T, dir string, args ...string) {
	t.Helper()
	//nolint:gosec // G204: test helper; dir is t.TempDir(), args are literals
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	// Isolate fixtures from the developer's global/system git config —
	// commit signing, hooks, and templates must not leak into tests.
	cmd.Env = append(os.Environ(),
		"GIT_CONFIG_GLOBAL=/dev/null",
		"GIT_CONFIG_SYSTEM=/dev/null",
		"GIT_AUTHOR_DATE=2026-07-01T10:00:00Z",
		"GIT_COMMITTER_DATE=2026-07-01T10:00:00Z",
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
}

func commitFile(t *testing.T, dir, name, content, message string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	run(t, dir, "add", name)
	run(t, dir, "commit", "-q", "-m", message)
}

func TestDiscover(t *testing.T) {
	root := t.TempDir()
	for _, rel := range []string{"org/repo1", "org/repo2", "org/notrepo", "deep/a/b/repo3"} {
		if err := os.MkdirAll(filepath.Join(root, rel), 0o750); err != nil {
			t.Fatal(err)
		}
	}
	initRepo(t, filepath.Join(root, "org", "repo1"))
	initRepo(t, filepath.Join(root, "org", "repo2"))
	initRepo(t, filepath.Join(root, "deep", "a", "b", "repo3"))

	repos, err := Discover([]string{root}, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(repos) != 2 { // repo3 is at depth 4, beyond maxDepth 2
		t.Errorf("depth 2: got %d repos (%v), want 2", len(repos), repos)
	}

	repos, err = Discover([]string{root}, 4)
	if err != nil {
		t.Fatal(err)
	}
	if len(repos) != 3 {
		t.Errorf("depth 4: got %d repos, want 3", len(repos))
	}

	// A root that is itself a repo returns itself.
	repos, err = Discover([]string{filepath.Join(root, "org", "repo1")}, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(repos) != 1 {
		t.Errorf("repo root: got %d, want 1", len(repos))
	}

	if _, err := Discover([]string{root}, 0); err == nil {
		t.Error("maxDepth 0 should error")
	}
}

func TestRepoMetadata(t *testing.T) {
	dir := t.TempDir()
	initRepo(t, dir)
	commitFile(t, dir, "a.txt", "hello\n", "chore: init")
	run(t, dir, "remote", "add", "origin", "git@github.com:example/project.git")

	r, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	branch, err := r.Branch(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if branch != "main" {
		t.Errorf("branch: got %q, want main", branch)
	}

	origin, err := r.OriginURL(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if origin != "git@github.com:example/project.git" {
		t.Errorf("origin: got %q", origin)
	}

	if _, err := Open(t.TempDir()); err == nil {
		t.Error("Open on non-repo should error")
	}
}

func TestOriginURLMissingRemote(t *testing.T) {
	dir := t.TempDir()
	initRepo(t, dir)
	r, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	origin, err := r.OriginURL(context.Background())
	if err != nil {
		t.Fatalf("missing remote should not error: %v", err)
	}
	if origin != "" {
		t.Errorf("origin: got %q, want empty", origin)
	}
}

func TestLogTrailersAndStats(t *testing.T) {
	dir := t.TempDir()
	initRepo(t, dir)
	commitFile(t, dir, "a.txt", "one\ntwo\nthree\n", "feat: add feature\n\nBody text here.\n\nCo-authored-by: Claude <noreply@anthropic.com>\nReviewed-by: Jane Doe <jane@example.com>")
	commitFile(t, dir, "b.txt", "x\n", "fix: plain commit")

	r, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	commits, err := r.Log(context.Background(), LogOptions{IncludeStats: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(commits) != 2 {
		t.Fatalf("commits: got %d, want 2", len(commits))
	}

	// Newest first: commits[1] is the feature commit.
	feature := commits[1]
	if feature.Subject != "feat: add feature" {
		t.Errorf("subject: got %q", feature.Subject)
	}
	if feature.Author.Email != "test@example.com" {
		t.Errorf("author email: got %q", feature.Author.Email)
	}
	if want := time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC); !feature.AuthorDate.Equal(want) {
		t.Errorf("author date: got %s", feature.AuthorDate)
	}
	if len(feature.Trailers) != 2 {
		t.Fatalf("trailers: got %d (%+v), want 2", len(feature.Trailers), feature.Trailers)
	}
	coAuthors := feature.CoAuthors()
	if len(coAuthors) != 1 || coAuthors[0].Email != "noreply@anthropic.com" || coAuthors[0].Name != "Claude" {
		t.Errorf("co-authors: got %+v", coAuthors)
	}
	if feature.Insertions != 3 || feature.Deletions != 0 || feature.FilesChanged != 1 {
		t.Errorf("stats: got +%d -%d files=%d, want +3 -0 files=1",
			feature.Insertions, feature.Deletions, feature.FilesChanged)
	}

	plain := commits[0]
	if len(plain.Trailers) != 0 || len(plain.CoAuthors()) != 0 {
		t.Errorf("plain commit should have no trailers: %+v", plain.Trailers)
	}
	if plain.Insertions != 1 {
		t.Errorf("plain stats: got +%d, want +1", plain.Insertions)
	}
}

func TestLogWithoutStats(t *testing.T) {
	dir := t.TempDir()
	initRepo(t, dir)
	commitFile(t, dir, "a.txt", "one\n", "chore: a")
	commitFile(t, dir, "b.txt", "two\n", "chore: b")

	r, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	commits, err := r.Log(context.Background(), LogOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(commits) != 2 {
		t.Fatalf("commits: got %d, want 2", len(commits))
	}
	if commits[0].Insertions != 0 || commits[0].FilesChanged != 0 {
		t.Errorf("stats without --numstat should be zero: %+v", commits[0])
	}
}

func TestLogDateFilter(t *testing.T) {
	dir := t.TempDir()
	initRepo(t, dir)
	commitFile(t, dir, "a.txt", "one\n", "chore: in range")

	r, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	// Window containing the fixed commit date.
	commits, err := r.Log(ctx, LogOptions{
		Since: time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC),
		Until: time.Date(2026, 7, 2, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(commits) != 1 {
		t.Errorf("in-range: got %d commits, want 1", len(commits))
	}

	// Window before the commit.
	commits, err = r.Log(ctx, LogOptions{
		Until: time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(commits) != 0 {
		t.Errorf("out-of-range: got %d commits, want 0", len(commits))
	}
}

func TestLogEmptyRepo(t *testing.T) {
	dir := t.TempDir()
	initRepo(t, dir)
	r, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	commits, err := r.Log(context.Background(), LogOptions{})
	if err != nil {
		t.Fatalf("empty repo should not error: %v", err)
	}
	if len(commits) != 0 {
		t.Errorf("empty repo: got %d commits", len(commits))
	}
}

func TestParseSignature(t *testing.T) {
	cases := []struct {
		in    string
		name  string
		email string
		ok    bool
	}{
		{"Jane Doe <jane@example.com>", "Jane Doe", "jane@example.com", true},
		{"Claude <noreply@anthropic.com>", "Claude", "noreply@anthropic.com", true},
		{"no email at all", "", "", false},
		{"<only@email.com>", "", "only@email.com", true},
	}
	for _, c := range cases {
		sig, ok := parseSignature(c.in)
		if ok != c.ok || sig.Name != c.name || sig.Email != c.email {
			t.Errorf("parseSignature(%q): got %+v/%v, want %s/%s/%v", c.in, sig, ok, c.name, c.email, c.ok)
		}
	}
}
