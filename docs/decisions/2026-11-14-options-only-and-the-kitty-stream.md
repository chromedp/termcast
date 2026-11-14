# Options only, detection in Start and the Kitty stream

Status: Amends 2026-10-04-the-stream-clears-the-screen-and-holds-the-log.md.

The maintainer decided on 2026-11-14 to rework the API of the package. There is
no compatibility with v0.2. The maintainer discussed the design with three
models, OpenAI Pro, Gemini Pro and DeepSeek Pro, and chose from their answers.

## Context

`rasterm` v0.3 removes `Available`. A program now finds the terminal with
`rasterm.Detect`, which asks the terminal with a query and returns a
`rasterm.Terminal`. The query reads the input of the terminal, so `Detect` also
returns the keys that the user typed while it waited.

`rasterm` v0.3 also adds `KittyStream`. It shows a sequence of pictures of one
size as one Kitty image, and the terminal stores two frames. `Draw` writes the
next picture into the frame that is not shown, and then shows it. The picture
changes with no clear and no flicker.

The type `Flags` of v0.2 registered two flags on a `flag.FlagSet` and took a
bare `verbose` bool. It tied the package to the package `flag`, it owned the
names of the flags, and it did not fit a program that reads its settings in
another way.

## Decision

1. The package has no `Flags`. A program configures the stream only with
   options. The program owns its flags, and calls `Start` only when its flag
   asks for the stream. An option that makes `Start` return no stream was
   rejected, because a nil stream and a nil error mean "off", and not "started".
2. A nil `*Stream` is a stream that is off. `Stop`, `LogWriter`, `Terminal` and
   `Keys` are safe on it. The programs keep `defer s.Stop()` and
   `log.SetOutput(s.LogWriter())` with no check.
3. `Start` calls `rasterm.Detect`. `WithTerminal` gives the terminal and skips
   detection, and `WithDetectOptions` passes options to `Detect`.
   `Stream.Keys` returns the keys that detection read, and `Stream.Terminal`
   returns the terminal.
4. `Start` returns `ErrNoGraphics` when the protocol is `rasterm.None`. It does
   not fall back to half blocks. `TERM_GRAPHICS=halfblock` selects them.
5. On the Kitty protocol, the stream draws each frame into a `KittyStream`. It
   clears the screen once, for the first frame. When the size of the frame
   changes, it deletes the image, clears the screen and starts a new
   `KittyStream`. Other protocols clear the screen at each redraw, as before.
6. The Kitty stream needs a terminal that plays Kitty animations. A terminal
   that draws Kitty images and has no animation shows only the first frame. By
   default, the stream uses the Kitty stream when the terminal reports that it
   plays animations, or names itself kitty or Ghostty, which play them and do
   not report it. `WithAnimation` turns it on or off.
7. The decoded frames keep their JPEG, with `rasterm.DecodeJPEG`, so the iTerm2
   encoder sends the JPEG as it is.
8. `Stop` returns the first error of the stream. `Err` is gone.
9. `Fatal` is gone, because a package must not end the program. A program calls
   `s.Stop()` before `log.Fatal`.
10. `ErrVerbose` is gone. Output that draws over the frames, such as the
    protocol messages of `-v`, is a choice of the program.
11. `WithEncoder` is gone. The stream needs the protocol to choose the Kitty
    stream, and `WithTerminal` gives it.

## Why

The models agreed on points 1, 9, 10 and 11. On detection, Gemini Pro wanted
the program to call `Detect` and to pass the terminal, because detection reads
the input. OpenAI Pro and DeepSeek Pro wanted `Start` to detect and to give
back the keys. The maintainer chose the second, with `WithTerminal` for a
program that detects by itself. On the Kitty stream, OpenAI Pro and Gemini Pro
wanted it on by default, and DeepSeek Pro wanted it off by default, because the
report of the terminal is not reliable. The default of point 6 uses the Kitty
stream only where it is known to work.

## Consequences

Each example program defines its own flags, or uses a helper of the examples
repository. The `chromedp` README shows the new pattern.

Point 2 of the decision of 2026-10-04 holds only when there is no Kitty stream.
Its point 6 and the `Fatal` part of its point 8 no longer hold.
