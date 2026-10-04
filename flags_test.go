package termcast_test

import (
	"flag"
	"io"

	"github.com/chromedp/termcast"
)

// newFlagSet returns a flag set with the flags of f registered.
func newFlagSet(f *termcast.Flags) *flag.FlagSet {
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	f.Register(fs)
	return fs
}
