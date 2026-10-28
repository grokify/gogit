package gitgrep

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
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
// error the walk stops, git is stopped, and that error is returned.
//
// Output is read as git writes it and each file diff is passed to fn as
// soon as it is complete, so memory use is bounded by the largest single
// file diff rather than the whole history, and fn's work overlaps git's.
func StreamPatches(ctx context.Context, repoPath, rng string, fn func(Patch) error) error {
	args := []string{"log", "-p", logFormat}
	if rng != "" {
		args = append(args, rng)
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	cmd := gitCommand(ctx, repoPath, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("gogit/gitgrep: git %s in %s: %w", strings.Join(args, " "), repoPath, err)
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("gogit/gitgrep: git %s in %s: %w", strings.Join(args, " "), repoPath, err)
	}
	if err := readPatches(stdout, fn); err != nil {
		// Stop git rather than reading the rest of its output. Its exit
		// status is then a consequence of the kill, so the error to report
		// is the one that stopped the walk.
		cancel()
		_ = cmd.Wait() // error discarded on purpose: see above
		return err
	}
	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("gogit/gitgrep: git %s in %s: %w: %s",
			strings.Join(args, " "), repoPath, err, strings.TrimSpace(stderr.String()))
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
	// Collecting into a slice cannot fail, so readPatches cannot either.
	if err := readPatches(strings.NewReader(out), func(p Patch) error {
		patches = append(patches, p)
		return nil
	}); err != nil {
		panic(err)
	}
	return patches
}

// readPatches parses `git log -p` output (with logFormat) from r, calling fn
// for each file diff as soon as it is complete. A commit starts at a line
// beginning with the record separator, and a file diff at a line beginning
// with "diff --git ". Lines are read whole however long they are, since a
// minified or generated file can put megabytes on one line.
func readPatches(r io.Reader, fn func(Patch) error) error {
	br := bufio.NewReaderSize(r, 64<<10)
	var commit Patch // Commit, Author, Date of the commit being read
	inCommit := false
	var section []string
	flush := func() error {
		defer func() { section = section[:0] }()
		if !inCommit || len(section) == 0 || !strings.HasPrefix(section[0], "diff --git ") {
			return nil
		}
		hunk := strings.Join(section, "\n")
		path := diffPath(hunk)
		if path == "" {
			return nil
		}
		p := commit
		p.Path, p.Hunk = path, hunk
		return fn(p)
	}
	for {
		line, err := br.ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return fmt.Errorf("gogit/gitgrep: read git log output: %w", err)
		}
		if line != "" {
			line = strings.TrimSuffix(line, "\n")
			switch {
			case strings.HasPrefix(line, recordSep):
				if ferr := flush(); ferr != nil {
					return ferr
				}
				fields := strings.SplitN(strings.TrimPrefix(line, recordSep), fieldSep, 4)
				inCommit = len(fields) == 4
				if inCommit {
					commit = Patch{Commit: fields[0], Author: fields[1], Date: fields[2]}
				}
			case strings.HasPrefix(line, "diff --git "):
				if ferr := flush(); ferr != nil {
					return ferr
				}
				section = append(section, line)
			default:
				section = append(section, line)
			}
		}
		if errors.Is(err, io.EOF) {
			return flush()
		}
	}
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
