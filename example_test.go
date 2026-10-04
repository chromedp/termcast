package termcast_test

import (
	"context"
	"flag"
	"log"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/chromedp/termcast"
)

// A program registers the flags, starts the browser, and starts the stream
// after the page has loaded. The stream is nil when the flag is off, and a nil
// stream is safe to use.
func Example() {
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

	// Start the stream. It returns termcast.ErrVerbose for -v together with
	// -visible-on-terminal, and termcast.ErrNoGraphics for a terminal that has
	// no graphics.
	s, err := tc.Start(ctx, *verbose)
	if err != nil {
		log.Fatal(err)
	}
	// The log lines wait until the stream stops, so that they do not draw over
	// the frames.
	log.SetOutput(s.LogWriter())
	defer s.Stop()

	log.Println("the page is on the screen")
	if err := chromedp.Do(ctx, chromedp.Sleep(5*time.Second)); err != nil {
		// Fatal stops the stream, prints the log lines and exits.
		s.Fatal(err)
	}
}
