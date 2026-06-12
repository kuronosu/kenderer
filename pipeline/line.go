package pipeline

import (
	"github.com/kuronosu/kenderer/math3d"
	"github.com/kuronosu/kenderer/raster"
	"github.com/kuronosu/kenderer/scene"
	"github.com/kuronosu/kenderer/shading"
)

// axisFraction sets each object's local-axis length as a fraction of its own
// bounding box's largest extent. Above 1 the axes poke clearly outside the mesh
// (Blender style); with a small fraction the depth test would hide the part that
// lies inside the geometry, leaving only a short visible stub. Purely cosmetic.
const axisFraction = 1.25

// drawLines is the line pass: it runs serially in Render after the parallel
// triangle fill has finished (the wait barrier), so it reads a fully populated
// z-buffer and is deterministic regardless of the fill worker count. It draws the
// caller-provided world segments (Scene.Lines, e.g. the world axes) and, when
// object axes are enabled, each object's local coordinate frame.
//
// It is a strict no-op when there is nothing to draw, leaving the framebuffer
// byte-for-byte as the triangle fill left it — so the whole feature is inert (and
// the output unchanged) unless a caller opts in.
func (r *Renderer) drawLines(s scene.Scene, viewProj math3d.Mat4) {
	if len(s.Lines) == 0 && !r.objectAxes {
		return
	}
	for _, seg := range s.Lines {
		r.drawSegment(viewProj, seg.A, seg.B, seg.Color)
	}
	if r.objectAxes {
		for _, obj := range s.Objects {
			m := obj.Transform.Matrix()
			l := objectAxisLength(obj)
			origin := m.MulVec4(math3d.V4(0, 0, 0, 1)).XYZ()
			r.drawSegment(viewProj, origin, m.MulVec4(math3d.V4(l, 0, 0, 1)).XYZ(), scene.AxisColorX)
			r.drawSegment(viewProj, origin, m.MulVec4(math3d.V4(0, l, 0, 1)).XYZ(), scene.AxisColorY)
			r.drawSegment(viewProj, origin, m.MulVec4(math3d.V4(0, 0, l, 1)).XYZ(), scene.AxisColorZ)
		}
	}
}

// drawSegment transforms a world-space segment by viewProj into clip space, clips
// it against the frustum, perspective-divides and viewport-maps the survivors, and
// rasterizes the result in colorLin (encoded to sRGB once here, not per pixel).
// Segments wholly outside the frustum are dropped.
func (r *Renderer) drawSegment(viewProj math3d.Mat4, aWorld, bWorld, colorLin math3d.Vec3) {
	a := viewProj.MulVec4(aWorld.Vec4(1))
	b := viewProj.MulVec4(bWorld.Vec4(1))
	ca, cb, ok := clipSegment(a, b)
	if !ok {
		return
	}
	sa, _ := r.toScreen(ca)
	sb, _ := r.toScreen(cb)
	raster.DrawLine(r.fb, sa, sb, shading.ToRGBA(colorLin))
}

// toScreen performs the perspective divide and viewport map for one clip-space
// point, returning the screen-space position (x, y in pixels, z window depth) and
// the reciprocal clip-space w. It is the single clip→screen mapping shared by the
// triangle setup (prepare, which keeps invW for perspective-correct attributes)
// and the line pass (drawSegment, which discards it — lines carry no such
// attributes). Callers must ensure w > 0; clipping guarantees it for points inside
// the frustum.
func (r *Renderer) toScreen(clip math3d.Vec4) (math3d.Vec3, float64) {
	invW := 1 / clip.W
	ndc := clip.XYZ().Scale(invW)
	return r.viewport.MulVec4(ndc.Vec4(1)).XYZ(), invW
}

// clipSegment clips the clip-space segment a-b against the six frustum planes with
// the parametric Liang-Barsky algorithm, returning the visible sub-segment and
// ok=false when the segment lies entirely outside. Clipping happens before the
// perspective divide, so a segment crossing the near plane is trimmed in clip space
// and never reaches a divide by a non-positive w (the classic line-clipping bug).
// It reuses the very same clipPlanes as the triangle clipper, so lines and
// triangles agree on the frustum exactly.
func clipSegment(a, b math3d.Vec4) (math3d.Vec4, math3d.Vec4, bool) {
	tEnter, tLeave := 0.0, 1.0
	for _, dist := range clipPlanes {
		da, db := dist(a), dist(b)
		switch {
		case da < 0 && db < 0:
			return a, b, false // wholly on the outside of this plane
		case da >= 0 && db >= 0:
			// wholly inside this plane; no clip
		default:
			t := da / (da - db) // parameter where the edge crosses the plane
			if da < 0 {         // segment enters the inside half-space as t grows
				tEnter = max(tEnter, t)
			} else { // segment leaves the inside half-space
				tLeave = min(tLeave, t)
			}
		}
		if tEnter > tLeave {
			return a, b, false
		}
	}
	return a.Lerp(b, tEnter), a.Lerp(b, tLeave), true
}

// objectAxisLength returns the length of the object's local axes, measured in the
// object's own (model) space as axisFraction of its mesh bounding box's largest
// extent. Because the tips are then transformed by the model matrix, the per-object
// transform (including scale) carries through automatically, so the axes auto-scale
// to each object — unlike scene.Bounds, which ignores transforms. A degenerate
// (zero-extent) mesh falls back to unit length.
func objectAxisLength(obj scene.Object) float64 {
	lo, hi := obj.Mesh.Bounds()
	ext := hi.Sub(lo)
	size := max(ext.X, ext.Y, ext.Z)
	if size == 0 {
		size = 1
	}
	return axisFraction * size
}
