# Contributing to termcast

`termcast` streams the screen of a `chromedp` page to a terminal that has
graphics. Read these two things before you change anything.

[`AGENTS.md`](AGENTS.md) holds the rules, the layout and the commands. It is
written for a coding agent, and everything in it applies to a person.

[`docs/decisions/`](docs/decisions/README.md) holds every decision, one file
each, named by date. Read the status of a decision before you trust it.

## Before you send a change

```sh
gofmt -l .
GOWORK=off go build ./...
GOWORK=off go vet ./...
GOWORK=off go test -race ./...
```

`gofmt -l .` must print nothing. The tests start Chrome, and they need no
terminal. `go test ./docs/` needs no browser. See [`AGENTS.md`](AGENTS.md) for
the way to put Chrome on the `PATH`.

## Writing

Write in plain English. Use short sentences and the active voice. The
`simple-english` skill in `.agents/skills` has the full rules. `go test ./docs/`
finds the violations that a machine can find, in the documents and in the Go
comments.

To record a decision, add a file to `docs/decisions/`. Name it with the date, as
in `2026-10-04-short-title.md`. Then add its row to the index. The test prints
the row for you.

## Questions

Ask in the [GitHub Discussions of `chromedp`](https://github.com/chromedp/chromedp/discussions),
and not in an issue. The issue tracker is for bugs.
