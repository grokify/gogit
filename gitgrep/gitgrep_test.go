package gitgrep

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// newTestRepo creates a temporary git repo with two commits and returns its
// path. Fixtures use synthetic placeholder terms only.
func newTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		//nolint:gosec // G204: test helper; dir is t.TempDir(), args are literals
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		cmd.Env = append(os.Environ(),
			"LC_ALL=C", "GIT_TERMINAL_PROMPT=0",
			"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	write := func(name, content string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	run("init", "-q")
	run("config", "user.email", "t@example.com")
	run("config", "user.name", "Tester")

	write("a.txt", "line one Acme Corp here\nsecond line\nthird Acme again\n")
	write("b.txt", "ExampleCo mention\nno term here\n")
	run("add", "-A")
	run("commit", "-qm", "add fixtures with Acme Corp and ExampleCo")

	write("a.txt", "line one Acme Corp here\nsecond line\nthird Acme again\nappended Acme Corp\n")
	run("add", "-A")
	run("commit", "-qm", "second commit adds Acme Corp line")

	return dir
}

func TestGrepTreeWorkingTree(t *testing.T) {
	repo := newTestRepo(t)
	got, err := GrepTree(context.Background(), repo, Options{
		Patterns:   []Pattern{{Value: "Acme"}},
		SkipBinary: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("want 3 matches, got %d: %+v", len(got), got)
	}
	for _, m := range got {
		if m.Path != "a.txt" {
			t.Errorf("unexpected path %q", m.Path)
		}
		if m.Line == 0 || m.Text == "" {
			t.Errorf("missing line/text: %+v", m)
		}
	}
}

func TestGrepTreeIgnoreCaseAndRegex(t *testing.T) {
	repo := newTestRepo(t)
	got, err := GrepTree(context.Background(), repo, Options{
		Patterns: []Pattern{
			{Value: "acme", IgnoreCase: true}, // matches Acme case-insensitively
			{Value: "Example.o", Regex: true}, // matches ExampleCo
		},
		SkipBinary: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	var haveExample bool
	for _, m := range got {
		if m.Path == "b.txt" {
			haveExample = true
		}
	}
	if !haveExample {
		t.Errorf("regex pattern did not match ExampleCo: %+v", got)
	}
	if len(got) < 4 {
		t.Errorf("want >=4 matches (3 Acme + 1 ExampleCo), got %d", len(got))
	}
}

func TestGrepTreeNoMatchNoError(t *testing.T) {
	repo := newTestRepo(t)
	got, err := GrepTree(context.Background(), repo, Options{
		Patterns: []Pattern{{Value: "NoSuchTermHere"}},
	})
	if err != nil {
		t.Fatalf("no-match must not error: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("want 0 matches, got %d", len(got))
	}
}

func TestGrepTreeMutuallyExclusive(t *testing.T) {
	repo := newTestRepo(t)
	_, err := GrepTree(context.Background(), repo, Options{
		Patterns: []Pattern{{Value: "Acme"}},
		Staged:   true,
		Rev:      "HEAD",
	})
	if err == nil {
		t.Fatal("want error for Staged+Rev, got nil")
	}
}

func TestStreamPatches(t *testing.T) {
	repo := newTestRepo(t)
	var files []string
	var withTerm int
	err := StreamPatches(context.Background(), repo, "", func(p Patch) error {
		files = append(files, p.Path)
		if p.Commit == "" || p.Date == "" {
			t.Errorf("missing commit metadata: %+v", p)
		}
		if containsAcme(p.Hunk) {
			withTerm++
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	// a.txt appears in both commits; b.txt in the first.
	if len(files) < 3 {
		t.Errorf("want >=3 file patches, got %d: %v", len(files), files)
	}
	if withTerm < 2 {
		t.Errorf("want >=2 patches containing Acme, got %d", withTerm)
	}
}

func TestHistoryPickaxe(t *testing.T) {
	repo := newTestRepo(t)
	got, err := HistoryPickaxe(context.Background(), repo, Options{
		Patterns: []Pattern{{Value: "Acme Corp"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) == 0 {
		t.Fatal("want at least one pickaxe hit for 'Acme Corp'")
	}
	for _, h := range got {
		if h.Path != "a.txt" {
			t.Errorf("unexpected path %q in pickaxe hit", h.Path)
		}
		if h.Commit == "" || h.Pattern != "Acme Corp" {
			t.Errorf("bad hit: %+v", h)
		}
	}
}

func containsAcme(s string) bool {
	for i := 0; i+4 <= len(s); i++ {
		if s[i:i+4] == "Acme" {
			return true
		}
	}
	return false
}
