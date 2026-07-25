package gogit

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"sync/atomic"
	"testing"
)

func createTempRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	cmd := exec.Command("git", "init", "--initial-branch=main")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}

	cmd = exec.Command("git", "config", "user.name", "test") //nolint:gosec // test helper
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git config user.name: %v\n%s", err, out)
	}
	cmd = exec.Command("git", "config", "user.email", "test@test.com")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git config user.email: %v\n%s", err, out)
	}

	f := filepath.Join(dir, "README.md")
	if err := os.WriteFile(f, []byte("# test\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cmd = exec.Command("git", "add", ".")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git add: %v\n%s", err, out)
	}
	cmd = exec.Command("git", "commit", "-m", "feat: initial commit")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git commit: %v\n%s", err, out)
	}
	return dir
}

func TestRunAll(t *testing.T) {
	repo1 := createTempRepo(t)
	repo2 := createTempRepo(t)

	type branchResult struct {
		Branch string
	}

	results := RunAll(context.Background(), []string{repo1, repo2}, func(ctx context.Context, repo *Repo) (branchResult, error) {
		b, err := repo.Branch(ctx)
		return branchResult{Branch: b}, err
	}, 2)

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	for i, r := range results {
		if r.Err != nil {
			t.Fatalf("results[%d] error: %v", i, r.Err)
		}
		if r.Value.Branch != "main" {
			t.Fatalf("results[%d].Branch = %q, want main", i, r.Value.Branch)
		}
	}
}

func TestRunAllWithProgress(t *testing.T) {
	repo := createTempRepo(t)
	var progressCalls atomic.Int32

	results := RunAllWithProgress(context.Background(), []string{repo}, func(ctx context.Context, r *Repo) (string, error) {
		return r.Branch(ctx)
	}, 0, func(completed, total int, path string) {
		progressCalls.Add(1)
	})

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Err != nil {
		t.Fatalf("error: %v", results[0].Err)
	}
	if progressCalls.Load() != 1 {
		t.Fatalf("expected 1 progress call, got %d", progressCalls.Load())
	}
}

func TestRunAllNonRepo(t *testing.T) {
	dir := t.TempDir()
	results := RunAll(context.Background(), []string{dir}, func(_ context.Context, _ *Repo) (string, error) {
		return "unreachable", nil
	}, 1)

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Err == nil {
		t.Fatal("expected error for non-repo path")
	}
}

func TestRunAllCancelled(t *testing.T) {
	repo := createTempRepo(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	results := RunAll(ctx, []string{repo}, func(_ context.Context, _ *Repo) (string, error) {
		return "unreachable", nil
	}, 1)

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Err == nil {
		t.Fatal("expected context error")
	}
}

func TestRunAllPaths(t *testing.T) {
	dir1 := t.TempDir()
	dir2 := t.TempDir()

	results := RunAllPaths(context.Background(), []string{dir1, dir2}, func(_ context.Context, path string) (bool, error) {
		_, err := os.Stat(path)
		return err == nil, nil
	}, 2)

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	for i, r := range results {
		if r.Err != nil {
			t.Fatalf("results[%d] error: %v", i, r.Err)
		}
		if !r.Value {
			t.Fatalf("results[%d] expected true", i)
		}
	}
}
