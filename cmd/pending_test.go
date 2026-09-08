package cmd

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// pushableRepo creates a bare "origin" and a working repo with origin
// configured and the initial commit pushed with upstream tracking.
func pushableRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	origin := filepath.Join(root, "origin.git")
	gitRun(t, root, "init", "-q", "--bare", "-b", "main", origin)

	work := filepath.Join(root, "work")
	if err := os.MkdirAll(work, 0o750); err != nil {
		t.Fatal(err)
	}
	gitRun(t, work, "init", "-q", "-b", "main")
	gitRun(t, work, "config", "user.name", "Test User")
	gitRun(t, work, "config", "user.email", "test@example.com")
	if err := os.WriteFile(filepath.Join(work, "a.txt"), []byte("hello\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	gitRun(t, work, "add", "-A")
	gitRun(t, work, "commit", "-q", "-m", "chore: init")
	gitRun(t, work, "remote", "add", "origin", origin)
	gitRun(t, work, "push", "-q", "-u", "origin", "main")
	return work
}

func TestRunPendingNoUpstreamListsAll(t *testing.T) {
	resetFlags(t)
	root := t.TempDir()
	repo := fixtureRepo(t, root, "repo", "") // one commit, no remote/upstream

	out := captureStdout(t, func() {
		if err := runPending(nil, []string{repo}); err != nil {
			t.Fatalf("expected no error for a repo with no upstream, got %v", err)
		}
	})
	if !strings.Contains(out, "no upstream configured; all local commits unpushed") {
		t.Errorf("expected the no-upstream header, got: %s", out)
	}
	if !strings.Contains(out, "chore: init") {
		t.Errorf("expected the local commit listed as pending: %s", out)
	}
}

func TestRunPendingInvalidFormat(t *testing.T) {
	resetFlags(t)
	root := t.TempDir()
	repo := fixtureRepo(t, root, "repo", "")
	pendingSinceCommit = "HEAD" // avoid the upstream check entirely
	pendingFormat = "bogus"

	captureStdout(t, func() {
		if err := runPending(nil, []string{repo}); err == nil {
			t.Fatal("expected an error for an invalid --format value")
		}
	})
}

func TestRunPendingInvalidTZ(t *testing.T) {
	resetFlags(t)
	root := t.TempDir()
	repo := fixtureRepo(t, root, "repo", "")
	pendingSinceCommit = "HEAD" // avoid the upstream check entirely
	pendingTZ = "mars"

	captureStdout(t, func() {
		if err := runPending(nil, []string{repo}); err == nil {
			t.Fatal("expected an error for an invalid --tz value")
		}
	})
}

func TestRunPendingUTCTimezone(t *testing.T) {
	resetFlags(t)
	work := pushableRepo(t)
	if err := os.WriteFile(filepath.Join(work, "b.txt"), []byte("b\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	gitRun(t, work, "add", "-A")
	gitRun(t, work, "commit", "-q", "-m", "feat: add b")

	pendingTZ = "utc"
	out := captureStdout(t, func() {
		if err := runPending(nil, []string{work}); err != nil {
			t.Fatal(err)
		}
	})

	// Inspect the actual RFC3339 timestamps rather than scanning the whole
	// output for stray characters: a "+" or "Z" can legitimately appear in a
	// temp-dir path or commit message. Every rendered timestamp must carry a
	// "Z" offset (exact UTC), never a numeric "+HH:MM"/"-HH:MM" one.
	timestamps := regexp.MustCompile(`\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(Z|[+-]\d{2}:\d{2})`).FindAllString(out, -1)
	if len(timestamps) == 0 {
		t.Fatalf("expected at least one RFC3339 timestamp in output: %s", out)
	}
	for _, ts := range timestamps {
		if !strings.HasSuffix(ts, "Z") {
			t.Errorf("--tz utc should render Z-suffixed timestamps, got %q in: %s", ts, out)
		}
	}
}

func TestRunPendingEndToEnd(t *testing.T) {
	resetFlags(t)
	work := pushableRepo(t)
	if err := os.WriteFile(filepath.Join(work, "b.txt"), []byte("b\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	gitRun(t, work, "add", "-A")
	gitRun(t, work, "commit", "-q", "-m", "feat: add b")

	out := captureStdout(t, func() {
		if err := runPending(nil, []string{work}); err != nil {
			t.Fatal(err)
		}
	})
	if !strings.Contains(out, "feat: add b") {
		t.Errorf("expected the pending commit listed: %s", out)
	}
	if !strings.Contains(out, "Pending commits (not yet pushed to @{upstream}): 1") {
		t.Errorf("unexpected header: %s", out)
	}
}
