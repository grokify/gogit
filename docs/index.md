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
  commits after a given SHA (`sha..HEAD`) for high-water-mark workflows;
  `LogOptions.Rev` logs from an arbitrary revision instead of HEAD.
- **Reverse order** — `LogOptions.Reverse` returns commits oldest-first.
- **AI authorship** — `AnalyzeAuthorship` detects AI coding assistants
  (Claude Code, GitHub Copilot, Gemini CLI, Cursor, Aider) from
  `Co-authored-by` trailers, extracting tool names and model identities.
- **AI co-author parsing** — `Commit.AICoAuthors()` identifies AI tools
  from co-author trailers with model version extraction (e.g., "Sonnet 4").
- **Commit stats by category** — `Repo.CollectCommitStats` and
  `AggregateCommitStats` aggregate commit counts and LOC by conventional-
  commit type, with AI-assisted metrics per tool and model.
- **Conventional commits** — `ParseConventionalCommit` extracts type,
  scope, breaking flag, and subject from conventional commit messages.
- **Trailer lookup** — `Commit.TrailerValue` and `TrailerValues` provide
  case-insensitive access to any trailer key.
- **Parallel execution** — `RunAll` and `RunAllWithProgress` execute
  operations across multiple repositories concurrently with configurable
  worker count and context cancellation.
- **Metadata** — `Repo.Branch`, `Repo.OriginURL`, `NormalizeRemoteURL`
  (canonical `host/path` identifiers), `Repo.Tags`, `Repo.TagsWithDates`.
- **Pending & pushed commits** — `Repo.PendingCommits` lists commits ahead of
  the branch's push target (its upstream, or the matching remote-tracking
  branch such as `origin/main`), or after an explicit commit hash; a branch
  that was never pushed reports every commit as pending. `Repo.PushedCommits`
  lists the most recent commits already pushed. `Repo.HasUpstream` reports
  whether an upstream is configured.

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
dependency filters, release ordering, and workflow compliance. The
`pending` subcommand reports unpushed commits across one or many
repositories (sweep several org directories at once), and `pushed`
lists the most recent commits already pushed — both as an aligned
terminal table, copy-pasteable markdown, or JSON for agents.

```bash
go install github.com/grokify/gogit/cmd/gitscan@latest
```

See the [README](https://github.com/grokify/gogit#readme) for full CLI
usage, and [Releases](releases/v0.10.0.md) for version history.
