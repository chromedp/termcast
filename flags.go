package termcast

import (
	"context"
	"flag"
)

// Flags holds the command line flags that a program uses to turn the stream
// on. Call [Flags.Register] before the program parses its flags, and
// [Flags.Start] after the browser has started.
type Flags struct {
	// Enabled is the value of the flag -visible-on-terminal.
	Enabled bool
	// FPS is the value of the flag -terminal-fps.
	FPS float64
}

// Register adds the flags -visible-on-terminal and -terminal-fps to fs. A nil
// fs means [flag.CommandLine].
func (f *Flags) Register(fs *flag.FlagSet) {
	if fs == nil {
		fs = flag.CommandLine
	}
	fs.BoolVar(&f.Enabled, "visible-on-terminal", false, "draw the page in the terminal with terminal graphics, at 4 frames per second, and clear the screen at each frame. the stream does not work with -v")
	fs.Float64Var(&f.FPS, "terminal-fps", DefaultFPS, "frames per second of the stream on the terminal")
}

// Start starts a stream when the flag -visible-on-terminal is on. It returns a
// nil stream and no error when the flag is off. A nil stream is safe to use.
// It returns [ErrVerbose] when the flag is on and verbose is true, because the
// protocol messages of -v would draw over the frames. Otherwise it returns the
// result of [Start], with the frame rate of -terminal-fps.
func (f *Flags) Start(ctx context.Context, verbose bool, opts ...Option) (*Stream, error) {
	switch {
	case !f.Enabled:
		return nil, nil
	case verbose:
		return nil, ErrVerbose
	}
	if f.FPS > 0 {
		opts = append(opts[:len(opts):len(opts)], WithFPS(f.FPS))
	}
	return Start(ctx, opts...)
}
