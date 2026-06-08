package framebuffer

import (
	"image"
	"image/color"
	"math"
	"testing"
)

func TestNewDimensions(t *testing.T) {
	b := New(640, 480)
	if b.Width != 640 || b.Height != 480 {
		t.Errorf("dims = %dx%d, want 640x480", b.Width, b.Height)
	}
	if len(b.Depth) != 640*480 {
		t.Errorf("depth len = %d, want %d", len(b.Depth), 640*480)
	}
	if got := b.Image().Bounds(); got != image.Rect(0, 0, 640, 480) {
		t.Errorf("bounds = %v", got)
	}
}

func TestClear(t *testing.T) {
	b := New(4, 3)
	bg := color.RGBA{R: 10, G: 20, B: 30, A: 255}
	b.Clear(bg)

	for i, d := range b.Depth {
		if !math.IsInf(d, 1) {
			t.Errorf("depth[%d] = %v, want +Inf", i, d)
		}
	}
	for y := 0; y < b.Height; y++ {
		for x := 0; x < b.Width; x++ {
			if got := b.Color.RGBAAt(x, y); got != bg {
				t.Errorf("color at (%d,%d) = %v, want %v", x, y, got, bg)
			}
		}
	}
}

func TestSetAndDepthAt(t *testing.T) {
	b := New(2, 2)
	b.Clear(color.RGBA{A: 255})

	c := color.RGBA{R: 1, G: 2, B: 3, A: 255}
	b.Set(1, 0, c, 0.25)

	if got := b.DepthAt(1, 0); got != 0.25 {
		t.Errorf("DepthAt(1,0) = %v, want 0.25", got)
	}
	if got := b.Color.RGBAAt(1, 0); got != c {
		t.Errorf("color at (1,0) = %v, want %v", got, c)
	}
	// A neighboring pixel must be untouched.
	if got := b.DepthAt(0, 0); !math.IsInf(got, 1) {
		t.Errorf("untouched depth = %v, want +Inf", got)
	}
}
