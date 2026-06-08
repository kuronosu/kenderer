// Package shading turns interpolated triangle attributes (a Fragment) into a
// linear-RGB color. It defines the Shader interface and a Lambert diffuse
// shader, the light and material data they consume, the perspective-correct
// attribute combiner used by the rasterizer (CombineFragment), the linear
// interpolation used by the clipper (LerpFragment), and ToRGBA, which encodes a
// linear color to 8-bit sRGB.
//
// All shading math is performed in linear RGB; gamma (sRGB) encoding happens
// exactly once, in ToRGBA, immediately before pixels are written.
//
// Flat shading is simply Lambert fed a constant per-face normal; feeding it
// interpolated per-vertex normals instead yields Phong shading with no other
// change to the pipeline.
package shading
