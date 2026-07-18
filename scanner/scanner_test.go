package scanner

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// gitRun executes git in a fixture directory, isolated from the developer's
// global/system config (signing, hooks, templates).
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

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

// fixtureRepo creates a repo directory under root with one commit.
func fixtureRepo(t *testing.T, root, name, gomod string) string {
	t.Helper()
	dir := filepath.Join(root, name)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	gitRun(t, dir, "init", "-q", "-b", "main")
	gitRun(t, dir, "config", "user.name", "Test User")
	gitRun(t, dir, "config", "user.email", "test@example.com")
	writeFile(t, dir, "a.txt", "hello\n")
	if gomod != "" {
		writeFile(t, dir, "go.mod", gomod)
	}
	gitRun(t, dir, "add", "-A")
	gitRun(t, dir, "commit", "-q", "-m", "chore: init")
	return dir
}

func resultByName(results []RepoResult, name string) *RepoResult {
	for i := range results {
		if results[i].Name == name {
			return &results[i]
		}
	}
	return nil
}

func TestScanDirectory(t *testing.T) {
	root := t.TempDir()

	fixtureRepo(t, root, "clean", "module example.com/org/clean\n\ngo 1.22\n")
	dirty := fixtureRepo(t, root, "dirty", "module example.com/org/dirty\n\ngo 1.22\n")
	writeFile(t, dirty, "b.txt", "uncommitted\n")
	fixtureRepo(t, root, "mismatch", "module example.com/org/othername\n\ngo 1.22\n")
	fixtureRepo(t, root, "withreplace",
		"module example.com/org/withreplace\n\ngo 1.22\n\nreplace example.com/x => ../x\n")
	// Non-repo directory is skipped for git checks but still visited.
	if err := os.MkdirAll(filepath.Join(root, "notarepo"), 0o750); err != nil {
		t.Fatal(err)
	}

	results, err := ScanDirectoryWithProgress(root, nil, ScanOptions{
		GitBackend: NewCLIGitBackend(),
	})
	if err != nil {
		t.Fatal(err)
	}

	if r := resultByName(results, "clean"); r == nil || r.HasUncommittedChanges {
		t.Errorf("clean: %+v", r)
	}
	if r := resultByName(results, "dirty"); r == nil || !r.HasUncommittedChanges {
		t.Errorf("dirty should report uncommitted changes: %+v", r)
	}
	if r := resultByName(results, "mismatch"); r == nil || !r.HasModuleMismatch {
		t.Errorf("mismatch should report module/dir mismatch: %+v", r)
	}
	if r := resultByName(results, "withreplace"); r == nil || r.ReplaceCount != 1 {
		t.Errorf("withreplace should report one replace directive: %+v", r)
	}
}

func TestCLIGitBackend(t *testing.T) {
	root := t.TempDir()
	repo := fixtureRepo(t, root, "repo", "")
	backend := NewCLIGitBackend()

	if !backend.IsRepo(repo) {
		t.Error("IsRepo false for a repo")
	}
	if backend.IsRepo(filepath.Join(root, "missing")) {
		t.Error("IsRepo true for a non-repo")
	}

	uncommitted, unpushed := backend.GetStatus(repo, true)
	if uncommitted {
		t.Error("clean repo reported uncommitted changes")
	}
	// No upstream configured counts as unpushed.
	if !unpushed {
		t.Error("repo without upstream should count as unpushed")
	}

	writeFile(t, repo, "new.txt", "x\n")
	if uncommitted, _ := backend.GetStatus(repo, false); !uncommitted {
		t.Error("dirty repo should report uncommitted changes")
	}
}
