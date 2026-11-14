// Package termcast streams the screen of a chromedp page to a terminal that
// has graphics.
//
// It uses the screencast of the Chrome DevTools Protocol to get the frames of
// the page, and the package [rasterm] to draw them. [Start] asks the terminal
// which graphics it has, with [rasterm.Detect]. The terminal must support
// Kitty, iTerm2 or Sixel graphics. [Start] returns [ErrNoGraphics] when it
// does not.
//
// The browser sends a frame only when the page changes. The stream keeps the
// latest frame and draws it again only when it is new, so a page that stops
// changing stays on the screen. On a Kitty terminal that plays animations,
// the stream draws each frame into one Kitty image, so the picture changes
// with no flicker. See [WithAnimation]. On other terminals, the stream clears
// the screen at each redraw and draws the latest frame. It uses no alternate
// screen and does not hide the cursor. A program that ends with [log.Fatal]
// skips its deferred calls, so the stream must never leave the terminal in a
// changed state.
//
// The log lines of the program draw over the frames. [Stream.LogWriter] holds
// the lines while the stream runs and prints them when the stream stops, after
// a final frame. Other output that draws over the frames, such as the protocol
// messages of [chromedp.WithDebugf], does not work together with the stream.
// The program decides which one it shows.
//
// A nil [*Stream] is a stream that is off, and its methods are safe. So a
// program starts the stream only when its own flag asks for it, and writes
// defer s.Stop() and log.SetOutput(s.LogWriter()) in either case. See the
// example.
package termcast

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/draw"
	"io"
	"iter"
	"math"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
	"github.com/kenshaw/rasterm"
)

// Error is the type of the constant errors of this package.
type Error string

// Error satisfies the [error] interface.
func (err Error) Error() string {
	return string(err)
}

const (
	// ErrNoGraphics is the error that [Start] returns when the terminal has no
	// graphics. [rasterm.Detect] finds Kitty, iTerm2 and Sixel from a query to
	// the terminal and from the environment. The variable TERM_GRAPHICS can force
	// the type to kitty, iterm or sixel, and the value none turns graphics
	// off.
	ErrNoGraphics Error = "termcast: terminal graphics are not available. Use Kitty, iTerm2 or a Sixel terminal, or set TERM_GRAPHICS to kitty, iterm or sixel"
)

// clear moves the cursor to the top left and clears the screen. It does not
// clear the scrollback, and it does not change the screen mode.
const clear = "\x1b[H\x1b[2J"

// Stream draws the screen of a page in the terminal. [Start] makes it. A nil
// stream is a stream that is off. Every method is safe on a nil pointer, and
// safe to call from several goroutines.
type Stream struct {
	cfg *config
	// term is the terminal that the stream draws on, and keys is the input
	// that detection read and that is not an answer of the terminal.
	term rasterm.Terminal
	keys []byte
	// enc is the encoder of the terminal.
	enc rasterm.Encoder
	// kitty is the encoder of the Kitty stream, or nil when the stream clears
	// the screen at each frame.
	kitty *rasterm.KittyEncoder
	// ks is the Kitty stream that is on the screen. The drawing goroutine uses
	// it, and Stop uses it after that goroutine has ended.
	ks *rasterm.KittyStream
	// ctx is the context that the caller gave to Start.
	ctx    context.Context
	cancel context.CancelFunc
	// quit ends the watcher of ctx when the stream stops.
	quit chan struct{}
	// wg counts the reader and the drawing goroutine.
	wg       sync.WaitGroup
	stopOnce sync.Once

	// mu guards the latest frame, the flag that tells if it is new, and the
	// first error.
	mu    sync.Mutex
	frame image.Image
	// width is the width of the screen in CSS pixels at the time of frame. It
	// is 0 when the frame is a screenshot.
	width float64
	fresh bool
	err   error

	// logMu guards the held log lines and the flag that tells that the stream
	// has stopped.
	logMu  sync.Mutex
	held   bytes.Buffer
	closed bool
}

