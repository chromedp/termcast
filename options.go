package termcast

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/chromedp/cdproto/dom"
	"github.com/chromedp/chromedp"
	"github.com/kenshaw/rasterm"
)

// Default values of the options.
const (
	// DefaultFPS is the number of frames per second that the stream draws.
	DefaultFPS = 4
	// DefaultQuality is the JPEG quality that the screencast asks for.
	DefaultQuality = 80
	// DefaultWidth is the largest width of a frame in pixels.
	DefaultWidth = 1024
	// DefaultHeight is the largest height of a frame in pixels.
	DefaultHeight = 768
)

// Option changes how [Start] makes a stream.
type Option func(*config)

// config holds the settings of a stream.
type config struct {
	fps     float64
	quality int
	width   int
	height  int
	out     io.Writer
	logOut  io.Writer
	// term is the terminal that WithTerminal gives, or nil when Start detects
	// the terminal.
	term       *rasterm.Terminal
	detectOpts []rasterm.DetectOption
	// animation is the value of WithAnimation, or nil when the terminal
	// decides.
	animation *bool
	// encoder replaces the encoder of the terminal. Only the tests set it.
	encoder rasterm.Encoder
	// element returns the box model of the element that the stream shows, or
	// is nil when the stream shows the whole page.
	element func(context.Context) (*dom.BoxModel, error)
}

// newConfig returns the settings with the default values and the options
// applied. It returns an error for a value that cannot work.
func newConfig(opts []Option) (*config, error) {
	c := &config{
		fps:     DefaultFPS,
		quality: DefaultQuality,
		width:   DefaultWidth,
		height:  DefaultHeight,
		out:     os.Stdout,
		logOut:  os.Stderr,
	}
	for _, o := range opts {
		o(c)
	}
	switch {
	case !(c.fps > 0):
		return nil, fmt.Errorf("termcast: the frame rate must be more than 0, got %v", c.fps)
	case c.quality < 0 || c.quality > 100:
		return nil, fmt.Errorf("termcast: the quality must be from 0 to 100, got %d", c.quality)
	case c.width <= 0 || c.height <= 0:
		return nil, fmt.Errorf("termcast: the size must be more than 0, got %dx%d", c.width, c.height)
	}
	return c, nil
}

// WithFPS sets the number of frames per second that the stream draws at most.
// The default is [DefaultFPS].
func WithFPS(fps float64) Option {
	return func(c *config) { c.fps = fps }
}

// WithQuality sets the JPEG quality of the frames that the browser sends, from
// 0 to 100. The default is [DefaultQuality].
func WithQuality(quality int) Option {
	return func(c *config) { c.quality = quality }
}

// WithMaxSize sets the largest width and height of a frame in pixels. The
// browser scales a larger page down. The default is [DefaultWidth] by
// [DefaultHeight].
func WithMaxSize(width, height int) Option {
	return func(c *config) { c.width, c.height = width, height }
}

// WithOutput sets the writer that receives the frames. The default is
// [os.Stdout]. Detection asks the terminal on [os.Stdout] and [os.Stdin], so
// give [WithTerminal] too when w is not that terminal.
func WithOutput(w io.Writer) Option {
	return func(c *config) { c.out = w }
}

// WithLogOutput sets the writer that receives the held log lines when the
// stream stops. The default is [os.Stderr].
func WithLogOutput(w io.Writer) Option {
	return func(c *config) { c.logOut = w }
}

// WithTerminal makes [Start] use t, and not detect the terminal. Use it when
// the program detected the terminal itself, or when the output is not the
// terminal, for example rasterm.Terminal{Protocol: rasterm.Kitty} for a file.
// [Start] returns [ErrNoGraphics] when t.Protocol is [rasterm.None].
func WithTerminal(t rasterm.Terminal) Option {
	return func(c *config) { c.term = &t }
}

// WithDetectOptions gives options to [rasterm.Detect], such as
// [rasterm.WithTimeout]. [WithTerminal] turns detection off, and then the
// options do nothing.
func WithDetectOptions(opts ...rasterm.DetectOption) Option {
	return func(c *config) { c.detectOpts = append(c.detectOpts, opts...) }
}

// WithAnimation turns the Kitty stream on or off. The Kitty stream draws each
// frame into one Kitty image with [rasterm.KittyStream], so it does not clear
// the screen and does not flicker. It works only with the Kitty protocol.
//
// By default, the stream uses it when the terminal reports that it plays a
// Kitty animation, or when it names itself kitty or Ghostty. A terminal that
// draws Kitty images but has no animation shows only the first frame of a
// Kitty stream, so the default does not guess.
func WithAnimation(on bool) Option {
	return func(c *config) { c.animation = &on }
}

// WithElement makes the stream draw only the element that sel selects, and not
// the whole page. The element is the first one that the selector matches, for
// example chromedp.CSS("#chart"). The stream looks the element up again at each
// frame, so an element that moves or changes its size stays in view.
//
// The browser sends the viewport, so the stream shows the part of the element
// that is in the viewport. Scroll the element into view first, for example
// with [chromedp.ScrollIntoView]. While the page has no such element, the
// stream keeps what is on the screen.
func WithElement[S chromedp.Selectable](sel S) Option {
	return func(c *config) {
		c.element = func(ctx context.Context) (*dom.BoxModel, error) {
			return chromedp.Run(ctx, chromedp.Dimensions(sel))
		}
	}
}
