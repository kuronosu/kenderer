package raster

import (
	"image/color"
	"testing"

	"github.com/kuronosu/kenderer/framebuffer"
	"github.com/kuronosu/kenderer/math3d"
)

// TestDrawLineDepthTestNoWrite pins the line primitive's two depth invariants: it
// is occluded by nearer stored depth (test z < stored) and it never writes the
// depth buffer. A depth "wall" is seeded across the middle of a one-row buffer; a
// constant-depth line is drawn over the whole row, behind the wall but in front of
// the +Inf background.
func TestDrawLineDepthTestNoWrite(t *testing.T) {
	fb := framebuffer.New(10, 1)
	fb.Clear(color.RGBA{A: 255}) // depth +Inf, color opaque black

	const wallLo, wallHi = 3, 6
	wallColor := color.RGBA{R: 10, G: 10, B: 10, A: 255}
	for x := wallLo; x <= wallHi; x++ {
		fb.Set(x, 0, wallColor, 0.5) // nearer than the line's 0.8
	}
	depthBefore := append([]float64(nil), fb.Depth...)

	red := color.RGBA{R: 255, A: 255}
	DrawLine(fb, math3d.V3(0, 0, 0.8), math3d.V3(9, 0, 0.8), red)

	// The depth buffer must be byte-for-byte unchanged: lines test depth, never write it.
	for i := range depthBefore {
		if depthBefore[i] != fb.Depth[i] {
			t.Fatalf("DrawLine wrote depth at index %d: %v -> %v", i, depthBefore[i], fb.Depth[i])
		}
	}
	// Behind the wall (0.8 >= 0.5): occluded, the wall color survives.
	for x := wallLo; x <= wallHi; x++ {
		if fb.Color.RGBAAt(x, 0) == red {
			t.Errorf("pixel %d should be occluded by the depth wall", x)
		}
	}
	// Over the +Inf background (0.8 < Inf): drawn.
	for _, x := range []int{0, 1, 2, 7, 8, 9} {
		if got := fb.Color.RGBAAt(x, 0); got != red {
			t.Errorf("pixel %d should be drawn over the background, got %v", x, got)
		}
	}
}
