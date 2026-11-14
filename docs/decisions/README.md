# Decisions

Every decision of this project is a file in this folder. A file is named by its
date and a short title, as in
`2026-10-04-the-stream-clears-the-screen-and-holds-the-log.md`. This table is
the index.

Each file opens with its status. "Decided" means that the maintainer chose it.
"Proposed" means that somebody suggested it and the maintainer has not
confirmed it. "Open" means that nobody has chosen yet. A decision that changes
an earlier one says so in its status, as "Amends <file>". The earlier one says
it back, as "Amended by <file>" when the change is partial, or "Superseded by
<file>" when the new decision replaces it. Read the status before the decision.

Refer to a decision by its file name, never by a number. A new decision is a
new file with the date of the decision. Add its row here. `go test ./docs/`
fails when a decision has no row or a row is wrong, and it prints the row to
add.

| Date | Decision | Status |
| --- | --- | --- |
| 2026-10-04 | [The stream clears the screen and holds the log](2026-10-04-the-stream-clears-the-screen-and-holds-the-log.md) | Amended by 2026-11-14-options-only-and-the-kitty-stream.md |
| 2026-11-14 | [Options only, detection in Start and the Kitty stream](2026-11-14-options-only-and-the-kitty-stream.md) | Amends 2026-10-04-the-stream-clears-the-screen-and-holds-the-log.md |
