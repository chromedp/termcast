package termcast_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"io"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/chromedp/termcast"
)

const clearScreen = "\x1b[H\x1b[2J"

// page animates with a script, so the browser sends frames all the time.
const page = `<html><body style="font: 64px sans-serif">
<div id="n">0</div>
<script>
let n = 0;
window.timer = setInterval(() => { document.getElementById("n").textContent = ++n; }, 20);
</script></body></html>`

// syncBuffer is a bytes.Buffer that the stream and the test can use together.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// fakeEncoder writes a marker for each frame and counts the frames. It needs no
// terminal.
type fakeEncoder struct {
	unavailable bool
	frames      atomic.Int64
}

func (e *fakeEncoder) Available() bool { return !e.unavailable }

func (e *fakeEncoder) Encode(w io.Writer, img image.Image) error {
	n := e.frames.Add(1)
	b := img.Bounds()
	_, err := fmt.Fprintf(w, "[frame %d %dx%d]", n, b.Dx(), b.Dy())
	return err
}

// browser starts a browser with a page that animates, and returns its context.
func browser(t *testing.T) context.Context {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, page)
	}))
	t.Cleanup(srv.Close)
	ctx, cancel := chromedp.NewContext(context.Background())
	t.Cleanup(cancel)
	// The first call starts the browser, and the browser stops when the context
	// of that call ends. So the first call must use the context itself.
	if err := chromedp.Do(ctx, chromedp.Navigate(srv.URL)); err != nil {
		t.Fatal(err)
	}
	return ctx
}

// start starts a stream with the fake encoder and two buffers.
func start(t *testing.T, ctx context.Context, fps float64) (*termcast.Stream, *fakeEncoder, *syncBuffer, *syncBuffer) {
	t.Helper()
	enc, out, logs := new(fakeEncoder), new(syncBuffer), new(syncBuffer)
	s, err := termcast.Start(ctx,
		termcast.WithEncoder(enc),
		termcast.WithOutput(out),
		termcast.WithLogOutput(logs),
		termcast.WithFPS(fps),
		termcast.WithMaxSize(320, 240),
		termcast.WithQuality(50),
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Stop)
	return s, enc, out, logs
}

func TestFramesArriveAtTheRate(t *testing.T) {
	ctx := browser(t)
	s, enc, out, _ := start(t, ctx, 10)
	time.Sleep(2 * time.Second)
	n := enc.frames.Load()
	s.Stop()
	// About 20 frames in 2 seconds. The limits are wide for a slow machine.
	if n < 6 || n > 25 {
		t.Errorf("got %d frames in 2 seconds at 10 frames per second", n)
	}
	text := out.String()
	if got := strings.Count(text, clearScreen); got != strings.Count(text, "[frame ") {
		t.Errorf("got %d clear sequences for %d frames", got, strings.Count(text, "[frame "))
	}
	if !strings.HasPrefix(text, clearScreen+"[frame 1 ") {
		t.Errorf("the output does not start with the clear sequence and a frame: %q", text[:min(len(text), 40)])
	}
	// Every frame follows a clear sequence, and nothing else is in between.
	for _, part := range strings.Split(strings.TrimSuffix(text, "\n"), clearScreen)[1:] {
		if !strings.HasPrefix(part, "[frame ") || !strings.HasSuffix(part, "]") {
			t.Fatalf("a redraw is not a clear sequence and a frame: %q", part)
		}
	}
	if strings.Contains(text, "\x1b[?1049h") || strings.Contains(text, "\x1b[?25l") {
		t.Errorf("the output has an alternate screen or a hidden cursor: %q", text[:min(len(text), 80)])
	}
	if err := s.Err(); err != nil {
		t.Errorf("Err: %v", err)
	}
}

func TestMaxSize(t *testing.T) {
	ctx := browser(t)
	s, _, out, _ := start(t, ctx, 10)
	time.Sleep(time.Second)
	s.Stop()
	var w, h int
	i := strings.Index(out.String(), "[frame 1 ")
	if _, err := fmt.Sscanf(out.String()[i:], "[frame 1 %dx%d]", &w, &h); err != nil {
		t.Fatal(err)
	}
	if w > 320 || h > 240 || w == 0 || h == 0 {
		t.Errorf("the first frame is %dx%d, and the largest size is 320x240", w, h)
	}
}

