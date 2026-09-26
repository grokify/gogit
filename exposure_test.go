package gogit

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
)

// revParse returns the full hash of rev in dir.
func revParse(t *testing.T, repo *Repo, rev string) string {
	t.Helper()
	out, err := repo.git(context.Background(), "rev-parse", rev)
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(out)
}

func openRepo(t *testing.T, dir string) *Repo {
	t.Helper()
	repo, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	return repo
}

func TestRefsContaining(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	dir := filepath.Join(root, "work")
	remote := filepath.Join(root, "remote.git")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	initRepo(t, dir)
	run(t, root, "init", "-q", "--bare", remote)

	commitFile(t, dir, "a.txt", "a", "first")
	run(t, dir, "tag", "-a", "v1.0.0", "-m", "release 1")
	run(t, dir, "branch", "feature")
	commitFile(t, dir, "b.txt", "b", "second")
	run(t, dir, "tag", "v1.1.0") // lightweight
	run(t, dir, "remote", "add", "origin", remote)
	run(t, dir, "push", "-q", "origin", "main")
	run(t, dir, "remote", "set-head", "origin", "main") // symref; must be omitted
	commitFile(t, dir, "c.txt", "c", "third, unpushed")

	repo := openRepo(t, dir)
	first := revParse(t, repo, "HEAD~2")
	third := revParse(t, repo, "HEAD")

	got, err := repo.RefsContaining(ctx, first)
	if err != nil {
		t.Fatal(err)
	}
	want := []Ref{
		{Name: "feature", Kind: RefKindBranch},
		{Name: "main", Kind: RefKindBranch},
		{Name: "origin/main", Kind: RefKindRemote},
		{Name: "v1.0.0", Kind: RefKindTag},
		{Name: "v1.1.0", Kind: RefKindTag},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("first commit:\n got %+v\nwant %+v", got, want)
	}

	// An unpushed, untagged commit is only on the local branch.
	got, err = repo.RefsContaining(ctx, third[:10])
	if err != nil {
		t.Fatal(err)
	}
	want = []Ref{{Name: "main", Kind: RefKindBranch}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("unpushed commit: got %+v, want %+v", got, want)
	}

	for _, bad := range []string{"", "--all", "deadbeefdeadbeef"} {
		if _, err := repo.RefsContaining(ctx, bad); err == nil {
			t.Errorf("RefsContaining(%q): expected error", bad)
		}
	}
}

func TestTagsWithPath(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	initRepo(t, dir)
	repo := openRepo(t, dir)

	commitFile(t, dir, "README.md", "readme", "init")
	tags, err := repo.TagsWithPath(ctx, "README.md")
	if err != nil {
		t.Fatal(err)
	}
	if tags != nil {
		t.Errorf("no tags: got %v, want nil", tags)
	}

	if err := os.MkdirAll(filepath.Join(dir, "cfg"), 0o750); err != nil {
		t.Fatal(err)
	}
	commitFile(t, dir, "secret.env", "KEY=value", "add env")
	commitFile(t, dir, "cfg/app.yaml", "a: 1", "add cfg")
	run(t, dir, "tag", "-a", "v0.1.0", "-m", "annotated")
	run(t, dir, "rm", "-q", "secret.env")
	run(t, dir, "commit", "-q", "-m", "remove env")
	run(t, dir, "tag", "v0.2.0")           // lightweight
	run(t, dir, "tag", "v0.0.1", "HEAD~3") // predates the file

	cases := []struct {
		path string
		want []string
	}{
		{"secret.env", []string{"v0.1.0"}},
		{"./secret.env", []string{"v0.1.0"}},
		{"cfg", []string{"v0.1.0", "v0.2.0"}},
		{"cfg/", []string{"v0.1.0", "v0.2.0"}},
		{"README.md", []string{"v0.0.1", "v0.1.0", "v0.2.0"}},
		{"never.txt", nil},
	}
	for _, c := range cases {
		got, err := repo.TagsWithPath(ctx, c.path)
		if err != nil {
			t.Fatalf("TagsWithPath(%q): %v", c.path, err)
		}
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("TagsWithPath(%q) = %v, want %v", c.path, got, c.want)
		}
	}

	for _, bad := range []string{"", "/", "a\nb"} {
		if _, err := repo.TagsWithPath(ctx, bad); err == nil {
			t.Errorf("TagsWithPath(%q): expected error", bad)
		}
	}
}

func TestFilesEverAdded(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	initRepo(t, dir)

	commitFile(t, dir, "a.txt", "a", "add a")
	commitFile(t, dir, "secret.env", "KEY=value", "add env")
	commitFile(t, dir, "my file.txt", "spaces", "add spaced name")
	run(t, dir, "rm", "-q", "secret.env")
	run(t, dir, "commit", "-q", "-m", "remove env")
	run(t, dir, "mv", "a.txt", "b.txt")
	run(t, dir, "commit", "-q", "-m", "rename a to b")
	run(t, dir, "switch", "-q", "-c", "side")
	commitFile(t, dir, "side.txt", "side", "side-only file")
	run(t, dir, "switch", "-q", "main")
	commitFile(t, dir, "a.txt", "again", "re-add a") // duplicate path

	got, err := openRepo(t, dir).FilesEverAdded(ctx)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"a.txt", "b.txt", "my file.txt", "secret.env", "side.txt"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestIgnoredFiles(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	initRepo(t, dir)
	commitFile(t, dir, ".gitignore", "*.env\nbuild/\n", "add gitignore")

	for name, content := range map[string]string{
		"local.env":     "KEY=value",
		"build/out.bin": "bin",
		"build/log.txt": "log",
		"untracked.txt": "not ignored",
	} {
		p := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	got, err := openRepo(t, dir).IgnoredFiles(ctx)
	if err != nil {
		t.Fatal(err)
	}
	// Check membership rather than equality: the developer's global
	// excludes file also applies, and may ignore additional paths.
	for _, want := range []string{"local.env", "build/"} {
		if !slices.Contains(got, want) {
			t.Errorf("missing %q in %q", want, got)
		}
	}
	for _, notWant := range []string{"untracked.txt", ".gitignore", "build/out.bin", "build/log.txt"} {
		if slices.Contains(got, notWant) {
			t.Errorf("unexpected %q in %q", notWant, got)
		}
	}
}

func TestParseRefs(t *testing.T) {
	out := "refs/tags/v1\x00\n" +
		"refs/remotes/origin/HEAD\x00refs/remotes/origin/main\n" +
		"refs/remotes/origin/main\x00\n" +
		"refs/notes/commits\x00\n" +
		"refs/heads/feat/x\x00\n"
	want := []Ref{
		{Name: "feat/x", Kind: RefKindBranch},
		{Name: "origin/main", Kind: RefKindRemote},
		{Name: "v1", Kind: RefKindTag},
	}
	if got := parseRefs(out); !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}
