# The stream clears the screen and holds the log

Status: Decided.

## Context

The example programs of `chromedp` get a flag, `-visible-on-terminal`. With it,
a program draws the page in a terminal that has graphics. The `termcast`
package does the drawing. A program can end with `log.Fatal`, which skips every
deferred call. The package must work with that.

## Decision

1. The package has one call to start, `Start`. It starts the screencast of the
   DevTools protocol and draws each frame with the package `rasterm`.
2. At each redraw, the package moves the cursor to the top left and clears the
   screen with `\x1b[H\x1b[2J`. It then draws the latest frame. It does not
   clear the scrollback.
3. The package uses no alternate screen and does not hide the cursor. A program
   that ends with `log.Fatal` skips its deferred calls, so the package cannot
   restore the terminal. The terminal is never left in a changed state.
4. The default rate is 4 frames per second, and `WithFPS` changes it. The
   browser sends a frame only when the page changes. The package keeps the
   latest frame and draws it again on the timer only when it is new. A page that
   stops changing stays on the screen.
5. `Start` checks that the terminal has graphics. When it does not, it returns
   `ErrNoGraphics`. The message names the terminals that work (Kitty, iTerm2 and
   a Sixel terminal) and the variable `TERM_GRAPHICS` that `rasterm` reads.
6. The flag `-v` of the examples prints the protocol messages. It does not work
   with the stream, because the messages draw over the frames. `Flags.Start`
   returns `ErrVerbose` when both are on.
7. The log lines of the program must not be lost, and the next redraw must not
   erase them. `Stream.LogWriter` holds the lines while the stream runs. When the
   stream stops, the package draws a final frame and then prints the lines.
8. Every method of `*Stream` is safe on a nil pointer. A program writes
   `s.Stop()`, `s.Fatal(err)` and `s.LogWriter()` with no check, whether the
   stream is on or off. `Fatal` on a nil stream calls `log.Fatal`. On a stream
   that runs, it stops the stream first and then calls `log.Fatal`.

## Why

The examples are short programs, and many of them end with `log.Fatal`. A
design that needs a deferred call to restore the terminal fails in the case
that matters most, an error. Clearing and drawing again needs nothing to
restore. The cost is a small flicker at each redraw.

Holding the log lines keeps them out of the way of the frames. The program
still prints each line, at the end, and a program that fails shows its error
under the last frame.

## Consequences

A long run holds every log line in memory until the stream stops. The package
accepts this, because the examples log a few lines.

The stream also stops by itself when the context of `Start` ends. It then draws
the last frame that it has, if any, and prints the held lines, so a program
that forgets `Stop` loses no log line.
