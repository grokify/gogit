# Catch Unsigned Commits Before You Push

`gitscan pending` shows whether each unpushed commit is signed, and warns when
some are not. Use it as the last look before a push, so a missing signature
does not first show up as an "Unverified" badge on GitHub.

## One repository

```bash
gitscan pending .
```

```text
Repo: /home/me/src/acme/billing (branch: main)
Pending commits (not yet pushed to @{upstream}): 3

#  HASH     SIGNED  DAY  TIMESTAMP                  MESSAGE
1  a1b2c3d  yes     Thu  2026-10-08T09:12:04-07:00  feat: add invoice export
2  d4e5f6a  NO      Thu  2026-10-08T09:40:31-07:00  fix: round tax to cents
3  0f9e8d7  yes     Thu  2026-10-08T10:02:55-07:00  docs: describe the export format

WARNING: 1 of 3 commits are not signed.
```

The SIGNED column reads `yes` or `NO`. The warning appears when more than one
commit is listed and at least one is unsigned. A lone unsigned commit shows
`NO` in its row but gets no warning line.

Fix an unsigned commit before pushing by re-signing the range:

```bash
git rebase --exec 'git commit --amend --no-edit -S' @{upstream}
```

## Already-pushed commits

`gitscan pushed` has the same column and warning, so you can check what is
already on the remote:

```bash
gitscan pushed 10 .
```

## Many repositories

A fleet sweep shows the column and the warning per repository, so the one
repo with an unsigned commit stands out:

```bash
gitscan pending ~/src/acme ~/src/me
```

## For scripts and agents

In JSON each commit has a `signed` boolean, and each repository has an
`unsigned` count that is omitted when it is zero:

```bash
gitscan pending --format json ~/src/acme \
  | jq -r '.repos[] | select(.unsigned) | "\(.repo): \(.unsigned) unsigned"'
```

In `--format markdown` the table gains the same Signed column, so a pasted PR
description shows it too.

## What "signed" means

`gitscan` checks that the commit **carries** a signature. It does not verify
it. That keeps the check fast and independent of your setup: no keys, no
`gpg.ssh.allowedSignersFile`, and no call out to `gpg` or your SSH signing
program.

Git's own `%G?` status is not used, because with SSH signing and no
allowed-signers file it reports `N` (no signature) for commits that are signed.
That would flag every commit as unsigned.

A commit can carry a signature and still show as "Unverified" on GitHub, for
example when the signing key is not registered on your account or does not
match the committer email. `gitscan` reports `yes` for such a commit. It
catches the common case, commits that were never signed at all, and not a
signature GitHub will not accept.

## From Go

```go
reports, err := report.Pending(ctx, []string{"."}, report.PendingOptions{})
if err != nil { /* invalid options */ }
for _, rep := range reports {
    if msg := gogit.SignatureWarning(rep.Commits); msg != "" {
        fmt.Printf("%s: %s\n", rep.Repo, msg)
    }
}
```

Lower-level, `Repo.PendingCommits` and `Repo.PushedCommits` set
`Commit.Signed` (a `*bool`), and `PendingResult.UnsignedCount()` and
`UnsignedWarning()` give the summary. For any other query, set
`LogOptions.IncludeSignature`. `Signed` is nil when signature state was not
collected, and `Commit.IsUnsigned()` is true only for a commit known to be
unsigned.
