package report

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/grokify/gogit"
)

// git runs git in dir, isolated from the developer's global/system config
// (commit signing, hooks, templates).
func git(t *testing.T, dir string, args ...string) {
	t.Helper()
	//nolint:gosec // G204: test helper; dir is t.TempDir(), args are literals
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
}

// repoWithPending returns a repo with an upstream and n unsigned unpushed
// commits.
func repoWithPending(t *testing.T, n int) string {
	t.Helper()
	root := t.TempDir()
	origin := filepath.Join(root, "origin.git")
	git(t, root, "init", "-q", "--bare", "-b", "main", origin)
	work := filepath.Join(root, "work")
	if err := os.MkdirAll(work, 0o750); err != nil {
		t.Fatal(err)
	}
	git(t, work, "init", "-q", "-b", "main")
	git(t, work, "config", "user.name", "Test User")
	git(t, work, "config", "user.email", "test@example.com")
	commit := func(name string) {
		if err := os.WriteFile(filepath.Join(work, name), []byte(name+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		git(t, work, "add", "-A")
		git(t, work, "commit", "-q", "-m", "feat: add "+name)
	}
	commit("base.txt")
	git(t, work, "remote", "add", "origin", origin)
	git(t, work, "push", "-q", "-u", "origin", "main")
	for i := range n {
		commit(string(rune('a'+i)) + ".txt")
	}
	return work
}

func TestPendingReportsSignatureAndWarning(t *testing.T) {
	repo := repoWithPending(t, 2)
	reports, err := Pending(context.Background(), []string{repo}, PendingOptions{TZ: "utc", Files: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(reports) != 1 {
		t.Fatalf("expected 1 report, got %d", len(reports))
	}
	rep := reports[0]
	if rep.Mode != "unpushed" || rep.Ref != "@{upstream}" || len(rep.Commits) != 2 {
		t.Fatalf("unexpected report: %+v", rep)
	}
	if len(rep.Files) != 2 {
		t.Errorf("expected 2 files, got %v", rep.Files)
	}
	if got := gogit.SignatureWarning(rep.Commits); got != "2 of 2 commits are not signed" {
		t.Errorf("warning = %q", got)
	}

	env := BuildEnvelope(reports)
	if env.Summary.CommitsTotal != 2 || env.Repos[0].Unsigned != 2 {
		t.Errorf("unexpected envelope: %+v", env)
	}
	for _, row := range env.Repos[0].Commits {
		if row.Signed == nil || *row.Signed {
			t.Errorf("row %s: expected Signed=false", row.Hash)
		}
	}
}

func TestPendingFleetReportsBrokenRepoAsError(t *testing.T) {
	repo := repoWithPending(t, 1)
	notRepo := t.TempDir()
	reports, err := Pending(context.Background(), []string{repo, notRepo}, PendingOptions{})
	if err != nil {
		t.Fatalf("a broken repo must not fail the sweep: %v", err)
	}
	if len(reports) != 2 || reports[0].Err != "" || reports[1].Err == "" {
		t.Errorf("unexpected reports: %+v", reports)
	}
}

func TestPendingInvalidOptions(t *testing.T) {
	repo := repoWithPending(t, 1)
	if _, err := Pending(context.Background(), []string{repo}, PendingOptions{TZ: "mars"}); err == nil {
		t.Error("expected error for invalid tz")
	}
	_, err := Pending(context.Background(), []string{repo, repo}, PendingOptions{SinceCommit: "HEAD"})
	if err == nil || !strings.Contains(err.Error(), "single repository") {
		t.Errorf("expected single-repository error, got %v", err)
	}
}

func TestPendingMode(t *testing.T) {
	tests := []struct{ since, base, want string }{
		{"abc", "origin/main", "since-commit"},
		{"", "", "unpushed-all"},
		{"", "@{upstream}", "unpushed"},
	}
	for _, tt := range tests {
		if got := PendingMode(tt.since, tt.base); got != tt.want {
			t.Errorf("PendingMode(%q, %q) = %q, want %q", tt.since, tt.base, got, tt.want)
		}
	}
}

func TestPushedReport(t *testing.T) {
	repo := repoWithPending(t, 1)
	rep, err := Pushed(context.Background(), repo, 5, "")
	if err != nil {
		t.Fatal(err)
	}
	if rep.Mode != "pushed" || len(rep.Commits) != 1 || !rep.Commits[0].IsUnsigned() {
		t.Errorf("unexpected pushed report: %+v", rep)
	}
	if _, err := Pushed(context.Background(), repo, 5, "mars"); err == nil {
		t.Error("expected error for invalid tz")
	}
}
