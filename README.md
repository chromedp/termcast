<p align="center"><img src="https://raw.githubusercontent.com/chromedp/logo/main/chromedp.svg" alt="chromedp logo" width="160"></p>

# About termcast

`termcast` is a small Go package that streams the screen of a [`chromedp`][1]
page to a terminal that has graphics. It uses the screencast of the Chrome
DevTools Protocol to get the frames, and [`rasterm`][2] to draw them. It works
in Kitty, iTerm2 and Sixel terminals. You start it with one call.

[![Unit Tests][termcast-ci-status]][termcast-ci]
[![Go Reference][goref-termcast-status]][goref-termcast]
[![Releases][release-status]][releases]
[![Discord Discussion][discord-status]][discord]

## Install

The module needs Go 1.25 or later. It requires `chromedp` v0.20.0 and `cdproto` v0.157.8.

```sh
go get github.com/chromedp/termcast
```

## Usage

Start the browser first, because the stream needs a page. Then call
`Flags.Start` or `Start` with the context. The stream stops when you call
`Stop`, and when the context ends.

```go
package main

import (
	"context"
	"flag"
	"log"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/chromedp/termcast"
)

func main() {
	var tc termcast.Flags
	tc.Register(flag.CommandLine)
	verbose := flag.Bool("v", false, "print the protocol messages")
	flag.Parse()

	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	// Load the page. This call starts the browser.
	if err := chromedp.Do(ctx, chromedp.Navigate("https://example.com")); err != nil {
		log.Fatal(err)
	}

	// s is nil when the flag -visible-on-terminal is off. A nil stream is
	// safe to use.
	s, err := tc.Start(ctx, *verbose)
	if err != nil {
		log.Fatal(err)
	}
	log.SetOutput(s.LogWriter())
	defer s.Stop()

	log.Println("the page is on the screen")
	if err := chromedp.Do(ctx, chromedp.Sleep(10*time.Second)); err != nil {
		s.Fatal(err)
	}
}
```

Run it with the flag:

```sh
go run . -visible-on-terminal
```

To start the stream with no flag, call `termcast.Start(ctx, opts...)`.

## How it works

The browser sends a frame only when the page changes. The stream keeps the
latest frame. At each tick it clears the terminal and draws the latest frame,
but only when the frame is new. A page that stops changing stays on the screen.
A program can end before the browser sends the first frame. In that case `Stop`
takes one screenshot, so that the final frame is never empty.

The stream clears the screen with `\x1b[H\x1b[2J` at each redraw. It uses no
alternate screen and does not hide the cursor. A program that ends with
`log.Fatal` skips its deferred calls, and the terminal must not stay in a
changed state. See
[the decision](docs/decisions/2026-10-04-the-stream-clears-the-screen-and-holds-the-log.md).

## Options

| Option | Default | Meaning |
| --- | --- | --- |
| `WithFPS(fps)` | 4 | The most frames per second that the stream draws |
| `WithQuality(q)` | 80 | The JPEG quality of the frames, from 0 to 100 |
| `WithMaxSize(w, h)` | 1024 by 768 | The largest size of a frame in pixels |
| `WithOutput(w)` | `os.Stdout` | The writer that receives the frames |
| `WithLogOutput(w)` | `os.Stderr` | The writer that receives the held log lines |
| `WithElement(sel)` | the whole page | Draws only the element that the selector finds |
| `WithEncoder(e)` | Kitty, iTerm2 or Sixel | The `rasterm.Encoder` that draws a frame |

`Flags` registers two flags: `-visible-on-terminal`, which turns the stream on,
and `-terminal-fps`, which sets the frame rate and defaults to 4.

## Programs that use it

