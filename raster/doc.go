// Package raster rasterizes screen-space triangles into a framebuffer. It owns
// the inner loop: edge-function coverage with a top-left fill rule, the depth
// test against the Z-buffer, perspective-correct attribute interpolation, and
// invoking the shader for each surviving fragment.
//
// Input vertices are already in screen space (x, y in pixels, z in [0, 1]); the
// pipeline performs the transform, clip, perspective-divide and viewport stages
// that produce them.
//
// DrawLine is the line analog of the triangle fill: a screen-space DDA that draws
// a constant-color, one-pixel-wide segment, depth-testing each pixel (z < stored)
// but never writing the depth buffer, so debug lines are occluded by geometry yet
// leave the z-buffer untouched.
package raster
