package pipeline

import (
	"image/color"

	"github.com/kuronosu/kenderer/framebuffer"
	"github.com/kuronosu/kenderer/geometry"
	"github.com/kuronosu/kenderer/math3d"
	"github.com/kuronosu/kenderer/raster"
	"github.com/kuronosu/kenderer/scene"
	"github.com/kuronosu/kenderer/shading"
)

// CullMode selects which triangle facing is discarded before rasterization.
type CullMode int

const (
	CullBack  CullMode = iota // discard back-facing triangles (default)
	CullFront                 // discard front-facing triangles
	CullNone                  // draw both facings
)

// Options configures a Renderer.
type Options struct {
	Width, Height int
	Cull          CullMode
	Background    color.RGBA
}

// Renderer renders scenes into a single framebuffer that is reused across frames.
type Renderer struct {
	opts     Options
	fb       *framebuffer.Buffer
	viewport math3d.Mat4
}

// NewRenderer creates a Renderer with the given options. Width and Height must be
// positive.
func NewRenderer(opts Options) *Renderer {
	return &Renderer{
		opts:     opts,
		fb:       framebuffer.New(opts.Width, opts.Height),
		viewport: math3d.Viewport(0, 0, float64(opts.Width), float64(opts.Height)),
	}
}

// Resize reallocates the framebuffer to w x h pixels and recomputes the viewport.
// The projection follows automatically: Render derives the aspect ratio from the
// current size on every call. Width and Height must be positive.
func (r *Renderer) Resize(w, h int) {
	r.opts.Width, r.opts.Height = w, h
	r.fb = framebuffer.New(w, h)
	r.viewport = math3d.Viewport(0, 0, float64(w), float64(h))
}

// Render draws the scene into the Renderer's framebuffer and returns it.
//
// The returned *framebuffer.Buffer (and the image from its Image method) is
// reused on every call and is valid only until the next call to Render. A caller
// that needs to keep or share a frame must copy or encode it first; the GIF
// presenter is safe because it quantizes each frame before requesting the next.
func (r *Renderer) Render(s scene.Scene) *framebuffer.Buffer {
	r.fb.Clear(r.opts.Background)

	aspect := float64(r.opts.Width) / float64(r.opts.Height)
	viewProj := s.Camera.Projection(aspect).Mul(s.Camera.View())

	for _, obj := range s.Objects {
		model := obj.Transform.Matrix()
		mvp := viewProj.Mul(model)
		normalMat := math3d.NormalMatrix(model)
		shader := shading.Lambert{
			Light:    s.Light,
			Ambient:  s.Ambient,
			Material: obj.Material,
		}
		mesh := obj.Mesh
		for i := 0; i < mesh.NumTriangles(); i++ {
			a, b, c := mesh.Triangle(i)
			r.drawTriangle(a, b, c, model, mvp, normalMat, shader)
		}
	}
	return r.fb
}

// drawTriangle runs the per-triangle stages: vertex transform, flat face normal,
// frustum clip, then rasterization of each resulting sub-triangle.
func (r *Renderer) drawTriangle(a, b, c geometry.Vertex, model, mvp math3d.Mat4, normalMat math3d.Mat3, shader shading.Shader) {
	v0 := processVertex(a, model, mvp, normalMat)
	v1 := processVertex(b, model, mvp, normalMat)
	v2 := processVertex(c, model, mvp, normalMat)

	// Flat shading: compute the face normal once on the original (pre-clip)
	// triangle in world space and assign it to all three vertices, so the clipper
	// carries a constant normal to every fragment. Recomputing it per clipped
	// sub-triangle would be unstable on the near-degenerate slivers clipping can
	// produce.
	n := faceNormal(v0.Frag.WorldPos, v1.Frag.WorldPos, v2.Frag.WorldPos)
	v0.Frag.Normal = n
	v1.Frag.Normal = n
	v2.Frag.Normal = n

	for _, tri := range clipTriangle(v0, v1, v2) {
		r.rasterize(tri, shader)
	}
}

// processVertex transforms a mesh vertex into clip space and gathers its
// world-space varyings.
func processVertex(v geometry.Vertex, model, mvp math3d.Mat4, normalMat math3d.Mat3) clipVertex {
	pos4 := v.Position.Vec4(1)
	return clipVertex{
		Pos: mvp.MulVec4(pos4),
		Frag: shading.Fragment{
			WorldPos: model.MulVec4(pos4).XYZ(),
			Normal:   normalMat.MulVec3(v.Normal),
			UV:       v.UV,
			Color:    v.Color,
		},
	}
}

// rasterize performs the perspective divide, viewport transform and backface
// cull for one clipped triangle, then hands it to the rasterizer.
func (r *Renderer) rasterize(tri [3]clipVertex, shader shading.Shader) {
	var rv [3]raster.Vertex
	for i, cv := range tri {
		invW := 1 / cv.Pos.W
		ndc := cv.Pos.XYZ().Scale(invW)
		screen := r.viewport.MulVec4(ndc.Vec4(1)).XYZ()
		rv[i] = raster.Vertex{Pos: screen, InvW: invW, Frag: cv.Frag}
	}
	area := raster.SignedArea(rv[0].Pos.XY(), rv[1].Pos.XY(), rv[2].Pos.XY())
	if r.culls(area) {
		return
	}
	raster.DrawTriangle(r.fb, rv[0], rv[1], rv[2], shader)
}

// culls reports whether a triangle with the given screen-space signed area is
// removed under the configured cull mode. With the Y-flipping viewport, a
// front-facing (CCW in NDC) triangle has negative screen-space area.
func (r *Renderer) culls(area float64) bool {
	switch r.opts.Cull {
	case CullBack:
		return area > 0
	case CullFront:
		return area < 0
	default:
		return false
	}
}

func faceNormal(a, b, c math3d.Vec3) math3d.Vec3 {
	return b.Sub(a).Cross(c.Sub(a)).Normalize()
}
