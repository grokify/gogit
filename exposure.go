package gogit

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

// RefKind classifies a git ref by namespace.
type RefKind string

const (
	RefKindBranch RefKind = "branch" // refs/heads/*
	RefKindRemote RefKind = "remote" // refs/remotes/*
	RefKindTag    RefKind = "tag"    // refs/tags/*
)

// Ref is a branch, remote-tracking branch, or tag.
type Ref struct {
	// Name is the ref's short name, e.g. "main", "origin/main", or "v1.2.0".
	Name string  `json:"name"`
	Kind RefKind `json:"kind"`
}

var refPrefixes = []struct {
	prefix string
	kind   RefKind
}{
	{"refs/heads/", RefKindBranch},
	{"refs/remotes/", RefKindRemote},
	{"refs/tags/", RefKindTag},
}

// RefsContaining returns the local branches, remote-tracking branches, and
// tags whose history contains commit (git for-each-ref --contains). It
// answers "has this commit been pushed, and has it been released?": a
// remote ref means the commit reached that remote as of the last fetch,
// and a tag means it is part of that tagged release's history.
//
// Symbolic refs such as origin/HEAD are omitted, since they duplicate the
// branch they point to. Results are ordered branches, remotes, then tags,
// each sorted by name.
func (r *Repo) RefsContaining(ctx context.Context, commit string) ([]Ref, error) {
	if commit == "" || strings.HasPrefix(commit, "-") {
		return nil, fmt.Errorf("gogit: invalid commit %q", commit)
	}
	// Resolve first so an unknown commit is an error rather than an empty
	// result, which would falsely read as "not pushed anywhere".
	if _, err := r.git(ctx, "rev-parse", "--verify", "--quiet", commit+"^{commit}"); err != nil {
		return nil, fmt.Errorf("gogit: unknown commit %q: %w", commit, err)
	}
	out, err := r.git(ctx, "for-each-ref", "--contains", commit,
		"--format=%(refname)%00%(symref)", "refs/heads", "refs/remotes", "refs/tags")
	if err != nil {
		return nil, err
	}
	return parseRefs(out), nil
}

// parseRefs parses `for-each-ref --format=%(refname)%00%(symref)` output,
// dropping symbolic refs and refs outside the known namespaces.
func parseRefs(out string) []Ref {
	var refs []Ref
	for _, line := range strings.Split(out, "\n") {
		full, symref, _ := strings.Cut(line, "\x00")
		if full == "" || symref != "" {
			continue
		}
		for _, p := range refPrefixes {
			if name, ok := strings.CutPrefix(full, p.prefix); ok {
				refs = append(refs, Ref{Name: name, Kind: p.kind})
				break
			}
		}
	}
	kindOrder := map[RefKind]int{RefKindBranch: 0, RefKindRemote: 1, RefKindTag: 2}
	sort.SliceStable(refs, func(i, j int) bool {
		if refs[i].Kind != refs[j].Kind {
			return kindOrder[refs[i].Kind] < kindOrder[refs[j].Kind]
		}
		return refs[i].Name < refs[j].Name
	})
	return refs
}

// TagsWithPath returns the tags whose tree contains path (a file or
// directory, relative to the repository root), sorted by name. This
// distinguishes content that only ever existed in commit history from
// content that shipped in a tagged release — for a Go module, a tagged
// tree is what proxy.golang.org archives permanently.
//
// All tags are checked in a single `git cat-file --batch-check` call, so
// cost does not grow with one process per tag.
func (r *Repo) TagsWithPath(ctx context.Context, path string) ([]string, error) {
	path = strings.TrimPrefix(strings.TrimSuffix(path, "/"), "./")
	if path == "" || strings.ContainsAny(path, "\n\x00") {
		return nil, fmt.Errorf("gogit: invalid path %q", path)
	}
	tags, err := r.Tags(ctx)
	if err != nil {
		return nil, err
	}
	if len(tags) == 0 {
		return nil, nil
	}
	var queries strings.Builder
	for _, tag := range tags {
		// <tag>:<path> peels annotated tags to their commit's tree.
		queries.WriteString("refs/tags/" + tag + ":" + path + "\n")
	}
	out, err := r.gitStdin(ctx, queries.String(), "cat-file", "--batch-check=%(objecttype)")
	if err != nil {
		return nil, err
	}
	// batch-check emits exactly one line per query, in order: the object
	// type when found, or "<query> missing" / "<query> ambiguous" when not.
	lines := strings.Split(strings.TrimSuffix(out, "\n"), "\n")
	if len(lines) != len(tags) {
		return nil, fmt.Errorf("gogit: cat-file returned %d results for %d tags", len(lines), len(tags))
	}
	var found []string
	for i, line := range lines {
		if line == "blob" || line == "tree" || line == "commit" {
			found = append(found, tags[i])
		}
	}
	sort.Strings(found)
	return found, nil
}

// FilesEverAdded returns every path added by any commit reachable from any
// ref (git log --all --diff-filter=A), sorted and de-duplicated. It
// surfaces files that were committed and later deleted — and so are absent
// from HEAD — but remain retrievable from history. Renames are reported
// under their new path as well, since rename detection is disabled.
//
// Files introduced only by a merge commit's conflict resolution are not
// reported, matching git log's default of not diffing merges.
func (r *Repo) FilesEverAdded(ctx context.Context) ([]string, error) {
	out, err := r.git(ctx, "log", "--all", "--no-renames", "--diff-filter=A",
		"--name-only", "-z", "--format=")
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	var files []string
	for _, f := range splitNUL(out) {
		// Commits are separated by a newline that -z does not replace.
		f = strings.TrimLeft(f, "\n")
		if f == "" || seen[f] {
			continue
		}
		seen[f] = true
		files = append(files, f)
	}
	sort.Strings(files)
	return files, nil
}

// IgnoredFiles returns untracked paths excluded by .gitignore,
// .git/info/exclude, or the global excludes file (git ls-files --others
// --ignored --exclude-standard --directory). A directory that is ignored
// in full is reported once, with a trailing "/", rather than file by file.
//
// Ignored files are where local secrets (credentials, .env files, keys)
// usually live; listing them shows what exists on disk that a forced
// `git add -f` could still commit.
func (r *Repo) IgnoredFiles(ctx context.Context) ([]string, error) {
	out, err := r.git(ctx, "ls-files", "-z", "--others", "--ignored",
		"--exclude-standard", "--directory")
	if err != nil {
		return nil, err
	}
	return splitNUL(out), nil
}