// Start starts the screencast of the page of ctx and draws it in the terminal.
// The context is a chromedp context. Start starts the browser and opens the
// target when the context has none yet, as [chromedp.Run] does.
//
// Start first detects the terminal with [rasterm.Detect], unless
// [WithTerminal] gives it. It returns [ErrNoGraphics] when the terminal has no
// graphics. The stream ends when the context ends, and then it acts as
// [Stream.Stop].
func Start(ctx context.Context, opts ...Option) (*Stream, error) {
	cfg, err := newConfig(opts)
	if err != nil {
		return nil, err
	}
	var term rasterm.Terminal
	var keys []byte
	if cfg.term != nil {
		term = *cfg.term
	} else if term, keys, err = rasterm.Detect(ctx, cfg.detectOpts...); err != nil {
		return nil, fmt.Errorf("detecting the terminal: %w", err)
	}
	// The encoder does not add a newline after an image. A newline at the
	// bottom of the screen scrolls the image.
	enc := cfg.encoder
	if enc == nil {
		enc = term.Encoder(rasterm.WithNoNewline())
	}
	if enc == nil {
		return nil, ErrNoGraphics
	}
	var kitty *rasterm.KittyEncoder
	if k, ok := enc.(rasterm.KittyEncoder); ok && animate(cfg, term) {
		kitty = &k
	}
	if err := chromedp.Do(ctx); err != nil {
		return nil, fmt.Errorf("starting the target: %w", err)
	}
	rctx, cancel := context.WithCancel(ctx)
	// Subscribe before the start of the screencast, so that the first frame is
	// not lost.
	frames := chromedp.Events(rctx, page.ScreencastFrame)
	if _, err := chromedp.Call(rctx, page.StartScreencast, page.StartScreencastParams{
		Format:        page.StartScreencastFormatJpeg,
		Quality:       ptr(int64(cfg.quality)),
		MaxWidth:      int64(cfg.width),
		MaxHeight:     int64(cfg.height),
		EveryNthFrame: 1,
	}); err != nil {
		cancel()
		return nil, fmt.Errorf("starting the screencast: %w", err)
	}
	s := &Stream{
		cfg:    cfg,
		term:   term,
		keys:   keys,
		enc:    enc,
		kitty:  kitty,
		ctx:    ctx,
		cancel: cancel,
		quit:   make(chan struct{}),
	}
	s.wg.Add(2)
	go s.read(rctx, frames)
	go s.loop(rctx)
	go func() {
		select {
		case <-ctx.Done():
			s.Stop()
		case <-s.quit:
		}
	}()
	return s, nil
}

// animate reports whether the stream draws with a Kitty stream on term. The
// option WithAnimation decides when it is given. Otherwise the terminal must
// report that it plays a Kitty animation, or name itself kitty or Ghostty,
// which play animations and do not report it.
func animate(cfg *config, term rasterm.Terminal) bool {
	if cfg.animation != nil {
		return *cfg.animation
	}
	return term.KittyAnimation || strings.EqualFold(term.Name, "kitty") || strings.EqualFold(term.Name, "ghostty")
}

// read acknowledges and decodes the frames, and keeps the latest one.
func (s *Stream) read(ctx context.Context, frames iter.Seq2[page.EventScreencastFrame, error]) {
	defer s.wg.Done()
	for ev, err := range frames {
		if err != nil {
			if ctx.Err() == nil {
				s.fail(fmt.Errorf("receiving a frame: %w", err))
			}
			return
		}
		// The browser sends no more frames until it receives the
		// acknowledgment, so send it before the work on the frame.
		if _, err := chromedp.Call(ctx, page.ScreencastFrameAck, page.ScreencastFrameAckParams{SessionID: ev.SessionID}); err != nil {
			if ctx.Err() == nil {
				s.fail(fmt.Errorf("acknowledging a frame: %w", err))
			}
			return
		}
		// The decoded frame keeps its JPEG, so the iTerm2 encoder sends it
		// as it is.
		img, err := rasterm.DecodeJPEG(ev.Data)
		if err != nil {
			s.fail(fmt.Errorf("decoding a frame: %w", err))
			continue
		}
		s.mu.Lock()
		s.frame, s.width, s.fresh = img, ev.Metadata.DeviceWidth, true
		s.mu.Unlock()
	}
}

// loop draws the latest frame at each tick when it is new.
func (s *Stream) loop(ctx context.Context) {
	defer s.wg.Done()
	t := time.NewTicker(time.Duration(float64(time.Second) / s.cfg.fps))
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			s.mu.Lock()
			img, width, fresh := s.frame, s.width, s.fresh
			s.fresh = false
			s.mu.Unlock()
			if !fresh {
				continue
			}
			if img = s.view(ctx, img, width); img == nil {
				continue
			}
			if err := s.draw(img); err != nil {
				s.fail(err)
				return
			}
		}
	}
}

