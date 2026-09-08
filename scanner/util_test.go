package scanner

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestModifiedSince(t *testing.T) {
	r := RepoResult{LatestModTime: time.Now()}
	if !r.ModifiedSince(time.Hour) {
		t.Error("expected true for a repo modified within the window")
	}

	old := RepoResult{LatestModTime: time.Now().Add(-30 * 24 * time.Hour)}
	if old.ModifiedSince(time.Hour) {
		t.Error("expected false for a repo modified outside the window")
	}

	zero := RepoResult{}
	if zero.ModifiedSince(time.Hour) {
		t.Error("expected false when LatestModTime was never computed")
	}
}

func TestNeedsPush(t *testing.T) {
	tests := []struct {
		name string
		r    RepoResult
		want bool
	}{
		{"clean", RepoResult{}, false},
		{"uncommitted", RepoResult{HasUncommittedChanges: true}, true},
		{"unpushed commits", RepoResult{HasUnpushedCommits: true}, true},
		{"status error treated as needing attention", RepoResult{StatusError: "boom"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.r.NeedsPush(); got != tt.want {
				t.Errorf("NeedsPush() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCountDirectories(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"repo1", "repo2", ".hidden"} {
		if err := os.MkdirAll(filepath.Join(root, name), 0o750); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "file.txt"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	count, err := CountDirectories(root)
	if err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Errorf("CountDirectories = %d, want 2 (hidden dirs and files excluded)", count)
	}
}

func TestCountDirectoriesMissingPath(t *testing.T) {
	if _, err := CountDirectories(filepath.Join(t.TempDir(), "nope")); err == nil {
		t.Error("expected an error for a missing directory")
	}
}

func TestScanDirectorySimpleWrapper(t *testing.T) {
	root := t.TempDir()
	fixtureRepo(t, root, "repo", "module github.com/example/repo\n\ngo 1.25\n")

	results, err := ScanDirectory(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Name != "repo" {
		t.Fatalf("expected 1 result named 'repo', got %+v", results)
	}
	if !results[0].IsGitRepo || !results[0].HasGoMod {
		t.Errorf("expected IsGitRepo and HasGoMod true, got %+v", results[0])
	}
}

func TestFindGoModFiles(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module root\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	nested := filepath.Join(root, "sub")
	if err := os.MkdirAll(nested, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nested, "go.mod"), []byte("module sub\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	vendored := filepath.Join(root, "vendor", "dep")
	if err := os.MkdirAll(vendored, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vendored, "go.mod"), []byte("module dep\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	got := findGoModFiles(root)
	if len(got) != 1 || got[0] != filepath.Join(nested, "go.mod") {
		t.Errorf("findGoModFiles = %v, want only %q (root and vendor go.mod excluded)", got, filepath.Join(nested, "go.mod"))
	}
}

func TestGetLatestModTime(t *testing.T) {
	root := t.TempDir()
	older := filepath.Join(root, "older.txt")
	newer := filepath.Join(root, "newer.txt")
	if err := os.WriteFile(older, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	oldTime := time.Now().Add(-time.Hour)
	if err := os.Chtimes(older, oldTime, oldTime); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(newer, []byte("y"), 0o600); err != nil {
		t.Fatal(err)
	}
	newTime := time.Now()
	if err := os.Chtimes(newer, newTime, newTime); err != nil {
		t.Fatal(err)
	}

	got := getLatestModTime(root)
	if got.Before(newTime.Add(-time.Second)) {
		t.Errorf("getLatestModTime = %v, want approximately %v", got, newTime)
	}
}

func TestGetLatestModTimeSkipsGitDir(t *testing.T) {
	root := t.TempDir()
	gitDir := filepath.Join(root, ".git")
	if err := os.MkdirAll(gitDir, 0o750); err != nil {
		t.Fatal(err)
	}
	future := time.Now().Add(24 * time.Hour)
	gitFile := filepath.Join(gitDir, "HEAD")
	if err := os.WriteFile(gitFile, []byte("ref: refs/heads/main\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(gitFile, future, future); err != nil {
		t.Fatal(err)
	}

	got := getLatestModTime(root)
	if got.Equal(future) || got.After(time.Now()) {
		t.Errorf("getLatestModTime should skip .git contents, got %v", got)
	}
}

func TestDefaultGitBackend(t *testing.T) {
	backend := DefaultGitBackend()
	if backend == nil {
		t.Fatal("expected a non-nil backend")
	}
	if _, ok := backend.(*CLIGitBackend); !ok {
		t.Errorf("expected DefaultGitBackend to return *CLIGitBackend, got %T", backend)
	}
}
