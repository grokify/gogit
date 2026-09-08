package cmd

import (
	"strings"
	"testing"
)

func TestRunOrderDefaultsToCurrentDir(t *testing.T) {
	resetFlags(t)
	root := t.TempDir()
	fixtureRepo(t, root, "base", "module github.com/example/base\n\ngo 1.25\n")
	t.Chdir(root)

	out := captureStdout(t, func() {
		if err := runOrder(nil, nil); err != nil {
			t.Fatal(err)
		}
	})
	if !strings.Contains(reportSection(out), "base") {
		t.Errorf("expected the repo listed when ordering the current directory: %s", out)
	}
}

func TestRunOrderInvalidDuration(t *testing.T) {
	resetFlags(t)
	orderSinceStr = "bogus"
	if err := runOrder(nil, []string{t.TempDir()}); err == nil {
		t.Fatal("expected an error for an invalid --since duration")
	}
}

func TestRunOrderEndToEnd(t *testing.T) {
	resetFlags(t)
	root := t.TempDir()
	fixtureRepo(t, root, "base", "module github.com/example/base\n\ngo 1.25\n")
	fixtureRepo(t, root, "dependent", "module github.com/example/dependent\n\ngo 1.25\n\nrequire github.com/example/base v0.0.0\n")

	out := captureStdout(t, func() {
		if err := runOrder(nil, []string{root}); err != nil {
			t.Fatal(err)
		}
	})
	report := reportSection(out)
	if !strings.Contains(report, "base") || !strings.Contains(report, "dependent") {
		t.Errorf("expected both repos listed: %s", report)
	}
	if strings.Index(report, "base") > strings.Index(report, "dependent") {
		t.Errorf("expected base before dependent in topological order: %s", report)
	}
}
