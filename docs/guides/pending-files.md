# Review What a Push Will Publish

`gitscan pending --files` lists every file path touched by the commits you
have made locally but not yet pushed. Use it as the last look before a push,
or as the input to a check that should run over exactly what is about to
leave your machine.

## One repository

```bash
gitscan pending --files .
```

```text
Repo: /home/me/src/acme/billing (branch: main)
Pending commits (not yet pushed to @{upstream}): 3

#  HASH     DAY  TIMESTAMP                  MESSAGE
1  a1b2c3d  Thu  2026-10-08T09:12:04-07:00  feat: add invoice export
2  d4e5f6a  Thu  2026-10-08T09:40:31-07:00  fix: round tax to cents
3  0f9e8d7  Thu  2026-10-08T10:02:55-07:00  docs: describe the export format

Files (4):
  docs/export.md
  go.mod
  internal/invoice/export.go
  internal/invoice/export_test.go
```

The list is merged across the commits: a file changed by all three commits
appears once. It is sorted, and every path is relative to the repository root.

## Many repositories

Pass several paths, or a directory of repositories, and each repository gets
its own list:

```bash
gitscan pending --files ~/src/acme ~/src/me
```

Repositories with nothing pending are left out of the table. For scripts, use
JSON, which has a `files` array on each repository:

```bash
gitscan pending --files --format json ~/src/acme \
  | jq -r '.repos[] | .repo as $r | .files[]? | "\($r)/\(.)"'
```

That prints absolute paths, one per line, ready to pipe into another tool. The
`files` key is left out for a repository with nothing pending, hence the `?`.

## What the list contains

The list is every path the pending commits carry, which is not always the same
as what is in the tree after them:

- A file that a later pending commit **deleted** is still listed.
- A **rename** lists both the old and the new path.
- **Merge commits** add nothing of their own. The commits they merge are in the
  range and are counted directly.

This is deliberate: a file that was added and then removed before the push is
still in the pushed history, so a check over "what this push publishes" should
see it. If you want only the paths that exist in the final tree, use git
directly:

```bash
git diff --name-only @{upstream}..HEAD
```

## Choosing the range

By default the range is the commits ahead of the branch's push target: its
configured upstream, or the matching remote-tracking branch. A branch that has
never been pushed is compared with the remote's default branch, so only the
commits unique to the branch count. If there is no remote at all, every commit
is pending.

To list the files since a specific commit instead, for one repository:

```bash
gitscan pending --files --since-commit abc1234 .
```

The commit list and the file list always describe the same range.

## Limits

- The list has paths only. It does not include file contents, per-file change
  status, or which commit touched which file.
- It reports facts, not findings. What to do with the paths, such as scanning
  them for secrets or internal names, is up to the tool you pipe them into.
