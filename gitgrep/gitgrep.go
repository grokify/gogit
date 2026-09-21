// Package gitgrep provides fast, reusable search over a git repository's
// working tree, index, a single revision, or history. It shells out to the
// native git CLI (git grep, the pickaxe, and streamed patches), which is the
// fastest and most portable approach and matches the rest of gogit.
//
// gitgrep is deliberately generic and free of any domain policy: callers
// pass patterns in and receive matches out. It defines no term lists, no
// severities, and no notion of what a match means. Callers own that, and
// callers are responsible for redacting Match.Text / Patch.Hunk, which are
// returned verbatim and may contain the very content being searched for.
package gitgrep

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// Pattern is a single search term.
type Pattern struct {
	// Value is a literal string when Regex is false, or a POSIX extended
	// regular expression (git grep -E / git log -G) when Regex is true.
	Value string
	// Regex selects extended-regex matching (-E) over fixed-string (-F).
	Regex bool
	// IgnoreCase performs case-insensitive matching (-i).
	IgnoreCase bool
}

// Options configures a search. Patterns are OR-combined: a line/commit
// matches if any pattern matches.
type Options struct {
	Patterns []Pattern
	// Pathspecs optionally limits the search to matching paths.
	Pathspecs []string
	// SkipBinary skips binary files (git grep -I). Recommended true.
	SkipBinary bool

	// Rev selects what GrepTree searches: "" searches the working tree,
	// otherwise a commit/tree-ish is searched. Ignored by history calls.
	Rev string
	// Staged searches the index (git grep --cached) instead of the working
	// tree. Mutually exclusive with Rev. Ignored by history calls.
	Staged bool
}

// Match is one matching line from GrepTree.
type Match struct {
	// Rev is the commit/tree-ish searched, or "" for the working tree/index.
	Rev  string `json:"rev,omitempty"`
	Path string `json:"path"`
	Line int    `json:"line"`
	// Text is the raw matched line. Callers must redact it if needed.
	Text string `json:"text"`
}

// runGit executes git in repoPath with gogit's standard hardening: LC_ALL=C
// pins messages to English and GIT_TERMINAL_PROMPT=0 prevents credential
// prompts from hanging. tolerateExit1 treats an exit status of 1 as a
// non-error empty result, which git grep and git log use to signal "no
// matches" rather than a failure.
func runGit(ctx context.Context, repoPath string, tolerateExit1 bool, args ...string) (string, error) {
	fullArgs := append([]string{"-C", repoPath}, args...)
	cmd := exec.CommandContext(ctx, "git", fullArgs...)
	cmd.Env = append(os.Environ(), "LC_ALL=C", "GIT_TERMINAL_PROMPT=0")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		var exitErr *exec.ExitError
		if tolerateExit1 && errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
			return stdout.String(), nil
		}
		return "", fmt.Errorf("gogit/gitgrep: git %s in %s: %w: %s",
			strings.Join(args, " "), repoPath, err, strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}

// patternGroup keys patterns by the git grep flags they require, since
// -i/-E/-F are global to a single git grep invocation.
type patternGroup struct {
	regex      bool
	ignoreCase bool
}

func groupPatterns(patterns []Pattern) map[patternGroup][]string {
	groups := map[patternGroup][]string{}
	for _, p := range patterns {
		k := patternGroup{regex: p.Regex, ignoreCase: p.IgnoreCase}
		groups[k] = append(groups[k], p.Value)
	}
	return groups
}

// GrepTree searches the working tree, the index (Staged), or a single
// revision (Rev) using git grep. It reports one Match per matching line.
// Only tracked content at the chosen tree is searched; brand-new untracked
// files are not covered (a caller needing those must add a filesystem pass).
//
// Because git grep applies -i/-E/-F globally, patterns are grouped by those
// flags and one git grep is run per group; results are concatenated in
// group order.
func GrepTree(ctx context.Context, repoPath string, opts Options) ([]Match, error) {
	if len(opts.Patterns) == 0 {
		return nil, fmt.Errorf("gogit/gitgrep: GrepTree requires at least one pattern")
	}
	if opts.Staged && opts.Rev != "" {
		return nil, fmt.Errorf("gogit/gitgrep: Staged and Rev are mutually exclusive")
	}

	var matches []Match
	for grp, values := range groupPatterns(opts.Patterns) {
		args := []string{"grep", "-n", "--null"}
		if opts.SkipBinary {
			args = append(args, "-I")
		}
		if grp.ignoreCase {
			args = append(args, "-i")
		}
		if grp.regex {
			args = append(args, "-E")
		} else {
			args = append(args, "-F")
		}
		if opts.Staged {
			args = append(args, "--cached")
		}
		for _, v := range values {
			args = append(args, "-e", v)
		}
		if opts.Rev != "" {
			args = append(args, opts.Rev)
		}
		if len(opts.Pathspecs) > 0 {
			args = append(args, "--")
			args = append(args, opts.Pathspecs...)
		}

		out, err := runGit(ctx, repoPath, true, args...)
		if err != nil {
			return nil, err
		}
		matches = append(matches, parseGrep(out, opts.Rev)...)
	}
	return matches, nil
}

// parseGrep parses `git grep -n --null` output. Each match line is
// path\0line\0text. When a revision is grepped, git prefixes the path with
// "<rev>:"; that prefix is stripped and recorded in Match.Rev.
func parseGrep(out, rev string) []Match {
	var matches []Match
	for _, line := range strings.Split(out, "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\x00", 3)
		if len(parts) != 3 {
			continue
		}
		path := parts[0]
		matchRev := ""
		if rev != "" {
			// git prefixes grepped-revision paths with "<rev>:".
			if i := strings.Index(path, ":"); i >= 0 {
				matchRev = path[:i]
				path = path[i+1:]
			} else {
				matchRev = rev
			}
		}
		n, _ := strconv.Atoi(parts[1])
		matches = append(matches, Match{
			Rev:  matchRev,
			Path: path,
			Line: n,
			Text: parts[2],
		})
	}
	return matches
}
