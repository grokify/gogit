# gogit

Generic, dependency-light Git ergonomics for Go, by shelling out to the
git CLI: repository discovery, commit-log parsing with trailers
(Co-authored-by) and change stats, calendar-date filtering, branch/origin
metadata, remote-URL normalization, and tag dates. The base layer for
higher-level tools — including the bundled `gitscan` CLI and the OmniDevX
telemetry collectors — in the same way
[gogithub](https://github.com/grokify/gogithub) underlies GitHub
integrations.

```go
repo, _ := gogit.Open("/path/to/repo")
commits, _ := repo.Log(ctx, gogit.LogOptions{
    Since:        weekStart,
    IncludeStats: true,
})
for _, c := range commits {
    fmt.Println(c.Hash, c.Author.Email, c.CoAuthors(), c.Insertions)
}
```

Renamed from `gitscan` (the CLI lives on at `cmd/gitscan`).

## gitscan CLI

[![Build Status][build-status-svg]][build-status-url]
[![Lint Status][lint-status-svg]][lint-status-url]
[![Go Report Card][goreport-svg]][goreport-url]
[![Docs][docs-godoc-svg]][docs-godoc-url]
[![Visualization][viz-svg]][viz-url]

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
cd gitscan
go build -o gitscan .
```

## Usage

```bash
gitscan <directory>              # Scan for issues
gitscan since <duration> [dir]   # Filter by modification time
gitscan dep <module> [dir]       # Filter by dependency
gitscan order [dir]              # Show repos in dependency order
```

### Root Command (Issue Scanning)

Scan repos for uncommitted changes, replace directives, and module mismatches:

```bash
gitscan ~/go/src/github.com/grokify
```

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--dir` | `-d` | (required) | Directory containing repos to scan |
| `--format` | `-f` | `list` | Output format: `list` or `table` |
| `--show-clean` | | `false` | Show repos with no issues |
| `--summary` | | `true` | Show summary at the end |
| `--go-git` | | `false` | Use go-git library instead of git CLI |

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
| `--recurse` | `-r` | `false` | Check nested go.mod files |
| `--go-git` | | `false` | Use go-git library instead of git CLI |

Duration formats: `7d` (days), `2w` (weeks), `1m` (months), `24h` (hours)

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
| `--go-git` | | `false` | Use go-git library instead of git CLI |

### Dep Examples

```bash
# Find repos depending on a module
gitscan dep github.com/grokify/mogo ~/go/src/github.com/grokify

# Include nested go.mod files (monorepos)
gitscan dep github.com/grokify/mogo -r ~/go/src/github.com/grokify
```

## Order Subcommand

Show repos in topological dependency order - dependencies first, then dependents. Helps determine the correct order to update and release Go modules.

```bash
gitscan order [directory]
```

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--dir` | `-d` | (required) | Directory containing repos to scan |
| `--since` | `-s` | (none) | Filter repos modified within duration |
| `--transitive` | `-t` | `false` | Include repos that transitively depend on modified repos |
| `--unpushed` | `-u` | `false` | Only show repos with uncommitted changes or unpushed commits |
| `--go-git` | | `false` | Use go-git library instead of git CLI |

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

### Git Backend Options

By default, gitscan uses the git CLI for repository status checks, which is fast and compatible with all git configurations. An optional `--go-git` flag enables the go-git library backend (pure Go, no process spawning):

| Backend | Speed | Compatibility |
|---------|-------|---------------|
| git CLI (default) | Fast (~2.5s for 600 repos) | Full compatibility |
| go-git (`--go-git`) | Slower (~10s for 600 repos) | Pure Go, no external deps |

The git CLI backend is recommended for most use cases. Use `--go-git` in environments where the git binary is unavailable.

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
