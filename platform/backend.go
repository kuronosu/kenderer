//go:build sdl

package platform

import (
	"time"

	"github.com/Zyko0/go-sdl3/sdl"
	"github.com/kuronosu/kenderer/scene"
)

// Backend renders scenes into the window and owns its presentation path end to
// end (SDL_Renderer blit or GPU swapchain, overlay included). The loop hands it
// the scene each frame; how pixels reach the screen is entirely the backend's
// business. This is the seam that lets a GPU implementation replace the software
// rasterizer without touching the loop, the app, or the core packages.
type Backend interface {
	// Init binds the backend to the window at its current drawable size in
	// pixels. It is called once, before the first frame.
	Init(window *sdl.Window, w, h int) error
	// RenderFrame renders s and presents it. A non-empty stats string asks the
	// backend to display it however it can (debug-text overlay, window title).
	// The returned busy duration is the work spent before presentation, so the
	// caller's frame-time stats exclude any vsync block inside the present.
	RenderFrame(s *scene.Scene, stats string) (busy time.Duration, err error)
	// Resize adapts the render targets to a new drawable size in pixels.
	Resize(w, h int) error
	// SetObjectAxes toggles drawing each object's local coordinate frame — the
	// backend half of the F3 toggle (world axes travel in Scene.Lines instead).
	// Safe to call at any time, including before Init.
	SetObjectAxes(on bool)
	// Close releases the backend's resources. The window outlives it.
	Close()
}
