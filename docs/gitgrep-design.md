# Design Note: `gitgrep` — git content & history search

Status: proposed
Package: `github.com/grokify/gogit/gitgrep`
Audience: gogit maintainers and any library/CLI consumer

## Motivation

gogit already wraps native `git` for repo enumeration and status
(`scanner` package, `GitBackend`). Consumers increasingly need a fast,
reusable primitive to **search repository content** — the working tree,
the index, a specific revision, or history — for one or more patterns.

Today this is often done ad hoc with `grep -iR {term} *`, which:

- ignores git semantics (searches untracked/ignored files, `.git/`, build
  artifacts),
- cannot search prior history, and
- has no structured (machine-readable) output.

`gitgrep` provides a small, policy-free API around the git commands that
are purpose-built and fast for this: `git grep`, the pickaxe
(`git log -S/-G`), and streamed patches (`git log -p`).

`gitgrep` is deliberately **generic and free of any domain policy**: it
takes patterns in and returns matches out. It defines no term lists, no
severities, and no notion of what a match "means." Callers own that.

## Non-goals

- No opinion on what patterns are "bad" (no built-in term lists).
- No redaction. `Match.Text` is returned raw; callers that display or log
  matches are responsible for any masking.
- No pure-Go git reimplementation. gogit's established approach is to shell
  out to native `git` (see `scanner/git_cli.go`), which is the fastest and
  most portable option. `gitgrep` follows the same approach and inherits the
  same one requirement: `git` must be on `PATH`.

## Why native `git`, not a pure-Go git library

Native `git` is the reference implementation and is consistently faster and
lighter than pure-Go git libraries for history walks (it owns packfile
mmap/delta handling). This matches gogit's existing `CLIGitBackend`
approach. All `gitgrep` operations spawn `git` with the same hardening
already used in gogit: `LC_ALL=C` and `GIT_TERMINAL_PROMPT=0`.

## API sketch

```go
package gitgrep

// Pattern is a single search term.
type Pattern struct {
    Value      string // literal or regex per Regex
    Regex      bool   // true → -E (POSIX ERE); false → -F (fixed string)
    IgnoreCase bool   // -i
}

// Options configures a search.
type Options struct {
    Patterns   []Pattern // OR-combined (git grep -e … -e …)
    Pathspecs  []string  // optional path limits (git pathspecs)
    SkipBinary bool      // -I (recommended true)

    // Scope (mutually exclusive selectors for GrepTree):
    Rev    string // "" = working tree; else a commit/tree-ish
    Staged bool   // --cached (search the index)
}

// Match is one matching line.
type Match struct {
    Rev  string // commit SHA, or "" for working tree
    Path string
    Line int
    Text string // raw matched line — caller must redact if needed
}

// HistoryMatch is a commit whose diff added/removed a pattern.
type HistoryMatch struct {
    Commit  string
    Author  string
    Date    string
    Path    string
    Pattern string
}

// Patch is one streamed diff hunk with commit context.
type Patch struct {
    Commit  string
    Author  string
    Date    string
    Path    string
    Hunk    string // raw added/removed lines for this file in this commit
}

// GrepTree searches the working tree, index, or a single revision
// (git grep). Fast; only tracked content at the chosen tree.
func GrepTree(repoPath string, opts Options) ([]Match, error)

// HistoryPickaxe finds commits whose diff adds/removes any pattern
// (git log -p -S<str> or -G<regex>). Efficient "when was it introduced".
func HistoryPickaxe(repoPath string, opts Options) ([]HistoryMatch, error)

// StreamPatches streams `git log -p [rng]` hunks to fn, so callers can run
// arbitrary detectors over diff text with commit/file context. rng may be a
// range ("base..head"), "--all", or "" (default log). Returning a non-nil
// error from fn stops the walk and is returned to the caller.
func StreamPatches(repoPath, rng string, fn func(Patch) error) error
```

### Command mapping

| Function          | git command                                               |
| ----------------- | -------------------------------------------------------- |
| `GrepTree`        | `git grep -I -n -E/-F [-i] -e P … [--cached] [<rev>] [-- <paths>]` |
| `HistoryPickaxe`  | `git log -p -S<str>` or `git log -p -G<regex>`           |
| `StreamPatches`   | `git log -p [--all] [<range>]` (streamed, parsed to `Patch`) |

`StreamPatches` is the general engine: one walk, caller-supplied detection.

## gitscan CLI surface

Extends the existing `gitscan` CLI with a `grep` subcommand:

```
gitscan grep -e ACME -e "Acme Corp" --ignore-case ./   # working tree, tracked files
gitscan grep --staged -e ACME                          # index
gitscan grep --rev HEAD~5 -e ACME                      # a specific revision
gitscan grep --history -S "Acme Corp"                  # pickaxe: which commit introduced it
gitscan grep --json ...                                # machine-readable output
```

## Caveats

- **Tracked content only** at a given tree. `git grep` on the working tree
  covers unstaged edits to *tracked* files but not brand-new untracked
  files; callers needing those must add a filesystem pass.
- **Exhaustive full-history grep** (`git grep $(git rev-list --all)`) is
  O(all blobs) and expensive on large repos. Prefer `HistoryPickaxe` or
  `StreamPatches`; gate any exhaustive mode behind an explicit flag.
- **Regex dialect:** pin `-E` (POSIX ERE). Do not assume PCRE
  (`--perl-regexp` is not always compiled in).
- **Binary files:** default `SkipBinary` (`-I`) true.
- **Raw output:** `Match.Text` / `Patch.Hunk` may contain the very content
  a caller was searching for. gogit returns it raw by design; redaction is
  the caller's responsibility.

## Testing

Follow gogit's existing pattern: table tests that construct temporary git
repos, commit fixtures, and assert on matches. Use synthetic placeholder
terms (e.g. `Acme Corp`, `ExampleCo`) in fixtures.
