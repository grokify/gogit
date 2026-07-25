# gogit

Generic, dependency-light Git ergonomics for Go, by shelling out to the
git CLI. The base layer for higher-level tools — including the bundled
`gitscan` CLI and the OmniDevX telemetry collectors — in the same way
[gogithub](https://github.com/grokify/gogithub) underlies GitHub
integrations.

## Library

- **Discovery** — `Discover(roots, maxDepth)` finds repositories under
  directory roots without descending into them.
- **Commit log** — `Repo.Log` parses commits with calendar-date filtering,
  author/committer identities, subjects, trailers (`Commit.CoAuthors()`
  for `Co-authored-by`), and `--numstat` change stats.
- **Incremental ingestion** — `LogOptions.SinceCommit` limits output to
  commits after a given SHA (`sha..HEAD`) for high-water-mark workflows.
- **Reverse order** — `LogOptions.Reverse` returns commits oldest-first.
- **AI authorship** — `AnalyzeAuthorship` detects AI coding assistants
  (Claude Code, GitHub Copilot, Gemini CLI, Cursor, Aider) from
  `Co-authored-by` trailers, extracting tool names and model identities.
- **Conventional commits** — `ParseConventionalCommit` extracts type,
  scope, breaking flag, and subject from conventional commit messages.
- **Trailer lookup** — `Commit.TrailerValue` and `TrailerValues` provide
  case-insensitive access to any trailer key.
- **Parallel execution** — `RunAll` and `RunAllWithProgress` execute
  operations across multiple repositories concurrently with configurable
  worker count and context cancellation.
- **Metadata** — `Repo.Branch`, `Repo.OriginURL`, `NormalizeRemoteURL`
  (canonical `host/path` identifiers), `Repo.Tags`, `Repo.TagsWithDates`.

```go
repo, _ := gogit.Open("/path/to/repo")
commits, _ := repo.Log(ctx, gogit.LogOptions{
    Since:        weekStart,
    IncludeStats: true,
})
for _, c := range commits {
    attr := gogit.AnalyzeAuthorship(c)
    fmt.Println(c.Hash, attr.Tools, c.Insertions)
}
```

Install:

```bash
go get github.com/grokify/gogit
```

## gitscan CLI

Scan many repositories for ones needing attention: uncommitted or
unpushed changes, `replace` directives, module/directory mismatches,
dependency filters, release ordering, and workflow compliance.

```bash
go install github.com/grokify/gogit/cmd/gitscan@latest
```

See the [README](https://github.com/grokify/gogit#readme) for full CLI
usage, and [Releases](releases/v0.6.0.md) for version history.
