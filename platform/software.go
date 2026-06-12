//go:build sdl

package platform

import (
	"fmt"
	"time"

	"github.com/Zyko0/go-sdl3/sdl"
	"github.com/kuronosu/kenderer/pipeline"
	"github.com/kuronosu/kenderer/scene"
)

// bgR, bgG, bgB, bgA are the SDL renderer's clear color; it is also restored
// after the stats overlay temporarily changes the draw color (see drawStats).
const bgR, bgG, bgB, bgA uint8 = 18, 18, 24, 255

// softwareBackend draws frames with the CPU rasterizer (pipeline.Renderer) and
// presents them by streaming the rendered image into an SDL texture. It is the
// reference Backend: the GPU implementation is validated against its output.
type softwareBackend struct {
	opts       pipeline.Options
	objectAxes bool

	renderer *pipeline.Renderer
	sdlr     *sdl.Renderer
	texture  *sdl.Texture
}

// NewSoftwareBackend returns the CPU rasterizer Backend. opts.Width and
// opts.Height are placeholders: the window's drawable size replaces them at
// Init and on every Resize.
func NewSoftwareBackend(opts pipeline.Options) Backend {
	return &softwareBackend{opts: opts}
}

func (b *softwareBackend) Init(window *sdl.Window, w, h int) error {
	// Empty name = no driver preference, exactly like CreateWindowAndRenderer.
	sdlr, err := window.CreateRenderer("")
	if err != nil {
		return fmt.Errorf("create renderer: %w", err)
	}
	b.sdlr = sdlr
	if err := b.sdlr.SetDrawColor(bgR, bgG, bgB, bgA); err != nil {
		return fmt.Errorf("set draw color: %w", err)
	}
	return b.Resize(w, h)
}

// Resize recreates the framebuffer and the streaming texture at the new
// drawable size. The texture must track the window exactly so the blit in
// RenderFrame stays 1:1.
func (b *softwareBackend) Resize(w, h int) error {
	b.opts.Width, b.opts.Height = w, h
	if b.renderer == nil {
		b.renderer = pipeline.NewRenderer(b.opts)
		b.renderer.SetObjectAxes(b.objectAxes)
	} else {
		b.renderer.Resize(w, h)
	}

	if b.texture != nil {
		b.texture.Destroy()
		b.texture = nil
	}
	texture, err := b.sdlr.CreateTexture(sdl.PIXELFORMAT_ABGR8888, sdl.TEXTUREACCESS_STREAMING, w, h)
	if err != nil {
		return fmt.Errorf("create texture: %w", err)
	}
	b.texture = texture
	return nil
}

func (b *softwareBackend) RenderFrame(s *scene.Scene, stats string) (time.Duration, error) {
	start := time.Now()
	img := b.renderer.Render(*s).Image()
	if err := b.texture.Update(nil, img.Pix, int32(img.Stride)); err != nil {
		return 0, fmt.Errorf("update texture: %w", err)
	}
	if err := b.sdlr.Clear(); err != nil {
		return 0, fmt.Errorf("clear: %w", err)
	}
	// nil dst fills the whole window; the texture tracks the window size, so 1:1.
	if err := b.sdlr.RenderTexture(b.texture, nil, nil); err != nil {
		return 0, fmt.Errorf("render texture: %w", err)
	}
	if stats != "" {
		if err := drawStats(b.sdlr, stats); err != nil {
			return 0, fmt.Errorf("draw stats: %w", err)
		}
	}
	// Measured BEFORE Present so a vsync block does not count as frame work.
	busy := time.Since(start)
	if err := b.sdlr.Present(); err != nil {
		return 0, fmt.Errorf("present: %w", err)
	}
	return busy, nil
}

func (b *softwareBackend) SetObjectAxes(on bool) {
	b.objectAxes = on
	if b.renderer != nil {
		b.renderer.SetObjectAxes(on)
	}
}

func (b *softwareBackend) Close() {
	if b.texture != nil {
		b.texture.Destroy()
		b.texture = nil
	}
	if b.sdlr != nil {
		b.sdlr.Destroy()
		b.sdlr = nil
	}
}

// drawStats overlays the smoothed FPS/ms text in the top-left corner using SDL's
// built-in 8x8 debug font, scaled 3x for legibility. It restores the render scale
// and the background draw color afterward so the next frame's Clear and texture
// blit are unaffected (both persist across frames otherwise).
func drawStats(r *sdl.Renderer, text string) error {
	if err := r.SetScale(3, 3); err != nil { // 8x8 glyphs -> ~24px tall
		return err
	}
	if err := r.SetDrawColor(0, 255, 0, 255); err != nil { // green
		return err
	}
	if err := r.DebugText(2, 2, text); err != nil { // (2,2) in scaled space -> (6,6) px
		return err
	}
	if err := r.SetScale(1, 1); err != nil { // reset so the next blit is 1:1
		return err
	}
	return r.SetDrawColor(bgR, bgG, bgB, bgA) // restore the Clear() color
}
