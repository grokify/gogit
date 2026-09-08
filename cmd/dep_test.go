package cmd

import (
	"strings"
	"testing"
)

func TestRunDepDefaultsToCurrentDir(t *testing.T) {
	resetFlags(t)
	root := t.TempDir()
	fixtureRepo(t, root, "consumer", "module github.com/example/consumer\n\ngo 1.25\n\nrequire github.com/grokify/mogo v0.74.8\n")
	t.Chdir(root)

	out := captureStdout(t, func() {
		if err := runDep(nil, []string{"github.com/grokify/mogo"}); err != nil {
			t.Fatal(err)
		}
	})
	if !strings.Contains(reportSection(out), "consumer") {
		t.Errorf("expected consumer listed when scanning the current directory: %s", out)
	}
}

func TestRunDepEndToEnd(t *testing.T) {
	resetFlags(t)
	root := t.TempDir()
	fixtureRepo(t, root, "consumer", "module github.com/example/consumer\n\ngo 1.25\n\nrequire github.com/grokify/mogo v0.74.8\n")
	fixtureRepo(t, root, "unrelated", "module github.com/example/unrelated\n\ngo 1.25\n")

	out := captureStdout(t, func() {
		if err := runDep(nil, []string{"github.com/grokify/mogo", root}); err != nil {
			t.Fatal(err)
		}
	})
	report := reportSection(out)
	if !strings.Contains(report, "consumer") {
		t.Errorf("expected consumer listed as a dependent: %s", report)
	}
	if strings.Contains(report, "unrelated") {
		t.Errorf("unrelated repo should not be listed: %s", report)
	}
}
