package cmd

import (
	"strings"
	"testing"
)

func TestRunSinceDefaultsToCurrentDir(t *testing.T) {
	resetFlags(t)
	root := t.TempDir()
	fixtureRepo(t, root, "repo", "")
	t.Chdir(root)

	out := captureStdout(t, func() {
		if err := runSince(nil, []string{"7d"}); err != nil {
			t.Fatal(err)
		}
	})
	if !strings.Contains(out, "repo") {
		t.Errorf("expected repo listed when scanning the current directory: %s", out)
	}
}

func TestRunSinceInvalidDuration(t *testing.T) {
	resetFlags(t)
	if err := runSince(nil, []string{"bogus", t.TempDir()}); err == nil {
		t.Fatal("expected an error for an invalid duration")
	}
}

func TestRunSinceEndToEnd(t *testing.T) {
	resetFlags(t)
	root := t.TempDir()
	fixtureRepo(t, root, "repo", "")

	out := captureStdout(t, func() {
		if err := runSince(nil, []string{"7d", root}); err != nil {
			t.Fatal(err)
		}
	})
	if !strings.Contains(out, "repo") {
		t.Errorf("expected repo listed as recently modified: %s", out)
	}
	if !strings.Contains(out, "Summary:") {
		t.Errorf("expected a summary line: %s", out)
	}
}
