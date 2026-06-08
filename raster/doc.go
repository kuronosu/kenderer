// Package raster rasterizes screen-space triangles into a framebuffer. It owns
// the inner loop: edge-function coverage with a top-left fill rule, the depth
// test against the Z-buffer, perspective-correct attribute interpolation, and
// invoking the shader for each surviving fragment.
//
// Input vertices are already in screen space (x, y in pixels, z in [0, 1]); the
// pipeline performs the transform, clip, perspective-divide and viewport stages
// that produce them.
package raster
