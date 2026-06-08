package framebuffer

import (
	"image"
	"image/color"
	"math"
)

// Buffer is an in-memory render target: an RGBA color buffer paired with a
// float64 depth (Z) buffer, indexed row-major. It carries no notion of display.
type Buffer struct {
	Width, Height int
	Color         *image.RGBA
	Depth         []float64
}

// New allocates a Buffer of the given pixel dimensions.
func New(width, height int) *Buffer {
	return &Buffer{
		Width:  width,
		Height: height,
		Color:  image.NewRGBA(image.Rect(0, 0, width, height)),
		Depth:  make([]float64, width*height),
	}
}

// Clear fills the color buffer with c and resets every depth sample to +Inf.
func (b *Buffer) Clear(c color.RGBA) {
	pix := b.Color.Pix
	for i := 0; i+3 < len(pix); i += 4 {
		pix[i+0] = c.R
		pix[i+1] = c.G
		pix[i+2] = c.B
		pix[i+3] = c.A
	}
	inf := math.Inf(1)
	for i := range b.Depth {
		b.Depth[i] = inf
	}
}

// DepthAt returns the stored depth at pixel (x, y). Coordinates must be in range.
func (b *Buffer) DepthAt(x, y int) float64 {
	return b.Depth[y*b.Width+x]
}

// Set writes color c and depth z at pixel (x, y) unconditionally; performing the
// depth test beforehand is the rasterizer's responsibility.
func (b *Buffer) Set(x, y int, c color.RGBA, z float64) {
	b.Depth[y*b.Width+x] = z
	b.Color.SetRGBA(x, y, c)
}

// Image returns the color buffer as an *image.RGBA for presentation. The pixels
// are shared with the Buffer, so callers that need the frame to outlive the next
// render must copy it.
func (b *Buffer) Image() *image.RGBA { return b.Color }
