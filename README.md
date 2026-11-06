# SVCS: a simple version control system in Go

A small command-line version control system with no dependencies beyond the Go standard library. It tracks files, saves a full copy of them on each commit, and restores any earlier commit.

Built as the [Version Control System (Go)](https://hyperskill.org/projects/420) project on Hyperskill. [My Hyperskill profile](https://hyperskill.org/profile/621043631).

## Getting started

Requires Go 1.21 or later.

```sh
cd "Version Control System (Go)/task"
go build -o svcs main.go
```

## Commands

| Command | Description |
|---|---|
| `config [name]` | Get or set the username |
| `add [file]` | List tracked files, or start tracking a file |
| `log` | Show commits, newest first |
| `commit "message"` | Save the tracked files if they changed |
| `checkout <commit id>` | Restore the tracked files from a commit |
| `--help` | Show the help page |

## Example

```console
$ ./svcs config Ada
The username is Ada.
$ ./svcs add notes.txt
The file 'notes.txt' is tracked.
$ ./svcs commit "First draft"
Changes are committed.
$ echo "hello again" > notes.txt
$ ./svcs commit "Second draft"
Changes are committed.
$ ./svcs log
commit 42a3d7e807d751014f85e356663a9cd195b97655d3d86f159f722214e326f427
Author: Ada
Second draft
commit 38df97050b751b52d52961d6cb0d92aa7c56b2dc6332a91a5ce5210b3d374e96
Author: Ada
First draft
$ ./svcs checkout 38df97050b751b52d52961d6cb0d92aa7c56b2dc6332a91a5ce5210b3d374e96
Switched to commit 38df97050b751b52d52961d6cb0d92aa7c56b2dc6332a91a5ce5210b3d374e96.
```

## How it works

Everything is stored in a `vcs` folder in the current directory:

```text
vcs/
├── config.txt          # username
├── index.txt           # tracked files
├── log.txt             # commit history
└── commits/
    └── <commit id>/    # full copy of the tracked files
```

The commit ID is a SHA-256 hash of the tracked files' names and contents. If the hash matches the latest commit, nothing has changed and no commit is created.

## Limitations

This is a learning project, so don't use it for anything you can't afford to lose.

- **Use it with files in the current directory only.** Commits store files by name without their folder. Checking out a commit that tracks `src/a.txt` deletes that file and restores it as `./a.txt`, and files with the same name overwrite each other.
- **Checkout discards uncommitted changes** to tracked files, without warning.
- **Commits can be empty.** Committing with no tracked files still creates a commit.
- **Commit IDs can repeat.** Returning a file to an earlier state and committing again reuses that commit's ID and creates a second log entry for it. Committing right after a checkout does the same.
- **Files can't be untracked**, and `add` accepts a directory, which makes later commits fail.

## Tests

`Version Control System (Go)/task/test/Tests.kt` is Hyperskill's test suite for this project. It needs Hyperskill's test harness, so it doesn't run with `go test`.
