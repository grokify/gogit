package cmd

import (
	"strings"
	"testing"
)

func TestRunOrderRequiresDirectory(t *testing.T) {
	resetFlags(t)
	if err := runOrder(nil, nil); err == nil {
		t.Fatal("expected an error when no directory is given")
	}
}

func TestRunOrderInvalidDuration(t *testing.T) {
	resetFlags(t)
	dirPath = t.TempDir()
	orderSinceStr = "bogus"
	if err := runOrder(nil, nil); err == nil {
		t.Fatal("expected an error for an invalid --since duration")
	}
}

func TestRunOrderEndToEnd(t *testing.T) {
	resetFlags(t)
	root := t.TempDir()
	fixtureRepo(t, root, "base", "module github.com/example/base\n\ngo 1.25\n")
	fixtureRepo(t, root, "dependent", "module github.com/example/dependent\n\ngo 1.25\n\nrequire github.com/example/base v0.0.0\n")

	dirPath = root
	out := captureStdout(t, func() {
		if err := runOrder(nil, nil); err != nil {
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
