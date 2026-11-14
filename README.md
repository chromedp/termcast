<p align="center"><img src="https://raw.githubusercontent.com/chromedp/logo/main/chromedp.svg" alt="chromedp logo" width="160"></p>

# About termcast

`termcast` is a small Go package that streams the screen of a [`chromedp`][1]
page to a terminal that has graphics. It uses the screencast of the Chrome
DevTools Protocol to get the frames, and [`rasterm`][2] to draw them. It works
in Kitty, iTerm2 and Sixel terminals. You start it with one call, and on a
Kitty terminal it draws the frames with no flicker.

[![Unit Tests][termcast-ci-status]][termcast-ci]
[![Go Reference][goref-termcast-status]][goref-termcast]
[![Releases][release-status]][releases]
[![Discord Discussion][discord-status]][discord]

## Install

The module needs Go 1.25 or later. It requires `chromedp` v0.23.1, `cdproto`
v0.160.1 and `rasterm` v0.3.1.

```sh
go get github.com/chromedp/termcast
```

## Usage

Load the page first, so that the stream has a page to show. `Start` also starts
the browser when the context has none. The stream stops when you call `Stop`,
and when the context ends.

The program owns its flags. Start the stream only when the flag of the program
asks for it. A nil `*Stream` is a stream that is off, and its methods are safe,
so the program calls `Stop` and `LogWriter` in either case.

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
	onTerminal := flag.Bool("visible-on-terminal", false, "draw the page in the terminal")
	fps := flag.Float64("terminal-fps", termcast.DefaultFPS, "frames per second of the stream")
	flag.Parse()

	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	// Load the page. This call starts the browser.
	if err := chromedp.Do(ctx, chromedp.Navigate("https://example.com")); err != nil {
		log.Fatal(err)
	}

	var s *termcast.Stream
	if *onTerminal {
		var err error
		if s, err = termcast.Start(ctx, termcast.WithFPS(*fps)); err != nil {
			log.Fatal(err)
		}
	}
	defer s.Stop()
	log.SetOutput(s.LogWriter())

	log.Println("the page is on the screen")
	if err := chromedp.Do(ctx, chromedp.Sleep(10*time.Second)); err != nil {
		// log.Fatal skips the deferred calls, so stop the stream first.
		s.Stop()
		log.Fatal(err)
	}
}
```

Run it with the flag:

```sh
go run . -visible-on-terminal
```

## How it works

`Start` asks the terminal which graphics protocols it has, with
`rasterm.Detect`. Then it starts the screencast of the page.

The browser sends a frame only when the page changes. The stream keeps the
latest frame and draws it at each tick, but only when the frame is new. A page
that stops changing stays on the screen. A program can end before the browser
sends the first frame. In that case `Stop` takes one screenshot, so that the
final frame is never empty.

The stream draws a frame in one of two ways:

1. On a Kitty terminal that plays animations, it draws each frame into one
   Kitty image with `rasterm.KittyStream`. The picture changes at once, with no
   clear and no flicker. When the size of the frame changes, the stream deletes
   the image, clears the screen once and starts a new image.
2. On other terminals, it clears the screen with `\x1b[H\x1b[2J` and draws the
   frame.

The stream uses no alternate screen and does not hide the cursor. A program
that ends with `log.Fatal` skips its deferred calls, and the terminal must not
stay in a changed state. See
[the decision](docs/decisions/2026-10-04-the-stream-clears-the-screen-and-holds-the-log.md)
and [the decision for the new API](docs/decisions/2026-11-14-options-only-and-the-kitty-stream.md).

## Options

| Option | Default | Meaning |
| --- | --- | --- |
| `WithFPS(fps)` | 4 | The most frames per second that the stream draws |
| `WithQuality(q)` | 80 | The JPEG quality of the frames, from 0 to 100 |
| `WithMaxSize(w, h)` | 1024 by 768 | The largest size of a frame in pixels |
| `WithOutput(w)` | `os.Stdout` | The writer that receives the frames |
| `WithLogOutput(w)` | `os.Stderr` | The writer that receives the held log lines |
| `WithElement(sel)` | the whole page | Draws only the element that the selector finds |
| `WithTerminal(t)` | detection | The `rasterm.Terminal` to draw on, with no detection |
| `WithDetectOptions(opts...)` | none | The options of `rasterm.Detect`, such as a timeout |
| `WithAnimation(on)` | the terminal decides | Turns the Kitty stream on or off |

## Programs that use it

The [`chromedp/examples`](https://github.com/chromedp/examples) repository has 34 programs that use `termcast`. Most of them take the flags `-visible-on-terminal` and `-terminal-fps`, which a small helper of that repository defines. The program [termcast](https://github.com/chromedp/examples/tree/main/termcast) calls `Start` directly. It plays an animated SVG and streams it to the terminal. These programs use the package: [click](https://github.com/chromedp/examples/tree/main/click), [console](https://github.com/chromedp/examples/tree/main/console), [cookie](https://github.com/chromedp/examples/tree/main/cookie), [dialogs](https://github.com/chromedp/examples/tree/main/dialogs), [download_file](https://github.com/chromedp/examples/tree/main/download_file), [download_image](https://github.com/chromedp/examples/tree/main/download_image), [dragdrop](https://github.com/chromedp/examples/tree/main/dragdrop), [emulate](https://github.com/chromedp/examples/tree/main/emulate), [eval](https://github.com/chromedp/examples/tree/main/eval), [eventsiter](https://github.com/chromedp/examples/tree/main/eventsiter), [exposefunc](https://github.com/chromedp/examples/tree/main/exposefunc), [extension](https://github.com/chromedp/examples/tree/main/extension), [fast](https://github.com/chromedp/examples/tree/main/fast), [forecast](https://github.com/chromedp/examples/tree/main/forecast), [frames](https://github.com/chromedp/examples/tree/main/frames), [geoip](https://github.com/chromedp/examples/tree/main/geoip), [headers](https://github.com/chromedp/examples/tree/main/headers), [intercept](https://github.com/chromedp/examples/tree/main/intercept), [keys](https://github.com/chromedp/examples/tree/main/keys), [latlon](https://github.com/chromedp/examples/tree/main/latlon), [logic](https://github.com/chromedp/examples/tree/main/logic), [pdf](https://github.com/chromedp/examples/tree/main/pdf), [pdfoptions](https://github.com/chromedp/examples/tree/main/pdfoptions), [proxy](https://github.com/chromedp/examples/tree/main/proxy), [remote](https://github.com/chromedp/examples/tree/main/remote), [screenshot](https://github.com/chromedp/examples/tree/main/screenshot), [selectors](https://github.com/chromedp/examples/tree/main/selectors), [structeval](https://github.com/chromedp/examples/tree/main/structeval), [submit](https://github.com/chromedp/examples/tree/main/submit), [subtree](https://github.com/chromedp/examples/tree/main/subtree), [termcast](https://github.com/chromedp/examples/tree/main/termcast), [text](https://github.com/chromedp/examples/tree/main/text), [upload](https://github.com/chromedp/examples/tree/main/upload) and [visible](https://github.com/chromedp/examples/tree/main/visible).

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

`Start` calls `rasterm.Detect`. It asks the terminal with a query, and reads the
environment when the terminal does not answer. The variable `TERM_GRAPHICS`
forces the protocol to `kitty`, `iterm`, `sixel` or `halfblock`, and the value
`none` turns graphics off. `Start` returns `ErrNoGraphics` when no protocol is
available.

The query reads the input of the terminal. When the user types while `Start`
waits for the answer, `Stream.Keys` returns those keys. A program that reads its
input must use them as its first input. `Stream.Terminal` returns what
detection found.

To skip detection, give the terminal with `WithTerminal`. Use it when the
output is not the terminal, for example when the frames go to a file:

```go
s, err := termcast.Start(ctx,
	termcast.WithOutput(f),
	termcast.WithTerminal(rasterm.Terminal{Protocol: rasterm.Kitty}),
)
```

The Kitty stream needs a terminal that plays Kitty animations. By default the
stream uses it when the terminal reports that it plays them, or names itself
kitty or Ghostty. Other terminals that draw Kitty images get a clear at each
frame. `WithAnimation(true)` forces the Kitty stream, and `WithAnimation(false)`
turns it off.

## Log lines

A log line that the program prints draws over the frames. `Stream.LogWriter`
returns a writer that holds the lines while the stream runs. When the stream
stops, it draws a final frame and then prints the lines to the log output. Use
`log.SetOutput(s.LogWriter())`. After `Stop`, the writer passes each line on at
once.

`log.Fatal` ends the program before a deferred `Stop` runs. So call `s.Stop()`
before `log.Fatal`, and the final frame and the held lines reach the terminal.
`Stop` returns the first error that the stream met.

Every method of `*Stream` is safe on a nil pointer. On a nil stream, `Stop` does
nothing and returns nil, `LogWriter` returns `os.Stderr`, `Terminal` returns the
zero `rasterm.Terminal` and `Keys` returns nil.

## Other output

Output that draws over the frames does not work together with the stream. An
example is the protocol messages of `chromedp.WithDebugf`, which the flag `-v`
of the examples turns on. The program decides which one it shows. The examples
stop with an error when both flags are on.

## Tests

The tests start Chrome and need no terminal. They give the terminal with
`WithTerminal`, use a fake encoder or a buffer, and load a local page that
animates. See [`AGENTS.md`](AGENTS.md) for the commands.

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
