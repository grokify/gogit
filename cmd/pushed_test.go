package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunPushedEndToEnd(t *testing.T) {
	resetFlags(t)
	work := pushableRepo(t) // one pushed commit: "chore: init"
	// Add an unpushed commit; pushed should NOT list it.
	if err := os.WriteFile(filepath.Join(work, "b.txt"), []byte("b\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	gitRun(t, work, "add", "-A")
	gitRun(t, work, "commit", "-q", "-m", "feat: add b")

	out := captureStdout(t, func() {
		if err := runPushed(nil, []string{work}); err != nil {
			t.Fatal(err)
		}
	})
	if !strings.Contains(out, "chore: init") {
		t.Errorf("expected the pushed commit listed: %s", out)
	}
	if strings.Contains(out, "feat: add b") {
		t.Errorf("unpushed commit should not appear in pushed output: %s", out)
	}
	if !strings.Contains(out, "Pushed commits (most recent first, from @{upstream}): 1") {
		t.Errorf("unexpected header: %s", out)
	}
}

func TestRunPushedCountArg(t *testing.T) {
	resetFlags(t)
	work := pushableRepo(t)
	for _, msg := range []string{"feat: two", "feat: three"} {
		f := strings.Fields(msg)[1] + ".txt"
		if err := os.WriteFile(filepath.Join(work, f), []byte("x\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		gitRun(t, work, "add", "-A")
		gitRun(t, work, "commit", "-q", "-m", msg)
	}
	gitRun(t, work, "push", "-q", "origin", "main") // 3 commits pushed

	// count passed as a positional arg, in either order relative to the dir.
	out := captureStdout(t, func() {
		if err := runPushed(nil, []string{"2", work}); err != nil {
			t.Fatal(err)
		}
	})
	if !strings.Contains(out, ": 2\n") {
		t.Errorf("expected the count of 2 in the header: %s", out)
	}
	if strings.Contains(out, "chore: init") {
		t.Errorf("count=2 should exclude the oldest commit: %s", out)
	}
}

func TestRunPushedInvalidCount(t *testing.T) {
	resetFlags(t)
	work := pushableRepo(t)
	if err := runPushed(nil, []string{"0", work}); err == nil {
		t.Fatal("expected an error for a non-positive count")
	}
}

func TestRunPushedNoUpstream(t *testing.T) {
	resetFlags(t)
	root := t.TempDir()
	repo := fixtureRepo(t, root, "repo", "") // one commit, no remote/upstream

	out := captureStdout(t, func() {
		if err := runPushed(nil, []string{repo}); err != nil {
			t.Fatal(err)
		}
	})
	if !strings.Contains(out, "branch has no upstream or remote-tracking branch") {
		t.Errorf("expected the no-push-target header, got: %s", out)
	}
	if strings.Contains(out, "chore: init") {
		t.Errorf("an unpushed local commit must not be reported as pushed: %s", out)
	}
}
