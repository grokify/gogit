package cmd

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// reportSection strips the leading progress-bar/scan-status noise from
// captured stdout, returning only the text from "Scan complete!" onward
// (where the actual rendered report begins). The progress bar prints each
// directory's name as it's scanned, which would otherwise produce false
// substring matches in assertions that check which repos appear in the
// final report.
func reportSection(out string) string {
	if _, after, found := strings.Cut(out, "Scan complete!"); found {
		return after
	}
	return out
}

// captureStdout redirects os.Stdout for the duration of fn and returns
// everything written to it. Tests must not run in parallel while using
// this helper, since it swaps a process-global.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	defer func() { os.Stdout = orig }()

	fn()

	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}

// gitRun executes git in a fixture directory, isolated from the
// developer's global/system config (signing, hooks, templates).
func gitRun(t *testing.T, dir string, args ...string) {
	t.Helper()
	//nolint:gosec // G204: test helper; dir is t.TempDir(), args are literals
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(),
		"GIT_CONFIG_GLOBAL=/dev/null",
		"GIT_CONFIG_SYSTEM=/dev/null",
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
}

// fixtureRepo creates a repo directory under root with one commit and an
// optional go.mod.
func fixtureRepo(t *testing.T, root, name, gomod string) string {
	t.Helper()
	dir := filepath.Join(root, name)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	gitRun(t, dir, "init", "-q", "-b", "main")
	gitRun(t, dir, "config", "user.name", "Test User")
	gitRun(t, dir, "config", "user.email", "test@example.com")
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("hello\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if gomod != "" {
		if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(gomod), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	gitRun(t, dir, "add", "-A")
	gitRun(t, dir, "commit", "-q", "-m", "chore: init")
	return dir
}

// resetFlags zeros every package-level flag variable the cmd package
// shares across subcommands, so tests that call runX functions directly
// don't leak state into each other.
func resetFlags(t *testing.T) {
	t.Helper()
	checkWorkflows = false
	refRepo = "plexusone/.github"
	showClean = false
	showSummary = true
	format = "list"
	sinceDepFilter = ""
	sinceUnpushedOnly = false
	sinceRecurse = false
	depRecurse = false
	depDirectOnly = false
	depPrefix = false
	orderSinceStr = ""
	includeTransitive = false
	unpushedOnly = false
	pendingSinceCommit = ""
	pendingFormat = "table"
	pendingTZ = "original"
	pendingDepth = 1
	pushedFormat = "table"
	pushedTZ = "original"
}
