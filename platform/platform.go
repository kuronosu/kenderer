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

// Config configures the window and loop.
type Config struct {
	Title         string
	Width, Height int
	FPS           int // target frames per second (<= 0 means 60)
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

	window, renderer, err := sdl.CreateWindowAndRenderer(cfg.Title, cfg.Width, cfg.Height, sdl.WINDOW_RESIZABLE)
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

	if err := renderer.SetDrawColor(18, 18, 24, 255); err != nil {
		return fmt.Errorf("platform: set draw color: %w", err)
	}

	app.Resize(w, h)

	in := input.Frame{Width: w, Height: h}
	fps := cfg.FPS
	if fps <= 0 {
		fps = 60
	}
	frameBudget := time.Second / time.Duration(fps)

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
		if err := renderer.Present(); err != nil {
			return fmt.Errorf("platform: present: %w", err)
		}

		if slack := frameBudget - time.Since(frameStart); slack > 0 {
			time.Sleep(slack)
		}
	}
	return nil
}