The [`chromedp/examples`](https://github.com/chromedp/examples) repository has 34 programs that use `termcast`. Most of them take the flags `-visible-on-terminal` and `-terminal-fps` with `Flags`. The program [termcast](https://github.com/chromedp/examples/tree/main/termcast) calls `Start` directly. It plays an animated SVG and streams it to the terminal. These programs use the package: [click](https://github.com/chromedp/examples/tree/main/click), [console](https://github.com/chromedp/examples/tree/main/console), [cookie](https://github.com/chromedp/examples/tree/main/cookie), [dialogs](https://github.com/chromedp/examples/tree/main/dialogs), [download_file](https://github.com/chromedp/examples/tree/main/download_file), [download_image](https://github.com/chromedp/examples/tree/main/download_image), [dragdrop](https://github.com/chromedp/examples/tree/main/dragdrop), [emulate](https://github.com/chromedp/examples/tree/main/emulate), [eval](https://github.com/chromedp/examples/tree/main/eval), [eventsiter](https://github.com/chromedp/examples/tree/main/eventsiter), [exposefunc](https://github.com/chromedp/examples/tree/main/exposefunc), [extension](https://github.com/chromedp/examples/tree/main/extension), [fast](https://github.com/chromedp/examples/tree/main/fast), [forecast](https://github.com/chromedp/examples/tree/main/forecast), [frames](https://github.com/chromedp/examples/tree/main/frames), [geoip](https://github.com/chromedp/examples/tree/main/geoip), [headers](https://github.com/chromedp/examples/tree/main/headers), [intercept](https://github.com/chromedp/examples/tree/main/intercept), [keys](https://github.com/chromedp/examples/tree/main/keys), [latlon](https://github.com/chromedp/examples/tree/main/latlon), [logic](https://github.com/chromedp/examples/tree/main/logic), [pdf](https://github.com/chromedp/examples/tree/main/pdf), [pdfoptions](https://github.com/chromedp/examples/tree/main/pdfoptions), [proxy](https://github.com/chromedp/examples/tree/main/proxy), [remote](https://github.com/chromedp/examples/tree/main/remote), [screenshot](https://github.com/chromedp/examples/tree/main/screenshot), [selectors](https://github.com/chromedp/examples/tree/main/selectors), [structeval](https://github.com/chromedp/examples/tree/main/structeval), [submit](https://github.com/chromedp/examples/tree/main/submit), [subtree](https://github.com/chromedp/examples/tree/main/subtree), [termcast](https://github.com/chromedp/examples/tree/main/termcast), [text](https://github.com/chromedp/examples/tree/main/text), [upload](https://github.com/chromedp/examples/tree/main/upload) and [visible](https://github.com/chromedp/examples/tree/main/visible).

## Show one element

`WithElement` takes a `chromedp` selector, such as `chromedp.CSS("#chart")`, and
draws only the first element that it matches.

```go
s, err := termcast.Start(ctx, termcast.WithElement(chromedp.CSS("#chart")))
```

The stream looks up the element again at each frame, so an element that moves
or changes its size stays in view. It crops the frame to the border box of the
element. The browser sends the viewport, so the stream shows the part of the
element that is in the viewport. Use `chromedp.ScrollIntoView` first when the
element is lower on the page. While the page has no such element, the stream
keeps what is on the screen.

## Terminals that work

The terminal must support one of these graphics protocols:

- Kitty and Ghostty
- iTerm2, WezTerm and mintty
- Sixel

`rasterm` detects the protocol from the environment (`TERM`, `TERM_PROGRAM`,
`LC_TERMINAL` and the variables of Kitty) and, for Sixel, with a query to the
terminal. The variable `TERM_GRAPHICS` forces the protocol to `kitty`, `iterm`
or `sixel`, and the value `none` turns graphics off. `Start` returns
`ErrNoGraphics` when no protocol is available. `Available` reports the same
thing before you start.

## Log lines

A log line that the program prints draws over the frames, and the next redraw
erases it. `Stream.LogWriter` returns a writer that holds the lines while the
stream runs. When the stream stops, it draws a final frame and then prints the
lines to the log output. Use `log.SetOutput(s.LogWriter())`. After `Stop`, the
writer passes each line on at once.

`Stream.Fatal` stops the stream, prints the held lines, and then calls
`log.Fatal`. Use it where a program calls `log.Fatal`, because `log.Fatal` ends
the program before a deferred `Stop` runs.

Every method of `*Stream` is safe on a nil pointer. On a nil stream, `Stop` does
nothing, `Fatal` calls `log.Fatal` and `LogWriter` returns `os.Stderr`.

## The flag -v

The flag `-v` of a program prints the protocol messages. They draw over the
frames, so the two do not work together. `Flags.Start` returns `ErrVerbose`
when both are on.

## Tests

The tests start Chrome and need no terminal. They use a fake encoder and a local
page that animates. See [`AGENTS.md`](AGENTS.md) for the commands.

```sh
GOWORK=off go test -race ./...
```

## Contributing

See [`CONTRIBUTING.md`](CONTRIBUTING.md). Ask questions in the
[GitHub Discussions of `chromedp`][3], and report bugs in the
[issues of `chromedp`][4]. The documents of this repository are `AGENTS.md`,
`CLAUDE.md`, `CONTRIBUTING.md` and `docs/decisions/README.md`.

[1]: https://github.com/chromedp/chromedp
[2]: https://github.com/kenshaw/rasterm
[3]: https://github.com/chromedp/chromedp/discussions
[4]: https://github.com/chromedp/chromedp/issues
[termcast-ci]: https://github.com/chromedp/termcast/actions/workflows/test.yml (Test CI)
[termcast-ci-status]: https://github.com/chromedp/termcast/actions/workflows/test.yml/badge.svg (Test CI)
[goref-termcast]: https://pkg.go.dev/github.com/chromedp/termcast
[goref-termcast-status]: https://pkg.go.dev/badge/github.com/chromedp/termcast.svg
[release-status]: https://img.shields.io/github/v/release/chromedp/termcast?display_name=tag&sort=semver (Latest Release)
[releases]: https://github.com/chromedp/termcast/releases (Releases)
[discord]: https://discord.gg/WDWAgXwJqN "Discord Discussion"
[discord-status]: https://img.shields.io/discord/829150509658013727.svg?label=Discord&logo=Discord&colorB=7289da&style=flat-square "Discord Discussion"
