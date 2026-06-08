// Package input defines backend-agnostic, real-time input state for the
// interactive runtime. It has no dependency on any windowing or SDL library, so
// the camera and the main loop depend only on these types and never on a
// specific backend.
package input

// Frame is the input state for a single frame. Absolute values (MouseX/MouseY,
// the button states, Width/Height) persist across frames; the per-frame deltas
// (MouseDX/MouseDY, Wheel) are reset to zero by the loop at the start of each
// frame and accumulate from the events seen during it.
type Frame struct {
	MouseX, MouseY      float64 // current cursor position, in pixels
	MouseDX, MouseDY    float64 // cursor movement during this frame
	Wheel               float64 // vertical wheel movement during this frame
	Left, Right, Middle bool    // mouse button states
	Shift               bool    // shift modifier held (e.g. shift+drag to pan)
	Width, Height       int     // current drawable size, in pixels
	Quit                bool    // user requested to close the window
}
