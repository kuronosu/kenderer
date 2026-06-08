// Package geometry defines the renderer's mesh representation - indexed triangle
// lists with per-vertex position, normal, texture UV and color - together with
// primitive constructors such as NewCube.
//
// Triangles are wound counter-clockwise as seen from the front (outside); the
// pipeline relies on that winding for backface culling.
package geometry
