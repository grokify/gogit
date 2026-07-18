// Package gogit provides generic, dependency-light Git ergonomics by
// shelling out to the git CLI: repository discovery, commit-log parsing with
// trailers and change stats, and repository metadata (branch, origin URL).
//
// It is the base layer for higher-level tools — the gitscan CLI
// (cmd/gitscan) and domain collectors such as OmniDevX — in the same way
// github.com/grokify/gogithub underlies GitHub integrations.
package gogit

import (
	"bytes"
	"context"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// IsRepo reports whether path contains a .git entry (directory or gitfile).
func IsRepo(path string) bool {
	_, err := os.Stat(filepath.Join(path, ".git"))
	return err == nil
}

// Discover walks each root up to maxDepth directory levels (1 = direct
// children) and returns paths that are git repositories. Discovery does not
// descend into repositories, so nested checkouts and vendored trees are not
// double-counted.
func Discover(roots []string, maxDepth int) ([]string, error) {
	if maxDepth < 1 {
		return nil, fmt.Errorf("gogit: maxDepth must be >= 1, got %d", maxDepth)
	}
	var repos []string
	for _, root := range roots {
		root = filepath.Clean(root)
		if IsRepo(root) {
			repos = append(repos, root)
			continue
		}
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !d.IsDir() {
				return nil
			}
			rel, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			if rel == "." {
				return nil
			}
			depth := strings.Count(rel, string(filepath.Separator)) + 1
			if IsRepo(path) {
				repos = append(repos, path)
				return fs.SkipDir
			}
			if depth >= maxDepth {
				return fs.SkipDir
			}
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("gogit: discover under %s: %w", root, err)
		}
	}
	return repos, nil
}

// Repo is a handle to a local git repository.
type Repo struct {
	path string
}

// Open returns a Repo for path, verifying it is a git repository.
func Open(path string) (*Repo, error) {
	if !IsRepo(path) {
		return nil, fmt.Errorf("gogit: not a git repository: %s", path)
	}
	return &Repo{path: path}, nil
}

// Path returns the repository's working-tree path.
func (r *Repo) Path() string { return r.path }

// Branch returns the current branch name, or "HEAD" when detached.
func (r *Repo) Branch(ctx context.Context) (string, error) {
	out, err := r.git(ctx, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// OriginURL returns the raw URL of the "origin" remote, or "" when no
// origin is configured.
func (r *Repo) OriginURL(ctx context.Context) (string, error) {
	out, err := r.git(ctx, "remote", "get-url", "origin")
	if err != nil {
		// A missing remote is a normal condition, not an error.
		if strings.Contains(strings.ToLower(err.Error()), "no such remote") {
			return "", nil
		}
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// git runs a git subcommand against the repository and returns stdout.
// LC_ALL=C pins git's messages to English so error-condition detection is
// locale-independent, and GIT_TERMINAL_PROMPT=0 ensures no command can
// hang waiting for credentials.
func (r *Repo) git(ctx context.Context, args ...string) (string, error) {
	fullArgs := append([]string{"-C", r.path}, args...)
	cmd := exec.CommandContext(ctx, "git", fullArgs...)
	cmd.Env = append(os.Environ(), "LC_ALL=C", "GIT_TERMINAL_PROMPT=0")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("gogit: git %s in %s: %w: %s",
			strings.Join(args, " "), r.path, err, strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}
