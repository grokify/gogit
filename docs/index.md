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
  for `Co-authored-by`), `--numstat` change stats, and an optional message
  body (`LogOptions.IncludeBody`, opt-in like stats since it costs more per
  commit).
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
  the branch's push target: its upstream, the matching remote-tracking
  branch (e.g. `origin/main`), or — for a branch never pushed under its own
  name — the remote's default branch, so only commits unique to the branch
  are reported. Only when none of these resolve is every commit reachable
  from `HEAD` reported as pending. `Repo.PushedCommits` lists the most
  recent commits already pushed (unaffected by the default-branch
  fallback, since it reports what this branch itself has pushed). Both
  results carry the current branch name (`Branch`), so a caller reporting
  "no upstream" also knows which branch that refers to. `Repo.HasUpstream`
  reports whether an upstream is configured.
- **File & content access** — `Repo.LsFiles` (tracked and, optionally,
  untracked files), `Repo.StagedFiles` (staged additions/modifications),
  `Repo.ShowContent` (object content at a git spec, e.g. `:path` for the
  staged version), `Repo.LsTree` (a revision's full file list), and
  `Repo.LastCommitTouching` (the most recent commit at or before a revision
  that modified a path).
- **Exposure** — answer how far a commit or file has travelled.
  `Repo.RefsContaining` lists the branches, remote-tracking branches, and
  tags containing a commit (symbolic refs such as `origin/HEAD` omitted);
  `Repo.TagsWithPath` lists the tags whose tree contains a path, separating
  content that only lives in history from content that shipped in a
  release; `Repo.FilesEverAdded` lists every path ever added on any ref,
  including files since deleted from `HEAD`; and `Repo.IgnoredFiles` lists
  untracked paths excluded by ignore rules.

## Exposure Queries

Once a repository scan turns something up — a credential, a file that
should never have been committed — the next question is what that finding
reaches. Four `Repo` methods answer it without ad-hoc git commands:

| Question | Method | Git equivalent |
|----------|--------|----------------|
| Is this commit pushed? Released? | `RefsContaining(ctx, commit)` | `git for-each-ref --contains` |
| Did this path ship in a tagged release? | `TagsWithPath(ctx, path)` | `git cat-file --batch-check` over `<tag>:<path>` |
| What has ever been committed, including since-deleted files? | `FilesEverAdded(ctx)` | `git log --all --diff-filter=A` |
| What exists locally but is ignored? | `IgnoredFiles(ctx)` | `git ls-files --others --ignored --directory` |

```go
repo, _ := gogit.Open("/path/to/repo")

// Where has the commit that introduced a file gone?
refs, _ := repo.RefsContaining(ctx, "a1b2c3d")
for _, ref := range refs {
    fmt.Println(ref.Kind, ref.Name) // branch main, remote origin/main, tag v1.2.0
}

// Was the file itself part of any tagged release tree?
tags, _ := repo.TagsWithPath(ctx, "config/local.env")
fmt.Println(len(tags) > 0)
```

`RefsContaining` and `TagsWithPath` answer different questions: a tag cut
after the commit that added a file is *downstream* of it, but does not
*contain* the file if a later commit deleted it before tagging. For Go
modules the tagged tree is what `proxy.golang.org` archives permanently, so
`TagsWithPath` is the check that tells "only in history" apart from
"published in a release."

Notes:

- `RefsContaining` omits symbolic refs (`origin/HEAD`) and returns an error
  for an unknown commit, so a typo never reads as "not pushed." Remote refs
  reflect the last fetch.
- `TagsWithPath` accepts files or directories relative to the repository
  root, and checks every tag in one git process.
- `FilesEverAdded` disables rename detection, so a renamed file appears
  under both names. Files introduced only by a merge commit's conflict
  resolution are not reported, matching `git log`'s default.
- `IgnoredFiles` collapses fully ignored directories to a single `dir/`
  entry and honors the global excludes file.

## gitgrep

A policy-free primitive for searching a repository's content and history,
by shelling out to native git. Callers supply patterns and get matches back;
`gitgrep` has no built-in notion of what a match means, so it is a
general-purpose building block rather than a leak/secret scanner itself.

- **`GrepTree`** — search the working tree, the index (`Staged`), or a
  revision (`Rev`) for one or more patterns (fixed-string or extended
  regex, case-insensitive optional).
- **`HistoryPickaxe`** — find commits whose diff added or removed a pattern
  (`git log -S`/`-G`), attributed to the specific file.
- **`StreamPatches`** — stream `git log -p` file diffs with commit/author/
  date context, for running custom detectors over a single git walk.

```go
matches, _ := gitgrep.GrepTree(ctx, repoPath, gitgrep.Options{
    Patterns: []gitgrep.Pattern{{Value: "Acme Corp", IgnoreCase: true}},
})
```

`Match.Text` and `Patch.Hunk` are returned verbatim; redacting them before
display or logging is the caller's responsibility. See the
[gitgrep design note](https://github.com/grokify/gogit/blob/main/docs/gitgrep-design.md)
for the full API and caveats.

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
terminal table, copy-pasteable markdown, or JSON for agents. `grep`
searches a single repository's working tree, staged index, a revision,
or history for one or more patterns.

```bash
go install github.com/grokify/gogit/cmd/gitscan@latest
```

See the [README](https://github.com/grokify/gogit#readme) for full CLI
usage, and [Releases](releases/v0.13.1.md) for version history.
