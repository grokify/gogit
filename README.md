# gogit

[![Go CI][go-ci-svg]][go-ci-url]
[![Go Lint][go-lint-svg]][go-lint-url]
[![Go SAST][go-sast-svg]][go-sast-url]
[![Docs][docs-godoc-svg]][docs-godoc-url]
[![Docs][docs-mkdoc-svg]][docs-mkdoc-url]
[![Visualization][viz-svg]][viz-url]
[![License][license-svg]][license-url]

 [go-ci-svg]: https://github.com/grokify/gogit/actions/workflows/go-ci.yaml/badge.svg?branch=main
 [go-ci-url]: https://github.com/grokify/gogit/actions/workflows/go-ci.yaml
 [go-lint-svg]: https://github.com/grokify/gogit/actions/workflows/go-lint.yaml/badge.svg?branch=main
 [go-lint-url]: https://github.com/grokify/gogit/actions/workflows/go-lint.yaml
 [go-sast-svg]: https://github.com/grokify/gogit/actions/workflows/go-sast-codeql.yaml/badge.svg?branch=main
 [go-sast-url]: https://github.com/grokify/gogit/actions/workflows/go-sast-codeql.yaml
 [docs-godoc-svg]: https://pkg.go.dev/badge/github.com/grokify/gogit
 [docs-godoc-url]: https://pkg.go.dev/github.com/grokify/gogit
 [docs-mkdoc-svg]: https://img.shields.io/badge/docs-MkDocs-blue.svg
 [docs-mkdoc-url]: https://grokify.github.io/gogit
 [viz-svg]: https://img.shields.io/badge/visualization-Go-blue.svg
 [viz-url]: https://mango-dune-07a8b7110.1.azurestaticapps.net/?repo=grokify%2Fgogit
 [loc-svg]: https://tokei.rs/b1/github/grokify/gogit
 [repo-url]: https://github.com/grokify/gogit
 [license-svg]: https://img.shields.io/badge/license-MIT-blue.svg
 [license-url]: https://github.com/grokify/gogit/blob/main/LICENSE

