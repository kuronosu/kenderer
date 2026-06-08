package pipeline

import (
	"github.com/kuronosu/kenderer/math3d"
	"github.com/kuronosu/kenderer/shading"
)

// clipVertex is a vertex in clip space (before the perspective divide) carrying
// the varyings that must be interpolated when an edge is split by clipping.
type clipVertex struct {
	Pos  math3d.Vec4
	Frag shading.Fragment
}

func lerpClipVertex(a, b clipVertex, t float64) clipVertex {
	return clipVertex{
		Pos:  a.Pos.Lerp(b.Pos, t),
		Frag: shading.LerpFragment(a.Frag, b.Frag, t),
	}
}

// clipPlanes are the six frustum planes expressed in clip space. Each returns a
// signed distance that is >= 0 when the point lies on the inside of the plane.
var clipPlanes = [...]func(math3d.Vec4) float64{
	func(p math3d.Vec4) float64 { return p.W + p.X }, // left
	func(p math3d.Vec4) float64 { return p.W - p.X }, // right
	func(p math3d.Vec4) float64 { return p.W + p.Y }, // bottom
	func(p math3d.Vec4) float64 { return p.W - p.Y }, // top
	func(p math3d.Vec4) float64 { return p.W + p.Z }, // near
	func(p math3d.Vec4) float64 { return p.W - p.Z }, // far
}

// clipTriangle clips a triangle against the full view frustum in clip space with
// the Sutherland-Hodgman algorithm, returning the result as a triangle fan (nil
// when the triangle lies entirely outside the frustum).
func clipTriangle(v0, v1, v2 clipVertex) [][3]clipVertex {
	poly := []clipVertex{v0, v1, v2}
	for _, dist := range clipPlanes {
		poly = clipAgainstPlane(poly, dist)
		if len(poly) < 3 {
			return nil
		}
	}
	tris := make([][3]clipVertex, 0, len(poly)-2)
	for i := 1; i+1 < len(poly); i++ {
		tris = append(tris, [3]clipVertex{poly[0], poly[i], poly[i+1]})
	}
	return tris
}

// clipAgainstPlane runs one Sutherland-Hodgman pass, clipping a convex polygon
// against a single plane. dist gives each vertex's signed distance, with inside
// being >= 0. Vertices exactly on the plane are kept.
func clipAgainstPlane(poly []clipVertex, dist func(math3d.Vec4) float64) []clipVertex {
	out := make([]clipVertex, 0, len(poly)+1)
	prev := poly[len(poly)-1]
	prevD := dist(prev.Pos)
	for _, curr := range poly {
		currD := dist(curr.Pos)
		switch {
		case currD >= 0:
			if prevD < 0 { // edge enters: add the crossing first
				out = append(out, lerpClipVertex(prev, curr, prevD/(prevD-currD)))
			}
			out = append(out, curr)
		case prevD >= 0: // edge leaves: add the crossing only
			out = append(out, lerpClipVertex(prev, curr, prevD/(prevD-currD)))
		}
		prev, prevD = curr, currD
	}
	return out
}