func TestStopIsIdempotentAndPrintsFinalFrameThenLogs(t *testing.T) {
	ctx := browser(t)
	s, enc, out, logs := start(t, ctx, 10)
	log := s.LogWriter()
	fmt.Fprintln(log, "first line")
	time.Sleep(time.Second)
	fmt.Fprintln(log, "second line")
	if got := logs.String(); got != "" {
		t.Fatalf("the log lines came out while the stream ran: %q", got)
	}
	if strings.Contains(out.String(), "line") {
		t.Fatalf("a log line is in the frames output: %q", out.String())
	}
	before := enc.frames.Load()
	s.Stop()
	after := enc.frames.Load()
	if after <= before {
		t.Errorf("Stop did not draw a final frame: %d frames before and %d after", before, after)
	}
	// The last marker in the output must be the final frame.
	if text := out.String(); strings.LastIndex(text, "[frame ") != strings.LastIndex(text, fmt.Sprintf("[frame %d ", after)) {
		t.Errorf("the last frame of the output is not the final frame %d", after)
	}
	if got, want := logs.String(), "first line\nsecond line\n"; got != want {
		t.Errorf("got the log lines %q, want %q", got, want)
	}

	// A second Stop does nothing.
	outLen, logLen := len(out.String()), len(logs.String())
	s.Stop()
	s.Stop()
	if len(out.String()) != outLen || len(logs.String()) != logLen {
		t.Error("a second Stop wrote more output")
	}

	// After Stop the log writer passes the lines on at once.
	fmt.Fprintln(log, "late line")
	if got, want := logs.String(), "first line\nsecond line\nlate line\n"; got != want {
		t.Errorf("got the log lines %q, want %q", got, want)
	}
}

func TestStopFromManyGoroutines(t *testing.T) {
	ctx := browser(t)
	s, _, _, _ := start(t, ctx, 10)
	time.Sleep(500 * time.Millisecond)
	var wg sync.WaitGroup
	for range 5 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.Stop()
		}()
	}
	wg.Wait()
}

func TestAPageThatStopsChangingStaysOnTheScreen(t *testing.T) {
	ctx := browser(t)
	s, enc, _, _ := start(t, ctx, 20)
	time.Sleep(time.Second)
	// Stop the animation. The page does not change, so no new frame comes.
	if _, err := chromedp.Run(ctx, chromedp.Evaluate[bool](`clearInterval(window.timer), true`)); err != nil {
		t.Fatal(err)
	}
	time.Sleep(time.Second)
	n := enc.frames.Load()
	time.Sleep(time.Second)
	if got := enc.frames.Load(); got != n {
		t.Errorf("the stream drew %d frames of a page that does not change", got-n)
	}
	s.Stop()
}

func TestNilStreamIsSafe(t *testing.T) {
	var s *termcast.Stream
	s.Stop()
	s.Stop()
	if err := s.Err(); err != nil {
		t.Errorf("Err on a nil stream: %v", err)
	}
	if w := s.LogWriter(); w == nil {
		t.Error("LogWriter on a nil stream is nil")
	}
}

func TestNoGraphics(t *testing.T) {
	// The check comes before the browser, so a context without a browser is
	// enough.
	enc := &fakeEncoder{unavailable: true}
	s, err := termcast.Start(context.Background(), termcast.WithEncoder(enc))
	if !errors.Is(err, termcast.ErrNoGraphics) || s != nil {
		t.Errorf("got %v and %v, want ErrNoGraphics and a nil stream", s, err)
	}
	for _, want := range []string{"Kitty", "iTerm2", "Sixel", "TERM_GRAPHICS"} {
		if !strings.Contains(termcast.ErrNoGraphics.Error(), want) {
			t.Errorf("the message of ErrNoGraphics does not name %s", want)
		}
	}
}

func TestInvalidOptions(t *testing.T) {
	for name, opt := range map[string]termcast.Option{
		"fps":     termcast.WithFPS(0),
		"quality": termcast.WithQuality(101),
		"size":    termcast.WithMaxSize(0, 10),
	} {
		if s, err := termcast.Start(context.Background(), termcast.WithEncoder(new(fakeEncoder)), opt); err == nil || s != nil {
			t.Errorf("%s: got %v and %v, want an error", name, s, err)
		}
	}
}

