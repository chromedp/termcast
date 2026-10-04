package termcast

import (
	"fmt"
	"io"
	"os"

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
	encoder rasterm.Encoder
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
	if c.encoder == nil {
		// The encoders do not add a newline after an image. A newline at the
		// bottom of the screen would scroll the image.
		c.encoder = rasterm.NewDefaultEncoder(
			rasterm.KittyEncoder{NoNewline: true},
			rasterm.ITermEncoder{NoNewline: true},
			rasterm.SixelEncoder{NoNewline: true},
		)
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
// [os.Stdout].
func WithOutput(w io.Writer) Option {
	return func(c *config) { c.out = w }
}

// WithLogOutput sets the writer that receives the held log lines when the
// stream stops. The default is [os.Stderr].
func WithLogOutput(w io.Writer) Option {
	return func(c *config) { c.logOut = w }
}

// WithEncoder sets the encoder that draws a frame. The default encoder uses
// Kitty, iTerm2 or Sixel graphics, as [rasterm] detects them. [Start] asks the
// encoder that you give if it is available, and it does not ask [rasterm].
func WithEncoder(enc rasterm.Encoder) Option {
	return func(c *config) { c.encoder = enc }
}
