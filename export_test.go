package termcast

import "github.com/kenshaw/rasterm"

// WithEncoder makes the stream draw with enc, and not with the encoder of the
// terminal. It also turns detection off, unless WithTerminal comes after it,
// so that a test never asks its own terminal. Only the tests use it.
func WithEncoder(enc rasterm.Encoder) Option {
	return func(c *config) {
		c.encoder = enc
		if c.term == nil {
			c.term = &rasterm.Terminal{Protocol: rasterm.Sixel}
		}
	}
}

// Animate reports whether a stream with the options draws with a Kitty stream
// on term.
func Animate(term rasterm.Terminal, opts ...Option) bool {
	cfg, err := newConfig(opts)
	if err != nil {
		panic(err)
	}
	return animate(cfg, term)
}