func TestFlags(t *testing.T) {
	var f termcast.Flags
	fs := newFlagSet(&f)
	if err := fs.Parse(nil); err != nil {
		t.Fatal(err)
	}
	if f.Enabled || f.FPS != 4 {
		t.Errorf("got the defaults %+v, want off and 4 frames per second", f)
	}
	// Off: no stream and no error, with or without -v.
	for _, verbose := range []bool{false, true} {
		if s, err := f.Start(context.Background(), verbose); s != nil || err != nil {
			t.Errorf("off, verbose %v: got %v and %v", verbose, s, err)
		}
	}
	if err := fs.Parse([]string{"-visible-on-terminal", "-terminal-fps=2.5"}); err != nil {
		t.Fatal(err)
	}
	if !f.Enabled || f.FPS != 2.5 {
		t.Errorf("got %+v, want on and 2.5 frames per second", f)
	}
	if s, err := f.Start(context.Background(), true); !errors.Is(err, termcast.ErrVerbose) || s != nil {
		t.Errorf("got %v and %v, want ErrVerbose and a nil stream", s, err)
	}
	if s, err := f.Start(context.Background(), false, termcast.WithEncoder(&fakeEncoder{unavailable: true})); !errors.Is(err, termcast.ErrNoGraphics) || s != nil {
		t.Errorf("got %v and %v, want ErrNoGraphics and a nil stream", s, err)
	}
}

func TestFlagsStartsAStream(t *testing.T) {
	ctx := browser(t)
	f := termcast.Flags{Enabled: true, FPS: 10}
	enc, out := new(fakeEncoder), new(syncBuffer)
	s, err := f.Start(ctx, false, termcast.WithEncoder(enc), termcast.WithOutput(out), termcast.WithLogOutput(io.Discard))
	if err != nil || s == nil {
		t.Fatalf("got %v and %v", s, err)
	}
	time.Sleep(time.Second)
	s.Stop()
	if enc.frames.Load() == 0 {
		t.Error("the stream drew no frame")
	}
}

func TestCanceledContextEndsTheStream(t *testing.T) {
	ctx := browser(t)
	// The first stream warms up the browser, so that the count of the
	// goroutines does not include a one-time start.
	warm, _, _, _ := start(t, ctx, 10)
	time.Sleep(500 * time.Millisecond)
	warm.Stop()
	settle()
	base := runtime.NumGoroutine()

	sctx, cancel := context.WithCancel(ctx)
	enc, out, logs := new(fakeEncoder), new(syncBuffer), new(syncBuffer)
	s, err := termcast.Start(sctx, termcast.WithEncoder(enc), termcast.WithOutput(out), termcast.WithLogOutput(logs), termcast.WithFPS(10))
	if err != nil {
		t.Fatal(err)
	}
	fmt.Fprintln(s.LogWriter(), "held line")
	time.Sleep(time.Second)
	cancel()

	// The stream stops by itself: it draws the final frame and prints the lines.
	deadline := time.Now().Add(10 * time.Second)
	for logs.String() == "" && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	if got := logs.String(); got != "held line\n" {
		t.Errorf("got the log lines %q after the cancel", got)
	}
	s.Stop()
	s.Stop()
	if !strings.HasSuffix(out.String(), "]\n") {
		t.Errorf("the output does not end with the final frame: %q", out.String()[max(0, len(out.String())-40):])
	}
	if n := settleTo(base + 2); n > base+2 {
		t.Errorf("got %d goroutines after the cancel, and %d before the stream", n, base)
	}
}

// settle gives the goroutines of a stopped stream time to end.
func settle() { time.Sleep(300 * time.Millisecond) }

// settleTo waits until the number of goroutines is at most limit, or for 5
// seconds, and returns the number.
func settleTo(limit int) int {
	deadline := time.Now().Add(5 * time.Second)
	for {
		n := runtime.NumGoroutine()
		if n <= limit || time.Now().After(deadline) {
			return n
		}
		time.Sleep(50 * time.Millisecond)
	}
}
