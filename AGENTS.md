# termcast

`termcast` is a small Go package that streams the screen of a `chromedp` page to
a terminal that has graphics (Kitty, iTerm2 or Sixel). It uses the screencast of
the Chrome DevTools Protocol and the package `github.com/kenshaw/rasterm`. The
module is `github.com/chromedp/termcast`. The example programs of
`chromedp/examples` use it for the flag `-visible-on-terminal`.

The package uses `chromedp` v0.20.0 and `cdproto` v0.157.8. It has no other
dependency, apart from `rasterm` and the Go standard library. The module needs
Go 1.25 or later.

## Standing rules

These hold in every `chromedp` repository, for every coding agent.

1. Stage changes for review. Commit and push only when the maintainer says so.
2. Load the `simple-english` skill before you write text that a person reads.
   Examples are a document, a code comment, an error message and a commit
   message. Follow the skill for that text.
3. Load the `go-pedantry` skill before you write or review Go code. Follow it
   where it does not conflict with a rule in this file. A rule here wins.

Questions and feature ideas go to GitHub Discussions of `chromedp/chromedp`,
and bugs go to issues. Do not open an issue for a question.

`CLAUDE.md` holds one line that imports this file, so that Claude Code and
every other agent read the same rules. Edit this file, not that one.

## Which document to read

`docs/decisions/README.md` is the index of every decision. A decision is one
file named by its date and a short title. Read the status of a decision before
you trust it, because a later decision can amend or replace it.

| If you are | Read |
| --- | --- |
| looking for what the package is and how to use it | `README.md` |
| asking why the stream clears the screen, holds the log lines and has no alternate screen | `docs/decisions/2026-10-04-the-stream-clears-the-screen-and-holds-the-log.md` |
| asking why something is the way it is | the index in `docs/decisions/README.md` |
| preparing a change as a person | `CONTRIBUTING.md` |

## Hard rules

1. The package uses only `chromedp`, `cdproto`, `rasterm` and the Go standard
   library. Do not add another dependency without asking the maintainer.
2. The stream must never leave the terminal in a changed state. Do not use the
   alternate screen. Do not hide the cursor. Do not rely on a deferred call,
   because `log.Fatal` skips it. See the decision for the stream.
3. Every method of `*Stream` must work on a nil pointer.
4. Wrap every error with `%w`. Write error messages in lower case, and do not
   start them with "failed to".
5. Do not leave a goroutine behind. A stopped stream and a canceled context must
   end every goroutine of the stream.
6. Never put a password, a key or a token in a file or in a message.

## Layout

| Path | Holds |
| --- | --- |
| `termcast.go` | the package comment, `Start`, `Stream` and the errors |
| `options.go` | the options and their defaults |
| `flags.go` | `Flags`, the helper for the flags of a program |
| `*_test.go` | the tests and the example. The tests need Chrome |
| `docs/decisions/` | the decisions and their index |
| `docs/docs_test.go` | the test of the documents and the Go comments |
| `.agents/skills/` and `.claude/skills/` | the two agent skills, as copies |
| `skills-lock.json` | the source of each skill |
| `.github/workflows/test.yml` | the workflow that builds, vets and tests |
| `go.mod`, `go.sum` | the module, which needs Go 1.25 |

The root of the repository holds `README.md`, `AGENTS.md`, `CLAUDE.md`,
`CONTRIBUTING.md` and `LICENSE` as text documents. Every other document goes in
`docs/`.

## Build and test

Run these commands in the repository root. Use `GOWORK=off`, so that no
workspace changes the result.

```sh
GOWORK=off go build ./...
GOWORK=off go vet ./...
GOWORK=off go test -race ./...
```

The tests start Chrome. If Chrome is not on the `PATH` under the name
`google-chrome`, `chromium` or `chrome`, link it there:

```sh
TMP=$(mktemp -d); ln -s /opt/google/chrome/chrome $TMP/google-chrome
PATH=$TMP:$PATH GOWORK=off go test -count=1 ./...
```

The tests need no terminal. They use a fake encoder and a local page that
animates.

## Before you commit

```sh
gofmt -l .
GOWORK=off go build ./...
GOWORK=off go vet ./...
GOWORK=off go test -race ./...
```

`gofmt -l .` must print nothing. `go test ./docs/` needs no browser. It tests
the links, the decision index, the document tables and the skill copies. It
also applies the prose rules to the documents and to the Go comments.

## Tags

Do not create a tag. The maintainer does.

## Writing documentation

Follow the `simple-english` skill for every word. Write sentences of 20 words
or fewer for a procedure, and 25 words or fewer for a description.

A new document goes in `docs/`. Add it to the tables in `README.md` and in this
file, and `go test ./docs/` fails if you do not.

Record a decision in a file in `docs/decisions/`. Name it
`YYYY-MM-DD-short-slug.md` with the date of the decision. Open it with
`# <Title>`, a blank line and `Status: Decided.`. Use `Proposed.`, `Open.`,
`Amends <file>.` or `Superseded by <file>.` when that is the status. Refer to a
decision by its file name, never by a number. State an amendment in both files.
Add a row to `docs/decisions/README.md`. The test prints the row for you.

The skills live in `.agents/skills` and `.claude/skills` as identical copies.
Never replace a copy with a symbolic link. `skills-lock.json` names the source
of each skill. The Claude Code permissions of one person go in
`.claude/settings.local.json`, which `.gitignore` lists.
