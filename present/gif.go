package present

import (
	"fmt"
	"image"
	"image/color"
	"image/color/palette"
	"image/draw"
	"image/gif"
	"math"
	"os"
	"time"
)

// GIF is a Presenter that writes the animation to an animated GIF file using
// only the standard library.
type GIF struct {
	Path string
	Loop int // 0 = loop forever
}

// Run renders one frame every 1/FPS over the configured Duration, quantizes each
// frame, and writes the animated GIF to Path.
//
// Quantization first tries an exact per-frame palette: if a frame has at most 256
// distinct colors (true for the flat-shaded cube) it is encoded with those exact
// colors and no dithering, so solid areas stay clean. Frames with more than 256
// colors fall back to the Plan9 palette with Floyd-Steinberg dithering.
//
// Note: GIF frame delays are whole centiseconds (100/FPS). When 100/FPS is not an
// integer it is rounded, and some viewers clamp very small delays to a minimum,
// so the played-back rate may not match high FPS values exactly.
func (g GIF) Run(frame FrameFunc, cfg Config) error {
	if cfg.FPS <= 0 {
		return fmt.Errorf("present: FPS must be positive, got %d", cfg.FPS)
	}
	if cfg.Duration <= 0 {
		return fmt.Errorf("present: Duration must be positive, got %v", cfg.Duration)
	}

	frameCount := int(math.Round(cfg.Duration.Seconds() * float64(cfg.FPS)))
	if frameCount < 1 {
		frameCount = 1
	}
	delay := int(math.Round(100 / float64(cfg.FPS)))
	if delay < 1 {
		delay = 1
	}

	anim := &gif.GIF{LoopCount: g.Loop}
	for i := 0; i < frameCount; i++ {
		// Sample strictly inside [0, Duration); t = Duration coincides with t = 0,
		// which keeps a periodic animation seamless.
		t := time.Duration(i) * time.Second / time.Duration(cfg.FPS)
		anim.Image = append(anim.Image, quantize(frame(t)))
		anim.Delay = append(anim.Delay, delay)
	}

	f, err := os.Create(g.Path)
	if err != nil {
		return err
	}
	encErr := gif.EncodeAll(f, anim)
	closeErr := f.Close()
	if encErr != nil {
		return encErr
	}
	return closeErr
}

// quantize converts a frame to a paletted image. When the frame has at most 256
// distinct colors it uses an exact palette with no dithering; otherwise it falls
// back to the Plan9 palette with Floyd-Steinberg dithering.
func quantize(src image.Image) *image.Paletted {
	if p := exactPaletted(src); p != nil {
		return p
	}
	dst := image.NewPaletted(src.Bounds(), palette.Plan9)
	draw.FloydSteinberg.Draw(dst, src.Bounds(), src, src.Bounds().Min)
	return dst
}

// exactPaletted returns an image.Paletted reproducing src exactly (one palette
// entry per distinct color, no dithering), or nil if src has more than 256
// distinct colors.
func exactPaletted(src image.Image) *image.Paletted {
	bounds := src.Bounds()
	at := rgbaAt(src)

	index := make(map[color.RGBA]uint8)
	var pal color.Palette
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			c := at(x, y)
			if _, ok := index[c]; !ok {
				if len(pal) >= 256 {
					return nil
				}
				index[c] = uint8(len(pal))
				pal = append(pal, c)
			}
		}
	}
	// A GIF color table needs at least two entries.
	for len(pal) < 2 {
		pal = append(pal, color.RGBA{A: 255})
	}

	dst := image.NewPaletted(bounds, pal)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			dst.Pix[dst.PixOffset(x, y)] = index[at(x, y)]
		}
	}
	return dst
}

// rgbaAt returns an accessor reading src as non-premultiplied color.RGBA, with a
// fast path for *image.RGBA.
func rgbaAt(src image.Image) func(x, y int) color.RGBA {
	if im, ok := src.(*image.RGBA); ok {
		return im.RGBAAt
	}
	return func(x, y int) color.RGBA {
		r, g, b, a := src.At(x, y).RGBA()
		return color.RGBA{R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(b >> 8), A: uint8(a >> 8)}
	}
}
