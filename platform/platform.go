//go:build sdl

// Package platform is the interactive, real-time runtime: it opens an OS window,
// runs the frame loop, translates SDL events into the backend-agnostic
// input.Frame, and hands each frame's scene to a rendering Backend — the
// software backend (the CPU rasterizer blitted through an SDL texture, and the
// reference for correctness) or the SDL_GPU backend (Vulkan via offline-compiled
// SPIR-V shaders, parity-tested against the software one). It is gated behind
// the "sdl" build tag, so the default build and the offline GIF path never
// depend on SDL.
//
// SDL3 is loaded from an embedded copy (github.com/Zyko0/go-sdl3/bin/binsdl), so
// no system SDL3 install is required.
package platform

import (
	"fmt"
	"runtime"
	"time"

	"github.com/Zyko0/go-sdl3/bin/binsdl"
	"github.com/Zyko0/go-sdl3/sdl"
	"github.com/kuronosu/kenderer/input"
	"github.com/kuronosu/kenderer/scene"
)

// Config configures the window and loop.
type Config struct {
	Title         string
	Width, Height int
	FPS           int  // target frames per second (<= 0 means 60)
	ShowStats     bool // show the FPS/frame-time overlay (toggle at runtime with F1)
	Fullscreen    bool // open fullscreen; bypasses the desktop compositor's Present pacing
}

// App is the interactive application driven by the loop. Update receives the
// elapsed time and current input; Scene returns the scene the Backend renders
// right after the update. The backend reads the returned scene during
// RenderFrame and does not retain it.
type App interface {
	Update(dt time.Duration, in input.Frame)
	Scene() *scene.Scene
}

// Run opens the window, initializes the backend on it, and runs the loop until
// the user quits (window close or Escape). SDL3 is loaded from the embedded
// library.
func Run(cfg Config, app App, backend Backend) error {
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
	window, err := sdl.CreateWindow(cfg.Title, cfg.Width, cfg.Height, flags)
	if err != nil {
		return fmt.Errorf("platform: create window: %w", err)
	}
	defer window.Destroy()

	w, h, err := drawableSize(window)
	if err != nil {
		return err
	}
	if err := backend.Init(window, w, h); err != nil {
		return fmt.Errorf("platform: backend init: %w", err)
	}
	defer backend.Close()

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
				nw, nh, e := drawableSize(window)
				if e == nil && (nw != w || nh != h) {
					w, h = nw, nh
					if err := backend.Resize(w, h); err != nil {
						return fmt.Errorf("platform: backend resize: %w", err)
					}
					in.Width, in.Height = w, h
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

		stats := ""
		if showStats {
			stats = statText
		}
		// busy is the frame's work: the loop's share measured here plus the
		// backend's share measured before its present, so neither a
		// vsync-blocking present nor the frame-cap sleep below leaks into it.
		busy := time.Since(frameStart)
		renderBusy, err := backend.RenderFrame(app.Scene(), stats)
		if err != nil {
			return fmt.Errorf("platform: render frame: %w", err)
		}
		busy += renderBusy

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

// drawableSize returns the window's current size in pixels (HiDPI-correct), not
// its logical size.
func drawableSize(window *sdl.Window) (int, int, error) {
	pw, ph, err := window.SizeInPixels()
	if err != nil {
		return 0, 0, fmt.Errorf("platform: window pixel size: %w", err)
	}
	return int(pw), int(ph), nil
}
