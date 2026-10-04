// Package termcast streams the screen of a chromedp page to a terminal that
// has graphics.
//
// It uses the screencast of the Chrome DevTools Protocol to get the frames of
// the page, and the package [rasterm] to draw them. The terminal must support
// Kitty, iTerm2 or Sixel graphics. [Start] returns [ErrNoGraphics] when it
// does not.
//
// The stream clears the terminal at each redraw and draws the latest frame. It
// uses no alternate screen and does not hide the cursor. A program that ends
// with [log.Fatal] skips its deferred calls, so the stream must never leave the
// terminal in a changed state. The browser sends a frame only when the page
// changes. The stream keeps the latest frame and draws it again only when it is
// new, so a page that stops changing stays on the screen.
//
// The log lines of the program draw over the frames, and the next redraw
// erases them. [Stream.LogWriter] holds the lines while the stream runs
// and prints them when the stream stops, after a final frame. The stream does
// not work together with the protocol messages of a flag such as -v, because
// they also draw over the frames. [Flags.Start] returns [ErrVerbose] for that
// combination.
//
// Every method of [*Stream] is safe on a nil pointer. A program can write
// s.Stop, s.Fatal and s.LogWriter whether the stream is on or off. See
// [Flags] and the example for the intended use.
package termcast

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/draw"
	"image/jpeg"
	"io"
	"iter"
	"log"
	"math"
	"os"
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
	// graphics. rasterm detects Kitty, iTerm2 and Sixel from the environment
	// and from a query to the terminal. The variable TERM_GRAPHICS can force
	// the type to kitty, iterm or sixel, and the value none turns graphics
	// off.
	ErrNoGraphics Error = "termcast: terminal graphics are not available. Use Kitty, iTerm2 or a Sixel terminal, or set TERM_GRAPHICS to kitty, iterm or sixel"
	// ErrVerbose is the error for the flag of the stream together with the
	// flag -v.
	ErrVerbose Error = "termcast: the stream on the terminal does not work with -v, because the protocol messages draw over the frames"
)

// clear moves the cursor to the top left and clears the screen. It does not
// clear the scrollback, and it does not change the screen mode.
const clear = "\x1b[H\x1b[2J"

// Available reports whether the terminal has graphics that [Start] can use with
// its default encoder. It reports what [rasterm.Available] reports.
func Available() bool {
	return rasterm.Available()
}

// Stream draws the screen of a page in the terminal. [Start] makes it. Every
// method is safe on a nil pointer, and safe to call from several goroutines.
type Stream struct {
	cfg *config
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
// Start returns [ErrNoGraphics] when the encoder is not available. The stream
// ends when the context ends, and then it acts as [Stream.Stop].
func Start(ctx context.Context, opts ...Option) (*Stream, error) {
	cfg, err := newConfig(opts)
	if err != nil {
		return nil, err
	}
	if !cfg.encoder.Available() {
		return nil, ErrNoGraphics
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
		Quality:       new(int64(cfg.quality)),
		MaxWidth:      int64(cfg.width),
		MaxHeight:     int64(cfg.height),
		EveryNthFrame: 1,
	}); err != nil {
		cancel()
		return nil, fmt.Errorf("starting the screencast: %w", err)
	}
	s := &Stream{
		cfg:    cfg,
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
		img, err := jpeg.Decode(bytes.NewReader(ev.Data))
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

// draw clears the screen and draws the image. It encodes the image first and
// writes the clear sequence and the image with one call, so that the screen
// stays empty for a short time.
func (s *Stream) draw(img image.Image) error {
	var buf bytes.Buffer
	buf.WriteString(clear)
	if err := s.cfg.encoder.Encode(&buf, img); err != nil {
		return fmt.Errorf("encoding a frame: %w", err)
	}
	if _, err := s.cfg.out.Write(buf.Bytes()); err != nil {
		return fmt.Errorf("drawing a frame: %w", err)
	}
	return nil
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
// frame once (it takes a screenshot when no frame arrived), and then prints the held log lines. It is safe to call more than
// once and from several goroutines. A call that arrives while another one runs
// waits until that one is done. The stream also stops by itself when the
// context of [Start] ends. It does nothing on a nil stream.
func (s *Stream) Stop() {
	if s == nil {
		return
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
		Quality: new(int64(s.cfg.quality)),
	})
	if err != nil {
		if s.ctx.Err() == nil {
			s.fail(fmt.Errorf("taking the final screenshot: %w", err))
		}
		return nil
	}
	img, err := jpeg.Decode(bytes.NewReader(res.Data))
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

// Fatal stops the stream, as [Stream.Stop] does, and then calls [log.Fatal]
// with v. The program ends with the exit status 1. On a nil stream, it only
// calls [log.Fatal].
func (s *Stream) Fatal(v ...any) {
	s.Stop()
	log.Fatal(v...)
}

// Err returns the first error that the stream met while it ran, such as an
// error of the encoder or of the output. It returns nil on a nil stream.
func (s *Stream) Err() error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.err
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
