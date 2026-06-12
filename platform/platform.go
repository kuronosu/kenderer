//go:build sdl

// Package platform is the interactive, real-time runtime: it opens an OS window,
// runs the frame loop, and translates SDL events into the backend-agnostic
// input.Frame. It is gated behind the "sdl" build tag, so the default build and
// the offline GIF path never depend on SDL.
//
// SDL3 is loaded from an embedded copy (github.com/Zyko0/go-sdl3/bin/binsdl), so
// no system SDL3 install is required.
package platform

import (
	"fmt"
	"image"
	"runtime"
	"time"

	"github.com/Zyko0/go-sdl3/bin/binsdl"
	"github.com/Zyko0/go-sdl3/sdl"
	"github.com/kuronosu/kenderer/input"
)

// bgR, bgG, bgB, bgA are the renderer's clear color; it is also restored after
// the stats overlay temporarily changes the draw color (see drawStats).
const bgR, bgG, bgB, bgA uint8 = 18, 18, 24, 255

// Config configures the window and loop.
type Config struct {
	Title         string
	Width, Height int
	FPS           int  // target frames per second (<= 0 means 60)
	ShowStats     bool // show the FPS/frame-time overlay (toggle at runtime with F1)
	Fullscreen    bool // open fullscreen; bypasses the desktop compositor's Present pacing
}

// App is the interactive application driven by the loop. Update receives the
// elapsed time and current input; Render returns the frame to display (its size
// must match the most recent Resize); Resize is called whenever the drawable
// size changes, including once at startup.
type App interface {
	Update(dt time.Duration, in input.Frame)
	Render() *image.RGBA
	Resize(w, h int)
}

