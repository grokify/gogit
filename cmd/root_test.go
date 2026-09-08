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
	for _, want := range []string{"since", "dep", "order", "pending"} {
		if !names[want] {
			t.Errorf("expected subcommand %q to be registered under rootCmd", want)
		}
	}
}

func TestRunScanRequiresDirectory(t *testing.T) {
	resetFlags(t)
	if err := runScan(nil, nil); err == nil {
		t.Fatal("expected an error when no directory is given")
	}
}

func TestRunScanInvalidFormat(t *testing.T) {
	resetFlags(t)
	dirPath = t.TempDir()
	format = "bogus"
	captureStdout(t, func() {
		if err := runScan(nil, nil); err == nil {
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

	dirPath = root
	out := captureStdout(t, func() {
		if err := runScan(nil, nil); err != nil {
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