// view returns the part of img that the stream draws. It returns img itself
// when the stream has no element. Otherwise it returns the border box of the
// element, which it looks up again at each call, so an element that moves or
// changes its size stays in view. width is the width of the screen in CSS
// pixels when the browser made the frame, or 0 for a frame that has the scale
// of the page. view returns nil when the element is not in the page or not in
// the frame, and then the stream keeps what is on the screen.
func (s *Stream) view(ctx context.Context, img image.Image, width float64) image.Image {
	if s.cfg.element == nil {
		return img
	}
	// A part of the frame needs the image itself, and not its JPEG.
	if u, ok := img.(interface{ Unwrap() image.Image }); ok {
		img = u.Unwrap()
	}
	// A page that does not have the element yet must not hold the stream.
	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	box, err := s.cfg.element(ctx)
	if err != nil || box == nil || len(box.Border) < 8 {
		return nil
	}
	b := img.Bounds()
	scale := 1.0
	if width > 0 {
		scale = float64(b.Dx()) / width
	}
	minX, minY, maxX, maxY := box.Border[0], box.Border[1], box.Border[0], box.Border[1]
	for i := 2; i < 8; i += 2 {
		minX, maxX = min(minX, box.Border[i]), max(maxX, box.Border[i])
		minY, maxY = min(minY, box.Border[i+1]), max(maxY, box.Border[i+1])
	}
	r := image.Rect(
		b.Min.X+int(math.Floor(minX*scale)), b.Min.Y+int(math.Floor(minY*scale)),
		b.Min.X+int(math.Ceil(maxX*scale)), b.Min.Y+int(math.Ceil(maxY*scale)),
	).Intersect(b)
	if r.Empty() {
		return nil
	}
	if si, ok := img.(interface {
		SubImage(image.Rectangle) image.Image
	}); ok {
		return si.SubImage(r)
	}
	out := image.NewRGBA(image.Rect(0, 0, r.Dx(), r.Dy()))
	draw.Draw(out, out.Bounds(), img, r.Min, draw.Src)
	return out
}

// draw draws the image. Without a Kitty stream, it clears the screen first. It
// encodes the image first and writes the clear sequence and the image with one
// call, so that the screen stays empty for a short time.
func (s *Stream) draw(img image.Image) error {
	if s.kitty != nil {
		return s.drawKitty(img)
	}
	var buf bytes.Buffer
	buf.WriteString(clear)
	if err := s.enc.Encode(&buf, img); err != nil {
		return fmt.Errorf("encoding a frame: %w", err)
	}
	if _, err := s.cfg.out.Write(buf.Bytes()); err != nil {
		return fmt.Errorf("drawing a frame: %w", err)
	}
	return nil
}

// drawKitty draws the image into the Kitty stream, which changes the picture
// on the screen with no clear and no flicker. It starts a Kitty stream for the
// first frame, and a new one when the size of the frame changes. Then it
// deletes the old image, clears the screen and places the new image at the top
// left.
func (s *Stream) drawKitty(img image.Image) error {
	if s.ks != nil {
		err := s.ks.Draw(img)
		switch {
		case err == nil:
			return nil
		case !errors.Is(err, rasterm.ErrFrameSize):
			return fmt.Errorf("drawing a frame: %w", err)
		}
		if err := s.ks.Delete(); err != nil {
			return fmt.Errorf("deleting the image of the old size: %w", err)
		}
		s.ks = nil
	}
	ks, err := s.kitty.Stream(&clearFirst{w: s.cfg.out}, img)
	if err != nil {
		return fmt.Errorf("drawing a frame: %w", err)
	}
	s.ks = ks
	return nil
}

// clearFirst is a writer that writes the clear sequence in front of the bytes
// of its first write, with one call, so that the screen stays empty for a short
// time.
type clearFirst struct {
	w    io.Writer
	done bool
}

// Write satisfies the [io.Writer] interface.
func (c *clearFirst) Write(p []byte) (int, error) {
	if c.done {
		return c.w.Write(p)
	}
	c.done = true
	n, err := c.w.Write(append([]byte(clear), p...))
	return max(n-len(clear), 0), err
}

// fail keeps the first error of the stream.
func (s *Stream) fail(err error) {
	s.mu.Lock()
	if s.err == nil {
		s.err = err
	}
	s.mu.Unlock()
}

