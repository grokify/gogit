package gitgrep

import (
	"context"
	"fmt"
	"regexp"
	"strings"
)

// HistoryMatch is a commit whose diff added or removed a pattern, attributed
// to the specific file whose diff contained it.
type HistoryMatch struct {
	Commit  string `json:"commit"`
	Author  string `json:"author"`
	Date    string `json:"date"`
	Path    string `json:"path"`
	Pattern string `json:"pattern"`
}

// Patch is one file's diff within one commit.
type Patch struct {
	Commit string `json:"commit"`
	Author string `json:"author"`
	Date   string `json:"date"`
	Path   string `json:"path"`
	// Hunk is the raw diff body for this file (from the first @@ onward, or
	// the whole file section when no @@ is present). It is returned verbatim
	// and may contain searched-for content; callers must redact if needed.
	Hunk string `json:"hunk"`
}

// logFormat prefixes each commit with the record separator and delimits the
// four header fields with the field separator, matching gogit/log.go's use
// of ASCII RS (0x1e) and US (0x1f), which cannot appear in this output.
const logFormat = "--format=%x1e%H%x1f%an%x1f%aI%x1f%s"

const (
	recordSep = "\x1e"
	fieldSep  = "\x1f"
)

// StreamPatches walks `git log -p` over rng and invokes fn once per file
// diff, with commit context. rng may be a range ("base..head"), a single
// rev, "--all", or "" (the default log from HEAD). If fn returns a non-nil
// error the walk stops and that error is returned.
//
// Exhaustive history walks (e.g. "--all" on a large repo) can produce large
// output; prefer a bounded range where possible.
func StreamPatches(ctx context.Context, repoPath, rng string, fn func(Patch) error) error {
	args := []string{"log", "-p", logFormat}
	if rng != "" {
		args = append(args, rng)
	}
	out, err := runGit(ctx, repoPath, false, args...)
	if err != nil {
		return err
	}
	for _, patch := range parsePatches(out) {
		if err := fn(patch); err != nil {
			return err
		}
	}
	return nil
}

// HistoryPickaxe finds commits whose diff adds or removes any pattern (git
// log -S for fixed strings, -G for regex), and attributes each to the file
// whose diff contained the pattern. One git log runs per pattern.
func HistoryPickaxe(ctx context.Context, repoPath string, opts Options) ([]HistoryMatch, error) {
	if len(opts.Patterns) == 0 {
		return nil, fmt.Errorf("gogit/gitgrep: HistoryPickaxe requires at least one pattern")
	}

	var out []HistoryMatch
	for _, p := range opts.Patterns {
		matcher, err := lineMatcher(p)
		if err != nil {
			return nil, err
		}

		args := []string{"log", "-p", logFormat}
		if p.IgnoreCase {
			args = append(args, "-i")
		}
		if p.Regex {
			args = append(args, "-G"+p.Value)
		} else {
			args = append(args, "-S"+p.Value)
		}
		raw, err := runGit(ctx, repoPath, false, args...)
		if err != nil {
			return nil, err
		}
		for _, patch := range parsePatches(raw) {
			if matcher(patch.Hunk) {
				out = append(out, HistoryMatch{
					Commit:  patch.Commit,
					Author:  patch.Author,
					Date:    patch.Date,
					Path:    patch.Path,
					Pattern: p.Value,
				})
			}
		}
	}
	return out, nil
}

// lineMatcher returns a predicate reporting whether a diff body contains the
// pattern, honoring Regex and IgnoreCase, used to attribute a pickaxe hit to
// a specific file.
func lineMatcher(p Pattern) (func(string) bool, error) {
	if p.Regex {
		expr := p.Value
		if p.IgnoreCase {
			expr = "(?i)" + expr
		}
		re, err := regexp.Compile(expr)
		if err != nil {
			return nil, fmt.Errorf("gogit/gitgrep: invalid regex %q: %w", p.Value, err)
		}
		return re.MatchString, nil
	}
	if p.IgnoreCase {
		needle := strings.ToLower(p.Value)
		return func(s string) bool { return strings.Contains(strings.ToLower(s), needle) }, nil
	}
	needle := p.Value
	return func(s string) bool { return strings.Contains(s, needle) }, nil
}

// parsePatches splits `git log -p` output (with logFormat) into one Patch per
// file per commit.
func parsePatches(out string) []Patch {
	var patches []Patch
	// The first segment before the first record separator is empty.
	for _, commit := range strings.Split(out, recordSep) {
		if strings.TrimSpace(commit) == "" {
			continue
		}
		nl := strings.IndexByte(commit, '\n')
		if nl < 0 {
			continue
		}
		header := commit[:nl]
		body := commit[nl+1:]
		fields := strings.SplitN(header, fieldSep, 4)
		if len(fields) != 4 {
			continue
		}
		sha, author, date, _ := fields[0], fields[1], fields[2], fields[3]

		for _, fileDiff := range splitFileDiffs(body) {
			path := diffPath(fileDiff)
			if path == "" {
				continue
			}
			patches = append(patches, Patch{
				Commit: sha,
				Author: author,
				Date:   date,
				Path:   path,
				Hunk:   fileDiff,
			})
		}
	}
	return patches
}

// splitFileDiffs splits a commit's patch body into per-file sections, each
// beginning with a "diff --git " line.
func splitFileDiffs(body string) []string {
	lines := strings.Split(body, "\n")
	var sections []string
	var cur []string
	flush := func() {
		if len(cur) > 0 {
			sections = append(sections, strings.Join(cur, "\n"))
			cur = nil
		}
	}
	for _, line := range lines {
		if strings.HasPrefix(line, "diff --git ") {
			flush()
		}
		cur = append(cur, line)
	}
	flush()
	return sections
}

// diffPath extracts the file path from a single file diff, preferring the
// "+++ b/<path>" line and falling back to "--- a/<path>" for deletions.
func diffPath(fileDiff string) string {
	var plus, minus string
	for _, line := range strings.Split(fileDiff, "\n") {
		switch {
		case strings.HasPrefix(line, "+++ b/"):
			plus = strings.TrimPrefix(line, "+++ b/")
		case strings.HasPrefix(line, "--- a/"):
			minus = strings.TrimPrefix(line, "--- a/")
		}
	}
	if plus != "" && plus != "/dev/null" {
		return plus
	}
	return minus
}
