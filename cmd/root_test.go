package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRootRegistersSubcommands(t *testing.T) {
	names := map[string]bool{}
	for _, c := range rootCmd.Commands() {
		names[c.Name()] = true
	}
	for _, want := range []string{"since", "dep", "order", "pending", "pushed"} {
		if !names[want] {
			t.Errorf("expected subcommand %q to be registered under rootCmd", want)
		}
	}
}

func TestRunScanDefaultsToCurrentDir(t *testing.T) {
	resetFlags(t)
	root := t.TempDir()
	fixtureRepo(t, root, "repo", "")
	t.Chdir(root)

	out := captureStdout(t, func() {
		if err := runScan(nil, nil); err != nil {
			t.Fatal(err)
		}
	})
	if !strings.Contains(out, "Summary:") {
		t.Errorf("expected a summary line when scanning the current directory: %s", out)
	}
}

func TestRunScanInvalidFormat(t *testing.T) {
	resetFlags(t)
	format = "bogus"
	captureStdout(t, func() {
		if err := runScan(nil, []string{t.TempDir()}); err == nil {
			t.Fatal("expected an error for an invalid --format value")
		}
	})
}

func TestRunScanEndToEnd(t *testing.T) {
	resetFlags(t)
	root := t.TempDir()
	repo := fixtureRepo(t, root, "dirty", "")
	if err := os.WriteFile(filepath.Join(repo, "untracked.txt"), []byte("x\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	out := captureStdout(t, func() {
		if err := runScan(nil, []string{root}); err != nil {
			t.Fatal(err)
		}
	})

	if !strings.Contains(out, "dirty") {
		t.Errorf("expected the dirty repo listed with an issue: %s", out)
	}
	if !strings.Contains(out, "Summary:") {
		t.Errorf("expected a summary line: %s", out)
	}
}