// Stop stops the screencast and the goroutines of the stream, draws the final
// frame once, and then prints the held log lines. When no frame arrived, it
// takes one screenshot as the final frame. It returns the first error that the
// stream met, such as an error of the encoder or of the output.
//
// Stop is safe to call more than once and from several goroutines. Each call
// returns the same error. A call that arrives while another one runs waits
// until that one is done. The stream also stops by itself when the context of
// [Start] ends. On a nil stream, Stop does nothing and returns nil.
func (s *Stream) Stop() error {
	if s == nil {
		return nil
	}
	s.stopOnce.Do(func() {
		close(s.quit)
		s.cancel()
		s.wg.Wait()
		// A context that has ended cannot send a command, and the browser has
		// stopped by then.
		if s.ctx.Err() == nil {
			ctx, cancel := context.WithTimeout(s.ctx, 5*time.Second)
			defer cancel()
			if _, err := chromedp.Call(ctx, page.StopScreencast, cdp.Empty{}); err != nil && !errors.Is(err, context.Canceled) {
				s.fail(fmt.Errorf("stopping the screencast: %w", err))
			}
		}
		s.mu.Lock()
		img, width := s.frame, s.width
		s.mu.Unlock()
		if img == nil {
			// A page that ends fast can end before the first frame arrives.
			// Take one screenshot, so that the final frame is never empty.
			img, width = s.screenshot(), 0
		}
		if img != nil {
			img = s.view(s.ctx, img, width)
		}
		if img != nil {
			if err := s.draw(img); err != nil {
				s.fail(err)
			}
			// The encoder leaves the cursor at the end of the image. Start a
			// new line for the log lines.
			io.WriteString(s.cfg.out, "\n")
		}
		s.flush()
	})
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.err
}

// screenshot takes one screenshot of the page and decodes it. It returns nil
// when the context has ended or the browser cannot take the screenshot. The
// error is kept for [Stream.Err].
func (s *Stream) screenshot() image.Image {
	if s.ctx.Err() != nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(s.ctx, 5*time.Second)
	defer cancel()
	res, err := chromedp.Call(ctx, page.CaptureScreenshot, page.CaptureScreenshotParams{
		Format:  page.CaptureScreenshotFormatJpeg,
		Quality: ptr(int64(s.cfg.quality)),
	})
	if err != nil {
		if s.ctx.Err() == nil {
			s.fail(fmt.Errorf("taking the final screenshot: %w", err))
		}
		return nil
	}
	img, err := rasterm.DecodeJPEG(res.Data)
	if err != nil {
		s.fail(fmt.Errorf("decoding the final screenshot: %w", err))
		return nil
	}
	return img
}

// flush prints the held log lines. After it, the log writer passes the lines
// on at once.
func (s *Stream) flush() {
	s.logMu.Lock()
	defer s.logMu.Unlock()
	s.closed = true
	if s.held.Len() > 0 {
		if _, err := s.cfg.logOut.Write(s.held.Bytes()); err != nil {
			s.fail(fmt.Errorf("printing the log lines: %w", err))
		}
		s.held.Reset()
	}
}

// Terminal returns the terminal that the stream draws on, as [rasterm.Detect]
// found it or as [WithTerminal] gave it. On a nil stream, it returns the zero
// [rasterm.Terminal], which has no graphics.
func (s *Stream) Terminal() rasterm.Terminal {
	if s == nil {
		return rasterm.Terminal{}
	}
	return s.term
}

// Keys returns the keys that the user typed while [Start] detected the
// terminal, in the order of the typing. Detection reads the input to get the
// answers of the terminal, so a program that reads its input must use these
// keys as its first input. It returns nil on a nil stream, and when
// [WithTerminal] turned detection off.
func (s *Stream) Keys() []byte {
	if s == nil {
		return nil
	}
	return s.keys
}

// LogWriter returns a writer for the log lines of the program, for example for
// [log.SetOutput]. While the stream runs, the writer holds the lines in
// memory. [Stream.Stop] prints them to the log output after the final frame.
// After [Stream.Stop], the writer passes each line on at once. On a nil
// stream, it returns [os.Stderr].
func (s *Stream) LogWriter() io.Writer {
	if s == nil {
		return os.Stderr
	}
	return logWriter{s}
}

// logWriter is the writer that [Stream.LogWriter] returns.
type logWriter struct {
	s *Stream
}

// Write satisfies the [io.Writer] interface.
func (w logWriter) Write(p []byte) (int, error) {
	w.s.logMu.Lock()
	defer w.s.logMu.Unlock()
	if w.s.closed {
		return w.s.cfg.logOut.Write(p)
	}
	return w.s.held.Write(p)
}

// ptr returns a pointer to a copy of v. The optional fields of the protocol
// commands are pointers, because zero is a valid value.
func ptr[T any](v T) *T { return &v }