Generic, dependency-light Git ergonomics for Go, by shelling out to the
git CLI: repository discovery, commit-log parsing with trailers
(Co-authored-by) and change stats, AI authorship detection, conventional
commit parsing, parallel multi-repo execution, calendar-date filtering,
branch/origin metadata, remote-URL normalization, and tag dates. The base
layer for higher-level tools — including the bundled `gitscan` CLI and
the OmniDevX telemetry collectors — in the same way
[gogithub](https://github.com/grokify/gogithub) underlies GitHub
integrations.

```go
repo, _ := gogit.Open("/path/to/repo")
commits, _ := repo.Log(ctx, gogit.LogOptions{
    Since:        weekStart,
    IncludeStats: true,
    Reverse:      true,
})
for _, c := range commits {
    attr := gogit.AnalyzeAuthorship(c)
    fmt.Println(c.Hash, c.Author.Email, attr.Tools, c.Insertions)
}
```

## Library Features

| Feature | Entry Point | Description |
|---------|-------------|-------------|
| Discovery | `Discover(roots, maxDepth)` | Depth-bounded repository discovery |
| Commit log | `Repo.Log(ctx, LogOptions)` | Parsed commits with dates, trailers, numstat |
| Incremental log | `LogOptions.SinceCommit` | High-water-mark ingestion (`sha..HEAD`) |
| Reverse order | `LogOptions.Reverse` | Chronological (oldest-first) iteration |
| Co-authors | `Commit.CoAuthors()` | `Co-authored-by` trailer extraction |
| AI authorship | `AnalyzeAuthorship(c)` | Detect AI tools, models, and human co-authors |
| AI provider registry | `DefaultAITools` | Claude Code, Copilot, Gemini CLI, Cursor, Aider |
| AI co-author parsing | `Commit.AICoAuthors()` | Identify AI tools with model version extraction |
| Commit stats | `Repo.CollectCommitStats` | Commit/LOC aggregation by conventional-commit category |
| Multi-repo stats | `AggregateCommitStats` | Parallel aggregation with AI-assisted metrics |
| Conventional commits | `ParseConventionalCommit(s)` | Type, scope, breaking flag, subject |
| Trailer lookup | `Commit.TrailerValue(key)` | Case-insensitive trailer access |
| Parallel execution | `RunAll(ctx, paths, fn, workers)` | Generic concurrent multi-repo operations |
| Progress reporting | `RunAllWithProgress(...)` | Parallel execution with progress callback |
| Metadata | `Repo.Branch`, `Repo.OriginURL` | Branch name and remote URL |
| Remote normalization | `NormalizeRemoteURL(url)` | Canonical `host/path` identifiers |
| Tags | `Repo.Tags`, `Repo.TagsWithDates` | Tag listing with creation dates |
| Pending commits | `Repo.PendingCommits(ctx, sinceCommit)` | Commits ahead of upstream, or after an explicit commit hash |
| Upstream check | `Repo.HasUpstream(ctx)` | Whether the current branch has an upstream configured |

Renamed from `gitscan` (the CLI lives on at `cmd/gitscan`).

## gitscan CLI

A CLI tool to scan multiple Git repositories and identify repos that need attention. Helps prioritize which repos to update, commit, and push.

## Installation

### Homebrew (macOS/Linux)

```bash
brew tap grokify/tap
brew install gitscan
```

### Go Install

```bash
go install github.com/grokify/gogit/cmd/gitscan@latest
```

### Build from Source

```bash
git clone https://github.com/grokify/gogit.git
cd gogit
go build -o gitscan ./cmd/gitscan
```

## Usage

```bash
gitscan [directory]              # Scan for issues (defaults to current dir)
gitscan since <duration> [dir]   # Filter by modification time
gitscan dep <module> [dir]       # Filter by dependency
gitscan order [dir]              # Show repos in dependency order
gitscan pending [path...]        # List unpushed commits, across one or many repos
gitscan pushed [count] [dir]     # List the most recent pushed commits
```

The scan directory is a positional argument that defaults to the current
directory; there is no `-d`/`--dir` flag.

### Root Command (Issue Scanning)

Scan repos for uncommitted changes, replace directives, and module mismatches:

```bash
gitscan ~/go/src/github.com/grokify
```

The directory to scan is the (optional) positional argument, defaulting to the current directory.

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--format` | `-f` | `list` | Output format: `list` or `table` |
| `--show-clean` | | `false` | Show repos with no issues |
| `--summary` | | `true` | Show summary at the end |
| `--check-workflows` | | `false` | Check GitHub Actions workflow compliance against a reference repo |
| `--ref-repo` | | `plexusone/.github` | Reference workflow repository for `--check-workflows` |

### Examples

```bash
# Scan all repos in a directory
gitscan ~/go/src/github.com/grokify

# Output as markdown table (compact view)
gitscan -f table ~/go/src/github.com/grokify

# Show all repos including clean ones
gitscan --show-clean ~/projects
```

## Since Subcommand

Filter repos by modification time, with optional dependency filtering:

```bash
gitscan since <duration> [directory]
```

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--dep` | | (none) | Also filter by dependency (AND logic) |
| `--unpushed` | `-u` | `false` | Only show repos with uncommitted changes or unpushed commits (AND logic) |
| `--recurse` | `-r` | `false` | Check nested go.mod files |

Duration formats: `7d` (days), `2w` (weeks), `1m` (months), `24h` (hours). `directory` defaults to the current directory.

### Since Examples

```bash
# Repos modified in last 7 days
gitscan since 7d ~/go/src/github.com/grokify

# Repos modified in last 7 days AND depending on a module
gitscan since 7d --dep github.com/grokify/mogo ~/go/src/github.com/grokify
```

## Dep Subcommand

Filter repos by dependency on a specific module:

```bash
gitscan dep <module> [directory]
```

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--recurse` | `-r` | `false` | Check nested go.mod files |
| `--direct-only` | `-D` | `false` | Only match direct requirements (excludes `// indirect`) |
| `--prefix` | | `false` | Match module path as a prefix (e.g. to match any major version) |

### Dep Examples

```bash
# Find repos depending on a module
gitscan dep github.com/grokify/mogo ~/go/src/github.com/grokify

# Include nested go.mod files (monorepos)
gitscan dep github.com/grokify/mogo -r ~/go/src/github.com/grokify

# Only repos that require the module themselves, not just transitively
gitscan dep github.com/google/go-github/v88 ~/go/src/github.com/grokify --direct-only

# Match any major version of a module in one pass
gitscan dep github.com/google/go-github ~/go/src/github.com/grokify --prefix --direct-only
```

## Pending Subcommand

Report commits that are ahead of their upstream — not yet pushed — across one or more repositories. Useful for pre-push review, for a morning sweep of everything you have waiting to push, or for feeding an agent a structured list of what's about to go out.

```bash
gitscan pending [path...]
```

Each `path` is either a git repository (reported directly) or a directory whose repositories are discovered (one level deep by default; use `--depth` to go deeper) and each reported in turn. Pass several paths — e.g. one per GitHub org you manage — to sweep them together. With no path, the current directory is used.

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--since-commit` | | (none) | List commits after this hash instead of unpushed commits (single repo only) |
| `--format` | `-f` | `table` | Output format: `table` (aligned, for terminals), `markdown` (copy-pasteable), or `json` |
| `--tz` | | `original` | Timestamp timezone: `original` (as recorded by git, per-commit), `local` (this machine's timezone), or `utc` |
| `--depth` | | `1` | How many directory levels below each path to search for repositories |

By default the baseline for "pending" is each branch's push target — its configured upstream, or the matching remote-tracking branch (e.g. `origin/main`). A branch that was **never pushed** has no such target, so *all* of its commits are reported as pending (rather than erroring). Use `--since-commit` to list commits after a specific hash instead; that applies to a single repository only.

The output shape is invariant in the number of repositories: a single repo is just a fleet of one. In a multi-repo sweep, repositories with nothing pending are omitted from the table/markdown views (the summary still counts them), and progress is shown on stderr so stdout stays clean for piping and JSON.

Timestamps are RFC 3339 with an explicit UTC offset (`Z` for exact UTC, otherwise numeric, e.g. `-07:00`). By default each commit keeps its own recorded timezone; `--tz utc` or `--tz local` converts every timestamp to one consistent zone.

### Pending Examples

```bash
# Unpushed commits in the current repo
gitscan pending

# ...in another repo
gitscan pending ~/go/src/github.com/me/repo

# Sweep every repo across several orgs you manage
gitscan pending ~/go/src/github.com/{myorg,myuser}

# Commits after a specific hash (single repo)
gitscan pending --since-commit abc1234

# Copy-pasteable markdown table (e.g. for a PR description)
gitscan pending --format markdown

# Machine-readable output for agents
gitscan pending --format json

# Normalize all timestamps to UTC (or --tz local for this machine's timezone)
gitscan pending --tz utc
```

### Pending Output

Table format (default; aligned columns via `text/tabwriter`, meant to be read directly in a terminal). A multi-repo sweep prints one section per repo with pending work, then a summary:

```
Repo: /Users/me/go/src/github.com/myorg/service-a
Pending commits (not yet pushed to @{upstream}): 2

#  HASH     DAY  TIMESTAMP                  MESSAGE
1  1fbde76  Mon  2026-09-07T12:16:02-07:00  feat: add b
2  af0101e  Mon  2026-09-07T12:16:05-07:00  feat: add c

Repo: /Users/me/go/src/github.com/myorg/service-b
Pending commits (no upstream configured; all local commits unpushed): 1

#  HASH     DAY  TIMESTAMP                  MESSAGE
1  9c1d2e0  Tue  2026-09-08T09:03:11-07:00  feat: initial import

Summary: 42 repos scanned, 2 with unpushed commits, 3 commits total
```

Markdown format (`--format markdown`; valid GitHub-flavored markdown, e.g. for pasting into a PR description or issue):

```
| # | Hash | Day | Timestamp | Message |
|---|------|-----|-----------|---------|
| 1 | 1fbde76 | Mon | 2026-09-07T12:16:02-07:00 | feat: add b |
| 2 | af0101e | Mon | 2026-09-07T12:16:05-07:00 | feat: add c |
```

JSON format (`--format json`) is always a `repos` array plus a `summary`, whether one repository or many — so consumers never branch on repo count:

```json
{
  "repos": [
    {
      "repo": "/Users/me/go/src/github.com/myorg/service-a",
      "mode": "unpushed",
      "ref": "@{upstream}",
      "count": 2,
      "commits": [
        {
          "hash": "1fbde76e3b4c9ff29974e52b2e67bcece53ddaf6",
          "weekday": "Mon",
          "date": "2026-09-07",
          "time": "12:16:02",
          "timestamp": "2026-09-07T12:16:02-07:00",
          "message": "feat: add b"
        }
      ]
    }
  ],
  "summary": {
    "reposScanned": 42,
    "reposWithCommits": 2,
    "commitsTotal": 3
  }
}
```

The `mode` field is one of `unpushed` (ahead of the push baseline in `ref`), `unpushed-all` (no push target — every local commit is pending), or `since-commit` (commits after the `ref` hash).

## Pushed Subcommand

The counterpart to `pending`: list the most recent commits already pushed on a repository's current branch — those reachable from its push target (upstream or matching remote-tracking branch) — newest first. Together, `pending` and `pushed` show the recent commits on either side of what has reached the remote.

```bash
gitscan pushed [count] [directory]
```

`count` is the number of commits to show and defaults to `10`; `directory` defaults to the current directory. Either positional may be given in either order (the numeric one is the count). When the branch has no push target, nothing is reported as pushed.

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--format` | `-f` | `table` | Output format: `table`, `markdown`, or `json` |
| `--tz` | | `original` | Timestamp timezone: `original`, `local`, or `utc` |

### Pushed Examples

```bash
# Last 10 pushed commits in the current repo
gitscan pushed

# Last 25 pushed commits
gitscan pushed 25

# ...in another repo (count and directory in either order)
gitscan pushed 25 ~/go/src/github.com/me/repo

# Machine-readable output for agents
gitscan pushed --format json
```

### Pushed Output

```
Repo: /Users/me/go/src/github.com/me/repo
Pushed commits (most recent first, from @{upstream}): 3

#  HASH     DAY  TIMESTAMP                  MESSAGE
1  90da58c  Mon  2026-09-07T20:41:29-07:00  fix(release): target the current repo name in goreleaser config
2  882488a  Mon  2026-09-07T19:43:37-07:00  docs: update changelog for the full commit range
3  d5f5fa1  Mon  2026-09-07T18:12:04-07:00  feat(cmd): show RFC3339 timestamps with weekday
```

`pushed` shares `pending`'s `markdown` and `json` formats (the same invariant `repos` + `summary` envelope, with `mode` set to `pushed`).

## Order Subcommand

Show repos in topological dependency order - dependencies first, then dependents. Helps determine the correct order to update and release Go modules.

```bash
gitscan order [directory]
```

`directory` is a positional argument that defaults to the current directory.

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--since` | `-s` | (none) | Filter repos modified within duration |
| `--transitive` | `-t` | `false` | Include repos that transitively depend on modified repos |
| `--unpushed` | `-u` | `false` | Only show repos with uncommitted changes or unpushed commits |

### Order Examples

```bash
# Show all repos in dependency order
gitscan order ~/go/src/github.com/grokify

# Repos modified in last 7 days, in dependency order
gitscan order -s 7d ~/go/src/github.com/grokify

# Include transitive dependents (repos depending on modified repos)
gitscan order -s 7d -t ~/go/src/github.com/grokify

# Only show repos that need to be pushed
gitscan order -s 7d -t -u ~/go/src/github.com/grokify
```

### Order Output

```
Update order (dependencies first):
----------------------------------
  1. mogo                  2026-02-08 12:28
  2. gogithub              2026-02-07 08:09 (depends on: mogo)
  3. goauth                2026-02-09 19:38 (depends on: mogo)
  4. gogoogle              2026-02-09 17:31 (depends on: goauth, mogo)
  5. go-aha                2026-02-09 02:15 (depends on: goauth, gogoogle, mogo)

Total: 5 repos in dependency order
```

## Checks Performed

For each direct subdirectory, gitscan checks:

1. **Uncommitted Changes** - Detects modified, added, or deleted files using `git status --porcelain`

2. **Replace Directives** - Parses `go.mod` for `replace` directives (both single-line and block format), which may indicate local development dependencies that shouldn't be committed

3. **Module Name Mismatch** - Compares the module name in `go.mod` with the directory name to identify renamed or copied repos

4. **Unpushed Commits** - Detects commits that haven't been pushed to remote (with `-u` flag)

## Output Format

During scanning, a progress bar shows real-time status:

```
Scanning: /Users/you/go/src/github.com/grokify
Found 584 directories to scan

[████████████████░░░░░░░░░░░░░░░░░░░░░░░░]  42% (245/584) my-current-repo
```

### List Format (default)

Repos are shown in a numbered list with issues and internal dependencies:

```
  1. mogo                  2026-02-08 12:28
  2. gogithub              2026-02-07 08:09 (depends on: mogo)
  3. my-service            2026-02-10 15:30 [uncommitted, replace:2]

Summary: 100 repos scanned, 25 modified within 7d
```

### Table Format (`-f table`)

Compact markdown table with one repo per row:

```
| # | Repository | Uncommitted | Replace | Mismatch | Git | go.mod |
|---|------------|-------------|---------|----------|-----|--------|
| 1 | omnistorage |  |  | X | Y | Y |
| 2 | omnistorage-github | X |  |  | Y | - |
| 3 | structured-changelog | X |  |  | Y | Y |
| 5 | structured-roadmap |  | 5 |  | - | Y |
```

Column legend:

- **Uncommitted**: `X` = has uncommitted changes
- **Replace**: number of replace directives in go.mod
- **Mismatch**: `X` = module name doesn't match directory
- **Git**: `Y` = is a git repo, `-` = not a git repo
- **go.mod**: `Y` = has go.mod, `-` = no go.mod

## Finding Dependents

When making breaking changes to a library, find all local repos that depend on it:

```bash
# Find repos depending on a module
gitscan dep github.com/grokify/gogithub ~/go/src/github.com/grokify

# Include nested go.mod files (monorepos, nested modules)
gitscan dep github.com/grokify/gogithub -r ~/go/src/github.com/grokify

# Find recently modified repos that depend on a module
gitscan since 7d --dep github.com/grokify/mogo ~/go/src/github.com/grokify
```

## Performance

gitscan uses parallel scanning with a goroutine worker pool (defaults to GOMAXPROCS workers) for fast scanning of large directory trees. Expensive operations like modification time calculation and unpushed commit detection are performed lazily only when needed.

### Why the Git CLI, Not go-git

gitscan shells out to the `git` binary for repository status checks rather than using the pure-Go [go-git](https://github.com/go-git/go-git) library:

| Backend | Speed | Compatibility |
|---------|-------|---------------|
| git CLI (used) | Fast (~2.5s for 600 repos) | Full compatibility with git's index/fsmonitor optimizations |
| go-git (not used) | Slower (~10s for 600 repos) | Pure Go, but must re-implement status/porcelain semantics in userspace |

go-git's main selling point — no dependency on a `git` binary — doesn't apply here: gitscan's entire job is scanning directories that are *already* git repositories, so a working `git` install is a given. Given that, the CLI backend's speed and exact compatibility with real git semantics (ahead/behind counts, detached HEAD, porcelain edge cases) outweigh go-git's portability benefit, so gitscan doesn't carry a second backend to build and keep bug-for-bug identical to the first.

### Cold vs. Warm Cache

For a fleet-wide sweep (e.g. `gitscan pending` across several org directories), the dominant cost is the OS reading each repository's `.git` metadata, not gitscan's own work. Measured across ~640 repositories:

- **First run of the session (cold filesystem cache):** ~22s — mostly disk I/O paging in git metadata.
- **Subsequent runs (warm cache):** ~5.5s — already close to the floor of one `git` process spawn per repository.

Extra worker parallelism does not help here: the run is bounded by per-repository `git` startup, not CPU. Reducing the number of `git` invocations per repository would trim warm runs modestly but has little effect on the cold-cache first run, since that data must be read from disk regardless.

### Future: Skip Unchanged Repositories

The highest-leverage speedup for repeated fleet sweeps is to **avoid visiting repos that cannot have changed**. A `--modified-since <duration>` prefilter (reusing the duration parsing already used by `gitscan since`) would `stat` each repository's `.git` and skip any untouched within the window before spawning `git` at all. On a typical day only a handful of a large fleet's repos have recent activity, so this could cut the working set — and the wall time — by 10–50x. This is a planned enhancement, not yet implemented.

## Use Cases

- **Pre-push audit**: Identify repos with uncommitted work before leaving for vacation
- **Dependency cleanup**: Find repos with local `replace` directives that need resolution
- **Repo hygiene**: Detect copied/renamed repos with mismatched module names
- **Breaking changes**: Find all repos to update before releasing library changes
- **Security patches**: Locate repos using vulnerable dependencies
- **Release ordering**: Determine correct order to update and release interdependent modules
- **Prioritization**: Focus on repos that need immediate attention

## License

MIT

 [build-status-svg]: https://github.com/grokify/gogit/actions/workflows/ci.yaml/badge.svg?branch=main
 [build-status-url]: https://github.com/grokify/gogit/actions/workflows/ci.yaml
 [lint-status-svg]: https://github.com/grokify/gogit/actions/workflows/lint.yaml/badge.svg?branch=main
 [lint-status-url]: https://github.com/grokify/gogit/actions/workflows/lint.yaml
 [goreport-svg]: https://goreportcard.com/badge/github.com/grokify/gogit
 [goreport-url]: https://goreportcard.com/report/github.com/grokify/gogit
 [docs-godoc-svg]: https://pkg.go.dev/badge/github.com/grokify/gogit
 [docs-godoc-url]: https://pkg.go.dev/github.com/grokify/gogit
 [viz-svg]: https://img.shields.io/badge/visualizaton-Go-blue.svg
 [viz-url]: https://mango-dune-07a8b7110.1.azurestaticapps.net/?repo=grokify%2Fgitscan
 [loc-svg]: https://tokei.rs/b1/github/grokify/gogit
 [repo-url]: https://github.com/grokify/gogit
 [license-svg]: https://img.shields.io/badge/license-MIT-blue.svg
 [license-url]: https://github.com/grokify/gogit/blob/master/LICENSE
