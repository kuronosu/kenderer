package raster

import (
	"image/color"
	"math"

	"github.com/kuronosu/kenderer/framebuffer"
	"github.com/kuronosu/kenderer/math3d"
)

// DrawLine rasterizes the screen-space segment a-b into fb with a DDA, in a
// constant color c and one pixel wide. The endpoints are already in screen space:
// x, y in pixels and z the window depth in [0, 1], exactly like raster.Vertex.Pos.
//
// Each plotted pixel is depth-TESTED against fb (it is drawn only where its
// interpolated window z is nearer than the stored depth, z < fb.DepthAt) but the
// depth buffer is NOT written. Lines are therefore occluded by previously
// rasterized geometry yet never disturb the z-buffer, so several debug lines that
// cross — the axes meeting at the origin, say — do not z-fight each other; at an
// exact overlap the last one drawn wins, which is deterministic for a serial pass.
//
// Window z is linear in screen space (the same convention the triangle fill uses),
// so it is interpolated with the plain segment parameter — no perspective
// correction. The transform, clip-against-the-frustum and perspective divide that
// produce valid screen-space endpoints are the pipeline's job (see pipeline.Renderer
// line pass); DrawLine is the screen-space inner loop, the line analog of DrawBand.
func DrawLine(fb *framebuffer.Buffer, a, b math3d.Vec3, c color.RGBA) {
	dx, dy := b.X-a.X, b.Y-a.Y
	steps := int(math.Ceil(math.Max(math.Abs(dx), math.Abs(dy))))
	if steps == 0 { // degenerate: both endpoints map to one pixel
		plot(fb, a.X, a.Y, a.Z, c)
		return
	}
	inv := 1 / float64(steps)
	for i := 0; i <= steps; i++ {
		t := float64(i) * inv
		plot(fb, a.X+dx*t, a.Y+dy*t, a.Z+(b.Z-a.Z)*t, c)
	}
}

// plot writes c at the pixel containing (fx, fy) if that pixel is inside the
// framebuffer and the sample depth z passes the depth test (z < stored). It never
// writes the depth buffer; see DrawLine.
func plot(fb *framebuffer.Buffer, fx, fy, z float64, c color.RGBA) {
	x, y := int(math.Round(fx)), int(math.Round(fy))
	if x < 0 || y < 0 || x >= fb.Width || y >= fb.Height {
		return
	}
	if z < fb.DepthAt(x, y) {
		fb.Color.SetRGBA(x, y, c)
	}
}
