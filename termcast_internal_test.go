package termcast

import (
	"bytes"
	"image"
	"strings"
	"testing"

	"github.com/kenshaw/rasterm"
)

func TestKittyStreamStartsAgainWhenTheSizeChanges(t *testing.T) {
	var out bytes.Buffer
	s := &Stream{
		cfg:   &config{out: &out},
		kitty: &rasterm.KittyEncoder{NoNewline: true},
	}
	for _, size := range []int{40, 40, 60, 60} {
		if err := s.draw(image.NewRGBA(image.Rect(0, 0, size, size))); err != nil {
			t.Fatal(err)
		}
	}
	got := out.String()
	// Two sizes need two images, and each new image clears the screen first.
	if n := strings.Count(got, clear); n != 2 {
		t.Errorf("the stream cleared the screen %d times, want 2", n)
	}
	// The new size deletes the image of the old size.
	if !strings.Contains(got, "a=d") {
		t.Error("the stream did not delete the image of the old size")
	}
}
