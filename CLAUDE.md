# CLAUDE.md

Project-specific instructions for Claude Code. See
`~/go/src/github.com/grokify/.github/CLAUDE.md` for org-wide conventions.

## Development

```bash
go build ./...              # Build the library and gitscan CLI
go test ./...                # Run tests
golangci-lint run            # Lint
go build -o gitscan ./cmd/gitscan
```

## Changelog Generation

Use `schangelog` (TOON output, ~8x more token-efficient than raw `git log`)
rather than raw git commands for commit analysis:

| Instead of | Use |
|------------|-----|
| `git log --oneline --reverse <tag>..HEAD` | `schangelog parse-commits --since=<tag>` |
| `git log --oneline \| head -N` | `schangelog parse-commits --last=N` |

Release workflow:

1. Parse commits: `schangelog parse-commits --since=<tag>`
2. Add a release entry to `CHANGELOG.json` (see existing entries for the
   category shape: `highlights`, `breaking`, `added`, `fixed`, `changed`,
   `tests`, `documentation`, `dependencies`, `performance`,
   `infrastructure`)
3. Validate: `schangelog validate CHANGELOG.json`
4. Regenerate: `schangelog generate CHANGELOG.json -o CHANGELOG.md`
5. Write `docs/releases/vX.Y.Z.md` (follow the format of prior release notes)
6. Add the new release page to `mkdocs.yml` nav
7. Update `docs/index.md` and `README.md` for any new user-facing feature
8. Bump the `version` const in `cmd/root.go`

Do not tag until commits are pushed and CI passes (per the org pre-push /
release-tagging checklist).

## Key Packages

- Root package (`gogit`) — repository discovery, commit-log parsing with
  trailers and change stats, AI-authorship detection, file/content access
  (`Repo.LsFiles`, `StagedFiles`, `ShowContent`, `LsTree`), exposure
  queries (`RefsContaining`, `TagsWithPath`, `FilesEverAdded`,
  `IgnoredFiles` in `exposure.go`), parallel multi-repo execution. Like
  `gitgrep`, exposure queries report facts only — no severity or policy.
- `gitgrep/` — policy-free content and history search (`GrepTree`,
  `HistoryPickaxe`, `StreamPatches`), by shelling out to native git. See
  `docs/gitgrep-design.md` for the full design (motivation, non-goals, API,
  caveats). Keep this package free of any domain policy (term lists,
  severities) — callers own that; gogit stays generic and reusable.
- `scanner/` — fleet enumeration (`ScanDirectory`) and `GitBackend` for
  multi-repo status/dependency scanning; backs the `gitscan` CLI.
- `cmd/` — the `gitscan` Cobra CLI (root scan, `since`, `dep`, `order`,
  `pending`, `pushed`, `grep`).

## Design Principle: Native Git, Not go-git

All git access shells out to the `git` CLI (via `exec.Command`) rather than
using a pure-Go git library. This is faster and lighter for history walks —
gitleaks itself dropped go-git for `git log -p` in v8.0.0 — and `git` being
on `PATH` is already assumed by any tool operating on git repositories. Keep
new git operations consistent with this: use `LC_ALL=C` and
`GIT_TERMINAL_PROMPT=0` (see `gogit.go`'s `git()` helper and
`gitgrep.runGit`). Prefer one batched git process over one per item — e.g.
`TagsWithPath` feeds every `<tag>:<path>` query to a single
`git cat-file --batch-check` via `gitStdin()` instead of spawning a process
per tag.

## No PubGuard Dependency

gogit is a generic, dependency-light git toolkit and must never import or
reference `github.com/grokify/pubguard` (or any other domain-specific
consumer). Consumers depend on gogit; gogit does not depend on them.
