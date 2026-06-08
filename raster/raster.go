package raster

import (
	"math"

	"github.com/kuronosu/kenderer/framebuffer"
	"github.com/kuronosu/kenderer/math3d"
	"github.com/kuronosu/kenderer/shading"
)

// Vertex is a triangle vertex ready for rasterization: position already in
// screen space, the reciprocal of clip-space w for perspective-correct attribute
// interpolation, and the varyings passed to the shader.
type Vertex struct {
	Pos  math3d.Vec3      // x, y in pixels; z is window depth in [0, 1]
	InvW float64          // 1 / clip-space w
	Frag shading.Fragment // world position, normal, UV, color
}

// SignedArea returns twice the signed area of triangle (a, b, c). It is positive
// when the vertices wind counter-clockwise in a Y-up frame and serves as the
// edge function used for both coverage and barycentric weights.
func SignedArea(a, b, c math3d.Vec2) float64 {
	return (b.X-a.X)*(c.Y-a.Y) - (b.Y-a.Y)*(c.X-a.X)
}

// Barycentric returns the barycentric weights of p relative to triangle
// (a, b, c). They sum to 1 and are all non-negative exactly when p is inside.
func Barycentric(a, b, c, p math3d.Vec2) (b0, b1, b2 float64) {
	inv := 1 / SignedArea(a, b, c)
	return SignedArea(b, c, p) * inv, SignedArea(c, a, p) * inv, SignedArea(a, b, p) * inv
}

// DrawTriangle rasterizes one screen-space triangle into fb, shading the covered
// pixels that pass the depth test.
//
// Coverage uses edge functions with a top-left fill rule, so a shared edge is
// rasterized by exactly one of the adjacent triangles (watertight, no double
// shading). Window depth is interpolated linearly with the geometric barycentric
// weights, which is exact in screen space; the shader varyings are interpolated
// perspective-correctly via shading.CombineFragment.
func DrawTriangle(fb *framebuffer.Buffer, v0, v1, v2 Vertex, sh shading.Shader) {
	p0, p1, p2 := v0.Pos.XY(), v1.Pos.XY(), v2.Pos.XY()

	area := SignedArea(p0, p1, p2)
	if area == 0 {
		return // degenerate, zero-area triangle
	}
	invArea := 1 / area
	s := math.Copysign(1, area) // winding sign

	// Classify each edge (edge i is opposite vertex i) for the top-left rule.
	tl0 := topLeft(p2.Sub(p1), s)
	tl1 := topLeft(p0.Sub(p2), s)
	tl2 := topLeft(p1.Sub(p0), s)

	minX, minY, maxX, maxY := boundingBox(p0, p1, p2, fb.Width, fb.Height)

	for y := minY; y < maxY; y++ {
		for x := minX; x < maxX; x++ {
			p := math3d.V2(float64(x)+0.5, float64(y)+0.5)
			e0 := SignedArea(p1, p2, p)
			e1 := SignedArea(p2, p0, p)
			e2 := SignedArea(p0, p1, p)

			// Normalize by winding so "inside" means E >= 0 either way.
			E0, E1, E2 := s*e0, s*e1, s*e2
			if E0 < 0 || E1 < 0 || E2 < 0 {
				continue
			}
			// An exactly-on-edge sample is taken only for top-left edges.
			if (E0 == 0 && !tl0) || (E1 == 0 && !tl1) || (E2 == 0 && !tl2) {
				continue
			}

			b0, b1, b2 := e0*invArea, e1*invArea, e2*invArea

			// Window depth is linear in screen space: geometric weights, no 1/w.
			z := b0*v0.Pos.Z + b1*v1.Pos.Z + b2*v2.Pos.Z
			if z >= fb.DepthAt(x, y) {
				continue
			}

			frag := shading.CombineFragment(
				v0.Frag, v1.Frag, v2.Frag,
				b0, b1, b2,
				v0.InvW, v1.InvW, v2.InvW,
			)
			fb.Set(x, y, shading.ToRGBA(sh.Shade(frag)), z)
		}
	}
}

// topLeft reports whether an edge with delta d (oriented along the triangle's
// boundary, winding sign s) is a top or left edge. Such edges own the samples
// that fall exactly on them. Derivation: the interior lies toward +s*(-d.Y, d.X),
// so a left edge has s*d.Y < 0 and a (horizontal) top edge has s*d.X > 0.
func topLeft(d math3d.Vec2, s float64) bool {
	return s*d.Y < 0 || (d.Y == 0 && s*d.X > 0)
}

// boundingBox returns the half-open pixel rectangle [minX,maxX) x [minY,maxY)
// covering the triangle, clamped to the framebuffer.
func boundingBox(p0, p1, p2 math3d.Vec2, w, h int) (minX, minY, maxX, maxY int) {
	minX = int(math.Floor(min(p0.X, p1.X, p2.X)))
	minY = int(math.Floor(min(p0.Y, p1.Y, p2.Y)))
	maxX = int(math.Ceil(max(p0.X, p1.X, p2.X)))
	maxY = int(math.Ceil(max(p0.Y, p1.Y, p2.Y)))
	if minX < 0 {
		minX = 0
	}
	if minY < 0 {
		minY = 0
	}
	if maxX > w {
		maxX = w
	}
	if maxY > h {
		maxY = h
	}
	return minX, minY, maxX, maxY
}
