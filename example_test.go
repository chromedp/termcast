package termcast_test

import (
	"context"
	"flag"
	"log"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/chromedp/termcast"
)

// A program has its own flag for the stream, and starts the stream only when
// the flag is on. A nil stream is a stream that is off, so the program calls
// Stop and LogWriter in either case.
func Example() {
	onTerminal := flag.Bool("visible-on-terminal", false, "draw the page in the terminal")
	fps := flag.Float64("terminal-fps", termcast.DefaultFPS, "frames per second of the stream")
	flag.Parse()

	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	// Load the page. This call starts the browser.
	if err := chromedp.Do(ctx, chromedp.Navigate("https://example.com")); err != nil {
		log.Fatal(err)
	}

	// Start the stream. Start returns termcast.ErrNoGraphics for a terminal
	// that has no graphics.
	var s *termcast.Stream
	if *onTerminal {
		var err error
		if s, err = termcast.Start(ctx, termcast.WithFPS(*fps)); err != nil {
			log.Fatal(err)
		}
	}
	defer s.Stop()
	// The log lines wait until the stream stops, so that they do not draw over
	// the frames.
	log.SetOutput(s.LogWriter())

	log.Println("the page is on the screen")
	if err := chromedp.Do(ctx, chromedp.Sleep(5*time.Second)); err != nil {
		// log.Fatal skips the deferred calls. Stop first, so that the final
		// frame and the held log lines reach the terminal.
		s.Stop()
		log.Fatal(err)
	}
}
