package cliutil

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolvePath(t *testing.T) {
	dir := t.TempDir()

	got, err := ResolvePath(dir)
	if err != nil {
		t.Fatalf("ResolvePath(%q): %v", dir, err)
	}
	want, err := filepath.Abs(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Errorf("ResolvePath(%q) = %q, want %q", dir, got, want)
	}
}

func TestResolvePathExpandsHome(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("no home directory available")
	}

	got, err := ResolvePath("~")
	if err != nil {
		t.Fatalf("ResolvePath(~): %v", err)
	}
	wantAbs, err := filepath.Abs(home)
	if err != nil {
		t.Fatal(err)
	}
	if got != wantAbs {
		t.Errorf("ResolvePath(~) = %q, want %q", got, wantAbs)
	}
}

func TestResolvePathErrors(t *testing.T) {
	if _, err := ResolvePath(""); err == nil {
		t.Error("expected error for empty path")
	}

	notADir := filepath.Join(t.TempDir(), "file.txt")
	if err := os.WriteFile(notADir, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ResolvePath(notADir); err == nil {
		t.Error("expected error for a path that is a file, not a directory")
	}

	missing := filepath.Join(t.TempDir(), "does-not-exist")
	if _, err := ResolvePath(missing); err == nil {
		t.Error("expected error for a missing path")
	}
}
