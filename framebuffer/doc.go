// Package framebuffer provides the renderer's in-memory render target: an RGBA
// color buffer paired with a float64 depth buffer. It is agnostic to how the
// result is displayed - presentation backends simply consume Buffer.Image().
//
// Depth uses the pipeline convention of window z in [0, 1] (0 = near, 1 = far).
// Clear resets depth to +Inf so the first fragment written to any pixel always
// passes the depth test.
package framebuffer
