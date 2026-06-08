package present

import (
	"fmt"
	"image"
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
// frame to a 256-color palette (Plan9) with Floyd-Steinberg dithering, and writes
// the animated GIF to Path.
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
		src := frame(t)

		bounds := src.Bounds()
		paletted := image.NewPaletted(bounds, palette.Plan9)
		draw.FloydSteinberg.Draw(paletted, bounds, src, bounds.Min)

		anim.Image = append(anim.Image, paletted)
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
