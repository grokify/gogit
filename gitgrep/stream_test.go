package gitgrep

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestStreamPatchesStopsOnCallbackError(t *testing.T) {
	repo := newTestRepo(t)
	stop := errors.New("stop")
	calls := 0
	err := StreamPatches(context.Background(), repo, "", func(Patch) error {
		calls++
		return stop
	})
	if !errors.Is(err, stop) {
		t.Fatalf("err = %v, want the callback's error", err)
	}
	if calls != 1 {
		t.Errorf("callback ran %d times after returning an error, want 1", calls)
	}
}

func TestStreamPatchesBadRangeErrors(t *testing.T) {
	repo := newTestRepo(t)
	err := StreamPatches(context.Background(), repo, "no-such-rev..HEAD", func(Patch) error { return nil })
	if err == nil {
		t.Fatal("want an error for an unknown revision")
	}
}

// gitLog builds `git log -p` output in logFormat, for parser tests.
func gitLog(commits ...string) string {
	return strings.Join(commits, "")
}

func commitHeader(sha string) string {
	return recordSep + sha + fieldSep + "Tester" + fieldSep + "2026-01-01T00:00:00Z" + fieldSep + "subject\n\n"
}

func TestReadPatchesLongLine(t *testing.T) {
	long := strings.Repeat("x", 2<<20)
	out := gitLog(commitHeader("c1") +
		"diff --git a/min.js b/min.js\n--- /dev/null\n+++ b/min.js\n@@ -0,0 +1 @@\n+" + long + "\n")
	patches := parsePatches(out)
	if len(patches) != 1 {
		t.Fatalf("want 1 patch, got %d", len(patches))
	}
	if !strings.Contains(patches[0].Hunk, "+"+long) {
		t.Error("long line was not kept whole")
	}
}

func TestReadPatchesPathComesFromHeader(t *testing.T) {
	// An added line "++ b/evil.txt" shows as "+++ b/evil.txt" in the diff
	// body; it must not be taken for the file's path.
	out := gitLog(commitHeader("c1") +
		"diff --git a/real.txt b/real.txt\n--- a/real.txt\n+++ b/real.txt\n@@ -1 +1,2 @@\n ctx\n+++ b/evil.txt\n")
	patches := parsePatches(out)
	if len(patches) != 1 || patches[0].Path != "real.txt" {
		t.Fatalf("got %+v, want one patch for real.txt", patches)
	}
}

func TestReadPatchesCommitsAndFiles(t *testing.T) {
	out := gitLog(
		commitHeader("c2")+
			"diff --git a/a.txt b/a.txt\n--- a/a.txt\n+++ b/a.txt\n@@ -1 +1 @@\n-old\n+new\n"+
			"diff --git a/gone.txt b/gone.txt\ndeleted file mode 100644\n--- a/gone.txt\n+++ /dev/null\n@@ -1 +0,0 @@\n-bye\n",
		commitHeader("c1")+
			"diff --git a/a.txt b/a.txt\nnew file mode 100644\n--- /dev/null\n+++ b/a.txt\n@@ -0,0 +1 @@\n+old\n",
	)
	patches := parsePatches(out)
	got := make([]string, 0, len(patches))
	for _, p := range patches {
		got = append(got, p.Commit+":"+p.Path)
		if !strings.HasPrefix(p.Hunk, "diff --git ") {
			t.Errorf("hunk for %s does not start at its diff header: %q", p.Path, p.Hunk)
		}
	}
	want := "c2:a.txt c2:gone.txt c1:a.txt"
	if strings.Join(got, " ") != want {
		t.Errorf("patches = %v, want %s", got, want)
	}
}