// Run opens the window and runs the loop until the user quits (window close or
// Escape). SDL3 is loaded from the embedded library.
func Run(cfg Config, app App) error {
	// SDL's video/event handling must stay on the main OS thread.
	runtime.LockOSThread()

	defer binsdl.Load().Unload()

	if err := sdl.Init(sdl.INIT_VIDEO); err != nil {
		return fmt.Errorf("platform: init: %w", err)
	}
	defer sdl.Quit()

	flags := sdl.WINDOW_RESIZABLE
	if cfg.Fullscreen {
		// Fullscreen lets the swapchain bypass the windowed compositor (DWM),
		// which otherwise paces Present to the display's refresh rate.
		flags |= sdl.WINDOW_FULLSCREEN
	}
	window, renderer, err := sdl.CreateWindowAndRenderer(cfg.Title, cfg.Width, cfg.Height, flags)
	if err != nil {
		return fmt.Errorf("platform: create window/renderer: %w", err)
	}
	defer window.Destroy()
	defer renderer.Destroy()

	// Use the drawable size in pixels (HiDPI-correct), not the logical window size.
	pw, ph, err := renderer.CurrentOutputSize()
	if err != nil {
		return fmt.Errorf("platform: output size: %w", err)
	}
	w, h := int(pw), int(ph)

	texture, err := renderer.CreateTexture(sdl.PIXELFORMAT_ABGR8888, sdl.TEXTUREACCESS_STREAMING, w, h)
	if err != nil {
		return fmt.Errorf("platform: create texture: %w", err)
	}
	defer func() {
		if texture != nil {
			texture.Destroy()
		}
	}()

	if err := renderer.SetDrawColor(bgR, bgG, bgB, bgA); err != nil {
		return fmt.Errorf("platform: set draw color: %w", err)
	}

	app.Resize(w, h)

	in := input.Frame{Width: w, Height: h}
	fps := cfg.FPS
	if fps <= 0 {
		fps = 60
	}
	frameBudget := time.Second / time.Duration(fps)

	// Stats overlay state. showStats is the live toggle (F1); the counters
	// accumulate over statInterval and feed the smoothed statText.
	showStats := cfg.ShowStats
	var prevF1 bool
	const statInterval = 500 * time.Millisecond
	var (
		statFrames int
		statWall   time.Duration // wall-clock elapsed, for FPS
		statBusy   time.Duration // per-frame work, for ms
		statText   = "-- FPS  -- ms"
	)

	last := time.Now()
	var event sdl.Event
	for !in.Quit {
		frameStart := time.Now()
		dt := frameStart.Sub(last)
		last = frameStart

		// Per-frame deltas reset; absolute state (position, buttons, size) persists.
		in.MouseDX, in.MouseDY, in.Wheel = 0, 0, 0

		for sdl.PollEvent(&event) {
			switch event.Type {
			case sdl.EVENT_QUIT:
				in.Quit = true
			case sdl.EVENT_MOUSE_MOTION:
				m := event.MouseMotionEvent()
				in.MouseX, in.MouseY = float64(m.X), float64(m.Y)
				in.MouseDX += float64(m.Xrel)
				in.MouseDY += float64(m.Yrel)
			case sdl.EVENT_MOUSE_BUTTON_DOWN, sdl.EVENT_MOUSE_BUTTON_UP:
				b := event.MouseButtonEvent()
				down := event.Type == sdl.EVENT_MOUSE_BUTTON_DOWN
				switch b.Button {
				case uint8(sdl.BUTTON_LEFT):
					in.Left = down
				case uint8(sdl.BUTTON_MIDDLE):
					in.Middle = down
				case uint8(sdl.BUTTON_RIGHT):
					in.Right = down
				}
			case sdl.EVENT_MOUSE_WHEEL:
				in.Wheel += float64(event.MouseWheelEvent().Y)
			case sdl.EVENT_WINDOW_RESIZED, sdl.EVENT_WINDOW_PIXEL_SIZE_CHANGED:
				nw, nh, e := renderer.CurrentOutputSize()
				if e == nil && (int(nw) != w || int(nh) != h) {
					w, h = int(nw), int(nh)
					texture.Destroy()
					if texture, err = renderer.CreateTexture(sdl.PIXELFORMAT_ABGR8888, sdl.TEXTUREACCESS_STREAMING, w, h); err != nil {
						return fmt.Errorf("platform: recreate texture: %w", err)
					}
					in.Width, in.Height = w, h
					app.Resize(w, h)
				}
			}
		}

		// Frame-coherent keyboard snapshot for the shift modifier and Escape-to-quit.
		keys := sdl.GetKeyboardState()
		in.Shift = keys[int(sdl.SCANCODE_LSHIFT)] || keys[int(sdl.SCANCODE_RSHIFT)]
		if keys[int(sdl.SCANCODE_ESCAPE)] {
			in.Quit = true
		}
		// App-controlled toggle keys exposed as level state; the app edge-detects.
		in.F2 = keys[int(sdl.SCANCODE_F2)]
		in.F3 = keys[int(sdl.SCANCODE_F3)]

		// F1 toggles the stats overlay on the press edge (not while held).
		f1 := keys[int(sdl.SCANCODE_F1)]
		if f1 && !prevF1 {
			showStats = !showStats
		}
		prevF1 = f1

		app.Update(dt, in)

		img := app.Render()
		if err := texture.Update(nil, img.Pix, int32(img.Stride)); err != nil {
			return fmt.Errorf("platform: update texture: %w", err)
		}
		if err := renderer.Clear(); err != nil {
			return fmt.Errorf("platform: clear: %w", err)
		}
		// nil dst fills the whole window; texture tracks the window size, so 1:1.
		if err := renderer.RenderTexture(texture, nil, nil); err != nil {
			return fmt.Errorf("platform: render texture: %w", err)
		}
		if showStats {
			if err := drawStats(renderer, statText); err != nil {
				return fmt.Errorf("platform: draw stats: %w", err)
			}
		}
		// busy is the frame's work, measured BEFORE Present so neither a
		// vsync-blocking Present nor the frame-cap sleep below leaks into it.
		busy := time.Since(frameStart)
		if err := renderer.Present(); err != nil {
			return fmt.Errorf("platform: present: %w", err)
		}

		// Smoothed stats, refreshed ~twice a second so the text doesn't flicker:
		// FPS from wall-clock dt, ms from work time (real cost even when capped).
		statFrames++
		statWall += dt
		statBusy += busy
		if statWall >= statInterval {
			statFPS := float64(statFrames) / statWall.Seconds()
			statMS := statBusy.Seconds() / float64(statFrames) * 1000
			statText = fmt.Sprintf("%.0f FPS  %.1f ms", statFPS, statMS)
			statFrames, statWall, statBusy = 0, 0, 0
		}

		if slack := frameBudget - time.Since(frameStart); slack > 0 {
			time.Sleep(slack)
		}
	}
	return nil
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
